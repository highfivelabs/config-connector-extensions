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

// ComputeRegionNetworkFirewallPolicyRule end to end on the fake cloud
// (pkg/test/mockenv, patch 0020) or a real project (docs/live-runs.md). The
// case: the rules of a regional policy for a VPC, as fabric's
// net-firewall-policy module makes them: an ingress rule for health check
// ranges and an egress rule, each at its own priority. The policy is the
// run's own, made from its Config Connector-style object (our kind) and
// deleted at the end; policies take no labels, so the run's name prefix is
// how the sweeper finds it.

import (
	"strings"
	"testing"
	"time"

	gcp "cloud.google.com/go/compute/apiv1"
	pb "cloud.google.com/go/compute/apiv1/computepb"
	"github.com/GoogleCloudPlatform/k8s-config-connector/apis/iam/iamrefs"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/k8s"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"

	computev1alpha1 "github.com/highfivelabs/config-connector-extensions/apis/compute/v1alpha1"
	"github.com/highfivelabs/config-connector-extensions/pkg/test/mockenv"
)

func TestComputeRegionNetworkFirewallPolicyRule(t *testing.T) {
	policyGVK := computev1alpha1.ComputeRegionNetworkFirewallPolicyGVK
	ruleGVK := computev1alpha1.ComputeRegionNetworkFirewallPolicyRuleGVK
	env := mockenv.New(t, namespace)
	policies, err := gcp.NewRegionNetworkFirewallPoliciesRESTClient(env.Ctx, env.GCPOptions()...)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { policies.Close() })

	const policyObj = "rules-policy"
	policyName := mockenv.Name(policyObj)
	policy := newRegionFirewallPolicy(policyObj, "rules under test")
	if err := env.Kube.Create(env.Ctx, policy); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		op, err := policies.Delete(env.Ctx, &pb.DeleteRegionNetworkFirewallPolicyRequest{
			Project: mockenv.Project, Region: region, FirewallPolicy: policyName,
		})
		if err == nil {
			err = op.Wait(env.Ctx)
		}
		if err != nil && !direct.IsNotFound(err) {
			t.Errorf("deleting policy %s: %v", policyName, err)
		}
	})
	env.WaitReady(policyGVK, namespace, policyObj)

	// cloudRule is the rule at a priority in the cloud.
	cloudRule := func(t *testing.T, priority int32) (*pb.FirewallPolicyRule, error) {
		t.Helper()
		return policies.GetRule(env.Ctx, &pb.GetRuleRegionNetworkFirewallPolicyRequest{
			Project: mockenv.Project, Region: region, FirewallPolicy: policyName, Priority: mockenv.Ptr(priority),
		})
	}
	writes := func() int {
		n := 0
		for _, r := range env.Requests() {
			if strings.HasPrefix(r, "POST ") && strings.Contains(r, "/firewallPolicies/"+policyName+"/") &&
				(strings.HasSuffix(r, "/patchRule") || strings.Contains(r, "/patchRule?") || strings.Contains(r, "/addRule") || strings.Contains(r, "/removeRule")) {
				n++
			}
		}
		return n
	}
	getObj := func(t *testing.T, name string) *computev1alpha1.ComputeRegionNetworkFirewallPolicyRule {
		t.Helper()
		obj := &computev1alpha1.ComputeRegionNetworkFirewallPolicyRule{}
		if err := env.Kube.Get(env.Ctx, types.NamespacedName{Namespace: namespace, Name: name}, obj); err != nil {
			t.Fatal(err)
		}
		return obj
	}
	update := func(t *testing.T, name string, change func(*computev1alpha1.ComputeRegionNetworkFirewallPolicyRule)) {
		t.Helper()
		o := getObj(t, name)
		change(o)
		if err := env.Kube.Update(env.Ctx, o); err != nil {
			t.Fatal(err)
		}
	}
	resync := func(t *testing.T, name string) {
		t.Helper()
		update(t, name, func(o *computev1alpha1.ComputeRegionNetworkFirewallPolicyRule) {
			o.Annotations = map[string]string{k8s.InternalForceReconcileAnnotation: time.Now().Format(time.RFC3339Nano)}
		})
	}
	newRule := func(name string, priority int64) *computev1alpha1.ComputeRegionNetworkFirewallPolicyRule {
		return &computev1alpha1.ComputeRegionNetworkFirewallPolicyRule{
			ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: name},
			Spec: computev1alpha1.ComputeRegionNetworkFirewallPolicyRuleSpec{
				FirewallPolicyRef: computev1alpha1.ComputeRegionNetworkFirewallPolicyRef{Name: policyObj},
				Priority:          priority,
				Action:            "allow",
				Direction:         "INGRESS",
				Description:       mockenv.Ptr("health checks"),
				Match: computev1alpha1.RegionFirewallPolicyRuleMatch{
					SrcIPRanges: []string{"130.211.0.0/22", "35.191.0.0/16"},
					Layer4Configs: []computev1alpha1.RegionFirewallPolicyRuleLayer4Config{
						{IPProtocol: "tcp", Ports: []string{"80", "443"}},
					},
				},
			},
		}
	}

	const health = "health-checks"
	if err := env.Kube.Create(env.Ctx, newRule(health, 1000)); err != nil {
		t.Fatal(err)
	}

	t.Run("create adds the rule at its priority", func(t *testing.T) {
		u := env.WaitReady(ruleGVK, namespace, health)
		got, err := cloudRule(t, 1000)
		if err != nil {
			t.Fatalf("reading the rule from the cloud: %v", err)
		}
		if got.GetAction() != "allow" || got.GetDirection() != "INGRESS" || got.GetDescription() != "health checks" {
			t.Errorf("cloud rule = %v", got)
		}
		if r := got.GetMatch().GetSrcIpRanges(); len(r) != 2 || r[0] != "130.211.0.0/22" {
			t.Errorf("cloud source ranges = %v", r)
		}
		if l := got.GetMatch().GetLayer4Configs(); len(l) != 1 || l[0].GetIpProtocol() != "tcp" || len(l[0].GetPorts()) != 2 {
			t.Errorf("cloud layer 4 configs = %v", l)
		}
		want := "projects/" + mockenv.Project + "/regions/" + region + "/firewallPolicies/" + policyName + "/rules/1000"
		if ref, _, _ := unstructured.NestedString(u.Object, "status", "externalRef"); ref != want {
			t.Errorf("status.externalRef = %q, want %q", ref, want)
		}
		if n, _, _ := unstructured.NestedInt64(u.Object, "status", "ruleTupleCount"); n == 0 {
			t.Errorf("status.ruleTupleCount is not set")
		}
	})

	t.Run("a resync writes nothing", func(t *testing.T) {
		before := writes()
		resync(t, health)
		time.Sleep(3 * time.Second)
		env.WaitReady(ruleGVK, namespace, health)
		if n := writes() - before; n != 0 {
			t.Errorf("a resync wrote %d times", n)
		}
	})

	t.Run("changed fields are patched once", func(t *testing.T) {
		before := writes()
		update(t, health, func(o *computev1alpha1.ComputeRegionNetworkFirewallPolicyRule) {
			o.Spec.Description = mockenv.Ptr("health checks and the proxy")
			o.Spec.EnableLogging = mockenv.Ptr(true)
			o.Spec.RuleName = mockenv.Ptr("health")
			o.Spec.Match.SrcIPRanges = []string{"130.211.0.0/22", "35.191.0.0/16", "10.0.0.0/8"}
			o.Spec.Match.Layer4Configs[0].Ports = []string{"80", "443", "8080-8090"}
		})
		waitFor(t, func() bool {
			got, err := cloudRule(t, 1000)
			return err == nil && got.GetEnableLogging() && len(got.GetMatch().GetSrcIpRanges()) == 3
		})
		env.WaitReady(ruleGVK, namespace, health)
		got, _ := cloudRule(t, 1000)
		if got.GetDescription() != "health checks and the proxy" || got.GetRuleName() != "health" || len(got.GetMatch().GetLayer4Configs()[0].GetPorts()) != 3 {
			t.Errorf("cloud rule = %v", got)
		}
		if n := writes() - before; n != 1 {
			t.Errorf("wrote %d times, want once", n)
		}
	})

	t.Run("taking fields off the spec puts their defaults back", func(t *testing.T) {
		update(t, health, func(o *computev1alpha1.ComputeRegionNetworkFirewallPolicyRule) {
			o.Spec.Description = nil
			o.Spec.EnableLogging = nil
			o.Spec.RuleName = nil
		})
		waitFor(t, func() bool {
			got, err := cloudRule(t, 1000)
			return err == nil && !got.GetEnableLogging() && got.GetDescription() == "" && got.GetRuleName() == ""
		})
		env.WaitReady(ruleGVK, namespace, health)
	})

	t.Run("lists taken off the spec are cleared", func(t *testing.T) {
		update(t, health, func(o *computev1alpha1.ComputeRegionNetworkFirewallPolicyRule) {
			o.Spec.Match.SrcIPRanges = nil
			o.Spec.Match.SrcRegionCodes = []string{"US"}
			o.Spec.Match.Layer4Configs[0].Ports = nil
		})
		waitFor(t, func() bool {
			got, err := cloudRule(t, 1000)
			return err == nil && len(got.GetMatch().GetSrcRegionCodes()) == 1
		})
		env.WaitReady(ruleGVK, namespace, health)
		got, _ := cloudRule(t, 1000)
		if r := got.GetMatch().GetSrcIpRanges(); len(r) != 0 {
			t.Errorf("source ranges still in the cloud rule: %v", r)
		}
		if l := got.GetMatch().GetLayer4Configs(); len(l) != 1 || len(l[0].GetPorts()) != 0 {
			t.Errorf("ports still in the cloud rule: %v", l)
		}
		// A resync finds nothing to change.
		before := writes()
		resync(t, health)
		time.Sleep(3 * time.Second)
		env.WaitReady(ruleGVK, namespace, health)
		if n := writes() - before; n != 0 {
			t.Errorf("a resync wrote %d times", n)
		}
	})

	t.Run("a rule changed out of band is put back", func(t *testing.T) {
		got, _ := cloudRule(t, 1000)
		got.Action = mockenv.Ptr("deny")
		// A rule just changed is "not ready" to the API for a while, and a
		// patch then answers 400: try again.
		var patchErr error
		for i := 0; i < 60; i++ {
			var op *gcp.Operation
			op, patchErr = policies.PatchRule(env.Ctx, &pb.PatchRuleRegionNetworkFirewallPolicyRequest{
				Project: mockenv.Project, Region: region, FirewallPolicy: policyName, Priority: mockenv.Ptr(int32(1000)),
				FirewallPolicyRuleResource: got,
			})
			if patchErr == nil {
				patchErr = op.Wait(env.Ctx)
			}
			if patchErr == nil || !strings.Contains(patchErr.Error(), "is not ready") {
				break
			}
			time.Sleep(2 * time.Second)
		}
		if patchErr != nil {
			t.Fatalf("changing the rule out of band: %v", patchErr)
		}
		resync(t, health)
		waitFor(t, func() bool {
			got, err := cloudRule(t, 1000)
			return err == nil && got.GetAction() == "allow"
		})
		env.WaitReady(ruleGVK, namespace, health)
	})

	t.Run("another priority is another rule", func(t *testing.T) {
		const egress = "egress-dns"
		r := newRule(egress, 2000)
		r.Spec.Direction = "EGRESS"
		r.Spec.Action = "deny"
		r.Spec.Description = nil
		r.Spec.Match = computev1alpha1.RegionFirewallPolicyRuleMatch{
			DestIPRanges:  []string{"192.0.2.0/24"},
			Layer4Configs: []computev1alpha1.RegionFirewallPolicyRuleLayer4Config{{IPProtocol: "udp", Ports: []string{"53"}}},
		}
		r.Spec.TargetServiceAccounts = []iamrefs.IAMServiceAccountRef{{External: "app@" + mockenv.Project + ".iam.gserviceaccount.com"}}
		if err := env.Kube.Create(env.Ctx, r); err != nil {
			t.Fatal(err)
		}
		env.WaitReady(ruleGVK, namespace, egress)
		got, err := cloudRule(t, 2000)
		if err != nil {
			t.Fatal(err)
		}
		if got.GetDirection() != "EGRESS" || got.GetAction() != "deny" || len(got.GetTargetServiceAccounts()) != 1 || !strings.HasPrefix(got.GetTargetServiceAccounts()[0], "app@") {
			t.Errorf("cloud rule = %v", got)
		}
		if first, err := cloudRule(t, 1000); err != nil || first.GetDirection() != "INGRESS" {
			t.Errorf("the first rule changed: %v, %v", first, err)
		}
		// Deleting one leaves the other.
		if err := env.Kube.Delete(env.Ctx, r); err != nil {
			t.Fatal(err)
		}
		env.WaitGone(r)
		if _, err := cloudRule(t, 2000); err == nil {
			t.Error("the deleted rule is still in the policy")
		}
		if _, err := cloudRule(t, 1000); err != nil {
			t.Errorf("deleting one rule took the other: %v", err)
		}
	})

	t.Run("the priority and the policy cannot change", func(t *testing.T) {
		o := getObj(t, health)
		o.Spec.Priority = 1001
		if err := env.Kube.Update(env.Ctx, o); err == nil || !strings.Contains(err.Error(), "priority is immutable") {
			t.Errorf("changing the priority: err = %v", err)
		}
		o = getObj(t, health)
		o.Spec.FirewallPolicyRef = computev1alpha1.ComputeRegionNetworkFirewallPolicyRef{External: "projects/" + mockenv.Project + "/regions/" + region + "/firewallPolicies/other"}
		if err := env.Kube.Update(env.Ctx, o); err == nil || !strings.Contains(err.Error(), "immutable") {
			t.Errorf("changing the policy: err = %v", err)
		}
	})

	t.Run("a rule waits for the policy it names, then is made", func(t *testing.T) {
		// Config Connector's reconciler watches the missing policy until it
		// is Ready (a watch that outlives the object), so the policy is
		// made here rather than left missing: kube-apiserver would not
		// stop with the watch open.
		const lateObj = "not-yet"
		r := newRule("early", 3000)
		r.Spec.FirewallPolicyRef = computev1alpha1.ComputeRegionNetworkFirewallPolicyRef{Name: lateObj}
		if err := env.Kube.Create(env.Ctx, r); err != nil {
			t.Fatal(err)
		}
		if msg := env.WaitNotReady(ruleGVK, namespace, "early"); !strings.Contains(msg, lateObj) {
			t.Errorf("the message does not name the missing policy: %q", msg)
		}
		late := newRegionFirewallPolicy(lateObj, "made after its rule")
		lateName := mockenv.Name(lateObj)
		t.Cleanup(func() {
			op, err := policies.Delete(env.Ctx, &pb.DeleteRegionNetworkFirewallPolicyRequest{
				Project: mockenv.Project, Region: region, FirewallPolicy: lateName,
			})
			if err == nil {
				err = op.Wait(env.Ctx)
			}
			if err != nil && !direct.IsNotFound(err) {
				t.Errorf("deleting policy %s: %v", lateName, err)
			}
		})
		if err := env.Kube.Create(env.Ctx, late); err != nil {
			t.Fatal(err)
		}
		env.WaitReady(policyGVK, namespace, lateObj)
		env.WaitReady(ruleGVK, namespace, "early")
		if got, err := policies.GetRule(env.Ctx, &pb.GetRuleRegionNetworkFirewallPolicyRequest{
			Project: mockenv.Project, Region: region, FirewallPolicy: lateName, Priority: mockenv.Ptr(int32(3000)),
		}); err != nil || got.GetAction() != "allow" {
			t.Errorf("the rule in the late policy: %v, %v", got, err)
		}
		if err := env.Kube.Delete(env.Ctx, r); err != nil {
			t.Fatal(err)
		}
		env.WaitGone(r)
		if err := env.Kube.Delete(env.Ctx, late); err != nil {
			t.Fatal(err)
		}
		env.WaitGone(late)
	})

	t.Run("delete removes the rule", func(t *testing.T) {
		if err := env.Kube.Delete(env.Ctx, getObj(t, health)); err != nil {
			t.Fatal(err)
		}
		env.WaitGone(&computev1alpha1.ComputeRegionNetworkFirewallPolicyRule{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: health}})
		_, err := cloudRule(t, 1000)
		if err == nil {
			t.Fatal("the rule is still in the policy")
		}
		// What the API answers for a priority with no rule: the controller
		// takes a 404 and a 400 with "does not contain a rule at priority"
		// as "not there" (docs/live-runs.md).
		t.Logf("the answer for a priority with no rule: %v", err)
	})

	t.Run("abandon keeps the rule", func(t *testing.T) {
		kept := newRule("kept", 4000)
		kept.Annotations = map[string]string{"cnrm.cloud.google.com/deletion-policy": "abandon"}
		if err := env.Kube.Create(env.Ctx, kept); err != nil {
			t.Fatal(err)
		}
		env.WaitReady(ruleGVK, namespace, "kept")
		if err := env.Kube.Delete(env.Ctx, kept); err != nil {
			t.Fatal(err)
		}
		env.WaitGone(kept)
		if _, err := cloudRule(t, 4000); err != nil {
			t.Errorf("the abandoned rule is gone: %v", err)
		}
	})
}
