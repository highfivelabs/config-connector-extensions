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

package sqladmin_test

// SQLAdminBackup end to end on the fake cloud (pkg/test/mockenv), or a real
// project with E2E_GCP_TARGET=real (make test-live). The case:
// an on-demand backup of an application's database before a change that
// could lose data (a major version upgrade, a destructive migration).

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	refsv1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/apis/refs/v1beta1"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/k8s"
	"google.golang.org/api/googleapi"
	sqladmin "google.golang.org/api/sqladmin/v1beta4"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"

	sqladminv1alpha1 "github.com/highfivelabs/config-connector-extensions/apis/sqladmin/v1alpha1"
	"github.com/highfivelabs/config-connector-extensions/pkg/test/mockenv"
)

const namespace = "example"

// newCloudSQL is a client for Cloud SQL and the instance the backups are
// of (mockenv's: made in the fake cloud, or the live run's shared one).
func newCloudSQL(t *testing.T, env *mockenv.Env) (*sqladmin.Service, string) {
	t.Helper()
	svc, err := sqladmin.NewService(env.Ctx, env.GCPOptions()...)
	if err != nil {
		t.Fatal(err)
	}
	return svc, env.SQLInstance()
}

func newBackup(instance, name, description string) *sqladminv1alpha1.SQLAdminBackup {
	return &sqladminv1alpha1.SQLAdminBackup{
		ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: name, Labels: mockenv.Labels()},
		Spec: sqladminv1alpha1.SQLAdminBackupSpec{
			ProjectRef:  &refsv1beta1.ProjectRef{External: mockenv.Project},
			InstanceRef: &refsv1beta1.SQLInstanceRef{External: fmt.Sprintf("projects/%s/instances/%s", mockenv.Project, instance)},
			Location:    mockenv.Ptr("us"),
			Description: mockenv.Ptr(description),
		},
	}
}

// backupID reads the backup run's ID from status.externalRef
// (projects/{project}/backups/{id}) and checks observedState.id agrees.
func backupID(t *testing.T, u *unstructured.Unstructured) int64 {
	t.Helper()
	ref, _, _ := unstructured.NestedString(u.Object, "status", "externalRef")
	prefix := "projects/" + mockenv.Project + "/backups/"
	if !strings.HasPrefix(ref, prefix) {
		t.Fatalf("status.externalRef = %q, want %s<id>", ref, prefix)
	}
	id, err := strconv.ParseInt(strings.TrimPrefix(ref, prefix), 10, 64)
	if err != nil {
		t.Fatalf("status.externalRef = %q: %v", ref, err)
	}
	observed, _, _ := unstructured.NestedInt64(u.Object, "status", "observedState", "id")
	if observed != id {
		t.Errorf("status.observedState.id = %d, want %d (from externalRef)", observed, id)
	}
	return id
}

// waitOperation waits for a Cloud SQL operation to finish without error.
func waitOperation(t *testing.T, env *mockenv.Env, cloud *sqladmin.Service, op *sqladmin.Operation) {
	t.Helper()
	deadline := time.Now().Add(mockenv.Timeout())
	for op.Status != "DONE" {
		if time.Now().After(deadline) {
			t.Fatalf("operation %s did not finish", op.Name)
		}
		time.Sleep(time.Second)
		var err error
		if op, err = cloud.Operations.Get(mockenv.Project, op.Name).Context(env.Ctx).Do(); err != nil {
			t.Fatal(err)
		}
	}
	if op.Error != nil && len(op.Error.Errors) > 0 {
		t.Fatalf("operation %s failed: %s", op.Name, op.Error.Errors[0].Message)
	}
}

func isNotFound(err error) bool {
	var gerr *googleapi.Error
	return errors.As(err, &gerr) && gerr.Code == http.StatusNotFound
}

func TestSQLAdminBackup(t *testing.T) {
	env := mockenv.New(t, namespace)
	cloud, instance := newCloudSQL(t, env)
	gvk := sqladminv1alpha1.SQLAdminBackupGVK

	const name = "before-upgrade"
	backup := newBackup(instance, name, "before the Postgres 17 upgrade")
	if err := env.Kube.Create(env.Ctx, backup); err != nil {
		t.Fatal(err)
	}

	var id int64
	t.Run("create takes an on-demand backup", func(t *testing.T) {
		u := env.WaitReady(gvk, namespace, name)
		id = backupID(t, u)
		run, err := cloud.BackupRuns.Get(mockenv.Project, instance, id).Context(env.Ctx).Do()
		if err != nil {
			t.Fatalf("reading the backup run from the cloud: %v", err)
		}
		if run.Type != "ON_DEMAND" || run.Status != "SUCCESSFUL" {
			t.Errorf("backup run type/status = %s/%s, want ON_DEMAND/SUCCESSFUL", run.Type, run.Status)
		}
		if run.Description != "before the Postgres 17 upgrade" || run.Location != "us" {
			t.Errorf("backup run description/location = %q/%q, want the spec's", run.Description, run.Location)
		}
		if status, _, _ := unstructured.NestedString(u.Object, "status", "observedState", "status"); status != "SUCCESSFUL" {
			t.Errorf("status.observedState.status = %q, want SUCCESSFUL", status)
		}
	})

	t.Run("a resync takes no second backup", func(t *testing.T) {
		u := &unstructured.Unstructured{}
		u.SetGroupVersionKind(gvk)
		if err := env.Kube.Get(env.Ctx, types.NamespacedName{Namespace: namespace, Name: name}, u); err != nil {
			t.Fatal(err)
		}
		// Touch an annotation: the reconciler runs again on the same spec.
		ann := u.GetAnnotations()
		if ann == nil {
			ann = map[string]string{}
		}
		ann["test.gcp.hfl.dev/touch"] = "1"
		u.SetAnnotations(ann)
		if err := env.Kube.Update(env.Ctx, u); err != nil {
			t.Fatal(err)
		}
		time.Sleep(2 * time.Second)
		u = env.WaitReady(gvk, namespace, name)
		// mockgcp's backupRuns has no list; a second insert would get a new
		// ID (they are time-based), so an unchanged ID is the check.
		if got := backupID(t, u); got != id {
			t.Errorf("backup ID after resync = %d, want %d", got, id)
		}
	})

	t.Run("a backup deleted outside Kubernetes is taken again", func(t *testing.T) {
		// Real Cloud SQL keeps a deleted run, with status DELETED
		// (docs/live-runs.md); the controller must see it as gone.
		op, err := cloud.BackupRuns.Delete(mockenv.Project, instance, id).Context(env.Ctx).Do()
		if err != nil {
			t.Fatalf("deleting the backup run outside Kubernetes: %v", err)
		}
		waitOperation(t, env, cloud, op)
		u := &unstructured.Unstructured{}
		u.SetGroupVersionKind(gvk)
		if err := env.Kube.Get(env.Ctx, types.NamespacedName{Namespace: namespace, Name: name}, u); err != nil {
			t.Fatal(err)
		}
		ann := u.GetAnnotations()
		if ann == nil {
			ann = map[string]string{}
		}
		ann[k8s.InternalForceReconcileAnnotation] = "after-out-of-band-delete"
		u.SetAnnotations(ann)
		if err := env.Kube.Update(env.Ctx, u); err != nil {
			t.Fatal(err)
		}
		deadline := time.Now().Add(mockenv.Timeout())
		for {
			if err := env.Kube.Get(env.Ctx, types.NamespacedName{Namespace: namespace, Name: name}, u); err != nil {
				t.Fatal(err)
			}
			if ready, _ := mockenv.Ready(u); ready && backupID(t, u) != id {
				break
			}
			if time.Now().After(deadline) {
				t.Fatalf("no new backup after the old one was deleted outside Kubernetes; status: %v", u.Object["status"])
			}
			time.Sleep(time.Second)
		}
		newID := backupID(t, u)
		run, err := cloud.BackupRuns.Get(mockenv.Project, instance, newID).Context(env.Ctx).Do()
		if err != nil || run.Status != "SUCCESSFUL" {
			t.Errorf("the new backup run: %v, %v; want SUCCESSFUL", run, err)
		}
		id = newID
	})

	t.Run("a backup cannot be changed", func(t *testing.T) {
		obj := &sqladminv1alpha1.SQLAdminBackup{}
		if err := env.Kube.Get(env.Ctx, types.NamespacedName{Namespace: namespace, Name: name}, obj); err != nil {
			t.Fatal(err)
		}
		obj.Spec.Description = mockenv.Ptr("something else")
		if err := env.Kube.Update(env.Ctx, obj); err != nil {
			t.Fatal(err)
		}
		msg := env.WaitNotReady(gvk, namespace, name)
		if !strings.Contains(msg, "immutable") || !strings.Contains(msg, "description") {
			t.Errorf("Ready message = %q, want it to name the immutable description", msg)
		}
	})

	t.Run("delete removes the backup", func(t *testing.T) {
		if err := env.Kube.Delete(env.Ctx, backup); err != nil {
			t.Fatal(err)
		}
		env.WaitGone(&sqladminv1alpha1.SQLAdminBackup{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: name}})
		// Real Cloud SQL keeps a deleted run, with status DELETED
		// (docs/live-runs.md); mockgcp does the same (patch 0013).
		if run, err := cloud.BackupRuns.Get(mockenv.Project, instance, id).Context(env.Ctx).Do(); !isNotFound(err) {
			if err != nil || run.Status != "DELETED" {
				t.Errorf("reading the deleted backup run: got %v, %v; want 404 or status DELETED", run, err)
			}
		}
	})

	t.Run("abandon keeps the backup", func(t *testing.T) {
		const kept = "kept"
		b := newBackup(instance, kept, "kept after its object is deleted")
		b.Annotations = map[string]string{"cnrm.cloud.google.com/deletion-policy": "abandon"}
		if err := env.Kube.Create(env.Ctx, b); err != nil {
			t.Fatal(err)
		}
		keptID := backupID(t, env.WaitReady(gvk, namespace, kept))
		// The test deletes what it abandoned (a live run must leave nothing).
		t.Cleanup(func() {
			if _, err := cloud.BackupRuns.Delete(mockenv.Project, instance, keptID).Context(env.Ctx).Do(); err != nil && !isNotFound(err) {
				t.Errorf("deleting the abandoned backup run: %v", err)
			}
		})
		if err := env.Kube.Delete(env.Ctx, b); err != nil {
			t.Fatal(err)
		}
		env.WaitGone(&sqladminv1alpha1.SQLAdminBackup{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: kept}})
		if _, err := cloud.BackupRuns.Get(mockenv.Project, instance, keptID).Context(env.Ctx).Do(); err != nil {
			t.Errorf("the abandoned backup run is gone from the cloud: %v", err)
		}
	})
}
