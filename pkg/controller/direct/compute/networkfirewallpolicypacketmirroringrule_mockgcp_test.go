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

package compute_test

// ComputeNetworkFirewallPolicyPacketMirroringRule end to end on the fake
// cloud (pkg/test/mockenv; mockgcp patch 0031) or a real project
// (docs/live-runs.md). The case: a security team's packet mirroring on a
// VPC's global network firewall policy: web traffic is not mirrored (so a
// broader mirror rule below it skips it), and the rest goes to the next
// rule. A mirror rule needs a security profile group with a custom
// mirroring profile and a mirroring endpoint group, which a live run does
// not make: it runs on the fake only. The policy is made in the cloud and
// named by external, as in the network firewall policy rule's test.

import (
	"slices"
	"strings"
	"testing"
	"time"

	gcp "cloud.google.com/go/compute/apiv1"
	pb "cloud.google.com/go/compute/apiv1/computepb"
	kcccomputev1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/apis/compute/v1beta1"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/k8s"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"

	computev1alpha1 "github.com/highfivelabs/config-connector-extensions/apis/compute/v1alpha1"
	"github.com/highfivelabs/config-connector-extensions/pkg/test/mockenv"
)

func TestComputeNetworkFirewallPolicyPacketMirroringRule(t *testing.T) {
	gvk := computev1alpha1.ComputeNetworkFirewallPolicyPacketMirroringRuleGVK
	env := mockenv.New(t, namespace)
	policies, err := gcp.NewNetworkFirewallPoliciesRESTClient(env.Ctx, env.GCPOptions()...)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { policies.Close() })

	// The policy, made in the cloud and deleted when the test ends (with
	// what rules are left in it, the abandoned one included).
	policyName := mockenv.Name("mirroring")
	op, err := policies.Insert(env.Ctx, &pb.InsertNetworkFirewallPolicyRequest{
		Project: mockenv.Project,
		FirewallPolicyResource: &pb.FirewallPolicy{
			Name:        mockenv.Ptr(policyName),
			Description: mockenv.Ptr("packet mirroring rules under test"),
		},
	})
	if err == nil {
		err = op.Wait(env.Ctx)
	}
	if err != nil {
		t.Fatalf("making policy %s: %v", policyName, err)
	}
	t.Cleanup(func() {
		op, err := policies.Delete(env.Ctx, &pb.DeleteNetworkFirewallPolicyRequest{Project: mockenv.Project, FirewallPolicy: policyName})
		if err == nil {
			err = op.Wait(env.Ctx)
		}
		if err != nil && !direct.IsNotFound(err) {
			t.Errorf("deleting policy %s: %v", policyName, err)
		}
	})
	policyPath := "projects/" + mockenv.Project + "/global/firewallPolicies/" + policyName

	cloudRule := func(priority int32) (*pb.FirewallPolicyRule, error) {
		return policies.GetPacketMirroringRule(env.Ctx, &pb.GetPacketMirroringRuleNetworkFirewallPolicyRequest{
			Project: mockenv.Project, FirewallPolicy: policyName, Priority: mockenv.Ptr(priority),
		})
	}
	missing := func(err error) bool {
		return err != nil && (direct.IsNotFound(err) || strings.Contains(err.Error(), "does not contain a"))
	}
	writes := func(method string) int {
		n := 0
		for _, r := range env.Requests() {
			if strings.HasPrefix(r, "POST ") && strings.Contains(r, "/firewallPolicies/"+policyName+"/"+method) {
				n++
			}
		}
		return n
	}
	getObj := func(t *testing.T, name string) *computev1alpha1.ComputeNetworkFirewallPolicyPacketMirroringRule {
		t.Helper()
		obj := &computev1alpha1.ComputeNetworkFirewallPolicyPacketMirroringRule{}
		if err := env.Kube.Get(env.Ctx, types.NamespacedName{Namespace: namespace, Name: name}, obj); err != nil {
			t.Fatal(err)
		}
		return obj
	}
	update := func(t *testing.T, name string, change func(*computev1alpha1.ComputeNetworkFirewallPolicyPacketMirroringRule)) {
		t.Helper()
		o := getObj(t, name)
		change(o)
		if err := env.Kube.Update(env.Ctx, o); err != nil {
			t.Fatal(err)
		}
	}
	newRule := func(name string, priority int64, action string) *computev1alpha1.ComputeNetworkFirewallPolicyPacketMirroringRule {
		return &computev1alpha1.ComputeNetworkFirewallPolicyPacketMirroringRule{
			ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: name},
			Spec: computev1alpha1.ComputeNetworkFirewallPolicyPacketMirroringRuleSpec{
				FirewallPolicyRef: kcccomputev1beta1.ComputeNetworkFirewallPolicyRef{External: policyPath},
				Priority:          priority,
				Action:            action,
				Direction:         "INGRESS",
				Description:       mockenv.Ptr("web traffic is not mirrored"),
				Match: computev1alpha1.NetworkFirewallPolicyPacketMirroringRuleMatch{
					SrcIPRanges: []string{"10.0.0.0/8"},
					Layer4Configs: []computev1alpha1.NetworkFirewallPolicyPacketMirroringRuleLayer4Config{
						{IPProtocol: "tcp", Ports: []string{"80", "443"}},
					},
				},
			},
		}
	}

	const web = "no-web-mirroring"
	if err := env.Kube.Create(env.Ctx, newRule(web, 1000, "do_not_mirror")); err != nil {
		t.Fatal(err)
	}

	t.Run("create adds the packet mirroring rule at its priority", func(t *testing.T) {
		u := env.WaitReady(gvk, namespace, web)
		got, err := cloudRule(1000)
		if err != nil {
			t.Fatalf("reading the rule from the cloud: %v", err)
		}
		if got.GetAction() != "do_not_mirror" || got.GetDirection() != "INGRESS" || got.GetDescription() != "web traffic is not mirrored" {
			t.Errorf("cloud rule = %v", got)
		}
		if r := got.GetMatch().GetSrcIpRanges(); !slices.Equal(r, []string{"10.0.0.0/8"}) {
			t.Errorf("cloud source ranges = %v", r)
		}
		if l := got.GetMatch().GetLayer4Configs(); len(l) != 1 || l[0].GetIpProtocol() != "tcp" || len(l[0].GetPorts()) != 2 {
			t.Errorf("cloud layer 4 configs = %v", l)
		}
		want := policyPath + "/packetMirroringRules/1000"
		if ref, _, _ := unstructured.NestedString(u.Object, "status", "externalRef"); ref != want {
			t.Errorf("status.externalRef = %q, want %q", ref, want)
		}
		if n, _, _ := unstructured.NestedInt64(u.Object, "status", "ruleTupleCount"); n == 0 {
			t.Errorf("status.ruleTupleCount is not set")
		}
		// The policy's firewall rules are not touched.
		if n := writes("addRule") + writes("patchRule") + writes("removeRule"); n != 0 {
			t.Errorf("%d writes of the policy's firewall rules, want 0", n)
		}
	})

	t.Run("a resync writes nothing", func(t *testing.T) {
		before := writes("patchPacketMirroringRule") + writes("addPacketMirroringRule")
		update(t, web, func(o *computev1alpha1.ComputeNetworkFirewallPolicyPacketMirroringRule) {
			o.Annotations = map[string]string{k8s.InternalForceReconcileAnnotation: time.Now().Format(time.RFC3339Nano)}
		})
		time.Sleep(3 * time.Second)
		env.WaitReady(gvk, namespace, web)
		if n := writes("patchPacketMirroringRule") + writes("addPacketMirroringRule") - before; n != 0 {
			t.Errorf("a resync wrote %d times", n)
		}
	})

	t.Run("changed fields and an emptied list are one patch", func(t *testing.T) {
		before := writes("patchPacketMirroringRule")
		update(t, web, func(o *computev1alpha1.ComputeNetworkFirewallPolicyPacketMirroringRule) {
			o.Spec.Description = mockenv.Ptr("web traffic to the frontends is not mirrored")
			o.Spec.Match.SrcIPRanges = nil
			o.Spec.Match.DestIPRanges = []string{"10.10.0.0/16"}
			o.Spec.RuleName = mockenv.Ptr("no-web")
		})
		waitFor(t, func() bool {
			got, err := cloudRule(1000)
			return err == nil && got.GetDescription() == "web traffic to the frontends is not mirrored" &&
				len(got.GetMatch().GetSrcIpRanges()) == 0 && slices.Equal(got.GetMatch().GetDestIpRanges(), []string{"10.10.0.0/16"}) &&
				got.GetRuleName() == "no-web"
		})
		env.WaitReady(gvk, namespace, web)
		if n := writes("patchPacketMirroringRule") - before; n != 1 {
			t.Errorf("the change patched the rule %d times, want 1", n)
		}
	})

	t.Run("a second rule at its own priority", func(t *testing.T) {
		next := newRule("rest-goes-on", 2000, "goto_next")
		next.Spec.Direction = "EGRESS"
		next.Spec.Description = nil
		next.Spec.Match = computev1alpha1.NetworkFirewallPolicyPacketMirroringRuleMatch{
			DestIPRanges:  []string{"0.0.0.0/0"},
			Layer4Configs: []computev1alpha1.NetworkFirewallPolicyPacketMirroringRuleLayer4Config{{IPProtocol: "all"}},
		}
		if err := env.Kube.Create(env.Ctx, next); err != nil {
			t.Fatal(err)
		}
		env.WaitReady(gvk, namespace, "rest-goes-on")
		got, err := cloudRule(2000)
		if err != nil || got.GetAction() != "goto_next" || got.GetDirection() != "EGRESS" {
			t.Fatalf("cloud rule at 2000 = %v, %v", got, err)
		}
		if _, err := cloudRule(1000); err != nil {
			t.Errorf("the rule at 1000 after adding 2000: %v", err)
		}
	})

	t.Run("a mirror rule without a security profile group is refused", func(t *testing.T) {
		o := newRule("mirror-without-group", 3000, "mirror")
		err := env.Kube.Create(env.Ctx, o)
		if err == nil || !strings.Contains(err.Error(), "a mirror rule needs securityProfileGroup") {
			t.Errorf("creating a mirror rule without a group: got %v, want the API server to refuse it", err)
		}
	})

	t.Run("a mirror rule names its security profile group", func(t *testing.T) {
		if mockenv.Live() {
			t.Skip("a live mirror rule needs a security profile group with a custom mirroring profile and a mirroring endpoint group, which a live run does not make")
		}
		o := newRule("mirror-rest", 3000, "mirror")
		group := "//networksecurity.googleapis.com/projects/" + mockenv.Project + "/locations/global/securityProfileGroups/mirroring"
		o.Spec.SecurityProfileGroup = mockenv.Ptr(group)
		if err := env.Kube.Create(env.Ctx, o); err != nil {
			t.Fatal(err)
		}
		env.WaitReady(gvk, namespace, "mirror-rest")
		got, err := cloudRule(3000)
		if err != nil || got.GetAction() != "mirror" || got.GetSecurityProfileGroup() != group {
			t.Fatalf("cloud rule at 3000 = %v, %v", got, err)
		}
		if err := env.Kube.Delete(env.Ctx, getObj(t, "mirror-rest")); err != nil {
			t.Fatal(err)
		}
		env.WaitGone(&computev1alpha1.ComputeNetworkFirewallPolicyPacketMirroringRule{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: "mirror-rest"}})
	})

	t.Run("the priority and the policy are immutable", func(t *testing.T) {
		o := getObj(t, web)
		o.Spec.Priority = 1001
		if err := env.Kube.Update(env.Ctx, o); err == nil || !strings.Contains(err.Error(), "priority is immutable") {
			t.Errorf("changing the priority: got %v, want the API server to refuse it", err)
		}
		o = getObj(t, web)
		o.Spec.FirewallPolicyRef = kcccomputev1beta1.ComputeNetworkFirewallPolicyRef{External: policyPath + "-other"}
		if err := env.Kube.Update(env.Ctx, o); err == nil || !strings.Contains(err.Error(), "firewallPolicyRef is immutable") {
			t.Errorf("changing the policy: got %v, want the API server to refuse it", err)
		}
	})

	t.Run("delete removes the rule", func(t *testing.T) {
		if err := env.Kube.Delete(env.Ctx, getObj(t, web)); err != nil {
			t.Fatal(err)
		}
		env.WaitGone(&computev1alpha1.ComputeNetworkFirewallPolicyPacketMirroringRule{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: web}})
		if _, err := cloudRule(1000); !missing(err) {
			t.Errorf("after delete, reading the rule at 1000: %v, want it missing", err)
		}
	})

	t.Run("abandon keeps the rule", func(t *testing.T) {
		update(t, "rest-goes-on", func(o *computev1alpha1.ComputeNetworkFirewallPolicyPacketMirroringRule) {
			o.Annotations = map[string]string{"cnrm.cloud.google.com/deletion-policy": "abandon"}
		})
		if err := env.Kube.Delete(env.Ctx, getObj(t, "rest-goes-on")); err != nil {
			t.Fatal(err)
		}
		env.WaitGone(&computev1alpha1.ComputeNetworkFirewallPolicyPacketMirroringRule{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: "rest-goes-on"}})
		if _, err := cloudRule(2000); err != nil {
			t.Errorf("after abandon, reading the rule at 2000: %v, want it kept", err)
		}
	})
}
