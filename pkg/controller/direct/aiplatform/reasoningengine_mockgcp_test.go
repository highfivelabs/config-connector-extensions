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

package aiplatform_test

// AIPlatformReasoningEngine end to end on the fake cloud (pkg/test/mockenv)
// or a real project (docs/live-runs.md). The case: an agent platform keeps
// an Agent Engine with no code of its own (sessions and memory only), as
// the catalog's geap-agent-runtime module does, so the test needs no
// pickled agent or container image. The engine's ID is the API's, so the
// test checks that the object finds it again by status.externalRef, and
// that a second object acquires an engine by resourceID.
//
// A live run needs aiplatform.googleapis.com on and roles/aiplatform.user
// for the live-test service account; it skips the test unless
// E2E_VERTEX_AI=true.

import (
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	refsv1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/apis/refs/v1beta1"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/k8s"
	"google.golang.org/api/googleapi"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"

	aiplatformv1beta1 "github.com/highfivelabs/config-connector-extensions/apis/aiplatform/v1beta1"
	"github.com/highfivelabs/config-connector-extensions/pkg/discokind"
	"github.com/highfivelabs/config-connector-extensions/pkg/test/mockenv"
)

const (
	namespace = "example"
	location  = "us-central1"
)

func TestAIPlatformReasoningEngine(t *testing.T) {
	if mockenv.Live() && os.Getenv("E2E_VERTEX_AI") != "true" {
		t.Skip("a live AIPlatformReasoningEngine needs aiplatform.googleapis.com on and roles/aiplatform.user for the live-test service account; set E2E_VERTEX_AI=true where it has them (docs/live-runs.md)")
	}
	gvk := aiplatformv1beta1.AIPlatformReasoningEngineGVK
	env := mockenv.New(t, namespace)
	cloud := &discokind.API{HTTP: env.HTTP, BaseURL: "https://" + location + "-aiplatform.googleapis.com/v1/", PollInterval: time.Second}

	parent := "projects/" + mockenv.Project + "/locations/" + location
	notFound := func(err error) bool {
		e, ok := err.(*googleapi.Error)
		return ok && e.Code == http.StatusNotFound
	}
	writes := func() []string {
		var out []string
		for _, r := range env.Requests() {
			if (strings.HasPrefix(r, "POST ") || strings.HasPrefix(r, "PATCH ")) && strings.Contains(r, "/reasoningEngines") && !strings.Contains(r, "/operations/") {
				out = append(out, r)
			}
		}
		return out
	}
	getObj := func(t *testing.T, n string) *aiplatformv1beta1.AIPlatformReasoningEngine {
		t.Helper()
		obj := &aiplatformv1beta1.AIPlatformReasoningEngine{}
		if err := env.Kube.Get(env.Ctx, types.NamespacedName{Namespace: namespace, Name: n}, obj); err != nil {
			t.Fatal(err)
		}
		return obj
	}
	externalRef := func(t *testing.T, u *unstructured.Unstructured) string {
		t.Helper()
		ref, _, _ := unstructured.NestedString(u.Object, "status", "externalRef")
		if !strings.HasPrefix(ref, parent+"/reasoningEngines/") || len(ref) == len(parent+"/reasoningEngines/") {
			t.Fatalf("status.externalRef = %q, want %s/reasoningEngines/<the API's ID>", ref, parent)
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
	// Delete what the test made, the abandoned engine included, and what a
	// failed step left.
	var made []string
	t.Cleanup(func() {
		for _, n := range made {
			op, err := cloud.Do(env.Ctx, http.MethodDelete, n, nil, nil)
			if err == nil {
				_, err = cloud.Wait(env.Ctx, op)
			}
			if err != nil && !notFound(err) {
				t.Errorf("cleaning up %s: %v", n, err)
			}
		}
	})

	newEngine := func(n string) *aiplatformv1beta1.AIPlatformReasoningEngine {
		labels := map[string]string{"team": "agents"}
		for k, v := range mockenv.Labels() {
			labels[k] = v
		}
		return &aiplatformv1beta1.AIPlatformReasoningEngine{
			ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: n, Labels: mockenv.Labels()},
			Spec: aiplatformv1beta1.AIPlatformReasoningEngineSpec{
				ProjectRef:  &refsv1beta1.ProjectRef{External: mockenv.Project},
				Location:    mockenv.Ptr(location),
				DisplayName: mockenv.Ptr(mockenv.Name(n)),
				Description: mockenv.Ptr("sessions and memory for the support agent"),
				Labels:      labels,
			},
		}
	}

	if err := env.Kube.Create(env.Ctx, newEngine("support-agent")); err != nil {
		t.Fatal(err)
	}
	var engine string

	t.Run("create makes an engine with the API's ID", func(t *testing.T) {
		u := env.WaitReady(gvk, namespace, "support-agent")
		engine = externalRef(t, u)
		made = append(made, engine)
		got, err := cloud.Get(env.Ctx, engine)
		if err != nil {
			t.Fatalf("reading %s: %v", engine, err)
		}
		if got["displayName"] != mockenv.Name("support-agent") || got["description"] != "sessions and memory for the support agent" {
			t.Errorf("engine = %v", got)
		}
		if l, _ := got["labels"].(map[string]any); l["team"] != "agents" {
			t.Errorf("engine labels = %v, want team=agents", got["labels"])
		}
		if ct, _, _ := unstructured.NestedString(u.Object, "status", "observedState", "createTime"); ct == "" {
			t.Errorf("status.observedState.createTime is empty")
		}
		if w := writes(); len(w) != 1 {
			t.Errorf("writes = %v, want one create", w)
		}
	})

	t.Run("a resync finds the engine by its ID and writes nothing", func(t *testing.T) {
		before := len(writes())
		o := getObj(t, "support-agent")
		o.Annotations = map[string]string{k8s.InternalForceReconcileAnnotation: "1"}
		if err := env.Kube.Update(env.Ctx, o); err != nil {
			t.Fatal(err)
		}
		time.Sleep(3 * time.Second)
		u := env.WaitReady(gvk, namespace, "support-agent")
		if got := externalRef(t, u); got != engine {
			t.Errorf("status.externalRef = %q, want %q (the same engine)", got, engine)
		}
		if w := writes()[before:]; len(w) != 0 {
			t.Errorf("a resync wrote %v", w)
		}
	})

	t.Run("a changed description is one update of it", func(t *testing.T) {
		before := len(writes())
		o := getObj(t, "support-agent")
		o.Spec.Description = mockenv.Ptr("sessions, memory and the example store")
		if err := env.Kube.Update(env.Ctx, o); err != nil {
			t.Fatal(err)
		}
		waitFor(t, "the description to change", func() bool {
			got, err := cloud.Get(env.Ctx, engine)
			return err == nil && got["description"] == "sessions, memory and the example store"
		})
		env.WaitReady(gvk, namespace, "support-agent")
		w := writes()[before:]
		if len(w) != 1 || !strings.HasPrefix(w[0], "PATCH ") || !strings.Contains(w[0], "updateMask=description") {
			t.Errorf("writes = %v, want one PATCH with updateMask=description", w)
		}
	})

	t.Run("the CRD refuses a change of location", func(t *testing.T) {
		o := getObj(t, "support-agent")
		o.Spec.Location = mockenv.Ptr("europe-west4")
		err := env.Kube.Update(env.Ctx, o)
		if err == nil || !strings.Contains(err.Error(), "location is immutable") {
			t.Errorf("changing location: got %v, want the API server to refuse it", err)
		}
	})

	t.Run("abandon keeps the engine", func(t *testing.T) {
		o := getObj(t, "support-agent")
		o.Annotations = map[string]string{"cnrm.cloud.google.com/deletion-policy": "abandon"}
		if err := env.Kube.Update(env.Ctx, o); err != nil {
			t.Fatal(err)
		}
		if err := env.Kube.Delete(env.Ctx, o); err != nil {
			t.Fatal(err)
		}
		env.WaitGone(&aiplatformv1beta1.AIPlatformReasoningEngine{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: "support-agent"}})
		if _, err := cloud.Get(env.Ctx, engine); err != nil {
			t.Errorf("after abandon, reading the engine: %v, want it kept", err)
		}
	})

	t.Run("resourceID acquires the abandoned engine, and delete deletes it", func(t *testing.T) {
		before := len(writes())
		o := newEngine("acquired")
		o.Spec.DisplayName = mockenv.Ptr(mockenv.Name("support-agent"))
		o.Spec.Description = mockenv.Ptr("sessions, memory and the example store")
		o.Spec.ResourceID = mockenv.Ptr(engine[strings.LastIndex(engine, "/")+1:])
		if err := env.Kube.Create(env.Ctx, o); err != nil {
			t.Fatal(err)
		}
		u := env.WaitReady(gvk, namespace, "acquired")
		if got := externalRef(t, u); got != engine {
			t.Errorf("status.externalRef = %q, want %q (the abandoned engine)", got, engine)
		}
		if w := writes()[before:]; len(w) != 0 {
			t.Errorf("acquiring an unchanged engine wrote %v", w)
		}
		if err := env.Kube.Delete(env.Ctx, getObj(t, "acquired")); err != nil {
			t.Fatal(err)
		}
		env.WaitGone(&aiplatformv1beta1.AIPlatformReasoningEngine{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: "acquired"}})
		if _, err := cloud.Get(env.Ctx, engine); !notFound(err) {
			t.Errorf("after delete, reading the engine: %v, want not found", err)
		}
	})
}
