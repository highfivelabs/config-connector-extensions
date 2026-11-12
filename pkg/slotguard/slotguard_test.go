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

package slotguard

import (
	"context"
	"errors"
	"maps"
	"strings"
	"testing"

	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct/directbase"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/record"
)

func TestResolve(t *testing.T) {
	t.Setenv(EnvVar, "from-env")
	if got := Resolve("from-flag"); got != "from-flag" {
		t.Errorf("Resolve(flag) = %q, want the flag", got)
	}
	if got := Resolve(""); got != "from-env" {
		t.Errorf("Resolve(\"\") = %q, want %s", got, EnvVar)
	}
	t.Setenv(EnvVar, "")
	if got := Resolve(""); got != "" {
		t.Errorf("Resolve with neither = %q, want \"\" (the guard off)", got)
	}
}

func TestValidate(t *testing.T) {
	for _, ok := range []string{"blue", "v1-2", "a", "slot_2", strings.Repeat("a", 63)} {
		if err := Validate(ok); err != nil {
			t.Errorf("Validate(%q) = %v, want nil", ok, err)
		}
	}
	for _, bad := range []string{"", "Blue", "-a", "a-", "a.b", "a/b", strings.Repeat("a", 64)} {
		if Validate(bad) == nil {
			t.Errorf("Validate(%q) = nil, want an error", bad)
		}
	}
}

// fakeModel and fakeAdapter record what the guard let through.
type fakeModel struct{ desired *unstructured.Unstructured }

func (m *fakeModel) AdapterForObject(_ context.Context, op *directbase.AdapterForObjectOperation) (directbase.Adapter, error) {
	m.desired = op.GetUnstructured()
	return &fakeAdapter{}, nil
}
func (m *fakeModel) AdapterForURL(context.Context, string) (directbase.Adapter, error) {
	return nil, nil
}

type fakeAdapter struct {
	labels                  map[string]string
	created, updated, deled bool
}

func (a *fakeAdapter) Find(context.Context) (bool, error) { return a.labels != nil, nil }
func (a *fakeAdapter) Create(context.Context, *directbase.CreateOperation) error {
	a.created = true
	return nil
}
func (a *fakeAdapter) Update(context.Context, *directbase.UpdateOperation) error {
	a.updated = true
	return nil
}
func (a *fakeAdapter) Delete(context.Context, *directbase.DeleteOperation) (bool, error) {
	a.deled = true
	return true, nil
}
func (a *fakeAdapter) Export(context.Context) (*unstructured.Unstructured, error) { return nil, nil }
func (a *fakeAdapter) CloudLabels() map[string]string                             { return a.labels }

var gvk = schema.GroupVersionKind{Group: "test.gcp.hfl.dev", Version: "v1", Kind: "Thing"}

func object(specLabels map[string]string, annotations map[string]string) *unstructured.Unstructured {
	u := &unstructured.Unstructured{Object: map[string]any{"spec": map[string]any{}}}
	u.SetGroupVersionKind(gvk)
	u.SetName("thing")
	u.SetLabels(map[string]string{"app": "x", "team.example.com/owner": "y"})
	u.SetAnnotations(annotations)
	if specLabels != nil {
		m := map[string]any{}
		for k, v := range specLabels {
			m[k] = v
		}
		u.Object["spec"].(map[string]any)["labels"] = m
	}
	return u
}

func TestDesiredLabels(t *testing.T) {
	for _, tc := range []struct {
		name       string
		src        LabelSource
		specLabels map[string]string
		wantMeta   map[string]string
		wantSpec   map[string]string
	}{
		{name: "none", src: None,
			wantMeta: map[string]string{"app": "x", "team.example.com/owner": "y", ObjectLabel: "blue"}},
		{name: "metadata", src: Metadata,
			wantMeta: map[string]string{"app": "x", "team.example.com/owner": "y", ObjectLabel: "blue", CloudLabel: "blue"}},
		{name: "metadata, the object sets spec.labels", src: Metadata, specLabels: map[string]string{"a": "1"},
			wantMeta: map[string]string{"app": "x", "team.example.com/owner": "y", ObjectLabel: "blue", CloudLabel: "blue"},
			wantSpec: map[string]string{"a": "1", CloudLabel: "blue"}},
		{name: "spec", src: Spec,
			wantMeta: map[string]string{"app": "x", "team.example.com/owner": "y", ObjectLabel: "blue"},
			wantSpec: map[string]string{CloudLabel: "blue"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			inner := &fakeModel{}
			g := &Guard{Slot: "blue", Recorder: record.NewFakeRecorder(10)}
			m, err := g.Wrap(gvk, inner, tc.src)
			if err != nil {
				t.Fatal(err)
			}
			u := object(tc.specLabels, nil)
			if _, err := m.AdapterForObject(context.Background(), &directbase.AdapterForObjectOperation{Object: u}); err != nil {
				t.Fatal(err)
			}
			if got := inner.desired.GetLabels(); !maps.Equal(got, tc.wantMeta) {
				t.Errorf("desired metadata.labels = %v, want %v", got, tc.wantMeta)
			}
			gotSpec, _, _ := unstructured.NestedStringMap(inner.desired.Object, "spec", "labels")
			if !maps.Equal(gotSpec, tc.wantSpec) {
				t.Errorf("desired spec.labels = %v, want %v", gotSpec, tc.wantSpec)
			}
			// The object itself gets only its own label: the cloud label
			// stays out of it.
			if got := u.GetLabels()[ObjectLabel]; got != "blue" {
				t.Errorf("object label %s = %q, want blue", ObjectLabel, got)
			}
			if _, ok := u.GetLabels()[CloudLabel]; ok {
				t.Errorf("object has the cloud label %s: %v", CloudLabel, u.GetLabels())
			}
			if s, _, _ := unstructured.NestedStringMap(u.Object, "spec", "labels"); !maps.Equal(s, tc.specLabels) {
				t.Errorf("object spec.labels = %v, want them unchanged %v", s, tc.specLabels)
			}
		})
	}
}

func TestHolder(t *testing.T) {
	for _, tc := range []struct {
		name     string
		cloud    map[string]string
		takeover string
		wantHeld bool
	}{
		{name: "unmarked is claimed", cloud: map[string]string{"app": "x"}},
		{name: "ours", cloud: map[string]string{CloudLabel: "blue"}},
		{name: "another slot's", cloud: map[string]string{CloudLabel: "green"}, wantHeld: true},
		{name: "the takeover slot's", cloud: map[string]string{CloudLabel: "green"}, takeover: "green"},
		{name: "a third slot's, with a takeover", cloud: map[string]string{CloudLabel: "red"}, takeover: "green", wantHeld: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec := record.NewFakeRecorder(10)
			g := &Guard{Slot: "blue", Recorder: rec}
			m, _ := g.Wrap(gvk, &fakeModel{}, Spec)
			var ann map[string]string
			if tc.takeover != "" {
				ann = map[string]string{TakeoverAnnotation: tc.takeover}
			}
			a, err := m.AdapterForObject(context.Background(), &directbase.AdapterForObjectOperation{Object: object(nil, ann)})
			if err != nil {
				t.Fatal(err)
			}
			inner := a.(*adapter).inner.(*fakeAdapter)
			inner.labels = tc.cloud
			if found, err := a.Find(context.Background()); !found || err != nil {
				t.Fatalf("Find = %v, %v", found, err)
			}
			_, err = a.Delete(context.Background(), &directbase.DeleteOperation{})
			if tc.wantHeld {
				if inner.deled || err == nil || !strings.HasPrefix(err.Error(), Reason+": ") {
					t.Errorf("delete of a held resource: sent %v, err %v; want refused with %s", inner.deled, err, Reason)
				}
				select {
				case e := <-rec.Events:
					if !strings.HasPrefix(e, "Warning "+Reason+" cloud resource is held by slot ") {
						t.Errorf("event %q", e)
					}
				default:
					t.Error("no event")
				}
			} else if !inner.deled || err != nil {
				t.Errorf("delete: sent %v, err %v; want it sent", inner.deled, err)
			}
		})
	}
}

func TestFindNeedsCloudLabels(t *testing.T) {
	g := &Guard{Slot: "blue", Recorder: record.NewFakeRecorder(1)}
	m, _ := g.Wrap(gvk, &noLabelsModel{}, Spec)
	a, _ := m.AdapterForObject(context.Background(), &directbase.AdapterForObjectOperation{Object: object(nil, nil)})
	if _, err := a.Find(context.Background()); err == nil {
		t.Error("Find on a labelled kind whose adapter cannot read labels: no error")
	}
}

type noLabelsModel struct{}

func (noLabelsModel) AdapterForObject(context.Context, *directbase.AdapterForObjectOperation) (directbase.Adapter, error) {
	return noLabels{}, nil
}
func (noLabelsModel) AdapterForURL(context.Context, string) (directbase.Adapter, error) {
	return nil, nil
}

type noLabels struct{}

func (noLabels) Find(context.Context) (bool, error)                        { return true, nil }
func (noLabels) Create(context.Context, *directbase.CreateOperation) error { return errors.New("no") }
func (noLabels) Update(context.Context, *directbase.UpdateOperation) error { return errors.New("no") }
func (noLabels) Delete(context.Context, *directbase.DeleteOperation) (bool, error) {
	return false, errors.New("no")
}
func (noLabels) Export(context.Context) (*unstructured.Unstructured, error) { return nil, nil }
