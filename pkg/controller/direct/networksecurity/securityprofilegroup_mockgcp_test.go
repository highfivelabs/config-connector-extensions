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

package networksecurity_test

// NetworkSecuritySecurityProfileGroup end to end on the fake cloud
// (pkg/test/mockenv, patch 0026) or a real project (docs/live-runs.md).
// The case: a firewall policy rule applies a security profile group, which
// names the profiles to use, the organization's for its hierarchical
// firewall policies and a project's for that project's network firewall
// policies. A live run makes only a project's (in global); it never
// touches an organization, whose case runs in the fake cloud only.

import (
	"strings"
	"testing"
	"time"

	gcp "cloud.google.com/go/networksecurity/apiv1"
	pb "cloud.google.com/go/networksecurity/apiv1/networksecuritypb"
	refsv1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/apis/refs/v1beta1"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/k8s"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"

	networksecurityv1alpha1 "github.com/highfivelabs/config-connector-extensions/apis/networksecurity/v1alpha1"
	"github.com/highfivelabs/config-connector-extensions/pkg/test/mockenv"
)

func TestNetworkSecuritySecurityProfileGroup(t *testing.T) {
	gvk := networksecurityv1alpha1.NetworkSecuritySecurityProfileGroupGVK
	profileGVK := networksecurityv1alpha1.NetworkSecuritySecurityProfileGVK
	env := mockenv.New(t, namespace)
	projects, err := gcp.NewSecurityProfileGroupRESTClient(env.Ctx, env.GCPOptions()...)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { projects.Close() })
	orgs, err := gcp.NewOrganizationSecurityProfileGroupRESTClient(env.Ctx, env.GCPOptions()...)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { orgs.Close() })

	parent := "projects/" + mockenv.Project + "/locations/" + location
	groupName := func(name string) string { return parent + "/securityProfileGroups/" + mockenv.Name(name) }
	profileName := func(name string) string { return parent + "/securityProfiles/" + mockenv.Name(name) }
	cloud := func(t *testing.T, name string) (*pb.SecurityProfileGroup, error) {
		t.Helper()
		return projects.GetSecurityProfileGroup(env.Ctx, &pb.GetSecurityProfileGroupRequest{Name: groupName(name)})
	}
	// Delete what the test made, groups first (they hold the profiles), the
	// abandoned group included, and what a failed step left.
	t.Cleanup(func() {
		for _, name := range []string{"spg", "kept-spg"} {
			op, err := projects.DeleteSecurityProfileGroup(env.Ctx, &pb.DeleteSecurityProfileGroupRequest{Name: groupName(name)})
			if err == nil {
				err = op.Wait(env.Ctx)
			}
			if err != nil && !direct.IsNotFound(err) {
				t.Errorf("deleting security profile group %s: %v", groupName(name), err)
			}
		}
		for _, name := range []string{"spg-threats", "spg-urls"} {
			op, err := projects.DeleteSecurityProfile(env.Ctx, &pb.DeleteSecurityProfileRequest{Name: profileName(name)})
			if err == nil {
				err = op.Wait(env.Ctx)
			}
			if err != nil && !direct.IsNotFound(err) {
				t.Errorf("deleting security profile %s: %v", profileName(name), err)
			}
		}
	})
	patches := func(name string) int {
		n := 0
		for _, r := range env.Requests() {
			if strings.HasPrefix(r, "PATCH ") && strings.Contains(r, "/securityProfileGroups/"+name) {
				n++
			}
		}
		return n
	}
	getObj := func(t *testing.T, name string) *networksecurityv1alpha1.NetworkSecuritySecurityProfileGroup {
		t.Helper()
		obj := &networksecurityv1alpha1.NetworkSecuritySecurityProfileGroup{}
		if err := env.Kube.Get(env.Ctx, types.NamespacedName{Namespace: namespace, Name: name}, obj); err != nil {
			t.Fatal(err)
		}
		return obj
	}
	newGroup := func(name string) *networksecurityv1alpha1.NetworkSecuritySecurityProfileGroup {
		return &networksecurityv1alpha1.NetworkSecuritySecurityProfileGroup{
			ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: name, Labels: mockenv.Labels()},
			Spec: networksecurityv1alpha1.NetworkSecuritySecurityProfileGroupSpec{
				ProjectRef:                 &refsv1beta1.ProjectRef{External: mockenv.Project},
				Location:                   mockenv.Ptr(location),
				ResourceID:                 mockenv.Ptr(mockenv.Name(name)),
				Description:                mockenv.Ptr("inspect egress"),
				Labels:                     labels(map[string]string{"team": "security"}),
				ThreatPreventionProfileRef: &networksecurityv1alpha1.NetworkSecuritySecurityProfileRef{Name: "spg-threats"},
			},
		}
	}

	// The profiles the groups name, made through our profile kind.
	threats := newThreatPreventionProfile("spg-threats")
	urls := newThreatPreventionProfile("spg-urls")
	urls.Spec.Type = mockenv.Ptr("URL_FILTERING")
	urls.Spec.ThreatPreventionProfile = nil
	urls.Spec.URLFilteringProfile = &networksecurityv1alpha1.URLFilteringProfile{
		URLFilters: []networksecurityv1alpha1.URLFilter{
			{FilteringAction: mockenv.Ptr("DENY"), Urls: []string{"example.com"}, Priority: mockenv.Ptr(int32(10))},
		},
	}
	for _, p := range []*networksecurityv1alpha1.NetworkSecuritySecurityProfile{threats, urls} {
		if err := env.Kube.Create(env.Ctx, p); err != nil {
			t.Fatal(err)
		}
	}
	env.WaitReady(profileGVK, namespace, "spg-threats")
	env.WaitReady(profileGVK, namespace, "spg-urls")

	if err := env.Kube.Create(env.Ctx, newGroup("spg")); err != nil {
		t.Fatal(err)
	}

	t.Run("create makes the project's group with the profile it names", func(t *testing.T) {
		u := env.WaitReady(gvk, namespace, "spg")
		if ref, _, _ := unstructured.NestedString(u.Object, "status", "externalRef"); ref != groupName("spg") {
			t.Errorf("status.externalRef = %q, want %q", ref, groupName("spg"))
		}
		if created, _, _ := unstructured.NestedString(u.Object, "status", "observedState", "createTime"); created == "" {
			t.Errorf("status.observedState.createTime is empty")
		}
		got, err := cloud(t, "spg")
		if err != nil {
			t.Fatalf("reading the group: %v", err)
		}
		if got.GetThreatPreventionProfile() != profileName("spg-threats") || got.GetDescription() != "inspect egress" {
			t.Errorf("cloud threat prevention profile, description = %q, %q; want %q", got.GetThreatPreventionProfile(), got.GetDescription(), profileName("spg-threats"))
		}
		if got.GetLabels()["team"] != "security" {
			t.Errorf("cloud labels = %v", got.GetLabels())
		}
	})

	t.Run("a resync writes nothing", func(t *testing.T) {
		before := patches(mockenv.Name("spg"))
		o := getObj(t, "spg")
		o.Annotations = map[string]string{k8s.InternalForceReconcileAnnotation: "1"}
		if err := env.Kube.Update(env.Ctx, o); err != nil {
			t.Fatal(err)
		}
		time.Sleep(3 * time.Second)
		env.WaitReady(gvk, namespace, "spg")
		if n := patches(mockenv.Name("spg")) - before; n != 0 {
			t.Errorf("a resync patched the group %d times", n)
		}
	})

	t.Run("a profile added is patched once", func(t *testing.T) {
		before := patches(mockenv.Name("spg"))
		o := getObj(t, "spg")
		o.Spec.URLFilteringProfileRef = &networksecurityv1alpha1.NetworkSecuritySecurityProfileRef{External: profileName("spg-urls")}
		if err := env.Kube.Update(env.Ctx, o); err != nil {
			t.Fatal(err)
		}
		deadline := time.Now().Add(mockenv.Timeout())
		for {
			got, err := cloud(t, "spg")
			if err != nil {
				t.Fatal(err)
			}
			if got.GetUrlFilteringProfile() == profileName("spg-urls") && got.GetThreatPreventionProfile() == profileName("spg-threats") {
				break
			}
			if time.Now().After(deadline) {
				t.Fatalf("cloud profiles = %q, %q", got.GetThreatPreventionProfile(), got.GetUrlFilteringProfile())
			}
			time.Sleep(time.Second)
		}
		env.WaitReady(gvk, namespace, "spg")
		if n := patches(mockenv.Name("spg")) - before; n != 1 {
			t.Errorf("the change patched the group %d times, want 1", n)
		}
	})

	t.Run("exactly one parent", func(t *testing.T) {
		for name, mutate := range map[string]func(*networksecurityv1alpha1.NetworkSecuritySecurityProfileGroup){
			"both-parents": func(o *networksecurityv1alpha1.NetworkSecuritySecurityProfileGroup) {
				o.Spec.OrganizationRef = &refsv1beta1.OrganizationRef{External: org}
			},
			"no-parent": func(o *networksecurityv1alpha1.NetworkSecuritySecurityProfileGroup) { o.Spec.ProjectRef = nil },
		} {
			o := newGroup(name)
			mutate(o)
			err := env.Kube.Create(env.Ctx, o)
			if err == nil || !strings.Contains(err.Error(), "exactly one of organizationRef and projectRef") {
				t.Errorf("creating with %s: got %v, want the API server to refuse it", name, err)
			}
		}
	})

	t.Run("the organization's group names the organization's profile", func(t *testing.T) {
		if mockenv.Live() {
			t.Skip("a live run never touches an organization (docs/live-runs.md)")
		}
		p := &networksecurityv1alpha1.NetworkSecuritySecurityProfile{
			ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: "corp-spg-threats"},
			Spec: networksecurityv1alpha1.NetworkSecuritySecurityProfileSpec{
				OrganizationRef:         &refsv1beta1.OrganizationRef{External: org},
				Location:                mockenv.Ptr(location),
				Type:                    mockenv.Ptr("THREAT_PREVENTION"),
				ThreatPreventionProfile: &networksecurityv1alpha1.ThreatPreventionProfile{},
			},
		}
		if err := env.Kube.Create(env.Ctx, p); err != nil {
			t.Fatal(err)
		}
		env.WaitReady(profileGVK, namespace, "corp-spg-threats")
		o := &networksecurityv1alpha1.NetworkSecuritySecurityProfileGroup{
			ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: "corp-spg"},
			Spec: networksecurityv1alpha1.NetworkSecuritySecurityProfileGroupSpec{
				OrganizationRef:            &refsv1beta1.OrganizationRef{External: org},
				Location:                   mockenv.Ptr(location),
				ThreatPreventionProfileRef: &networksecurityv1alpha1.NetworkSecuritySecurityProfileRef{Name: "corp-spg-threats"},
			},
		}
		if err := env.Kube.Create(env.Ctx, o); err != nil {
			t.Fatal(err)
		}
		orgName := org + "/locations/" + location + "/securityProfileGroups/corp-spg"
		orgProfile := org + "/locations/" + location + "/securityProfiles/corp-spg-threats"
		u := env.WaitReady(gvk, namespace, "corp-spg")
		if ref, _, _ := unstructured.NestedString(u.Object, "status", "externalRef"); ref != orgName {
			t.Errorf("status.externalRef = %q, want %q", ref, orgName)
		}
		got, err := orgs.GetSecurityProfileGroup(env.Ctx, &pb.GetSecurityProfileGroupRequest{Name: orgName})
		if err != nil {
			t.Fatalf("reading the organization's group: %v", err)
		}
		if got.GetThreatPreventionProfile() != orgProfile {
			t.Errorf("cloud threat prevention profile = %q, want %q", got.GetThreatPreventionProfile(), orgProfile)
		}

		o = getObj(t, "corp-spg")
		o.Spec.Description = mockenv.Ptr("the organization's")
		if err := env.Kube.Update(env.Ctx, o); err != nil {
			t.Fatal(err)
		}
		deadline := time.Now().Add(mockenv.Timeout())
		for {
			got, err := orgs.GetSecurityProfileGroup(env.Ctx, &pb.GetSecurityProfileGroupRequest{Name: orgName})
			if err != nil {
				t.Fatal(err)
			}
			if got.GetDescription() == "the organization's" {
				break
			}
			if time.Now().After(deadline) {
				t.Fatalf("cloud description = %q after the update", got.GetDescription())
			}
			time.Sleep(time.Second)
		}

		if err := env.Kube.Delete(env.Ctx, getObj(t, "corp-spg")); err != nil {
			t.Fatal(err)
		}
		env.WaitGone(&networksecurityv1alpha1.NetworkSecuritySecurityProfileGroup{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: "corp-spg"}})
		if _, err := orgs.GetSecurityProfileGroup(env.Ctx, &pb.GetSecurityProfileGroupRequest{Name: orgName}); !direct.IsNotFound(err) {
			t.Errorf("after delete, reading the organization's group: %v, want not found", err)
		}
		if err := env.Kube.Delete(env.Ctx, p); err != nil {
			t.Fatal(err)
		}
		env.WaitGone(&networksecurityv1alpha1.NetworkSecuritySecurityProfile{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: "corp-spg-threats"}})
	})

	t.Run("a changed parent is refused", func(t *testing.T) {
		if mockenv.Live() {
			t.Skip("the new parent would be an organization")
		}
		o := getObj(t, "spg")
		o.Spec.ProjectRef = nil
		o.Spec.OrganizationRef = &refsv1beta1.OrganizationRef{External: org}
		if err := env.Kube.Update(env.Ctx, o); err != nil {
			t.Fatal(err)
		}
		if msg := env.WaitNotReady(gvk, namespace, "spg"); !strings.Contains(msg, "cannot change NetworkSecuritySecurityProfileGroup identity") {
			t.Errorf("Ready message = %q, want the change refused", msg)
		}
		o = getObj(t, "spg")
		o.Spec.OrganizationRef = nil
		o.Spec.ProjectRef = &refsv1beta1.ProjectRef{External: mockenv.Project}
		if err := env.Kube.Update(env.Ctx, o); err != nil {
			t.Fatal(err)
		}
		env.WaitReady(gvk, namespace, "spg")
	})

	t.Run("delete deletes the group", func(t *testing.T) {
		if err := env.Kube.Delete(env.Ctx, getObj(t, "spg")); err != nil {
			t.Fatal(err)
		}
		env.WaitGone(&networksecurityv1alpha1.NetworkSecuritySecurityProfileGroup{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: "spg"}})
		if _, err := cloud(t, "spg"); !direct.IsNotFound(err) {
			t.Errorf("after delete, reading the group: %v, want not found", err)
		}
	})

	t.Run("abandon keeps the group", func(t *testing.T) {
		kept := newGroup("kept-spg")
		kept.Annotations = map[string]string{"cnrm.cloud.google.com/deletion-policy": "abandon"}
		if err := env.Kube.Create(env.Ctx, kept); err != nil {
			t.Fatal(err)
		}
		env.WaitReady(gvk, namespace, "kept-spg")
		if err := env.Kube.Delete(env.Ctx, getObj(t, "kept-spg")); err != nil {
			t.Fatal(err)
		}
		env.WaitGone(&networksecurityv1alpha1.NetworkSecuritySecurityProfileGroup{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: "kept-spg"}})
		if _, err := cloud(t, "kept-spg"); err != nil {
			t.Errorf("after abandon, reading the group: %v, want it kept", err)
		}
	})

	// The profiles go after the groups that name them; the cleanup above
	// deletes the abandoned group first.
	t.Run("the profiles are deleted", func(t *testing.T) {
		op, err := projects.DeleteSecurityProfileGroup(env.Ctx, &pb.DeleteSecurityProfileGroupRequest{Name: groupName("kept-spg")})
		if err == nil {
			err = op.Wait(env.Ctx)
		}
		if err != nil && !direct.IsNotFound(err) {
			t.Fatalf("deleting the abandoned group: %v", err)
		}
		for _, p := range []*networksecurityv1alpha1.NetworkSecuritySecurityProfile{threats, urls} {
			if err := env.Kube.Delete(env.Ctx, p); err != nil {
				t.Fatal(err)
			}
			env.WaitGone(&networksecurityv1alpha1.NetworkSecuritySecurityProfile{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: p.Name}})
		}
	})
}
