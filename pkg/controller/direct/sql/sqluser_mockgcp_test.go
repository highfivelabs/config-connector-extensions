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

package sql_test

// SQLUser end to end on the fake cloud (pkg/test/mockenv): a real
// kube-apiserver (envtest), Config Connector's unchanged direct reconciler,
// our SQLUser adapter, and Config Connector's mockgcp (with our
// database_roles patch) as Cloud SQL; or a real project's Cloud SQL with
// E2E_GCP_TARGET=real (make test-live).
//
// Needs KUBEBUILDER_ASSETS (make test sets it).

import (
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"testing"

	refsv1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/apis/refs/v1beta1"
	"google.golang.org/api/googleapi"
	sqladmin "google.golang.org/api/sqladmin/v1beta4"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"

	sqlv1beta1 "github.com/highfivelabs/config-connector-extensions/apis/sql/v1beta1"
	"github.com/highfivelabs/config-connector-extensions/pkg/test/mockenv"
)

const namespace = "example"

type harness struct {
	t        *testing.T
	env      *mockenv.Env
	sql      *sqladmin.Service
	instance string
	userName string
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	env := mockenv.New(t, namespace)
	sqlService, err := sqladmin.NewService(env.Ctx, env.GCPOptions()...)
	if err != nil {
		t.Fatal(err)
	}
	// An IAM database user is named by its service account's email without
	// ".gserviceaccount.com".
	userName := strings.TrimSuffix(mockenv.ServiceAccount(t), ".gserviceaccount.com")
	return &harness{t: t, env: env, sql: sqlService, instance: env.SQLInstance(), userName: userName}
}

func (h *harness) cloudUser() (*sqladmin.User, error) {
	return h.sql.Users.Get(mockenv.Project, h.instance, h.userName).Context(h.env.Ctx).Do()
}

func (h *harness) cloudRoles() []string {
	h.t.Helper()
	u, err := h.cloudUser()
	if err != nil {
		h.t.Fatalf("reading the user from the cloud: %v", err)
	}
	roles := slices.Clone(u.DatabaseRoles)
	slices.Sort(roles)
	return roles
}

func (h *harness) setRoles(name string, roles []string) {
	h.t.Helper()
	obj := &sqlv1beta1.SQLUser{}
	if err := h.env.Kube.Get(h.env.Ctx, types.NamespacedName{Namespace: namespace, Name: name}, obj); err != nil {
		h.t.Fatal(err)
	}
	obj.Spec.DatabaseRoles = roles
	if err := h.env.Kube.Update(h.env.Ctx, obj); err != nil {
		h.t.Fatal(err)
	}
}

// The case: an application's Google service account as an IAM database
// user, granted its roles declaratively.
func TestSQLUserDatabaseRoles(t *testing.T) {
	h := newHarness(t)
	const name = "app-runtime"
	gvk := sqlv1beta1.SQLUserGVK

	user := &sqlv1beta1.SQLUser{
		ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: name, Labels: mockenv.Labels()},
		Spec: sqlv1beta1.SQLUserSpec{
			ResourceID:    mockenv.Ptr(h.userName),
			Type:          mockenv.Ptr("CLOUD_IAM_SERVICE_ACCOUNT"),
			InstanceRef:   refsv1beta1.SQLInstanceRef{External: fmt.Sprintf("projects/%s/instances/%s", mockenv.Project, h.instance)},
			DatabaseRoles: []string{"cloudsqlsuperuser"},
		},
	}
	if err := h.env.Kube.Create(h.env.Ctx, user); err != nil {
		t.Fatal(err)
	}

	t.Run("create grants the roles", func(t *testing.T) {
		u := h.env.WaitReady(gvk, namespace, name)
		if got := h.cloudRoles(); !slices.Equal(got, []string{"cloudsqlsuperuser"}) {
			t.Errorf("cloud roles after create = %v, want [cloudsqlsuperuser]", got)
		}
		cu, _ := h.cloudUser()
		if cu.Type != "CLOUD_IAM_SERVICE_ACCOUNT" {
			t.Errorf("cloud user type = %q, want CLOUD_IAM_SERVICE_ACCOUNT", cu.Type)
		}
		applied, _, _ := unstructured.NestedStringSlice(u.Object, "status", "appliedDatabaseRoles")
		if !slices.Equal(applied, []string{"cloudsqlsuperuser"}) {
			t.Errorf("status.appliedDatabaseRoles = %v, want [cloudsqlsuperuser]", applied)
		}
	})

	t.Run("adding a role grants it", func(t *testing.T) {
		h.setRoles(name, []string{"cloudsqlsuperuser", "pg_read_all_data"})
		h.env.WaitReady(gvk, namespace, name)
		if got := h.cloudRoles(); !slices.Equal(got, []string{"cloudsqlsuperuser", "pg_read_all_data"}) {
			t.Errorf("cloud roles = %v, want [cloudsqlsuperuser pg_read_all_data]", got)
		}
	})

	t.Run("removing a role revokes it", func(t *testing.T) {
		h.setRoles(name, []string{"pg_read_all_data"})
		h.env.WaitReady(gvk, namespace, name)
		if got := h.cloudRoles(); !slices.Equal(got, []string{"pg_read_all_data"}) {
			t.Errorf("cloud roles = %v, want [pg_read_all_data]", got)
		}
	})

	t.Run("delete removes the user", func(t *testing.T) {
		if err := h.env.Kube.Delete(h.env.Ctx, user); err != nil {
			t.Fatal(err)
		}
		h.env.WaitGone(&sqlv1beta1.SQLUser{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: name}})
		_, err := h.cloudUser()
		var gerr *googleapi.Error
		if err == nil {
			t.Errorf("user still exists in the cloud after delete")
		} else if !errors.As(err, &gerr) || gerr.Code != http.StatusNotFound {
			t.Errorf("reading deleted user: got %v, want 404", err)
		}
	})
}
