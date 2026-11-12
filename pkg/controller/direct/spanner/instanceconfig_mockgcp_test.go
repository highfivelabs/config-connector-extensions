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

package spanner_test

// SpannerInstanceConfig end to end on the fake cloud (pkg/test/mockenv,
// patch 0036) or a real project (docs/live-runs.md). The case: a Spanner
// instance needs a read-only replica beside its region, as the catalog's
// spanner-instance module makes one, so it gets a user-managed
// configuration: the regional base configuration with one read-only replica
// added. Making the configuration costs nothing; only an instance in it
// would.
//
// A live run needs spanner.googleapis.com on and roles/spanner.admin for
// the live-test service account; it skips the test unless E2E_SPANNER=true.

import (
	"os"
	"strings"
	"testing"
	"time"

	instance "cloud.google.com/go/spanner/admin/instance/apiv1"
	pb "cloud.google.com/go/spanner/admin/instance/apiv1/instancepb"
	refv1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/apis/refs/v1beta1"
	upstream "github.com/GoogleCloudPlatform/k8s-config-connector/apis/spanner/v1alpha1"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/k8s"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"

	spannerv1beta1 "github.com/highfivelabs/config-connector-extensions/apis/spanner/v1beta1"
	"github.com/highfivelabs/config-connector-extensions/pkg/test/mockenv"
)

const namespace = "example"

func TestSpannerInstanceConfig(t *testing.T) {
	if mockenv.Live() && os.Getenv("E2E_SPANNER") != "true" {
		t.Skip("a live SpannerInstanceConfig needs spanner.googleapis.com on and roles/spanner.admin for the live-test service account; set E2E_SPANNER=true where it has them (docs/live-runs.md)")
	}
	gvk := spannerv1beta1.SpannerInstanceConfigGVK
	env := mockenv.New(t, namespace)
	cloud, err := instance.NewInstanceAdminRESTClient(env.Ctx, env.GCPOptions()...)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cloud.Close() })

	project := "projects/" + mockenv.Project
	// A user-managed configuration's ID starts with "custom-".
	configID := func(base string) string { return "custom-" + mockenv.Name(base) }
	configName := func(base string) string { return project + "/instanceConfigs/" + configID(base) }
	get := func(n string) (*pb.InstanceConfig, error) {
		return cloud.GetInstanceConfig(env.Ctx, &pb.GetInstanceConfigRequest{Name: n})
	}
	writes := func() []string {
		var out []string
		for _, r := range env.Requests() {
			if (strings.HasPrefix(r, "POST ") || strings.HasPrefix(r, "PATCH ")) && strings.Contains(r, "/instanceConfigs") && !strings.Contains(r, "/operations") {
				out = append(out, r)
			}
		}
		return out
	}
	getObj := func(t *testing.T, n string) *spannerv1beta1.SpannerInstanceConfig {
		t.Helper()
		obj := &spannerv1beta1.SpannerInstanceConfig{}
		if err := env.Kube.Get(env.Ctx, types.NamespacedName{Namespace: namespace, Name: n}, obj); err != nil {
			t.Fatal(err)
		}
		return obj
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
	// Delete what the test made, the abandoned configuration included.
	t.Cleanup(func() {
		for _, base := range []string{"read-replica", "kept"} {
			err := cloud.DeleteInstanceConfig(env.Ctx, &pb.DeleteInstanceConfigRequest{Name: configName(base)})
			if err != nil && !direct.IsNotFound(err) {
				t.Errorf("cleaning up %s: %v", configName(base), err)
			}
		}
	})

	newConfig := func(n string) *spannerv1beta1.SpannerInstanceConfig {
		labels := map[string]string{"team": "data"}
		for k, v := range mockenv.Labels() {
			labels[k] = v
		}
		return &spannerv1beta1.SpannerInstanceConfig{
			ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: n, Labels: mockenv.Labels()},
			Spec: spannerv1beta1.SpannerInstanceConfigSpec{
				Parent:        spannerv1beta1.Parent{ProjectRef: &refv1beta1.ProjectRef{External: mockenv.Project}},
				ResourceID:    mockenv.Ptr(configID(n)),
				DisplayName:   mockenv.Ptr("us-central1 with a read replica"),
				BaseConfigRef: &upstream.InstanceConfigRef{External: project + "/instanceConfigs/regional-us-central1"},
				Replicas: []upstream.ReplicaInfo{
					{Location: mockenv.Ptr("us-central1"), Type: mockenv.Ptr("READ_WRITE"), DefaultLeaderLocation: mockenv.Ptr(true)},
					{Location: mockenv.Ptr("us-central1"), Type: mockenv.Ptr("READ_WRITE")},
					{Location: mockenv.Ptr("us-central1"), Type: mockenv.Ptr("READ_WRITE")},
					{Location: mockenv.Ptr("us-east1"), Type: mockenv.Ptr("READ_ONLY")},
				},
				Labels: labels,
			},
		}
	}

	if err := env.Kube.Create(env.Ctx, newConfig("read-replica")); err != nil {
		t.Fatal(err)
	}

	t.Run("create makes the user-managed configuration", func(t *testing.T) {
		u := env.WaitReady(gvk, namespace, "read-replica")
		if ref, _, _ := unstructured.NestedString(u.Object, "status", "externalRef"); ref != configName("read-replica") {
			t.Errorf("status.externalRef = %q, want %q", ref, configName("read-replica"))
		}
		got, err := get(configName("read-replica"))
		if err != nil {
			t.Fatalf("reading %s: %v", configName("read-replica"), err)
		}
		if got.DisplayName != "us-central1 with a read replica" || len(got.Replicas) != 4 || got.Labels["team"] != "data" {
			t.Errorf("configuration = %v", got)
		}
		if got.BaseConfig != project+"/instanceConfigs/regional-us-central1" {
			t.Errorf("baseConfig = %q", got.BaseConfig)
		}
		if ct, _, _ := unstructured.NestedString(u.Object, "status", "observedState", "configType"); ct != "USER_MANAGED" {
			t.Errorf("status.observedState.configType = %q, want USER_MANAGED", ct)
		}
	})

	t.Run("a resync writes nothing", func(t *testing.T) {
		before := len(writes())
		o := getObj(t, "read-replica")
		o.Annotations = map[string]string{k8s.InternalForceReconcileAnnotation: "1"}
		if err := env.Kube.Update(env.Ctx, o); err != nil {
			t.Fatal(err)
		}
		time.Sleep(3 * time.Second)
		env.WaitReady(gvk, namespace, "read-replica")
		if w := writes()[before:]; len(w) != 0 {
			t.Errorf("a resync wrote %v", w)
		}
	})

	t.Run("a changed display name is one update of it", func(t *testing.T) {
		before := len(writes())
		o := getObj(t, "read-replica")
		o.Spec.DisplayName = mockenv.Ptr("us-central1 and us-east1")
		if err := env.Kube.Update(env.Ctx, o); err != nil {
			t.Fatal(err)
		}
		waitFor(t, "the display name to change", func() bool {
			got, err := get(configName("read-replica"))
			return err == nil && got.DisplayName == "us-central1 and us-east1"
		})
		u := env.WaitReady(gvk, namespace, "read-replica")
		if ref, _, _ := unstructured.NestedString(u.Object, "status", "externalRef"); ref != configName("read-replica") {
			t.Errorf("after the update, status.externalRef = %q, want %q", ref, configName("read-replica"))
		}
		w := writes()[before:]
		if len(w) != 1 || !strings.HasPrefix(w[0], "PATCH ") {
			t.Errorf("writes = %v, want one PATCH", w)
		}
	})

	t.Run("the CRD refuses a change of replicas", func(t *testing.T) {
		o := getObj(t, "read-replica")
		o.Spec.Replicas = o.Spec.Replicas[:3]
		err := env.Kube.Update(env.Ctx, o)
		if err == nil || !strings.Contains(err.Error(), "replicas is immutable") {
			t.Errorf("changing replicas: got %v, want the API server to refuse it", err)
		}
	})

	t.Run("delete deletes the configuration", func(t *testing.T) {
		if err := env.Kube.Delete(env.Ctx, getObj(t, "read-replica")); err != nil {
			t.Fatal(err)
		}
		env.WaitGone(&spannerv1beta1.SpannerInstanceConfig{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: "read-replica"}})
		if _, err := get(configName("read-replica")); !direct.IsNotFound(err) {
			t.Errorf("after delete, reading the configuration: %v, want not found", err)
		}
	})

	t.Run("abandon keeps the configuration", func(t *testing.T) {
		if err := env.Kube.Create(env.Ctx, newConfig("kept")); err != nil {
			t.Fatal(err)
		}
		env.WaitReady(gvk, namespace, "kept")
		o := getObj(t, "kept")
		o.Annotations = map[string]string{"cnrm.cloud.google.com/deletion-policy": "abandon"}
		if err := env.Kube.Update(env.Ctx, o); err != nil {
			t.Fatal(err)
		}
		if err := env.Kube.Delete(env.Ctx, o); err != nil {
			t.Fatal(err)
		}
		env.WaitGone(&spannerv1beta1.SpannerInstanceConfig{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: "kept"}})
		if _, err := get(configName("kept")); err != nil {
			t.Errorf("after abandon, reading the configuration: %v, want it kept", err)
		}
	})
}
