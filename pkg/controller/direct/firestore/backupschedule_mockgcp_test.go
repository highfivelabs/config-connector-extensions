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

package firestore_test

// FirestoreBackupSchedule end to end on the fake cloud (pkg/test/mockenv)
// or a real project (docs/live-runs.md). The case: a Firestore database
// keeps weekly backups for a week, as the catalog's firestore module
// configures, then keeps them longer. The schedule's ID is the API's, so
// the test checks that the object finds it again by status.externalRef.
//
// A live run needs a Firestore database in the project (named by
// E2E_FIRESTORE_DATABASE, projects/P/databases/D) and
// roles/datastore.owner for the live-test service account; without them
// the test skips live. A database's ID cannot be reused for a while after
// it is deleted, so no test makes one.

import (
	"os"
	"strings"
	"testing"
	"time"

	admin "cloud.google.com/go/firestore/apiv1/admin"
	pb "cloud.google.com/go/firestore/apiv1/admin/adminpb"
	upstream "github.com/GoogleCloudPlatform/k8s-config-connector/apis/firestore/v1alpha1"
	firestorev1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/apis/firestore/v1beta1"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/k8s"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"

	krm "github.com/highfivelabs/config-connector-extensions/apis/firestore/v1beta1"
	"github.com/highfivelabs/config-connector-extensions/pkg/test/mockenv"
)

const namespace = "example"

func TestFirestoreBackupSchedule(t *testing.T) {
	database := "projects/" + mockenv.Project + "/databases/" + mockenv.Name("orders")
	if mockenv.Live() {
		database = os.Getenv("E2E_FIRESTORE_DATABASE")
		if database == "" {
			t.Skip("a live FirestoreBackupSchedule needs a Firestore database (E2E_FIRESTORE_DATABASE=projects/P/databases/D) and roles/datastore.owner for the live-test service account (docs/live-runs.md)")
		}
	}
	gvk := krm.FirestoreBackupScheduleGVK
	env := mockenv.New(t, namespace)
	cloud, err := admin.NewFirestoreAdminRESTClient(env.Ctx, env.GCPOptions()...)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cloud.Close() })

	get := func(n string) (*pb.BackupSchedule, error) {
		return cloud.GetBackupSchedule(env.Ctx, &pb.GetBackupScheduleRequest{Name: n})
	}
	writes := func() []string {
		var out []string
		for _, r := range env.Requests() {
			if (strings.HasPrefix(r, "POST ") || strings.HasPrefix(r, "PATCH ")) && strings.Contains(r, "/backupSchedules") {
				out = append(out, r)
			}
		}
		return out
	}
	getObj := func(t *testing.T, n string) *krm.FirestoreBackupSchedule {
		t.Helper()
		obj := &krm.FirestoreBackupSchedule{}
		if err := env.Kube.Get(env.Ctx, types.NamespacedName{Namespace: namespace, Name: n}, obj); err != nil {
			t.Fatal(err)
		}
		return obj
	}
	externalRef := func(t *testing.T, u *unstructured.Unstructured) string {
		t.Helper()
		ref, _, _ := unstructured.NestedString(u.Object, "status", "externalRef")
		if !strings.HasPrefix(ref, database+"/backupSchedules/") || len(ref) == len(database+"/backupSchedules/") {
			t.Fatalf("status.externalRef = %q, want %s/backupSchedules/<the API's ID>", ref, database)
		}
		return ref
	}
	waitFor := func(t *testing.T, what string, cond func() bool) {
		t.Helper()
		deadline := time.Now().Add(mockenv.Timeout())
		for !cond() {
			if time.Now().After(deadline) {
				t.Fatalf("timed out waiting for %s", what)
			}
			time.Sleep(500 * time.Millisecond)
		}
	}
	// Delete what the test made, the abandoned schedule included, and what
	// a failed step left.
	var made []string
	t.Cleanup(func() {
		for _, n := range made {
			err := cloud.DeleteBackupSchedule(env.Ctx, &pb.DeleteBackupScheduleRequest{Name: n})
			if err != nil && !direct.IsNotFound(err) {
				t.Errorf("cleaning up %s: %v", n, err)
			}
		}
	})

	newSchedule := func(n string) *krm.FirestoreBackupSchedule {
		return &krm.FirestoreBackupSchedule{
			ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: n, Labels: mockenv.Labels()},
			Spec: krm.FirestoreBackupScheduleSpec{
				DatabaseRef:      firestorev1beta1.FirestoreDatabaseRef{External: database},
				Retention:        mockenv.Ptr("604800s"),
				WeeklyRecurrence: &upstream.WeeklyRecurrence{Day: mockenv.Ptr("MONDAY")},
			},
		}
	}

	t.Run("the CRD refuses two recurrences", func(t *testing.T) {
		o := newSchedule("both")
		o.Spec.DailyRecurrence = &upstream.DailyRecurrence{}
		err := env.Kube.Create(env.Ctx, o)
		if err == nil || !strings.Contains(err.Error(), "at most one of dailyRecurrence and weeklyRecurrence") {
			t.Errorf("creating a schedule with both recurrences: got %v, want the API server to refuse it", err)
		}
	})

	if err := env.Kube.Create(env.Ctx, newSchedule("weekly")); err != nil {
		t.Fatal(err)
	}
	var schedule string

	t.Run("create makes a schedule with the API's ID", func(t *testing.T) {
		u := env.WaitReady(gvk, namespace, "weekly")
		schedule = externalRef(t, u)
		made = append(made, schedule)
		got, err := get(schedule)
		if err != nil {
			t.Fatalf("reading %s: %v", schedule, err)
		}
		if got.GetRetention().AsDuration() != 7*24*time.Hour || got.GetWeeklyRecurrence().GetDay().String() != "MONDAY" {
			t.Errorf("schedule = %v", got)
		}
		if ct, _, _ := unstructured.NestedString(u.Object, "status", "observedState", "createTime"); ct == "" {
			t.Errorf("status.observedState.createTime is empty")
		}
		if w := writes(); len(w) != 1 {
			t.Errorf("writes = %v, want one create", w)
		}
	})

	t.Run("a resync finds the schedule by its ID and writes nothing", func(t *testing.T) {
		before := len(writes())
		o := getObj(t, "weekly")
		o.Annotations = map[string]string{k8s.InternalForceReconcileAnnotation: "1"}
		if err := env.Kube.Update(env.Ctx, o); err != nil {
			t.Fatal(err)
		}
		time.Sleep(3 * time.Second)
		u := env.WaitReady(gvk, namespace, "weekly")
		if got := externalRef(t, u); got != schedule {
			t.Errorf("status.externalRef = %q, want %q (the same schedule)", got, schedule)
		}
		if w := writes()[before:]; len(w) != 0 {
			t.Errorf("a resync wrote %v", w)
		}
	})

	t.Run("a changed retention is one update of it", func(t *testing.T) {
		before := len(writes())
		o := getObj(t, "weekly")
		o.Spec.Retention = mockenv.Ptr("1209600s")
		if err := env.Kube.Update(env.Ctx, o); err != nil {
			t.Fatal(err)
		}
		waitFor(t, "the retention to change", func() bool {
			got, err := get(schedule)
			return err == nil && got.GetRetention().AsDuration() == 14*24*time.Hour
		})
		env.WaitReady(gvk, namespace, "weekly")
		w := writes()[before:]
		if len(w) != 1 || !strings.HasPrefix(w[0], "PATCH ") || !strings.Contains(w[0], "updateMask=retention") {
			t.Errorf("writes = %v, want one PATCH with updateMask=retention", w)
		}
	})

	t.Run("the CRD refuses a change of database", func(t *testing.T) {
		o := getObj(t, "weekly")
		o.Spec.DatabaseRef = firestorev1beta1.FirestoreDatabaseRef{External: database + "-other"}
		err := env.Kube.Update(env.Ctx, o)
		if err == nil || !strings.Contains(err.Error(), "databaseRef is immutable") {
			t.Errorf("changing databaseRef: got %v, want the API server to refuse it", err)
		}
	})

	t.Run("delete deletes the schedule", func(t *testing.T) {
		if err := env.Kube.Delete(env.Ctx, getObj(t, "weekly")); err != nil {
			t.Fatal(err)
		}
		env.WaitGone(&krm.FirestoreBackupSchedule{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: "weekly"}})
		if _, err := get(schedule); !direct.IsNotFound(err) {
			t.Errorf("after delete, reading the schedule: %v, want not found", err)
		}
	})

	t.Run("abandon keeps the schedule", func(t *testing.T) {
		if err := env.Kube.Create(env.Ctx, newSchedule("kept")); err != nil {
			t.Fatal(err)
		}
		u := env.WaitReady(gvk, namespace, "kept")
		kept := externalRef(t, u)
		made = append(made, kept)
		o := getObj(t, "kept")
		o.Annotations = map[string]string{"cnrm.cloud.google.com/deletion-policy": "abandon"}
		if err := env.Kube.Update(env.Ctx, o); err != nil {
			t.Fatal(err)
		}
		if err := env.Kube.Delete(env.Ctx, o); err != nil {
			t.Fatal(err)
		}
		env.WaitGone(&krm.FirestoreBackupSchedule{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: "kept"}})
		if _, err := get(kept); err != nil {
			t.Errorf("after abandon, reading the schedule: %v, want it kept", err)
		}
	})
}
