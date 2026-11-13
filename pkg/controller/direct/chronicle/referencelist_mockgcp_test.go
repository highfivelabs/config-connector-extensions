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

package chronicle_test

// ChronicleReferenceList (chronicle.gcp.hfl.dev/v1beta1) end to end on the
// fake cloud (pkg/controller/direct/chronicle/fake) or a real Google SecOps
// instance (docs/live-runs.md). The case: the catalog's secops-rules
// module, a reference list made with its ID, its entries changed, a change
// made by hand put back, its description changed, the ID's form checked by
// the CRD, and the object deleted with the list left (the API has no
// delete) and taken back by a new object.

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	kccchroniclev1alpha1 "github.com/GoogleCloudPlatform/k8s-config-connector/apis/chronicle/v1alpha1"
	refsv1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/apis/refs/v1beta1"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/k8s"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"

	krm "github.com/highfivelabs/config-connector-extensions/apis/chronicle/v1beta1"
	"github.com/highfivelabs/config-connector-extensions/pkg/discokind"
	"github.com/highfivelabs/config-connector-extensions/pkg/test/mockenv"
)

func TestChronicleReferenceList(t *testing.T) {
	location, inst := instance(t)
	const ns = "referencelists"
	env := mockenv.New(t, ns)
	api := &discokind.API{HTTP: env.HTTP, BaseURL: "https://" + location + "-chronicle.googleapis.com/v1/"}
	parent := fmt.Sprintf("projects/%s/locations/%s/instances/%s", mockenv.Project, location, inst)
	listGVK := krm.ChronicleReferenceListGVK
	// The list's ID: the API takes letters, digits and underscores only, and
	// a list cannot be deleted, so live it carries the run ID.
	listID := strings.ReplaceAll(mockenv.Name("cce_hosts"), "-", "_")
	name := parent + "/referenceLists/" + listID

	entries := func(values ...string) []kccchroniclev1alpha1.ReferenceListEntry {
		var out []kccchroniclev1alpha1.ReferenceListEntry
		for _, v := range values {
			out = append(out, kccchroniclev1alpha1.ReferenceListEntry{Value: mockenv.Ptr(v)})
		}
		return out
	}
	cloudValues := func(t *testing.T) []string {
		t.Helper()
		got, err := api.Do(env.Ctx, http.MethodGet, name, map[string]string{"view": "REFERENCE_LIST_VIEW_FULL"}, nil)
		if err != nil {
			t.Fatalf("reading %s: %v", name, err)
		}
		var out []string
		l, _ := got["entries"].([]any)
		for _, e := range l {
			m, _ := e.(map[string]any)
			v, _ := m["value"].(string)
			out = append(out, v)
		}
		return out
	}
	writes := func() []string {
		var out []string
		for _, r := range env.Requests() {
			method, raw, _ := strings.Cut(r, " ")
			u, err := url.Parse(raw)
			if err != nil || method == http.MethodGet || !strings.Contains(u.Path, "/v1/"+parent+"/referenceLists") {
				continue
			}
			out = append(out, method+" "+u.Query().Get("updateMask")+u.Query().Get("referenceListId"))
		}
		return out
	}
	getObj := func(t *testing.T, objName string) *krm.ChronicleReferenceList {
		t.Helper()
		obj := &krm.ChronicleReferenceList{}
		if err := env.Kube.Get(env.Ctx, types.NamespacedName{Namespace: ns, Name: objName}, obj); err != nil {
			t.Fatal(err)
		}
		return obj
	}
	tryUpdate := func(t *testing.T, objName string, change func(*krm.ChronicleReferenceList)) error {
		t.Helper()
		return retry.RetryOnConflict(retry.DefaultRetry, func() error {
			o := getObj(t, objName)
			change(o)
			return env.Kube.Update(env.Ctx, o)
		})
	}
	update := func(t *testing.T, objName string, change func(*krm.ChronicleReferenceList)) {
		t.Helper()
		if err := tryUpdate(t, objName, change); err != nil {
			t.Fatal(err)
		}
	}
	resync := func(t *testing.T, objName string) {
		t.Helper()
		update(t, objName, func(o *krm.ChronicleReferenceList) {
			if o.Annotations == nil {
				o.Annotations = map[string]string{}
			}
			o.Annotations[k8s.InternalForceReconcileAnnotation] = time.Now().Format(time.RFC3339Nano)
		})
	}
	waitValues := func(t *testing.T, want ...string) {
		t.Helper()
		deadline := time.Now().Add(mockenv.Timeout())
		for {
			got := cloudValues(t)
			if strings.Join(got, ",") == strings.Join(want, ",") {
				return
			}
			if time.Now().After(deadline) {
				t.Fatalf("the list's entries are %q, want %q", got, want)
			}
			time.Sleep(time.Second)
		}
	}
	newObj := func(objName string, values ...string) *krm.ChronicleReferenceList {
		return &krm.ChronicleReferenceList{
			ObjectMeta: metav1.ObjectMeta{Namespace: ns, Name: objName, Labels: mockenv.Labels()},
			Spec: krm.ChronicleReferenceListSpec{
				ProjectRef:  &refsv1beta1.ProjectRef{External: mockenv.Project},
				Location:    location,
				Instance:    inst,
				ResourceID:  mockenv.Ptr(listID),
				Description: mockenv.Ptr("made by a config-connector-extensions test"),
				Entries:     entries(values...),
				SyntaxType:  mockenv.Ptr("REFERENCE_LIST_SYNTAX_TYPE_PLAIN_TEXT_STRING"),
			},
		}
	}

	obj := newObj("hosts", "cce-test-a.invalid", "cce-test-b.invalid")
	if err := env.Kube.Create(env.Ctx, obj); err != nil {
		t.Fatal(err)
	}

	t.Run("create makes the list with its ID and entries", func(t *testing.T) {
		u := env.WaitReady(listGVK, ns, "hosts")
		if got, _, _ := unstructured.NestedString(u.Object, "status", "externalRef"); got != name {
			t.Errorf("status.externalRef = %q, want %q", got, name)
		}
		if got := cloudValues(t); strings.Join(got, ",") != "cce-test-a.invalid,cce-test-b.invalid" {
			t.Errorf("entries = %q", got)
		}
		if w := writes(); len(w) != 1 || w[0] != "POST "+listID {
			t.Errorf("writes = %q, want one create", w)
		}
		if dn, _, _ := unstructured.NestedString(u.Object, "status", "observedState", "displayName"); dn != listID {
			t.Errorf("status.observedState.displayName = %q, want %q", dn, listID)
		}
	})

	t.Run("a resync with nothing changed writes nothing", func(t *testing.T) {
		before := len(writes())
		resync(t, "hosts")
		time.Sleep(3 * time.Second)
		env.WaitReady(listGVK, ns, "hosts")
		if n := len(writes()); n != before {
			t.Errorf("a resync wrote (%d writes, was %d)", n, before)
		}
	})

	t.Run("changed entries are patched alone", func(t *testing.T) {
		before := len(writes())
		update(t, "hosts", func(o *krm.ChronicleReferenceList) {
			o.Spec.Entries = entries("cce-test-b.invalid", "cce-test-c.invalid")
		})
		waitValues(t, "cce-test-b.invalid", "cce-test-c.invalid")
		env.WaitReady(listGVK, ns, "hosts")
		if w := writes()[before:]; len(w) != 1 || w[0] != "PATCH entries" {
			t.Errorf("writes = %q, want one PATCH of entries", w)
		}
	})

	t.Run("a change made by hand is put back", func(t *testing.T) {
		if _, err := api.Do(env.Ctx, http.MethodPatch, name, map[string]string{"updateMask": "entries"}, map[string]any{"entries": []any{map[string]any{"value": "by-hand.invalid"}}}); err != nil {
			t.Fatal(err)
		}
		resync(t, "hosts")
		waitValues(t, "cce-test-b.invalid", "cce-test-c.invalid")
	})

	t.Run("a changed description is patched alone", func(t *testing.T) {
		before := len(writes())
		update(t, "hosts", func(o *krm.ChronicleReferenceList) {
			o.Spec.Description = mockenv.Ptr("changed by a config-connector-extensions test")
		})
		deadline := time.Now().Add(mockenv.Timeout())
		for {
			got, err := api.Get(env.Ctx, name)
			if err == nil && got["description"] == "changed by a config-connector-extensions test" {
				break
			}
			if time.Now().After(deadline) {
				t.Fatalf("the description is not changed: %v (%v)", got, err)
			}
			time.Sleep(time.Second)
		}
		env.WaitReady(listGVK, ns, "hosts")
		if w := writes()[before:]; len(w) != 1 || w[0] != "PATCH description" {
			t.Errorf("writes = %q, want one PATCH of description", w)
		}
	})

	t.Run("the CRD refuses a hyphenated ID and a new instance", func(t *testing.T) {
		bad := newObj("bad-id")
		bad.Spec.ResourceID = mockenv.Ptr("has-hyphen")
		if err := env.Kube.Create(env.Ctx, bad); err == nil || !strings.Contains(err.Error(), "resourceID") {
			t.Errorf("a hyphenated ID: %v", err)
		}
		if err := tryUpdate(t, "hosts", func(o *krm.ChronicleReferenceList) { o.Spec.Instance = "00000000-0000-4000-8000-0000000000ff" }); err == nil || !strings.Contains(err.Error(), "instance is immutable") {
			t.Errorf("changing instance: %v", err)
		}
	})

	t.Run("delete leaves the list, and a new object takes it back", func(t *testing.T) {
		before := len(writes())
		o := getObj(t, "hosts")
		if err := env.Kube.Delete(env.Ctx, o); err != nil {
			t.Fatal(err)
		}
		env.WaitGone(o)
		if w := writes()[before:]; len(w) != 0 {
			t.Errorf("delete wrote %q", w)
		}
		if got := cloudValues(t); len(got) != 2 {
			t.Errorf("the list after delete: %q", got)
		}
		again := newObj("hosts-again", "cce-test-d.invalid")
		if err := env.Kube.Create(env.Ctx, again); err != nil {
			t.Fatal(err)
		}
		env.WaitReady(listGVK, ns, "hosts-again")
		waitValues(t, "cce-test-d.invalid")
		if w := writes()[before:]; len(w) != 1 || w[0] != "PATCH entries" {
			t.Errorf("taking the list back wrote %q, want one PATCH of entries", w)
		}
		if err := env.Kube.Delete(env.Ctx, again); err != nil {
			t.Fatal(err)
		}
		env.WaitGone(again)
	})
}
