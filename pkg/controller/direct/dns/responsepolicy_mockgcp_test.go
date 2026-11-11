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

package dns_test

// DNSResponsePolicy end to end on the fake cloud (pkg/test/mockenv; mockgcp
// serves response policies unpatched) or a real project
// (docs/live-runs.md). The case: a platform's DNS firewall, a response
// policy bound to the landing zone's VPC networks, whose rules
// (DNSResponsePolicyRule) answer or block names before Cloud DNS does.

import (
	"strings"
	"testing"
	"time"

	refs "github.com/GoogleCloudPlatform/k8s-config-connector/apis/refs"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/k8s"
	api "google.golang.org/api/dns/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"

	dnsv1beta1 "github.com/highfivelabs/config-connector-extensions/apis/dns/v1beta1"
	"github.com/highfivelabs/config-connector-extensions/pkg/test/mockenv"
)

func TestDNSResponsePolicy(t *testing.T) {
	skipLiveWithoutDNS(t)
	gvk := dnsv1beta1.DNSResponsePolicyGVK
	env := mockenv.New(t, namespace)
	cloud, err := api.NewService(env.Ctx, env.GCPOptions()...)
	if err != nil {
		t.Fatal(err)
	}
	// The networks are deleted after the policies (cleanups run last first).
	shared := newNetwork(t, env, "rp-shared")
	other := newNetwork(t, env, "rp-other")
	networkURL := func(path string) string { return "https://www.googleapis.com/compute/v1/" + path }

	get := func(id string) (*api.ResponsePolicy, error) {
		return cloud.ResponsePolicies.Get(mockenv.Project, id).Context(env.Ctx).Do()
	}
	// Delete what the test made, the abandoned policy included, and what a
	// failed step left; a policy bound to a network must be unbound first.
	t.Cleanup(func() {
		for _, base := range []string{"dns-firewall", "kept"} {
			id := mockenv.Name(base)
			if p, err := get(id); err == nil && len(p.Networks) > 0 {
				_, _ = cloud.ResponsePolicies.Patch(mockenv.Project, id, &api.ResponsePolicy{
					Id: p.Id, Networks: []*api.ResponsePolicyNetwork{}, ForceSendFields: []string{"Networks"},
				}).Context(env.Ctx).Do()
			}
			if err := cloud.ResponsePolicies.Delete(mockenv.Project, id).Context(env.Ctx).Do(); err != nil && !direct.IsNotFound(err) {
				t.Errorf("deleting response policy %s: %v", id, err)
			}
		}
	})
	// writes counts the requests with this method to the policy with this ID.
	writes := func(method, id string) int {
		n := 0
		for _, r := range env.Requests() {
			if strings.HasPrefix(r, method+" ") && strings.Contains(r, "/responsePolicies/"+id+"?") {
				n++
			}
		}
		return n
	}
	getObj := func(t *testing.T, name string) *dnsv1beta1.DNSResponsePolicy {
		t.Helper()
		obj := &dnsv1beta1.DNSResponsePolicy{}
		if err := env.Kube.Get(env.Ctx, types.NamespacedName{Namespace: namespace, Name: name}, obj); err != nil {
			t.Fatal(err)
		}
		return obj
	}
	newPolicy := func(name string, networks ...string) *dnsv1beta1.DNSResponsePolicy {
		o := &dnsv1beta1.DNSResponsePolicy{
			ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: name, Labels: labels(map[string]string{"team": "platform"})},
			Spec: dnsv1beta1.DNSResponsePolicySpec{
				ProjectRef:  refs.ProjectRef{External: "projects/" + mockenv.Project},
				ResourceID:  mockenv.Ptr(mockenv.Name(name)),
				Description: mockenv.Ptr("the landing zone's DNS firewall"),
			},
		}
		for _, n := range networks {
			// A short path, as a user writes it; the controller sends the
			// full URL.
			o.Spec.Networks = append(o.Spec.Networks, dnsv1beta1.ResponsepolicyNetworks{NetworkUrl: n})
		}
		return o
	}
	id := mockenv.Name("dns-firewall")

	if err := env.Kube.Create(env.Ctx, newPolicy("dns-firewall", shared)); err != nil {
		t.Fatal(err)
	}

	t.Run("create makes the policy, bound to the network, with the object's labels", func(t *testing.T) {
		u := env.WaitReady(gvk, namespace, "dns-firewall")
		want := "projects/" + mockenv.Project + "/locations/global/responsePolicies/" + id
		if ref, _, _ := unstructured.NestedString(u.Object, "status", "externalRef"); ref != want {
			t.Errorf("status.externalRef = %q, want %q", ref, want)
		}
		got, err := get(id)
		if err != nil {
			t.Fatalf("reading the response policy: %v", err)
		}
		if got.Description != "the landing zone's DNS firewall" {
			t.Errorf("cloud description = %q", got.Description)
		}
		if len(got.Networks) != 1 || !strings.HasSuffix(got.Networks[0].NetworkUrl, "/"+shared) {
			t.Errorf("cloud networks = %+v, want %s", got.Networks, networkURL(shared))
		}
		if got.Labels["team"] != "platform" {
			t.Errorf("cloud labels = %v, want team=platform", got.Labels)
		}
	})

	t.Run("a resync writes nothing", func(t *testing.T) {
		before := writes("PUT", id) + writes("PATCH", id)
		o := getObj(t, "dns-firewall")
		o.Annotations = map[string]string{k8s.InternalForceReconcileAnnotation: "1"}
		if err := env.Kube.Update(env.Ctx, o); err != nil {
			t.Fatal(err)
		}
		time.Sleep(3 * time.Second)
		env.WaitReady(gvk, namespace, "dns-firewall")
		if n := writes("PUT", id) + writes("PATCH", id) - before; n != 0 {
			t.Errorf("a resync wrote the response policy %d times", n)
		}
	})

	t.Run("a second network, a new description and a changed label are one update", func(t *testing.T) {
		before := writes("PUT", id)
		o := getObj(t, "dns-firewall")
		o.Spec.Description = mockenv.Ptr("the landing zone's DNS firewall, both networks")
		o.Spec.Networks = append(o.Spec.Networks, dnsv1beta1.ResponsepolicyNetworks{NetworkUrl: networkURL(other)})
		o.Labels["team"] = "network"
		if err := env.Kube.Update(env.Ctx, o); err != nil {
			t.Fatal(err)
		}
		waitFor(t, "the update in the cloud", func() bool {
			got, err := get(id)
			if err != nil {
				t.Fatal(err)
			}
			return len(got.Networks) == 2 && got.Description == "the landing zone's DNS firewall, both networks" && got.Labels["team"] == "network"
		})
		env.WaitReady(gvk, namespace, "dns-firewall")
		if n := writes("PUT", id) - before; n != 1 {
			t.Errorf("the change updated the response policy %d times, want 1", n)
		}
	})

	t.Run("an unbound network and a removed label are one update", func(t *testing.T) {
		before := writes("PUT", id)
		o := getObj(t, "dns-firewall")
		o.Spec.Networks = o.Spec.Networks[:1]
		delete(o.Labels, "team")
		if err := env.Kube.Update(env.Ctx, o); err != nil {
			t.Fatal(err)
		}
		waitFor(t, "the update in the cloud", func() bool {
			got, err := get(id)
			if err != nil {
				t.Fatal(err)
			}
			_, team := got.Labels["team"]
			return len(got.Networks) == 1 && !team
		})
		env.WaitReady(gvk, namespace, "dns-firewall")
		if n := writes("PUT", id) - before; n != 1 {
			t.Errorf("the change updated the response policy %d times, want 1", n)
		}
	})

	t.Run("projectRef and resourceID are immutable", func(t *testing.T) {
		o := getObj(t, "dns-firewall")
		o.Spec.ResourceID = mockenv.Ptr(mockenv.Name("renamed"))
		if err := env.Kube.Update(env.Ctx, o); err == nil || !strings.Contains(err.Error(), "resourceID is immutable") {
			t.Errorf("changing resourceID: got %v, want the API server to refuse it", err)
		}
		o = getObj(t, "dns-firewall")
		o.Spec.ProjectRef = refs.ProjectRef{External: "projects/another-project"}
		if err := env.Kube.Update(env.Ctx, o); err == nil || !strings.Contains(err.Error(), "projectRef is immutable") {
			t.Errorf("changing projectRef: got %v, want the API server to refuse it", err)
		}
	})

	t.Run("delete unbinds the network, then deletes the policy", func(t *testing.T) {
		if err := env.Kube.Delete(env.Ctx, getObj(t, "dns-firewall")); err != nil {
			t.Fatal(err)
		}
		env.WaitGone(&dnsv1beta1.DNSResponsePolicy{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: "dns-firewall"}})
		if _, err := get(id); !direct.IsNotFound(err) {
			t.Errorf("after delete, reading the response policy: %v, want not found", err)
		}
		if writes("PATCH", id) != 1 || writes("DELETE", id) != 1 {
			t.Errorf("delete made %d PATCH and %d DELETE requests, want 1 each (unbind, then delete)", writes("PATCH", id), writes("DELETE", id))
		}
	})

	t.Run("abandon keeps the policy", func(t *testing.T) {
		kept := newPolicy("kept")
		kept.Annotations = map[string]string{"cnrm.cloud.google.com/deletion-policy": "abandon"}
		if err := env.Kube.Create(env.Ctx, kept); err != nil {
			t.Fatal(err)
		}
		env.WaitReady(gvk, namespace, "kept")
		if err := env.Kube.Delete(env.Ctx, getObj(t, "kept")); err != nil {
			t.Fatal(err)
		}
		env.WaitGone(&dnsv1beta1.DNSResponsePolicy{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: "kept"}})
		if _, err := get(mockenv.Name("kept")); err != nil {
			t.Errorf("after abandon, reading the response policy: %v, want it kept", err)
		}
	})

	t.Run("an existing policy is acquired by its resourceID", func(t *testing.T) {
		// "kept", abandoned above, is still in the cloud.
		acquired := newPolicy("kept")
		if err := env.Kube.Create(env.Ctx, acquired); err != nil {
			t.Fatal(err)
		}
		env.WaitReady(gvk, namespace, "kept")
		creates := 0
		for _, r := range env.Requests() {
			if strings.HasPrefix(r, "POST ") && strings.Contains(r, "/projects/"+mockenv.Project+"/responsePolicies?") {
				creates++
			}
		}
		if creates != 2 {
			t.Errorf("%d creates of response policies in the test, want 2 (dns-firewall and kept once; the second kept is acquired)", creates)
		}
		if err := env.Kube.Delete(env.Ctx, getObj(t, "kept")); err != nil {
			t.Fatal(err)
		}
		env.WaitGone(&dnsv1beta1.DNSResponsePolicy{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: "kept"}})
		if _, err := get(mockenv.Name("kept")); !direct.IsNotFound(err) {
			t.Errorf("after delete, reading the response policy: %v, want not found", err)
		}
	})
}
