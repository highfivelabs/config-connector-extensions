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

package accesscontextmanager_test

// AccessContextManagerGCPUserAccessBinding (ours: the stable equivalent of
// Config Connector's alpha kind, with the fields it lacks) end to end on
// the fake cloud (pkg/test/mockenv; the user access binding fake,
// pkg/controller/direct/accesscontextmanager/fake). The cases: a binding
// with access levels, a dry-run level and session settings, its ID written
// back to spec.resourceID; a resync that writes nothing; scoped access
// settings set, the dry run and the session taken off; the CRD's refusals;
// a binding already in the organization found by its group key; abandon;
// delete. Live it skips: a binding needs an organization, a Google Group
// and an access policy's levels, which a live run does not touch.

import (
	"context"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/k8s"
	api "google.golang.org/api/accesscontextmanager/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"

	krm "github.com/highfivelabs/config-connector-extensions/apis/accesscontextmanager/v1beta1"
	"github.com/highfivelabs/config-connector-extensions/pkg/test/mockenv"
)

const (
	namespace = "example"
	org       = "organizations/1234567890"
	level     = "accessPolicies/1111/accessLevels/corp_devices"
	dryLevel  = "accessPolicies/1111/accessLevels/managed_devices"
)

func TestAccessContextManagerGCPUserAccessBinding(t *testing.T) {
	if mockenv.Live() {
		t.Skip("a user access binding needs an organization, a Google Group and an access policy's levels, which a live run does not touch (docs/live-runs.md)")
	}
	gvk := krm.AccessContextManagerGCPUserAccessBindingGVK
	env := mockenv.New(t, namespace)
	svc, err := api.NewService(env.Ctx, env.GCPOptions()...)
	if err != nil {
		t.Fatal(err)
	}
	cloud := func(name string) (*api.GcpUserAccessBinding, error) {
		return svc.Organizations.GcpUserAccessBindings.Get(name).Context(env.Ctx).Do()
	}
	writes := func(method string) int {
		n := 0
		for _, r := range env.Requests() {
			if strings.HasPrefix(r, method+" ") && strings.Contains(r, "/gcpUserAccessBindings") {
				n++
			}
		}
		return n
	}
	get := func(t *testing.T, name string) *krm.AccessContextManagerGCPUserAccessBinding {
		t.Helper()
		obj := &krm.AccessContextManagerGCPUserAccessBinding{}
		if err := env.Kube.Get(env.Ctx, types.NamespacedName{Namespace: namespace, Name: name}, obj); err != nil {
			t.Fatal(err)
		}
		return obj
	}
	update := func(t *testing.T, name string, change func(*krm.AccessContextManagerGCPUserAccessBinding)) {
		t.Helper()
		o := get(t, name)
		change(o)
		if err := env.Kube.Update(env.Ctx, o); err != nil {
			t.Fatal(err)
		}
	}
	resyncWritesNothing := func(t *testing.T, name string) {
		t.Helper()
		before := writes("PATCH") + writes("POST")
		update(t, name, func(o *krm.AccessContextManagerGCPUserAccessBinding) {
			if o.Annotations == nil {
				o.Annotations = map[string]string{}
			}
			o.Annotations[k8s.InternalForceReconcileAnnotation] = time.Now().Format(time.RFC3339Nano)
		})
		time.Sleep(3 * time.Second)
		env.WaitReady(gvk, namespace, name)
		if n := writes("PATCH") + writes("POST") - before; n != 0 {
			t.Errorf("a resync of %s wrote %d times", name, n)
		}
	}
	settle := func(t *testing.T, objName, name string, ok func(*api.GcpUserAccessBinding) bool) {
		t.Helper()
		deadline := time.Now().Add(mockenv.Timeout())
		for {
			got, err := cloud(name)
			if err == nil && ok(got) {
				break
			}
			if time.Now().After(deadline) {
				t.Fatalf("the binding %s did not change: %+v, %v", name, got, err)
			}
			time.Sleep(time.Second)
		}
		env.WaitReady(gvk, namespace, objName)
	}
	newBinding := func(name, groupKey string) *krm.AccessContextManagerGCPUserAccessBinding {
		return &krm.AccessContextManagerGCPUserAccessBinding{
			ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: name, Labels: mockenv.Labels()},
			Spec: krm.AccessContextManagerGCPUserAccessBindingSpec{
				OrganizationRef: &krm.OrganizationRef{External: org},
				GroupKey:        groupKey,
				AccessLevels:    []string{level},
			},
		}
	}

	const contractors = "contractors"
	b := newBinding(contractors, "01d520gv4vjcrht")
	b.Spec.DryRunAccessLevels = []string{dryLevel}
	b.Spec.SessionSettings = &krm.GCPUserAccessBindingSessionSettings{
		SessionLength:        mockenv.Ptr("43200s"),
		SessionLengthEnabled: mockenv.Ptr(true),
		SessionReauthMethod:  mockenv.Ptr("SECURITY_KEY"),
	}
	if err := env.Kube.Create(env.Ctx, b); err != nil {
		t.Fatal(err)
	}
	var name string

	t.Run("create makes the binding and writes its ID back", func(t *testing.T) {
		u := env.WaitReady(gvk, namespace, contractors)
		name, _, _ = unstructured.NestedString(u.Object, "status", "externalRef")
		if !strings.HasPrefix(name, org+"/gcpUserAccessBindings/") {
			t.Fatalf("status.externalRef = %q", name)
		}
		if rid := get(t, contractors).Spec.ResourceID; rid == nil || org+"/gcpUserAccessBindings/"+*rid != name {
			t.Errorf("spec.resourceID = %v, want the ID of %s", rid, name)
		}
		got, err := cloud(name)
		if err != nil {
			t.Fatal(err)
		}
		if got.GroupKey != "01d520gv4vjcrht" || !slices.Equal(got.AccessLevels, []string{level}) || !slices.Equal(got.DryRunAccessLevels, []string{dryLevel}) {
			t.Errorf("cloud binding = %+v", got)
		}
		if s := got.SessionSettings; s == nil || s.SessionLength != "43200s" || !s.SessionLengthEnabled || s.SessionReauthMethod != "SECURITY_KEY" {
			t.Errorf("cloud session settings = %+v", s)
		}
	})

	t.Run("a resync writes nothing", func(t *testing.T) {
		resyncWritesNothing(t, contractors)
	})

	t.Run("scoped access settings are set, the dry run and the session taken off", func(t *testing.T) {
		update(t, contractors, func(o *krm.AccessContextManagerGCPUserAccessBinding) {
			o.Spec.DryRunAccessLevels = nil
			o.Spec.SessionSettings = nil
			o.Spec.ScopedAccessSettings = []krm.GCPUserAccessBindingScopedAccessSettings{{
				Scope: &krm.GCPUserAccessBindingAccessScope{ClientScope: &krm.GCPUserAccessBindingClientScope{
					RestrictedClientApplication: &krm.GCPUserAccessBindingApplication{Name: mockenv.Ptr("Cloud Console")},
				}},
				ActiveSettings: &krm.GCPUserAccessBindingAccessSettings{
					AccessLevels:    []string{level},
					SessionSettings: &krm.GCPUserAccessBindingSessionSettings{MaxInactivity: mockenv.Ptr("3600s"), SessionLengthEnabled: mockenv.Ptr(true), SessionLength: mockenv.Ptr("28800s")},
				},
				DryRunSettings: &krm.GCPUserAccessBindingAccessSettings{AccessLevels: []string{dryLevel}},
			}}
		})
		settle(t, contractors, name, func(got *api.GcpUserAccessBinding) bool {
			return len(got.DryRunAccessLevels) == 0 && got.SessionSettings == nil && len(got.ScopedAccessSettings) == 1
		})
		got, _ := cloud(name)
		s := got.ScopedAccessSettings[0]
		if s.Scope == nil || s.Scope.ClientScope == nil || s.Scope.ClientScope.RestrictedClientApplication == nil || s.Scope.ClientScope.RestrictedClientApplication.Name != "Cloud Console" ||
			s.ActiveSettings == nil || s.ActiveSettings.SessionSettings == nil || s.ActiveSettings.SessionSettings.MaxInactivity != "3600s" ||
			s.DryRunSettings == nil || !slices.Equal(s.DryRunSettings.AccessLevels, []string{dryLevel}) {
			t.Errorf("cloud scoped access settings = %+v", s)
		}
		resyncWritesNothing(t, contractors)
	})

	t.Run("the CRD refuses a changed group key and organization, and applications with scoped settings", func(t *testing.T) {
		o := get(t, contractors)
		o.Spec.GroupKey = "02abcdefghijklm"
		if err := env.Kube.Update(env.Ctx, o); err == nil || !strings.Contains(err.Error(), "groupKey is immutable") {
			t.Errorf("changing the group key: %v, want refused", err)
		}
		o = get(t, contractors)
		o.Spec.OrganizationRef = &krm.OrganizationRef{External: "organizations/999"}
		if err := env.Kube.Update(env.Ctx, o); err == nil || !strings.Contains(err.Error(), "organizationRef is immutable") {
			t.Errorf("changing the organization: %v, want refused", err)
		}
		o = get(t, contractors)
		o.Spec.ResourceID = mockenv.Ptr("other")
		if err := env.Kube.Update(env.Ctx, o); err == nil || !strings.Contains(err.Error(), "resourceID is immutable") {
			t.Errorf("changing the resource ID: %v, want refused", err)
		}
		bad := newBinding("bad", "03abcdefghijklm")
		bad.Spec.RestrictedClientApplications = []krm.GCPUserAccessBindingApplication{{Name: mockenv.Ptr("Cloud Console")}}
		bad.Spec.ScopedAccessSettings = []krm.GCPUserAccessBindingScopedAccessSettings{{Scope: &krm.GCPUserAccessBindingAccessScope{}}}
		if err := env.Kube.Create(env.Ctx, bad); err == nil || !strings.Contains(err.Error(), "cannot both be set") {
			t.Errorf("creating applications with scoped settings: %v, want refused", err)
		}
	})

	t.Run("a binding already in the organization is found by its group key", func(t *testing.T) {
		op, err := svc.Organizations.GcpUserAccessBindings.Create(org, &api.GcpUserAccessBinding{
			GroupKey:     "04abcdefghijklm",
			AccessLevels: []string{level},
		}).Context(env.Ctx).Do()
		if err != nil || !op.Done {
			t.Fatalf("making a binding as another tool would: %+v, %v", op, err)
		}
		creates := writes("POST")
		taken := newBinding("taken-over", "04abcdefghijklm")
		taken.Spec.RestrictedClientApplications = []krm.GCPUserAccessBindingApplication{{ClientID: mockenv.Ptr("123.apps.googleusercontent.com")}}
		if err := env.Kube.Create(env.Ctx, taken); err != nil {
			t.Fatal(err)
		}
		u := env.WaitReady(gvk, namespace, "taken-over")
		ext, _, _ := unstructured.NestedString(u.Object, "status", "externalRef")
		got, err := cloud(ext)
		if err != nil || len(got.RestrictedClientApplications) != 1 || got.RestrictedClientApplications[0].ClientId != "123.apps.googleusercontent.com" {
			t.Errorf("the binding taken over = %+v, %v", got, err)
		}
		if n := writes("POST") - creates; n != 0 {
			t.Errorf("taking the binding over made %d, want 0", n)
		}
		resyncWritesNothing(t, "taken-over")
	})

	t.Run("abandon keeps the binding; delete removes it", func(t *testing.T) {
		update(t, "taken-over", func(o *krm.AccessContextManagerGCPUserAccessBinding) {
			o.Annotations = map[string]string{"cnrm.cloud.google.com/deletion-policy": "abandon"}
		})
		ext := *get(t, "taken-over").Status.ExternalRef
		if err := env.Kube.Delete(env.Ctx, get(t, "taken-over")); err != nil {
			t.Fatal(err)
		}
		env.WaitGone(&krm.AccessContextManagerGCPUserAccessBinding{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: "taken-over"}})
		if _, err := cloud(ext); err != nil {
			t.Errorf("after abandon, the binding: %v, want it kept", err)
		}
		if err := env.Kube.Delete(env.Ctx, get(t, contractors)); err != nil {
			t.Fatal(err)
		}
		env.WaitGone(&krm.AccessContextManagerGCPUserAccessBinding{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: contractors}})
		if _, err := cloud(name); err == nil {
			t.Errorf("after delete, the binding %s is still there", name)
		}
		op, err := svc.Organizations.GcpUserAccessBindings.Delete(ext).Context(context.Background()).Do()
		if err != nil || !op.Done {
			t.Errorf("cleaning up the abandoned binding: %+v, %v", op, err)
		}
	})
}
