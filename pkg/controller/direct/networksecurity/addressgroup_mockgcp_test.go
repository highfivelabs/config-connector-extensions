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

// NetworkSecurityAddressGroup end to end on the fake cloud
// (pkg/test/mockenv, patch 0025) or a real project (docs/live-runs.md).
// The case: a landing zone keeps the ranges its firewall rules allow in
// address groups, the organization's for its hierarchical firewall
// policies and a project's for that project's network firewall policies.
// A live run makes only the project's (in global); it never touches an
// organization, whose case runs in the fake cloud only.

import (
	"maps"
	"slices"
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

const (
	namespace = "example"
	location  = "global"
	org       = "organizations/987654"
)

var gvk = networksecurityv1alpha1.NetworkSecurityAddressGroupGVK

// labels are the group's own labels plus, live, the run's (the sweeper
// finds address groups by them).
func labels(extra map[string]string) map[string]string {
	out := map[string]string{}
	maps.Copy(out, extra)
	maps.Copy(out, mockenv.Labels())
	return out
}

// newProjectAddressGroup is the object name; the group's is mockenv.Name(name).
func newProjectAddressGroup(name string, items ...string) *networksecurityv1alpha1.NetworkSecurityAddressGroup {
	return &networksecurityv1alpha1.NetworkSecurityAddressGroup{
		ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: name, Labels: mockenv.Labels()},
		Spec: networksecurityv1alpha1.NetworkSecurityAddressGroupSpec{
			ProjectRef:  &refsv1beta1.ProjectRef{External: mockenv.Project},
			Location:    mockenv.Ptr(location),
			ResourceID:  mockenv.Ptr(mockenv.Name(name)),
			Description: mockenv.Ptr("on-premises ranges"),
			Labels:      labels(map[string]string{"team": "network"}),
			Type:        mockenv.Ptr("IPV4"),
			Capacity:    mockenv.Ptr(int32(10)),
			Items:       items,
		},
	}
}

func TestNetworkSecurityAddressGroup(t *testing.T) {
	env := mockenv.New(t, namespace)
	projects, err := gcp.NewAddressGroupRESTClient(env.Ctx, env.GCPOptions()...)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { projects.Close() })
	orgs, err := gcp.NewOrganizationAddressGroupRESTClient(env.Ctx, env.GCPOptions()...)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { orgs.Close() })

	projectName := func(name string) string {
		return "projects/" + mockenv.Project + "/locations/" + location + "/addressGroups/" + mockenv.Name(name)
	}
	cloud := func(t *testing.T, name string) (*pb.AddressGroup, error) {
		t.Helper()
		return projects.GetAddressGroup(env.Ctx, &pb.GetAddressGroupRequest{Name: projectName(name)})
	}
	// Delete what the test made, the abandoned group included, and what a
	// failed step left.
	t.Cleanup(func() {
		for _, name := range []string{"on-prem", "kept"} {
			op, err := projects.DeleteAddressGroup(env.Ctx, &pb.DeleteAddressGroupRequest{Name: projectName(name)})
			if err == nil {
				err = op.Wait(env.Ctx)
			}
			if err != nil && !direct.IsNotFound(err) {
				t.Errorf("deleting address group %s: %v", projectName(name), err)
			}
		}
	})
	patches := func(name string) int {
		n := 0
		for _, r := range env.Requests() {
			if strings.HasPrefix(r, "PATCH ") && strings.Contains(r, "/addressGroups/"+name) {
				n++
			}
		}
		return n
	}
	getObj := func(t *testing.T, name string) *networksecurityv1alpha1.NetworkSecurityAddressGroup {
		t.Helper()
		obj := &networksecurityv1alpha1.NetworkSecurityAddressGroup{}
		if err := env.Kube.Get(env.Ctx, types.NamespacedName{Namespace: namespace, Name: name}, obj); err != nil {
			t.Fatal(err)
		}
		return obj
	}

	if err := env.Kube.Create(env.Ctx, newProjectAddressGroup("on-prem", "10.0.0.0/8", "192.168.0.0/16")); err != nil {
		t.Fatal(err)
	}

	t.Run("create makes the project's address group", func(t *testing.T) {
		u := env.WaitReady(gvk, namespace, "on-prem")
		if ref, _, _ := unstructured.NestedString(u.Object, "status", "externalRef"); ref != projectName("on-prem") {
			t.Errorf("status.externalRef = %q, want %q", ref, projectName("on-prem"))
		}
		if created, _, _ := unstructured.NestedString(u.Object, "status", "observedState", "createTime"); created == "" {
			t.Errorf("status.observedState.createTime is empty")
		}
		got, err := cloud(t, "on-prem")
		if err != nil {
			t.Fatalf("reading the address group: %v", err)
		}
		if got.GetType() != pb.AddressGroup_IPV4 || got.GetCapacity() != 10 || got.GetDescription() != "on-premises ranges" {
			t.Errorf("cloud type, capacity, description = %v, %d, %q", got.GetType(), got.GetCapacity(), got.GetDescription())
		}
		if items := slices.Sorted(slices.Values(got.GetItems())); !slices.Equal(items, []string{"10.0.0.0/8", "192.168.0.0/16"}) {
			t.Errorf("cloud items = %v", items)
		}
		if got.GetLabels()["team"] != "network" {
			t.Errorf("cloud labels = %v", got.GetLabels())
		}
	})

	t.Run("a resync writes nothing", func(t *testing.T) {
		before := patches(mockenv.Name("on-prem"))
		o := getObj(t, "on-prem")
		o.Annotations = map[string]string{k8s.InternalForceReconcileAnnotation: "1"}
		if err := env.Kube.Update(env.Ctx, o); err != nil {
			t.Fatal(err)
		}
		time.Sleep(3 * time.Second)
		env.WaitReady(gvk, namespace, "on-prem")
		if n := patches(mockenv.Name("on-prem")) - before; n != 0 {
			t.Errorf("a resync patched the address group %d times", n)
		}
	})

	t.Run("an item added and one removed are patched", func(t *testing.T) {
		before := patches(mockenv.Name("on-prem"))
		o := getObj(t, "on-prem")
		o.Spec.Items = []string{"10.0.0.0/8", "172.16.0.0/12"}
		o.Spec.Description = mockenv.Ptr("on-premises and partner ranges")
		if err := env.Kube.Update(env.Ctx, o); err != nil {
			t.Fatal(err)
		}
		deadline := time.Now().Add(mockenv.Timeout())
		for {
			got, err := cloud(t, "on-prem")
			if err != nil {
				t.Fatal(err)
			}
			items := slices.Sorted(slices.Values(got.GetItems()))
			if slices.Equal(items, []string{"10.0.0.0/8", "172.16.0.0/12"}) && got.GetDescription() == "on-premises and partner ranges" {
				break
			}
			if time.Now().After(deadline) {
				t.Fatalf("cloud items, description = %v, %q", items, got.GetDescription())
			}
			time.Sleep(time.Second)
		}
		env.WaitReady(gvk, namespace, "on-prem")
		if n := patches(mockenv.Name("on-prem")) - before; n != 1 {
			t.Errorf("the change patched the address group %d times, want 1", n)
		}
	})

	t.Run("capacity is immutable", func(t *testing.T) {
		o := getObj(t, "on-prem")
		o.Spec.Capacity = mockenv.Ptr(int32(20))
		err := env.Kube.Update(env.Ctx, o)
		if err == nil || !strings.Contains(err.Error(), "capacity is immutable") {
			t.Errorf("changing capacity: got %v, want the API server to refuse it", err)
		}
	})

	t.Run("exactly one parent", func(t *testing.T) {
		for name, mutate := range map[string]func(*networksecurityv1alpha1.NetworkSecurityAddressGroup){
			"both": func(o *networksecurityv1alpha1.NetworkSecurityAddressGroup) {
				o.Spec.OrganizationRef = &refsv1beta1.OrganizationRef{External: org}
			},
			"neither": func(o *networksecurityv1alpha1.NetworkSecurityAddressGroup) { o.Spec.ProjectRef = nil },
		} {
			o := newProjectAddressGroup(name, "10.0.0.0/8")
			mutate(o)
			err := env.Kube.Create(env.Ctx, o)
			if err == nil || !strings.Contains(err.Error(), "exactly one of organizationRef and projectRef") {
				t.Errorf("creating with %s: got %v, want the API server to refuse it", name, err)
			}
		}
	})

	t.Run("the organization's address group", func(t *testing.T) {
		if mockenv.Live() {
			t.Skip("a live run never touches an organization (docs/live-runs.md)")
		}
		o := &networksecurityv1alpha1.NetworkSecurityAddressGroup{
			ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: "corp-egress"},
			Spec: networksecurityv1alpha1.NetworkSecurityAddressGroupSpec{
				OrganizationRef: &refsv1beta1.OrganizationRef{External: org},
				Location:        mockenv.Ptr(location),
				Type:            mockenv.Ptr("IPV4"),
				Capacity:        mockenv.Ptr(int32(100)),
				Items:           []string{"203.0.113.0/24"},
			},
		}
		if err := env.Kube.Create(env.Ctx, o); err != nil {
			t.Fatal(err)
		}
		orgName := org + "/locations/" + location + "/addressGroups/corp-egress"
		u := env.WaitReady(gvk, namespace, "corp-egress")
		if ref, _, _ := unstructured.NestedString(u.Object, "status", "externalRef"); ref != orgName {
			t.Errorf("status.externalRef = %q, want %q (the name is metadata.name without resourceID)", ref, orgName)
		}
		got, err := orgs.GetAddressGroup(env.Ctx, &pb.GetAddressGroupRequest{Name: orgName})
		if err != nil {
			t.Fatalf("reading the organization's address group: %v", err)
		}
		if !slices.Equal(got.GetItems(), []string{"203.0.113.0/24"}) || got.GetCapacity() != 100 {
			t.Errorf("cloud items, capacity = %v, %d", got.GetItems(), got.GetCapacity())
		}
		created := 0
		for _, r := range env.Requests() {
			if strings.HasPrefix(r, "POST ") && strings.Contains(r, "/v1/"+org+"/locations/"+location+"/addressGroups") {
				created++
			}
		}
		if created != 1 {
			t.Errorf("POST to the organization's address groups %d times, want 1", created)
		}

		o = getObj(t, "corp-egress")
		o.Spec.Items = []string{"203.0.113.0/24", "198.51.100.0/24"}
		if err := env.Kube.Update(env.Ctx, o); err != nil {
			t.Fatal(err)
		}
		deadline := time.Now().Add(mockenv.Timeout())
		for {
			got, err := orgs.GetAddressGroup(env.Ctx, &pb.GetAddressGroupRequest{Name: orgName})
			if err != nil {
				t.Fatal(err)
			}
			if len(got.GetItems()) == 2 {
				break
			}
			if time.Now().After(deadline) {
				t.Fatalf("cloud items = %v after the update", got.GetItems())
			}
			time.Sleep(time.Second)
		}

		if err := env.Kube.Delete(env.Ctx, getObj(t, "corp-egress")); err != nil {
			t.Fatal(err)
		}
		env.WaitGone(&networksecurityv1alpha1.NetworkSecurityAddressGroup{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: "corp-egress"}})
		if _, err := orgs.GetAddressGroup(env.Ctx, &pb.GetAddressGroupRequest{Name: orgName}); !direct.IsNotFound(err) {
			t.Errorf("after delete, reading the organization's address group: %v, want not found", err)
		}
	})

	t.Run("a changed parent is refused", func(t *testing.T) {
		if mockenv.Live() {
			t.Skip("the new parent would be an organization")
		}
		o := getObj(t, "on-prem")
		o.Spec.ProjectRef = nil
		o.Spec.OrganizationRef = &refsv1beta1.OrganizationRef{External: org}
		if err := env.Kube.Update(env.Ctx, o); err != nil {
			t.Fatal(err)
		}
		if msg := env.WaitNotReady(gvk, namespace, "on-prem"); !strings.Contains(msg, "cannot change NetworkSecurityAddressGroup identity") {
			t.Errorf("Ready message = %q, want the change refused", msg)
		}
		o = getObj(t, "on-prem")
		o.Spec.OrganizationRef = nil
		o.Spec.ProjectRef = &refsv1beta1.ProjectRef{External: mockenv.Project}
		if err := env.Kube.Update(env.Ctx, o); err != nil {
			t.Fatal(err)
		}
		env.WaitReady(gvk, namespace, "on-prem")
	})

	t.Run("delete deletes the address group", func(t *testing.T) {
		if err := env.Kube.Delete(env.Ctx, getObj(t, "on-prem")); err != nil {
			t.Fatal(err)
		}
		env.WaitGone(&networksecurityv1alpha1.NetworkSecurityAddressGroup{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: "on-prem"}})
		if _, err := cloud(t, "on-prem"); !direct.IsNotFound(err) {
			t.Errorf("after delete, reading the address group: %v, want not found", err)
		}
	})

	t.Run("abandon keeps the address group", func(t *testing.T) {
		kept := newProjectAddressGroup("kept", "10.1.0.0/16")
		kept.Annotations = map[string]string{"cnrm.cloud.google.com/deletion-policy": "abandon"}
		if err := env.Kube.Create(env.Ctx, kept); err != nil {
			t.Fatal(err)
		}
		env.WaitReady(gvk, namespace, "kept")
		if err := env.Kube.Delete(env.Ctx, getObj(t, "kept")); err != nil {
			t.Fatal(err)
		}
		env.WaitGone(&networksecurityv1alpha1.NetworkSecurityAddressGroup{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: "kept"}})
		if _, err := cloud(t, "kept"); err != nil {
			t.Errorf("after abandon, reading the address group: %v, want it kept", err)
		}
	})
}
