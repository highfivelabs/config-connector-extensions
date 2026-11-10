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

package securitycenter_test

// SecurityCenterMuteConfig end to end on the fake cloud (pkg/test/mockenv,
// patch 0030) or a real project (docs/live-runs.md). The case: a landing
// zone mutes Security Command Center findings it has accepted, a known
// public bucket in one project, a sandbox folder's findings, and an
// organization-wide finding category, each at its own level.
//
// A live run needs Security Command Center active for the project and
// securitycenter.muteconfigs.* there for the live-test service account; it
// skips the test unless E2E_SECURITY_CENTER=true. The folder's and the
// organization's mute configs run in the fake cloud only.

import (
	"os"
	"strings"
	"testing"
	"time"

	gcp "cloud.google.com/go/securitycenter/apiv1"
	pb "cloud.google.com/go/securitycenter/apiv1/securitycenterpb"
	refsv1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/apis/refs/v1beta1"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/k8s"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"

	securitycenterv1alpha1 "github.com/highfivelabs/config-connector-extensions/apis/securitycenter/v1alpha1"
	"github.com/highfivelabs/config-connector-extensions/pkg/test/mockenv"
)

const namespace = "example"

func TestSecurityCenterMuteConfig(t *testing.T) {
	if mockenv.Live() && os.Getenv("E2E_SECURITY_CENTER") != "true" {
		t.Skip("a live SecurityCenterMuteConfig needs Security Command Center active for the project and securitycenter.muteconfigs.* for the live-test service account; set E2E_SECURITY_CENTER=true where it has them (docs/live-runs.md)")
	}
	muteGVK := securitycenterv1alpha1.SecurityCenterMuteConfigGVK
	env := mockenv.New(t, namespace)
	cloud, err := gcp.NewRESTClient(env.Ctx, env.GCPOptions()...)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cloud.Close() })

	project := "projects/" + mockenv.Project
	name := func(parent, base string) string { return parent + "/muteConfigs/" + mockenv.Name(base) }
	get := func(n string) (*pb.MuteConfig, error) {
		return cloud.GetMuteConfig(env.Ctx, &pb.GetMuteConfigRequest{Name: n})
	}
	mustGet := func(t *testing.T, n string) *pb.MuteConfig {
		t.Helper()
		m, err := get(n)
		if err != nil {
			t.Fatalf("reading %s: %v", n, err)
		}
		return m
	}
	writes := func(parent string) []string {
		var out []string
		for _, r := range env.Requests() {
			if (strings.HasPrefix(r, "POST ") || strings.HasPrefix(r, "PATCH ")) && strings.Contains(r, "/v1/"+parent+"/muteConfigs") {
				out = append(out, r)
			}
		}
		return out
	}
	getObj := func(t *testing.T, n string) *securitycenterv1alpha1.SecurityCenterMuteConfig {
		t.Helper()
		obj := &securitycenterv1alpha1.SecurityCenterMuteConfig{}
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
	// Delete what the test made in the project, the abandoned mute config
	// included, and what a failed step left.
	t.Cleanup(func() {
		for _, base := range []string{"public-site", "kept"} {
			err := cloud.DeleteMuteConfig(env.Ctx, &pb.DeleteMuteConfigRequest{Name: name(project, base)})
			if err != nil && !direct.IsNotFound(err) {
				t.Errorf("cleaning up %s: %v", name(project, base), err)
			}
		}
	})

	newMute := func(n, filter string) *securitycenterv1alpha1.SecurityCenterMuteConfig {
		return &securitycenterv1alpha1.SecurityCenterMuteConfig{
			ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: n, Labels: mockenv.Labels()},
			Spec: securitycenterv1alpha1.SecurityCenterMuteConfigSpec{
				ResourceID:  mockenv.Ptr(mockenv.Name(n)),
				Filter:      mockenv.Ptr(filter),
				Description: mockenv.Ptr("accepted by the platform team"),
			},
		}
	}

	site := newMute("public-site", `category="PUBLIC_BUCKET_ACL" AND resource.name:"cce-public-site"`)
	site.Spec.ProjectRef = &refsv1beta1.ProjectRef{External: mockenv.Project}
	if err := env.Kube.Create(env.Ctx, site); err != nil {
		t.Fatal(err)
	}

	t.Run("create makes the project's mute config", func(t *testing.T) {
		u := env.WaitReady(muteGVK, namespace, "public-site")
		if ref, _, _ := unstructured.NestedString(u.Object, "status", "externalRef"); ref != name(project, "public-site") {
			t.Errorf("status.externalRef = %q, want %q", ref, name(project, "public-site"))
		}
		got := mustGet(t, name(project, "public-site"))
		if got.Filter != *site.Spec.Filter || got.Description != "accepted by the platform team" || got.Type != pb.MuteConfig_STATIC {
			t.Errorf("mute config = %v", got)
		}
		if ct, _, _ := unstructured.NestedString(u.Object, "status", "observedState", "createTime"); ct == "" {
			t.Errorf("status.observedState.createTime is empty")
		}
	})

	t.Run("a resync writes nothing", func(t *testing.T) {
		before := len(writes(project))
		o := getObj(t, "public-site")
		o.Annotations = map[string]string{k8s.InternalForceReconcileAnnotation: "1"}
		if err := env.Kube.Update(env.Ctx, o); err != nil {
			t.Fatal(err)
		}
		time.Sleep(3 * time.Second)
		env.WaitReady(muteGVK, namespace, "public-site")
		if w := writes(project)[before:]; len(w) != 0 {
			t.Errorf("a resync wrote %v", w)
		}
	})

	t.Run("a changed filter is one update of it", func(t *testing.T) {
		before := len(writes(project))
		o := getObj(t, "public-site")
		o.Spec.Filter = mockenv.Ptr(`category="PUBLIC_BUCKET_ACL"`)
		if err := env.Kube.Update(env.Ctx, o); err != nil {
			t.Fatal(err)
		}
		waitFor(t, "the filter to change", func() bool {
			m, err := get(name(project, "public-site"))
			return err == nil && m.Filter == `category="PUBLIC_BUCKET_ACL"`
		})
		env.WaitReady(muteGVK, namespace, "public-site")
		w := writes(project)[before:]
		if len(w) != 1 || !strings.Contains(w[0], "updateMask=filter") {
			t.Errorf("writes = %v, want one PATCH with updateMask=filter", w)
		}
	})

	t.Run("the CRD refuses a change of parent", func(t *testing.T) {
		o := getObj(t, "public-site")
		o.Spec.ProjectRef = &refsv1beta1.ProjectRef{External: "another-project"}
		err := env.Kube.Update(env.Ctx, o)
		if err == nil || !strings.Contains(err.Error(), "projectRef is immutable") {
			t.Errorf("changing projectRef: got %v, want the API server to refuse it", err)
		}
	})

	t.Run("the CRD refuses none or two parents", func(t *testing.T) {
		none := newMute("no-parent", `category="X"`)
		two := newMute("two-parents", `category="X"`)
		two.Spec.ProjectRef = &refsv1beta1.ProjectRef{External: mockenv.Project}
		two.Spec.FolderRef = &refsv1beta1.FolderRefDeprecated{External: "folders/123456"}
		for _, o := range []*securitycenterv1alpha1.SecurityCenterMuteConfig{none, two} {
			err := env.Kube.Create(env.Ctx, o)
			if err == nil || !strings.Contains(err.Error(), "exactly one of organizationRef, folderRef and projectRef") {
				t.Errorf("creating %s: got %v, want the API server to refuse it", o.Name, err)
			}
		}
	})

	t.Run("a folder's and an organization's mute configs", func(t *testing.T) {
		if mockenv.Live() {
			t.Skip("a live run never makes a folder's or an organization's mute config (docs/live-runs.md)")
		}
		folder := newMute("sandbox-folder", `resource.folders.resource_folder:"folders/123456"`)
		folder.Spec.FolderRef = &refsv1beta1.FolderRefDeprecated{External: "folders/123456"}
		folder.Spec.Type = mockenv.Ptr("DYNAMIC")
		org := newMute("org-category", `category="OPEN_FIREWALL"`)
		org.Spec.OrganizationRef = &refsv1beta1.OrganizationRef{External: "organizations/987654"}
		for _, o := range []*securitycenterv1alpha1.SecurityCenterMuteConfig{folder, org} {
			if err := env.Kube.Create(env.Ctx, o); err != nil {
				t.Fatal(err)
			}
		}
		for n, parent := range map[string]string{"sandbox-folder": "folders/123456", "org-category": "organizations/987654"} {
			u := env.WaitReady(muteGVK, namespace, n)
			if ref, _, _ := unstructured.NestedString(u.Object, "status", "externalRef"); ref != name(parent, n) {
				t.Errorf("%s: status.externalRef = %q, want %q", n, ref, name(parent, n))
			}
			mustGet(t, name(parent, n))
		}
		if got := mustGet(t, name("folders/123456", "sandbox-folder")); got.Type != pb.MuteConfig_DYNAMIC {
			t.Errorf("folder mute config type = %v, want DYNAMIC", got.Type)
		}
		for _, n := range []string{"sandbox-folder", "org-category"} {
			if err := env.Kube.Delete(env.Ctx, getObj(t, n)); err != nil {
				t.Fatal(err)
			}
			env.WaitGone(&securitycenterv1alpha1.SecurityCenterMuteConfig{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: n}})
		}
		if _, err := get(name("folders/123456", "sandbox-folder")); !direct.IsNotFound(err) {
			t.Errorf("after delete, reading the folder's mute config: %v, want not found", err)
		}
	})

	t.Run("delete deletes the mute config", func(t *testing.T) {
		if err := env.Kube.Delete(env.Ctx, getObj(t, "public-site")); err != nil {
			t.Fatal(err)
		}
		env.WaitGone(&securitycenterv1alpha1.SecurityCenterMuteConfig{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: "public-site"}})
		if _, err := get(name(project, "public-site")); !direct.IsNotFound(err) {
			t.Errorf("after delete, reading the mute config: %v, want not found", err)
		}
	})

	t.Run("abandon keeps the mute config", func(t *testing.T) {
		kept := newMute("kept", `category="X"`)
		kept.Spec.ProjectRef = &refsv1beta1.ProjectRef{External: mockenv.Project}
		if err := env.Kube.Create(env.Ctx, kept); err != nil {
			t.Fatal(err)
		}
		env.WaitReady(muteGVK, namespace, "kept")
		kept = getObj(t, "kept")
		kept.Annotations = map[string]string{"cnrm.cloud.google.com/deletion-policy": "abandon"}
		if err := env.Kube.Update(env.Ctx, kept); err != nil {
			t.Fatal(err)
		}
		if err := env.Kube.Delete(env.Ctx, kept); err != nil {
			t.Fatal(err)
		}
		env.WaitGone(&securitycenterv1alpha1.SecurityCenterMuteConfig{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: "kept"}})
		if _, err := get(name(project, "kept")); err != nil {
			t.Errorf("after abandon, reading the mute config: %v, want it kept", err)
		}
	})
}
