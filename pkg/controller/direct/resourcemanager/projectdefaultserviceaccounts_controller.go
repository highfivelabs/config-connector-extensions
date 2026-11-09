// Copyright 2026 High Five Labs
// SPDX-License-Identifier: Apache-2.0
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

// Package resourcemanager has the direct controllers for this project's
// resourcemanager.gcp.hfl.dev kinds.
//
// Written here: what a project does to its default service accounts. The
// behaviour is Terraform's google_project_default_service_accounts (read,
// not copied): list the project's service accounts and pick the defaults
// by display name; DELETE deletes, DISABLE disables, DEPRIVILEGE removes
// serviceAccount:{email} from every binding of the project's IAM policy;
// record unique ID to email; on delete, unless the restore policy is NONE,
// undelete (by unique ID) or enable them, logging instead of failing under
// REVERT_AND_IGNORE_FAILURE; DEPRIVILEGE is never reverted. Two
// differences: Terraform acts once, at create, where this acts on every
// reconcile (a default account enabled again by hand, or one that appears
// when App Engine is turned on later, is acted on); and the policy is read
// and written as version 3 with an etag retry (pkg/iampolicy), so
// conditional bindings survive and a concurrent change is not lost.
package resourcemanager

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"strings"

	"cloud.google.com/go/iam/apiv1/iampb"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/config"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct/common"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct/directbase"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct/registry"
	api "google.golang.org/api/iam/v1"
	"google.golang.org/api/option"
	htransport "google.golang.org/api/transport/http"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"

	krm "github.com/highfivelabs/config-connector-extensions/apis/resourcemanager/v1alpha1"
	"github.com/highfivelabs/config-connector-extensions/pkg/iampolicy"
)

const (
	actionDelete      = "DELETE"
	actionDisable     = "DISABLE"
	actionDeprivilege = "DEPRIVILEGE"

	restoreNone         = "NONE"
	restoreRevert       = "REVERT"
	restoreRevertIgnore = "REVERT_AND_IGNORE_FAILURE"
)

// IsDefaultServiceAccount says whether a display name is a default service
// account's, as Terraform decides it (case-insensitive): Google may change
// the accounts' emails, not these names.
func IsDefaultServiceAccount(displayName string) bool {
	switch strings.ToLower(displayName) {
	case "compute engine default service account", "app engine default service account":
		return true
	}
	return false
}

func init() {
	registry.RegisterModel(krm.ProjectDefaultServiceAccountsGVK, NewProjectDefaultServiceAccountsModel)
}

func NewProjectDefaultServiceAccountsModel(ctx context.Context, config *config.ControllerConfig) (directbase.Model, error) {
	return &defaultServiceAccountsModel{config: *config}, nil
}

type defaultServiceAccountsModel struct {
	config config.ControllerConfig
}

var _ directbase.Model = &defaultServiceAccountsModel{}

func (m *defaultServiceAccountsModel) AdapterForObject(ctx context.Context, op *directbase.AdapterForObjectOperation) (directbase.Adapter, error) {
	obj := &krm.ProjectDefaultServiceAccounts{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(op.GetUnstructured().Object, obj); err != nil {
		return nil, fmt.Errorf("error converting to %T: %w", obj, err)
	}
	if err := common.NormalizeReferences(ctx, op.Reader, obj, nil); err != nil {
		return nil, fmt.Errorf("normalizing references: %w", err)
	}
	id, err := obj.GetIdentity(ctx, op.Reader)
	if err != nil {
		return nil, err
	}
	opts, err := m.config.RESTClientOptions()
	if err != nil {
		return nil, err
	}
	svc, err := api.NewService(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("building IAM client: %w", err)
	}
	var h *http.Client
	if h, _, err = htransport.NewClient(ctx, append(opts, option.WithScopes("https://www.googleapis.com/auth/cloud-platform"))...); err != nil {
		return nil, fmt.Errorf("building the HTTP client: %w", err)
	}
	projectType, _ := iampolicy.Lookup("Project")
	return &defaultServiceAccountsAdapter{
		id:          id.(*krm.ProjectDefaultServiceAccountsIdentity),
		iam:         svc,
		policy:      &iampolicy.Client{HTTP: h},
		projectType: projectType,
		desired:     obj,
	}, nil
}

func (m *defaultServiceAccountsModel) AdapterForURL(ctx context.Context, url string) (directbase.Adapter, error) {
	return nil, nil
}

type defaultServiceAccountsAdapter struct {
	id          *krm.ProjectDefaultServiceAccountsIdentity
	iam         *api.Service
	policy      *iampolicy.Client
	projectType *iampolicy.ResourceType
	desired     *krm.ProjectDefaultServiceAccounts
	// defaults are the project's default service accounts that Find listed.
	defaults []*api.ServiceAccount
}

var _ directbase.Adapter = &defaultServiceAccountsAdapter{}

func (a *defaultServiceAccountsAdapter) restorePolicy() string {
	if p := direct.ValueOf(a.desired.Spec.RestorePolicy); p != "" {
		return p
	}
	return restoreRevert
}

// Find lists the project's default service accounts. The setting always
// exists, so Update does the work.
func (a *defaultServiceAccountsAdapter) Find(ctx context.Context) (bool, error) {
	a.defaults = nil
	call := a.iam.Projects.ServiceAccounts.List("projects/" + a.id.Project).PageSize(100)
	err := call.Pages(ctx, func(resp *api.ListServiceAccountsResponse) error {
		for _, sa := range resp.Accounts {
			if IsDefaultServiceAccount(sa.DisplayName) {
				a.defaults = append(a.defaults, sa)
			}
		}
		return nil
	})
	if err != nil {
		return false, fmt.Errorf("listing the service accounts of project %s: %w", a.id.Project, err)
	}
	return true, nil
}

// Create runs only if Find reports nothing, which it never does.
func (a *defaultServiceAccountsAdapter) Create(ctx context.Context, createOp *directbase.CreateOperation) error {
	if err := a.act(ctx); err != nil {
		return err
	}
	return a.updateStatus(ctx, createOp)
}

func (a *defaultServiceAccountsAdapter) Update(ctx context.Context, updateOp *directbase.UpdateOperation) error {
	if err := a.act(ctx); err != nil {
		return err
	}
	return a.updateStatus(ctx, updateOp)
}

// act does the action to every default account not already in its state.
func (a *defaultServiceAccountsAdapter) act(ctx context.Context) error {
	log := klog.FromContext(ctx)
	switch a.desired.Spec.Action {
	case actionDelete:
		for _, sa := range a.defaults {
			log.V(2).Info("deleting a default service account", "name", a.id, "email", sa.Email)
			if _, err := a.iam.Projects.ServiceAccounts.Delete(a.accountName(sa.UniqueId)).Context(ctx).Do(); err != nil && !direct.IsNotFound(err) {
				return fmt.Errorf("deleting service account %s: %w", sa.Email, err)
			}
		}
	case actionDisable:
		for _, sa := range a.defaults {
			if sa.Disabled {
				continue
			}
			log.V(2).Info("disabling a default service account", "name", a.id, "email", sa.Email)
			if _, err := a.iam.Projects.ServiceAccounts.Disable(a.accountName(sa.UniqueId), &api.DisableServiceAccountRequest{}).Context(ctx).Do(); err != nil {
				return fmt.Errorf("disabling service account %s: %w", sa.Email, err)
			}
		}
	case actionDeprivilege:
		if len(a.defaults) == 0 {
			return nil
		}
		members := map[string]bool{}
		for _, sa := range a.defaults {
			members["serviceaccount:"+strings.ToLower(sa.Email)] = true
		}
		changed, err := a.policy.Modify(ctx, a.projectType, "projects/"+a.id.Project, func(p *iampb.Policy) bool {
			return removeMembers(p, members)
		})
		if err != nil {
			return fmt.Errorf("removing the default service accounts from project %s's IAM policy: %w", a.id.Project, err)
		}
		if changed {
			log.V(2).Info("removed the default service accounts from the project's IAM policy", "name", a.id)
		}
	default:
		return fmt.Errorf("spec.action %q is not DELETE, DISABLE or DEPRIVILEGE", a.desired.Spec.Action)
	}
	return nil
}

// removeMembers takes the members (lower case) out of every binding, and
// drops a binding left with none. It reports whether it changed anything.
func removeMembers(p *iampb.Policy, members map[string]bool) bool {
	changed := false
	kept := p.Bindings[:0]
	for _, b := range p.Bindings {
		ms := b.Members[:0]
		for _, m := range b.Members {
			if members[strings.ToLower(m)] {
				changed = true
				continue
			}
			ms = append(ms, m)
		}
		b.Members = ms
		if len(ms) > 0 {
			kept = append(kept, b)
		}
	}
	p.Bindings = kept
	return changed
}

// accountName names an account by its unique ID, which outlives a delete
// and is the only name undelete takes.
func (a *defaultServiceAccountsAdapter) accountName(uniqueID string) string {
	return "projects/" + a.id.Project + "/serviceAccounts/" + uniqueID
}

// Delete reverts DELETE or DISABLE on the accounts in status, unless the
// restore policy is NONE.
func (a *defaultServiceAccountsAdapter) Delete(ctx context.Context, deleteOp *directbase.DeleteOperation) (bool, error) {
	policy := a.restorePolicy()
	if policy == restoreNone || a.desired.Spec.Action == actionDeprivilege {
		return true, nil
	}
	log := klog.FromContext(ctx)
	ids := make([]string, 0, len(a.desired.Status.ServiceAccounts))
	for id := range a.desired.Status.ServiceAccounts {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		email := a.desired.Status.ServiceAccounts[id]
		var err error
		switch a.desired.Spec.Action {
		case actionDelete:
			err = a.undelete(ctx, id)
		case actionDisable:
			_, err = a.iam.Projects.ServiceAccounts.Enable(a.accountName(id), &api.EnableServiceAccountRequest{}).Context(ctx).Do()
		}
		if err == nil {
			log.V(2).Info("reverted a default service account", "name", a.id, "email", email, "action", a.desired.Spec.Action)
			continue
		}
		err = fmt.Errorf("reverting %s on service account %s (%s): %w", a.desired.Spec.Action, email, id, err)
		if policy != restoreRevertIgnore {
			return false, err
		}
		log.Info("restore policy is REVERT_AND_IGNORE_FAILURE; going on", "name", a.id, "error", err.Error())
	}
	return true, nil
}

// undelete restores a deleted account; one that exists is left alone.
func (a *defaultServiceAccountsAdapter) undelete(ctx context.Context, uniqueID string) error {
	if _, err := a.iam.Projects.ServiceAccounts.Get(a.accountName(uniqueID)).Context(ctx).Do(); err == nil {
		return nil
	} else if !direct.IsNotFound(err) {
		return err
	}
	_, err := a.iam.Projects.ServiceAccounts.Undelete(a.accountName(uniqueID), &api.UndeleteServiceAccountRequest{}).Context(ctx).Do()
	return err
}

func (a *defaultServiceAccountsAdapter) Export(ctx context.Context) (*unstructured.Unstructured, error) {
	return nil, nil
}

// updateStatus adds the accounts acted on to the ones already recorded.
func (a *defaultServiceAccountsAdapter) updateStatus(ctx context.Context, op directbase.Operation) error {
	accounts := map[string]string{}
	for id, email := range a.desired.Status.ServiceAccounts {
		accounts[id] = email
	}
	for _, sa := range a.defaults {
		accounts[sa.UniqueId] = sa.Email
	}
	status := &krm.ProjectDefaultServiceAccountsStatus{ExternalRef: direct.LazyPtr(a.id.String())}
	if len(accounts) > 0 {
		status.ServiceAccounts = accounts
	}
	return op.UpdateStatus(ctx, status, nil)
}
