// Copyright 2026 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Modified by High Five Labs for gaps-gcp. Config Connector has no direct
// controller for SQLUser (on master it is Terraform-based, reconciled by the
// vendored google_sql_user). This file is Config Connector's direct
// SQLSSLCert controller, pkg/controller/direct/sql/sqlsslcert_controller.go
// (UPSTREAM.md), the sibling sub-resource of a Cloud SQL instance on the same
// sqladmin v1beta4 client, changed for users. Behaviour follows Terraform's
// google_sql_user (hashicorp/terraform-provider-google-beta,
// google-beta/services/sql/resource_sql_user.go), including database_roles:
// sent on insert, and on change sent to users.update with
// revokeExistingRoles=true. Terraform is MPL-2.0 and none of its code is
// copied here; only the API calls it makes are followed.

package sql

import (
	"context"
	"fmt"
	"reflect"
	"slices"
	"strings"
	"time"

	"github.com/googleapis/gax-go/v2"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/config"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct/directbase"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct/registry"
	api "google.golang.org/api/sqladmin/v1beta4"

	krm "github.com/highfivelabs/gaps-gcp/apis/sql/v1beta1"
)

func init() {
	registry.RegisterModel(krm.SQLUserGVK, NewSQLUserModel)
}

// NewSQLUserModel is the model factory registered for sql.gcp.hfl.dev/SQLUser.
func NewSQLUserModel(ctx context.Context, config *config.ControllerConfig) (directbase.Model, error) {
	return &sqlUserModel{config: config}, nil
}

type sqlUserModel struct {
	config *config.ControllerConfig
}

var _ directbase.Model = &sqlUserModel{}

func (m *sqlUserModel) AdapterForObject(ctx context.Context, op *directbase.AdapterForObjectOperation) (directbase.Adapter, error) {
	u := op.GetUnstructured()
	kube := op.Reader
	obj := &krm.SQLUser{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, &obj); err != nil {
		return nil, fmt.Errorf("converting to %T failed: %w", obj, err)
	}

	id, err := obj.GetIdentity(ctx, kube)
	if err != nil {
		return nil, err
	}

	typedID, ok := id.(*krm.SQLUserIdentity)
	if !ok {
		return nil, fmt.Errorf("expected SQLUserIdentity, got %T", id)
	}

	gcpClient, err := newGCPClient(ctx, m.config)
	if err != nil {
		return nil, err
	}

	return &sqlUserAdapter{
		id:                  typedID,
		gcpClient:           gcpClient,
		sqlOperationsClient: gcpClient.sqlOperationsClient(),
		sqlUsersClient:      gcpClient.sqlUsersClient(),
		reader:              kube,
		desired:             obj,
		generation:          u.GetGeneration(),
		initialStatus:       &obj.Status,
	}, nil
}

func (m *sqlUserModel) AdapterForURL(ctx context.Context, url string) (directbase.Adapter, error) {
	id := &krm.SQLUserIdentity{}
	if err := id.FromExternal(url); err != nil {
		return nil, nil
	}

	gcpClient, err := newGCPClient(ctx, m.config)
	if err != nil {
		return nil, err
	}

	return &sqlUserAdapter{
		id:                  id,
		gcpClient:           gcpClient,
		sqlOperationsClient: gcpClient.sqlOperationsClient(),
		sqlUsersClient:      gcpClient.sqlUsersClient(),
	}, nil
}

type sqlUserAdapter struct {
	id                  *krm.SQLUserIdentity
	gcpClient           *gcpClient
	sqlOperationsClient *api.OperationsService
	sqlUsersClient      *api.UsersService
	reader              client.Reader
	desired             *krm.SQLUser
	generation          int64
	actual              *api.User
	initialStatus       *krm.SQLUserStatus
}

var _ directbase.Adapter = &sqlUserAdapter{}

// Find retrieves the existing user from GCP.
func (a *sqlUserAdapter) Find(ctx context.Context) (bool, error) {
	call := a.sqlUsersClient.Get(a.id.Project, a.id.Instance, a.id.Name)
	if a.id.UserHost != "" {
		call = call.Host(a.id.UserHost)
	}
	user, err := call.Context(ctx).Do()
	if err != nil {
		if direct.IsNotFound(err) {
			return false, nil
		}
		return false, fmt.Errorf("getting SQLUser %q: %w", a.id.String(), err)
	}
	a.actual = user
	return true, nil
}

// Create creates the user in GCP, with its database roles.
func (a *sqlUserAdapter) Create(ctx context.Context, createOp *directbase.CreateOperation) error {
	log := klog.FromContext(ctx)
	log.V(2).Info("creating SQLUser", "name", a.id.String())

	user, err := a.desiredUser(ctx, true)
	if err != nil {
		return err
	}
	// As Terraform does: the roles go in the insert body.
	user.DatabaseRoles = desiredRoles(a.desired)

	op, err := a.sqlUsersClient.Insert(a.id.Project, a.id.Instance, user).Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("creating SQLUser %s: %w", a.id.String(), err)
	}
	if err := a.pollForLROCompletion(ctx, op, "creating"); err != nil {
		return err
	}

	if _, err := a.Find(ctx); err != nil {
		return fmt.Errorf("fetching newly created SQLUser %q: %w", a.id.String(), err)
	}
	if a.actual == nil {
		return fmt.Errorf("SQLUser %q not found after creation", a.id.String())
	}

	return a.updateStatus(ctx, createOp, desiredRoles(a.desired))
}

// Update brings an existing user to the spec. Type and host are immutable.
// Password and password policy are sent when they may have changed; the
// database roles are sent, with revokeExistingRoles, when spec.databaseRoles
// differs from status.appliedDatabaseRoles.
func (a *sqlUserAdapter) Update(ctx context.Context, updateOp *directbase.UpdateOperation) error {
	log := klog.FromContext(ctx)

	if a.actual == nil {
		return fmt.Errorf("actual state is missing in Update")
	}

	if want := direct.ValueOf(a.desired.Spec.Type); want != "" && a.actual.Type != "" && want != a.actual.Type {
		return fmt.Errorf("SQLUser %s: type is immutable (actual %q, desired %q)", a.id.String(), a.actual.Type, want)
	}

	applied := []string{}
	if a.initialStatus != nil {
		applied = sortedRoles(a.initialStatus.AppliedDatabaseRoles)
	}
	wantRoles := desiredRoles(a.desired)
	rolesChanged := !slices.Equal(applied, wantRoles)

	// The password cannot be read back. As Terraform sends it when the
	// configuration changes, we send it when the spec generation is one the
	// controller has not reconciled yet.
	observed := int64(-1)
	if a.initialStatus != nil && a.initialStatus.ObservedGeneration != nil {
		observed = *a.initialStatus.ObservedGeneration
	}
	sendPassword := a.desired.Spec.Password != nil && a.generation != observed

	policyChanged := !passwordPolicyEqual(desiredPasswordPolicy(a.desired), a.actual.PasswordPolicy)

	if !rolesChanged && !sendPassword && !policyChanged {
		return a.updateStatus(ctx, updateOp, applied)
	}

	log.V(2).Info("updating SQLUser", "name", a.id.String(), "roles", rolesChanged, "password", sendPassword, "passwordPolicy", policyChanged)
	updateOp.RecordUpdatingEvent()

	user, err := a.desiredUser(ctx, sendPassword)
	if err != nil {
		return err
	}

	call := a.sqlUsersClient.Update(a.id.Project, a.id.Instance, user).Name(a.id.Name)
	if a.id.UserHost != "" {
		call = call.Host(a.id.UserHost)
	}
	if rolesChanged {
		call = call.DatabaseRoles(wantRoles...).RevokeExistingRoles(true)
	}
	op, err := call.Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("updating SQLUser %s: %w", a.id.String(), err)
	}
	if err := a.pollForLROCompletion(ctx, op, "updating"); err != nil {
		return err
	}

	if _, err := a.Find(ctx); err != nil {
		return fmt.Errorf("fetching updated SQLUser %q: %w", a.id.String(), err)
	}
	if a.actual == nil {
		return fmt.Errorf("SQLUser %q not found after update", a.id.String())
	}
	return a.updateStatus(ctx, updateOp, wantRoles)
}

// Delete deletes the user in GCP.
func (a *sqlUserAdapter) Delete(ctx context.Context, deleteOp *directbase.DeleteOperation) (bool, error) {
	log := klog.FromContext(ctx)
	log.V(2).Info("deleting SQLUser", "name", a.id.String())

	call := a.sqlUsersClient.Delete(a.id.Project, a.id.Instance).Name(a.id.Name)
	if a.id.UserHost != "" {
		call = call.Host(a.id.UserHost)
	}
	op, err := call.Context(ctx).Do()
	if err != nil {
		if direct.IsNotFound(err) {
			return false, nil
		}
		return false, fmt.Errorf("deleting SQLUser %q: %w", a.id.String(), err)
	}
	if err := a.pollForLROCompletion(ctx, op, "deleting"); err != nil {
		return false, err
	}
	return true, nil
}

// Export exports the user as a KRM object. The password and the database
// roles are not exported: the API does not return the password and is not
// relied on to return roles.
func (a *sqlUserAdapter) Export(ctx context.Context) (*unstructured.Unstructured, error) {
	if a.actual == nil {
		return nil, fmt.Errorf("Find() not called")
	}

	obj := &krm.SQLUser{}
	obj.Spec.InstanceRef.External = a.id.ParentString()
	obj.Spec.ResourceID = direct.LazyPtr(a.actual.Name)
	obj.Spec.Host = direct.LazyPtr(a.actual.Host)
	obj.Spec.Type = direct.LazyPtr(a.actual.Type)
	if p := a.actual.PasswordPolicy; p != nil {
		obj.Spec.PasswordPolicy = &krm.UserPasswordPolicy{
			AllowedFailedAttempts:      direct.LazyPtr(p.AllowedFailedAttempts),
			EnableFailedAttemptsCheck:  direct.LazyPtr(p.EnableFailedAttemptsCheck),
			EnablePasswordVerification: direct.LazyPtr(p.EnablePasswordVerification),
			PasswordExpirationDuration: direct.LazyPtr(p.PasswordExpirationDuration),
		}
	}

	uObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
	if err != nil {
		return nil, err
	}
	u := &unstructured.Unstructured{Object: uObj}
	u.SetName(a.actual.Name)
	u.SetGroupVersionKind(krm.SQLUserGVK)
	return u, nil
}

// desiredUser builds the API user from the spec. The password is read from
// its Secret only when it is to be sent.
func (a *sqlUserAdapter) desiredUser(ctx context.Context, withPassword bool) (*api.User, error) {
	user := &api.User{
		Name:           a.id.Name,
		Host:           a.id.UserHost,
		Instance:       a.id.Instance,
		Project:        a.id.Project,
		Type:           direct.ValueOf(a.desired.Spec.Type),
		PasswordPolicy: desiredPasswordPolicy(a.desired),
	}
	if withPassword && a.desired.Spec.Password != nil {
		value, err := a.desired.Spec.Password.ReadSecretValue(ctx, "spec.password", a.desired.GetNamespace(), a.reader)
		if err != nil {
			return nil, err
		}
		user.Password = direct.ValueOf(value)
	}
	return user, nil
}

func desiredPasswordPolicy(obj *krm.SQLUser) *api.UserPasswordValidationPolicy {
	p := obj.Spec.PasswordPolicy
	if p == nil {
		return nil
	}
	return &api.UserPasswordValidationPolicy{
		AllowedFailedAttempts:      direct.ValueOf(p.AllowedFailedAttempts),
		EnableFailedAttemptsCheck:  direct.ValueOf(p.EnableFailedAttemptsCheck),
		EnablePasswordVerification: direct.ValueOf(p.EnablePasswordVerification),
		PasswordExpirationDuration: direct.ValueOf(p.PasswordExpirationDuration),
	}
}

// passwordPolicyEqual compares the settable fields; status is output only.
// An unset desired policy matches whatever the server holds.
func passwordPolicyEqual(desired, actual *api.UserPasswordValidationPolicy) bool {
	if desired == nil {
		return true
	}
	if actual == nil {
		actual = &api.UserPasswordValidationPolicy{}
	}
	return reflect.DeepEqual(
		[]any{desired.AllowedFailedAttempts, desired.EnableFailedAttemptsCheck, desired.EnablePasswordVerification, desired.PasswordExpirationDuration},
		[]any{actual.AllowedFailedAttempts, actual.EnableFailedAttemptsCheck, actual.EnablePasswordVerification, actual.PasswordExpirationDuration},
	)
}

func desiredRoles(obj *krm.SQLUser) []string {
	return sortedRoles(obj.Spec.DatabaseRoles)
}

// sortedRoles returns the roles sorted and de-duplicated, never nil.
func sortedRoles(roles []string) []string {
	out := slices.Clone(roles)
	slices.Sort(out)
	out = slices.Compact(out)
	if out == nil {
		out = []string{}
	}
	return out
}

func (a *sqlUserAdapter) pollForLROCompletion(ctx context.Context, op *api.Operation, verb string) error {
	log := klog.FromContext(ctx)
	var err error

	pollingBackoff := gax.Backoff{
		Initial:    time.Second,
		Max:        time.Minute,
		Multiplier: 2,
	}
	for {
		log.V(2).Info("polling", "op", op)

		if op.Status == "DONE" {
			if op.Error != nil && len(op.Error.Errors) > 0 {
				var errMsgs []string
				for _, opErr := range op.Error.Errors {
					errMsgs = append(errMsgs, opErr.Message)
				}
				return fmt.Errorf("operation failed: %s", strings.Join(errMsgs, "; "))
			}
			break
		}
		if err := gax.Sleep(ctx, pollingBackoff.Pause()); err != nil {
			return fmt.Errorf("waiting for SQLUser %s %s failed: %w", a.id.String(), verb, err)
		}
		name := op.Name
		op, err = a.sqlOperationsClient.Get(a.id.Project, name).Context(ctx).Do()
		if err != nil {
			return fmt.Errorf("getting SQLUser %s %s operation %s failed: %w", a.id.String(), verb, name, err)
		}
	}

	return nil
}

func (a *sqlUserAdapter) updateStatus(ctx context.Context, op directbase.Operation, appliedRoles []string) error {
	status := &krm.SQLUserStatus{}
	if len(appliedRoles) > 0 {
		status.AppliedDatabaseRoles = appliedRoles
	}
	if d := a.actual.SqlserverUserDetails; d != nil {
		status.SqlServerUserDetails = []krm.UserSqlServerUserDetailsStatus{{
			Disabled:    direct.LazyPtr(d.Disabled),
			ServerRoles: d.ServerRoles,
		}}
	}
	return op.UpdateStatus(ctx, status, nil)
}
