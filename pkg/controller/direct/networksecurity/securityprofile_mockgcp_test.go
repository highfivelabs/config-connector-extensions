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

// NetworkSecuritySecurityProfile end to end on the fake cloud
// (pkg/test/mockenv, patch 0026) or a real project (docs/live-runs.md).
// The case: a landing zone sets how its firewall policies treat threats and
// URLs in security profiles, the organization's for its hierarchical
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

// newThreatPreventionProfile is the object name; the profile's is
// mockenv.Name(name).
func newThreatPreventionProfile(name string) *networksecurityv1alpha1.NetworkSecuritySecurityProfile {
	return &networksecurityv1alpha1.NetworkSecuritySecurityProfile{
		ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: name, Labels: mockenv.Labels()},
		Spec: networksecurityv1alpha1.NetworkSecuritySecurityProfileSpec{
			ProjectRef:  &refsv1beta1.ProjectRef{External: mockenv.Project},
			Location:    mockenv.Ptr(location),
			ResourceID:  mockenv.Ptr(mockenv.Name(name)),
			Description: mockenv.Ptr("deny critical threats"),
			Labels:      labels(map[string]string{"team": "security"}),
			Type:        mockenv.Ptr("THREAT_PREVENTION"),
			ThreatPreventionProfile: &networksecurityv1alpha1.ThreatPreventionProfile{
				// Not in the API's order, which the comparison ignores.
				SeverityOverrides: []networksecurityv1alpha1.SeverityOverride{
					{Severity: mockenv.Ptr("LOW"), Action: mockenv.Ptr("ALERT")},
					{Severity: mockenv.Ptr("CRITICAL"), Action: mockenv.Ptr("DENY")},
				},
			},
		},
	}
}

func TestNetworkSecuritySecurityProfile(t *testing.T) {
	gvk := networksecurityv1alpha1.NetworkSecuritySecurityProfileGVK
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

	projectName := func(name string) string {
		return "projects/" + mockenv.Project + "/locations/" + location + "/securityProfiles/" + mockenv.Name(name)
	}
	cloud := func(t *testing.T, name string) (*pb.SecurityProfile, error) {
		t.Helper()
		return projects.GetSecurityProfile(env.Ctx, &pb.GetSecurityProfileRequest{Name: projectName(name)})
	}
	// Delete what the test made, the abandoned profile included, and what a
	// failed step left.
	t.Cleanup(func() {
		for _, name := range []string{"threats", "urls", "kept-profile"} {
			op, err := projects.DeleteSecurityProfile(env.Ctx, &pb.DeleteSecurityProfileRequest{Name: projectName(name)})
			if err == nil {
				err = op.Wait(env.Ctx)
			}
			if err != nil && !direct.IsNotFound(err) {
				t.Errorf("deleting security profile %s: %v", projectName(name), err)
			}
		}
	})
	patches := func(name string) int {
		n := 0
		for _, r := range env.Requests() {
			if strings.HasPrefix(r, "PATCH ") && strings.Contains(r, "/securityProfiles/"+name) {
				n++
			}
		}
		return n
	}
	getObj := func(t *testing.T, name string) *networksecurityv1alpha1.NetworkSecuritySecurityProfile {
		t.Helper()
		obj := &networksecurityv1alpha1.NetworkSecuritySecurityProfile{}
		if err := env.Kube.Get(env.Ctx, types.NamespacedName{Namespace: namespace, Name: name}, obj); err != nil {
			t.Fatal(err)
		}
		return obj
	}
	resyncWritesNothing := func(t *testing.T, name string) {
		t.Helper()
		before := patches(mockenv.Name(name))
		o := getObj(t, name)
		o.Annotations = map[string]string{k8s.InternalForceReconcileAnnotation: time.Now().String()}
		if err := env.Kube.Update(env.Ctx, o); err != nil {
			t.Fatal(err)
		}
		time.Sleep(3 * time.Second)
		env.WaitReady(gvk, namespace, name)
		if n := patches(mockenv.Name(name)) - before; n != 0 {
			t.Errorf("a resync patched the security profile %d times", n)
		}
	}

	if err := env.Kube.Create(env.Ctx, newThreatPreventionProfile("threats")); err != nil {
		t.Fatal(err)
	}

	t.Run("create makes the project's security profile", func(t *testing.T) {
		u := env.WaitReady(gvk, namespace, "threats")
		if ref, _, _ := unstructured.NestedString(u.Object, "status", "externalRef"); ref != projectName("threats") {
			t.Errorf("status.externalRef = %q, want %q", ref, projectName("threats"))
		}
		if created, _, _ := unstructured.NestedString(u.Object, "status", "observedState", "createTime"); created == "" {
			t.Errorf("status.observedState.createTime is empty")
		}
		got, err := cloud(t, "threats")
		if err != nil {
			t.Fatalf("reading the security profile: %v", err)
		}
		if got.GetType() != pb.SecurityProfile_THREAT_PREVENTION || got.GetDescription() != "deny critical threats" {
			t.Errorf("cloud type, description = %v, %q", got.GetType(), got.GetDescription())
		}
		overrides := map[string]string{}
		for _, o := range got.GetThreatPreventionProfile().GetSeverityOverrides() {
			overrides[o.GetSeverity().String()] = o.GetAction().String()
		}
		if overrides["CRITICAL"] != "DENY" || overrides["LOW"] != "ALERT" || len(overrides) != 2 {
			t.Errorf("cloud severity overrides = %v", overrides)
		}
		if got.GetLabels()["team"] != "security" {
			t.Errorf("cloud labels = %v", got.GetLabels())
		}
	})

	t.Run("a resync writes nothing", func(t *testing.T) {
		resyncWritesNothing(t, "threats")
	})

	t.Run("a changed override is patched once", func(t *testing.T) {
		before := patches(mockenv.Name("threats"))
		o := getObj(t, "threats")
		o.Spec.Description = mockenv.Ptr("deny critical and high threats")
		o.Spec.ThreatPreventionProfile.SeverityOverrides = append(o.Spec.ThreatPreventionProfile.SeverityOverrides,
			networksecurityv1alpha1.SeverityOverride{Severity: mockenv.Ptr("HIGH"), Action: mockenv.Ptr("DENY")})
		if err := env.Kube.Update(env.Ctx, o); err != nil {
			t.Fatal(err)
		}
		deadline := time.Now().Add(mockenv.Timeout())
		for {
			got, err := cloud(t, "threats")
			if err != nil {
				t.Fatal(err)
			}
			if len(got.GetThreatPreventionProfile().GetSeverityOverrides()) == 3 && got.GetDescription() == "deny critical and high threats" {
				break
			}
			if time.Now().After(deadline) {
				t.Fatalf("cloud overrides, description = %v, %q", got.GetThreatPreventionProfile().GetSeverityOverrides(), got.GetDescription())
			}
			time.Sleep(time.Second)
		}
		env.WaitReady(gvk, namespace, "threats")
		if n := patches(mockenv.Name("threats")) - before; n != 1 {
			t.Errorf("the change patched the security profile %d times, want 1", n)
		}
	})

	t.Run("type is immutable", func(t *testing.T) {
		o := getObj(t, "threats")
		o.Spec.Type = mockenv.Ptr("URL_FILTERING")
		err := env.Kube.Update(env.Ctx, o)
		if err == nil || !strings.Contains(err.Error(), "type is immutable") {
			t.Errorf("changing type: got %v, want the API server to refuse it", err)
		}
	})

	t.Run("exactly one parent and at most one profile", func(t *testing.T) {
		for name, c := range map[string]struct {
			mutate func(*networksecurityv1alpha1.NetworkSecuritySecurityProfile)
			want   string
		}{
			"both": {func(o *networksecurityv1alpha1.NetworkSecuritySecurityProfile) {
				o.Spec.OrganizationRef = &refsv1beta1.OrganizationRef{External: org}
			}, "exactly one of organizationRef and projectRef"},
			"neither": {func(o *networksecurityv1alpha1.NetworkSecuritySecurityProfile) { o.Spec.ProjectRef = nil },
				"exactly one of organizationRef and projectRef"},
			"two-profiles": {func(o *networksecurityv1alpha1.NetworkSecuritySecurityProfile) {
				o.Spec.URLFilteringProfile = &networksecurityv1alpha1.URLFilteringProfile{}
			}, "at most one of threatPreventionProfile"},
		} {
			o := newThreatPreventionProfile(name)
			c.mutate(o)
			err := env.Kube.Create(env.Ctx, o)
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Errorf("creating with %s: got %v, want the API server to refuse it with %q", name, err, c.want)
			}
		}
	})

	t.Run("a URL filtering profile, without the API's default filter", func(t *testing.T) {
		o := &networksecurityv1alpha1.NetworkSecuritySecurityProfile{
			ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: "urls", Labels: mockenv.Labels()},
			Spec: networksecurityv1alpha1.NetworkSecuritySecurityProfileSpec{
				ProjectRef: &refsv1beta1.ProjectRef{External: mockenv.Project},
				Location:   mockenv.Ptr(location),
				ResourceID: mockenv.Ptr(mockenv.Name("urls")),
				Labels:     labels(nil),
				Type:       mockenv.Ptr("URL_FILTERING"),
				URLFilteringProfile: &networksecurityv1alpha1.URLFilteringProfile{
					URLFilters: []networksecurityv1alpha1.URLFilter{
						{FilteringAction: mockenv.Ptr("DENY"), Urls: []string{"example.com"}, Priority: mockenv.Ptr(int32(20))},
						{FilteringAction: mockenv.Ptr("ALLOW"), Urls: []string{"www.example.org"}, Priority: mockenv.Ptr(int32(10))},
					},
				},
			},
		}
		if err := env.Kube.Create(env.Ctx, o); err != nil {
			t.Fatal(err)
		}
		env.WaitReady(gvk, namespace, "urls")
		got, err := cloud(t, "urls")
		if err != nil {
			t.Fatalf("reading the security profile: %v", err)
		}
		priorities := map[int32]string{}
		for _, f := range got.GetUrlFilteringProfile().GetUrlFilters() {
			priorities[f.GetPriority()] = f.GetFilteringAction().String()
		}
		if priorities[10] != "ALLOW" || priorities[20] != "DENY" {
			t.Errorf("cloud URL filters = %v", got.GetUrlFilteringProfile().GetUrlFilters())
		}
		// Live, the API adds a filter of priority 2147483647; it is not a
		// difference, so a resync writes nothing.
		resyncWritesNothing(t, "urls")
		if err := env.Kube.Delete(env.Ctx, getObj(t, "urls")); err != nil {
			t.Fatal(err)
		}
		env.WaitGone(&networksecurityv1alpha1.NetworkSecuritySecurityProfile{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: "urls"}})
	})

	t.Run("the organization's security profile", func(t *testing.T) {
		if mockenv.Live() {
			t.Skip("a live run never touches an organization (docs/live-runs.md)")
		}
		o := &networksecurityv1alpha1.NetworkSecuritySecurityProfile{
			ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: "corp-threats"},
			Spec: networksecurityv1alpha1.NetworkSecuritySecurityProfileSpec{
				OrganizationRef: &refsv1beta1.OrganizationRef{External: org},
				Location:        mockenv.Ptr(location),
				Type:            mockenv.Ptr("THREAT_PREVENTION"),
				ThreatPreventionProfile: &networksecurityv1alpha1.ThreatPreventionProfile{
					AntivirusOverrides: []networksecurityv1alpha1.AntivirusOverride{
						{Protocol: mockenv.Ptr("SMTP"), Action: mockenv.Ptr("DENY")},
					},
				},
			},
		}
		if err := env.Kube.Create(env.Ctx, o); err != nil {
			t.Fatal(err)
		}
		orgName := org + "/locations/" + location + "/securityProfiles/corp-threats"
		u := env.WaitReady(gvk, namespace, "corp-threats")
		if ref, _, _ := unstructured.NestedString(u.Object, "status", "externalRef"); ref != orgName {
			t.Errorf("status.externalRef = %q, want %q (the name is metadata.name without resourceID)", ref, orgName)
		}
		got, err := orgs.GetSecurityProfile(env.Ctx, &pb.GetSecurityProfileRequest{Name: orgName})
		if err != nil {
			t.Fatalf("reading the organization's security profile: %v", err)
		}
		if av := got.GetThreatPreventionProfile().GetAntivirusOverrides(); len(av) != 1 || av[0].GetProtocol() != pb.Protocol_SMTP {
			t.Errorf("cloud antivirus overrides = %v", av)
		}
		created := 0
		for _, r := range env.Requests() {
			if strings.HasPrefix(r, "POST ") && strings.Contains(r, "/v1/"+org+"/locations/"+location+"/securityProfiles") {
				created++
			}
		}
		if created != 1 {
			t.Errorf("POST to the organization's security profiles %d times, want 1", created)
		}

		o = getObj(t, "corp-threats")
		o.Spec.Description = mockenv.Ptr("the organization's")
		if err := env.Kube.Update(env.Ctx, o); err != nil {
			t.Fatal(err)
		}
		deadline := time.Now().Add(mockenv.Timeout())
		for {
			got, err := orgs.GetSecurityProfile(env.Ctx, &pb.GetSecurityProfileRequest{Name: orgName})
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

		if err := env.Kube.Delete(env.Ctx, getObj(t, "corp-threats")); err != nil {
			t.Fatal(err)
		}
		env.WaitGone(&networksecurityv1alpha1.NetworkSecuritySecurityProfile{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: "corp-threats"}})
		if _, err := orgs.GetSecurityProfile(env.Ctx, &pb.GetSecurityProfileRequest{Name: orgName}); !direct.IsNotFound(err) {
			t.Errorf("after delete, reading the organization's security profile: %v, want not found", err)
		}
	})

	t.Run("a changed parent is refused", func(t *testing.T) {
		if mockenv.Live() {
			t.Skip("the new parent would be an organization")
		}
		o := getObj(t, "threats")
		o.Spec.ProjectRef = nil
		o.Spec.OrganizationRef = &refsv1beta1.OrganizationRef{External: org}
		if err := env.Kube.Update(env.Ctx, o); err != nil {
			t.Fatal(err)
		}
		if msg := env.WaitNotReady(gvk, namespace, "threats"); !strings.Contains(msg, "cannot change NetworkSecuritySecurityProfile identity") {
			t.Errorf("Ready message = %q, want the change refused", msg)
		}
		o = getObj(t, "threats")
		o.Spec.OrganizationRef = nil
		o.Spec.ProjectRef = &refsv1beta1.ProjectRef{External: mockenv.Project}
		if err := env.Kube.Update(env.Ctx, o); err != nil {
			t.Fatal(err)
		}
		env.WaitReady(gvk, namespace, "threats")
	})

	t.Run("delete deletes the security profile", func(t *testing.T) {
		if err := env.Kube.Delete(env.Ctx, getObj(t, "threats")); err != nil {
			t.Fatal(err)
		}
		env.WaitGone(&networksecurityv1alpha1.NetworkSecuritySecurityProfile{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: "threats"}})
		if _, err := cloud(t, "threats"); !direct.IsNotFound(err) {
			t.Errorf("after delete, reading the security profile: %v, want not found", err)
		}
	})

	t.Run("abandon keeps the security profile", func(t *testing.T) {
		kept := newThreatPreventionProfile("kept-profile")
		kept.Annotations = map[string]string{"cnrm.cloud.google.com/deletion-policy": "abandon"}
		if err := env.Kube.Create(env.Ctx, kept); err != nil {
			t.Fatal(err)
		}
		env.WaitReady(gvk, namespace, "kept-profile")
		if err := env.Kube.Delete(env.Ctx, getObj(t, "kept-profile")); err != nil {
			t.Fatal(err)
		}
		env.WaitGone(&networksecurityv1alpha1.NetworkSecuritySecurityProfile{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: "kept-profile"}})
		if _, err := cloud(t, "kept-profile"); err != nil {
			t.Errorf("after abandon, reading the security profile: %v, want it kept", err)
		}
	})
}
