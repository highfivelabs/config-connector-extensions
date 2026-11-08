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

// SQLUser end to end on a fake cloud: a real kube-apiserver (envtest), Config
// Connector's unchanged direct reconciler, our SQLUser adapter, and
// Config Connector's mockgcp (with our database_roles patch) as Cloud SQL.
// No real Google Cloud, no recordings.
//
// Needs KUBEBUILDER_ASSETS (make test sets it).

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	refsv1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/apis/refs/v1beta1"
	"github.com/GoogleCloudPlatform/k8s-config-connector/mockgcp"
	"github.com/GoogleCloudPlatform/k8s-config-connector/mockgcp/pkg/storage"
	operatorv1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/operator/pkg/apis/core/v1beta1"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/config"
	"golang.org/x/oauth2"
	cloudresourcemanagerv1 "google.golang.org/api/cloudresourcemanager/v1"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/option"
	sqladmin "google.golang.org/api/sqladmin/v1beta4"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	sqlv1beta1 "github.com/highfivelabs/gaps-gcp/apis/sql/v1beta1"
	"github.com/highfivelabs/gaps-gcp/pkg/controller"
)

const (
	project   = "mock-project"
	instance  = "example"
	namespace = "example"
	userName  = "app-runtime@mock-project.iam"
)

type harness struct {
	t    *testing.T
	ctx  context.Context
	kube client.Client
	sql  *sqladmin.Service
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	if os.Getenv("KUBEBUILDER_ASSETS") == "" {
		t.Fatal("KUBEBUILDER_ASSETS is not set; run `make test`, which installs envtest's kube-apiserver and etcd")
	}

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	root := filepath.Join("..", "..", "..", "..")
	env := &envtest.Environment{
		CRDDirectoryPaths: []string{
			filepath.Join(root, "config", "crds"),
			// directbase looks up ConfigConnector / ConfigConnectorContext.
			filepath.Join(root, "third_party", "k8s-config-connector", "operator", "config", "crd", "bases"),
		},
		ErrorIfCRDPathMissing: true,
	}
	restConfig, err := env.Start()
	if err != nil {
		t.Fatalf("starting envtest: %v", err)
	}
	t.Cleanup(func() {
		if err := env.Stop(); err != nil {
			t.Errorf("stopping envtest: %v", err)
		}
	})

	scheme := runtime.NewScheme()
	for _, add := range []func(*runtime.Scheme) error{clientgoscheme.AddToScheme, operatorv1beta1.AddToScheme, sqlv1beta1.AddToScheme} {
		if err := add(scheme); err != nil {
			t.Fatal(err)
		}
	}
	kube, err := client.New(restConfig, client.Options{Scheme: scheme})
	if err != nil {
		t.Fatal(err)
	}

	// The fake cloud, as Config Connector's own test harness builds it
	// (config/tests/samples/create/harness.go).
	mockCloud := mockgcp.NewMockRoundTripperForTest(t, kube, storage.NewInMemoryStorage())
	gcpHTTP := &http.Client{Transport: mockCloud}
	ctx = context.WithValue(ctx, oauth2.HTTPClient, gcpHTTP)
	createMockProject(t, ctx, gcpHTTP, project)

	mgr, err := ctrl.NewManager(restConfig, ctrl.Options{
		Scheme:  scheme,
		Metrics: metricsserver.Options{BindAddress: "0"},
	})
	if err != nil {
		t.Fatal(err)
	}
	cfg := &config.ControllerConfig{HTTPClient: gcpHTTP}
	if err := controller.AddToManager(ctx, mgr, cfg); err != nil {
		t.Fatal(err)
	}
	// Stop the manager before envtest (cleanups run last-registered first),
	// or kube-apiserver waits on its open watches.
	mgrCtx, stopMgr := context.WithCancel(ctx)
	mgrDone := make(chan struct{})
	go func() {
		defer close(mgrDone)
		if err := mgr.Start(mgrCtx); err != nil {
			t.Errorf("manager: %v", err)
		}
	}()
	t.Cleanup(func() {
		stopMgr()
		<-mgrDone
	})

	sqlService, err := sqladmin.NewService(ctx, option.WithHTTPClient(gcpHTTP))
	if err != nil {
		t.Fatal(err)
	}

	if err := kube.Create(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespace}}); err != nil {
		t.Fatal(err)
	}

	return &harness{t: t, ctx: ctx, kube: kube, sql: sqlService}
}

func createMockProject(t *testing.T, ctx context.Context, httpClient *http.Client, projectID string) {
	t.Helper()
	crm, err := cloudresourcemanagerv1.NewService(ctx, option.WithHTTPClient(httpClient))
	if err != nil {
		t.Fatal(err)
	}
	op, err := crm.Projects.Create(&cloudresourcemanagerv1.Project{ProjectId: projectID}).Context(ctx).Do()
	if err != nil {
		t.Fatalf("creating mock project %q: %v", projectID, err)
	}
	for i := 0; i < 100 && !op.Done; i++ {
		time.Sleep(100 * time.Millisecond)
		if op, err = crm.Operations.Get(op.Name).Context(ctx).Do(); err != nil {
			t.Fatal(err)
		}
	}
	if !op.Done {
		t.Fatalf("mock project %q was not created", projectID)
	}
}

// waitReady waits until the object's Ready condition is True for its
// current generation, and returns the object.
func (h *harness) waitReady(name string) *unstructured.Unstructured {
	h.t.Helper()
	var last *unstructured.Unstructured
	deadline := time.Now().Add(60 * time.Second)
	for time.Now().Before(deadline) {
		u := &unstructured.Unstructured{}
		u.SetGroupVersionKind(sqlv1beta1.SQLUserGVK)
		if err := h.kube.Get(h.ctx, types.NamespacedName{Namespace: namespace, Name: name}, u); err != nil {
			h.t.Fatal(err)
		}
		last = u
		observed, _, _ := unstructured.NestedInt64(u.Object, "status", "observedGeneration")
		conditions, _, _ := unstructured.NestedSlice(u.Object, "status", "conditions")
		for _, c := range conditions {
			cm, _ := c.(map[string]any)
			if cm["type"] == "Ready" && cm["status"] == "True" && observed == u.GetGeneration() {
				return u
			}
		}
		time.Sleep(200 * time.Millisecond)
	}
	h.t.Fatalf("SQLUser %s did not become Ready; last status: %v", name, last.Object["status"])
	return nil
}

func (h *harness) cloudUser() (*sqladmin.User, error) {
	return h.sql.Users.Get(project, instance, userName).Context(h.ctx).Do()
}

func (h *harness) cloudRoles() []string {
	h.t.Helper()
	u, err := h.cloudUser()
	if err != nil {
		h.t.Fatalf("reading the user from the fake cloud: %v", err)
	}
	roles := slices.Clone(u.DatabaseRoles)
	slices.Sort(roles)
	return roles
}

func (h *harness) setRoles(name string, roles []string) {
	h.t.Helper()
	obj := &sqlv1beta1.SQLUser{}
	if err := h.kube.Get(h.ctx, types.NamespacedName{Namespace: namespace, Name: name}, obj); err != nil {
		h.t.Fatal(err)
	}
	obj.Spec.DatabaseRoles = roles
	if err := h.kube.Update(h.ctx, obj); err != nil {
		h.t.Fatal(err)
	}
}

// The case: an application's Google service account as an IAM database
// user, granted its roles declaratively.
func TestSQLUserDatabaseRoles(t *testing.T) {
	h := newHarness(t)
	const name = "app-runtime"

	user := &sqlv1beta1.SQLUser{
		ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: name},
		Spec: sqlv1beta1.SQLUserSpec{
			ResourceID:    ptr(userName),
			Type:          ptr("CLOUD_IAM_SERVICE_ACCOUNT"),
			InstanceRef:   refsv1beta1.SQLInstanceRef{External: fmt.Sprintf("projects/%s/instances/%s", project, instance)},
			DatabaseRoles: []string{"cloudsqlsuperuser"},
		},
	}
	if err := h.kube.Create(h.ctx, user); err != nil {
		t.Fatal(err)
	}

	t.Run("create grants the roles", func(t *testing.T) {
		u := h.waitReady(name)
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
		h.waitReady(name)
		if got := h.cloudRoles(); !slices.Equal(got, []string{"cloudsqlsuperuser", "pg_read_all_data"}) {
			t.Errorf("cloud roles = %v, want [cloudsqlsuperuser pg_read_all_data]", got)
		}
	})

	t.Run("removing a role revokes it", func(t *testing.T) {
		h.setRoles(name, []string{"pg_read_all_data"})
		h.waitReady(name)
		if got := h.cloudRoles(); !slices.Equal(got, []string{"pg_read_all_data"}) {
			t.Errorf("cloud roles = %v, want [pg_read_all_data]", got)
		}
	})

	t.Run("delete removes the user", func(t *testing.T) {
		if err := h.kube.Delete(h.ctx, user); err != nil {
			t.Fatal(err)
		}
		deadline := time.Now().Add(60 * time.Second)
		for {
			err := h.kube.Get(h.ctx, types.NamespacedName{Namespace: namespace, Name: name}, &sqlv1beta1.SQLUser{})
			if apierrors.IsNotFound(err) {
				break
			}
			if time.Now().After(deadline) {
				t.Fatalf("SQLUser was not deleted (finalizer not removed): %v", err)
			}
			time.Sleep(200 * time.Millisecond)
		}
		_, err := h.cloudUser()
		var gerr *googleapi.Error
		if err == nil {
			t.Errorf("user still exists in the fake cloud after delete")
		} else if !errors.As(err, &gerr) || gerr.Code != http.StatusNotFound {
			t.Errorf("reading deleted user: got %v, want 404", err)
		}
	})
}

func ptr[T any](v T) *T { return &v }
