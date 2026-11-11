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

// ComputeNetworkFirewallPolicyRule end to end on the fake cloud
// (pkg/test/mockenv; mockgcp serves a global policy's rules unpatched) or a
// real project (docs/live-runs.md). The case: the rules of a global network
// firewall policy for a VPC, as fabric's net-firewall-policy module makes
// them: an ingress rule for health check ranges and an egress rule, each at
// its own priority. The policy is Config Connector's kind, which this
// manager does not run, so the test makes the policy in the cloud and names
// it by external (and, once, by a Config Connector object whose status it
// writes as Config Connector would). Policies take no labels: the run's name
// prefix is how the sweeper finds it.

import (
	"strings"
	"testing"
	"time"

	gcp "cloud.google.com/go/compute/apiv1"
	pb "cloud.google.com/go/compute/apiv1/computepb"
	kcccomputev1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/apis/compute/v1beta1"
	"github.com/GoogleCloudPlatform/k8s-config-connector/apis/iam/iamrefs"
	refsv1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/apis/refs/v1beta1"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/k8s"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"

	computev1beta1 "github.com/highfivelabs/config-connector-extensions/apis/compute/v1beta1"
	"github.com/highfivelabs/config-connector-extensions/pkg/test/mockenv"
)

func TestComputeNetworkFirewallPolicyRule(t *testing.T) {
	ruleGVK := computev1beta1.ComputeNetworkFirewallPolicyRuleGVK
	env := mockenv.New(t, namespace)
	policies, err := gcp.NewNetworkFirewallPoliciesRESTClient(env.Ctx, env.GCPOptions()...)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { policies.Close() })

	// newPolicy makes a global network firewall policy in the cloud and
	// deletes it when the test ends; it returns its path.
	newPolicy := func(t *testing.T, base string) string {
		t.Helper()
		name := mockenv.Name(base)
		op, err := policies.Insert(env.Ctx, &pb.InsertNetworkFirewallPolicyRequest{
			Project: mockenv.Project,
			FirewallPolicyResource: &pb.FirewallPolicy{
				Name:        mockenv.Ptr(name),
				Description: mockenv.Ptr("rules under test"),
			},
		})
		if err == nil {
			err = op.Wait(env.Ctx)
		}
		if err != nil {
			t.Fatalf("making policy %s: %v", name, err)
		}
		t.Cleanup(func() {
			op, err := policies.Delete(env.Ctx, &pb.DeleteNetworkFirewallPolicyRequest{
				Project: mockenv.Project, FirewallPolicy: name,
			})
			if err == nil {
				err = op.Wait(env.Ctx)
			}
			if err != nil && !direct.IsNotFound(err) {
				t.Errorf("deleting policy %s: %v", name, err)
			}
		})
		return "projects/" + mockenv.Project + "/global/firewallPolicies/" + name
	}
	policyPath := newPolicy(t, "global-rules")
	policyName := policyPath[strings.LastIndex(policyPath, "/")+1:]

	// cloudRule is the rule at a priority in the cloud.
	cloudRule := func(t *testing.T, policy string, priority int32) (*pb.FirewallPolicyRule, error) {
		t.Helper()
		return policies.GetRule(env.Ctx, &pb.GetRuleNetworkFirewallPolicyRequest{
			Project: mockenv.Project, FirewallPolicy: policy, Priority: mockenv.Ptr(priority),
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
	getObj := func(t *testing.T, name string) *computev1beta1.ComputeNetworkFirewallPolicyRule {
		t.Helper()
		obj := &computev1beta1.ComputeNetworkFirewallPolicyRule{}
		if err := env.Kube.Get(env.Ctx, types.NamespacedName{Namespace: namespace, Name: name}, obj); err != nil {
			t.Fatal(err)
		}
		return obj
	}
	update := func(t *testing.T, name string, change func(*computev1beta1.ComputeNetworkFirewallPolicyRule)) {
		t.Helper()
		o := getObj(t, name)
		change(o)
		if err := env.Kube.Update(env.Ctx, o); err != nil {
			t.Fatal(err)
		}
	}
	resync := func(t *testing.T, name string) {
		t.Helper()
		update(t, name, func(o *computev1beta1.ComputeNetworkFirewallPolicyRule) {
			o.Annotations = map[string]string{k8s.InternalForceReconcileAnnotation: time.Now().Format(time.RFC3339Nano)}
		})
	}
	newRule := func(name string, priority int64) *computev1beta1.ComputeNetworkFirewallPolicyRule {
		return &computev1beta1.ComputeNetworkFirewallPolicyRule{
			ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: name},
			Spec: computev1beta1.ComputeNetworkFirewallPolicyRuleSpec{
				FirewallPolicyRef: kcccomputev1beta1.ComputeNetworkFirewallPolicyRef{External: policyPath},
				Priority:          priority,
				Action:            "allow",
				Direction:         "INGRESS",
				Description:       mockenv.Ptr("health checks"),
				Match: computev1beta1.NetworkFirewallPolicyRuleMatch{
					SrcIPRanges: []string{"130.211.0.0/22", "35.191.0.0/16"},
					Layer4Configs: []computev1beta1.NetworkFirewallPolicyRuleLayer4Config{
						{IPProtocol: "tcp", Ports: []string{"80", "443"}},
					},
				},
			},
		}
	}
	// settle waits until the cloud rule at priority meets cond and the object is Ready again.
	settle := func(t *testing.T, name string, cond func(*pb.FirewallPolicyRule) bool, priority int32) {
		t.Helper()
		waitFor(t, func() bool {
			got, err := cloudRule(t, policyName, priority)
			return err == nil && cond(got)
		})
		env.WaitReady(ruleGVK, namespace, name)
	}

	const health = "health-checks"
	if err := env.Kube.Create(env.Ctx, newRule(health, 1000)); err != nil {
		t.Fatal(err)
	}

	t.Run("create adds the rule at its priority", func(t *testing.T) {
		u := env.WaitReady(ruleGVK, namespace, health)
		got, err := cloudRule(t, policyName, 1000)
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
		want := policyPath + "/rules/1000"
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
		update(t, health, func(o *computev1beta1.ComputeNetworkFirewallPolicyRule) {
			o.Spec.Description = mockenv.Ptr("health checks and the proxy")
			o.Spec.EnableLogging = mockenv.Ptr(true)
			o.Spec.RuleName = mockenv.Ptr("health")
			o.Spec.Match.SrcIPRanges = []string{"130.211.0.0/22", "35.191.0.0/16", "10.0.0.0/8"}
			o.Spec.Match.Layer4Configs[0].Ports = []string{"80", "443", "8080-8090"}
		})
		settle(t, health, func(got *pb.FirewallPolicyRule) bool {
			return got.GetEnableLogging() && len(got.GetMatch().GetSrcIpRanges()) == 3
		}, 1000)
		got, _ := cloudRule(t, policyName, 1000)
		if got.GetDescription() != "health checks and the proxy" || got.GetRuleName() != "health" || len(got.GetMatch().GetLayer4Configs()[0].GetPorts()) != 3 {
			t.Errorf("cloud rule = %v", got)
		}
		if n := writes() - before; n != 1 {
			t.Errorf("wrote %d times, want once", n)
		}
	})

	t.Run("taking fields off the spec puts their defaults back", func(t *testing.T) {
		update(t, health, func(o *computev1beta1.ComputeNetworkFirewallPolicyRule) {
			o.Spec.Description = nil
			o.Spec.EnableLogging = nil
			o.Spec.RuleName = nil
		})
		settle(t, health, func(got *pb.FirewallPolicyRule) bool {
			return !got.GetEnableLogging() && got.GetDescription() == "" && got.GetRuleName() == ""
		}, 1000)
	})

	t.Run("lists taken off the spec are cleared", func(t *testing.T) {
		update(t, health, func(o *computev1beta1.ComputeNetworkFirewallPolicyRule) {
			o.Spec.Match.SrcIPRanges = nil
			o.Spec.Match.SrcRegionCodes = []string{"US"}
			o.Spec.Match.Layer4Configs[0].Ports = nil
		})
		settle(t, health, func(got *pb.FirewallPolicyRule) bool {
			return len(got.GetMatch().GetSrcRegionCodes()) == 1
		}, 1000)
		got, _ := cloudRule(t, policyName, 1000)
		if r := got.GetMatch().GetSrcIpRanges(); len(r) != 0 {
			t.Errorf("source ranges still in the cloud rule: %v", r)
		}
		if l := got.GetMatch().GetLayer4Configs(); len(l) != 1 || len(l[0].GetPorts()) != 0 {
			t.Errorf("ports still in the cloud rule: %v", l)
		}
		before := writes()
		resync(t, health)
		time.Sleep(3 * time.Second)
		env.WaitReady(ruleGVK, namespace, health)
		if n := writes() - before; n != 0 {
			t.Errorf("a resync wrote %d times", n)
		}
	})

	t.Run("the source network context is set and taken off", func(t *testing.T) {
		update(t, health, func(o *computev1beta1.ComputeNetworkFirewallPolicyRule) {
			o.Spec.Match.SrcNetworkContext = mockenv.Ptr("INTERNET")
		})
		settle(t, health, func(got *pb.FirewallPolicyRule) bool {
			return got.GetMatch().GetSrcNetworkContext() == "INTERNET"
		}, 1000)
		before := writes()
		resync(t, health)
		time.Sleep(3 * time.Second)
		env.WaitReady(ruleGVK, namespace, health)
		if n := writes() - before; n != 0 {
			t.Errorf("a resync wrote %d times", n)
		}
		update(t, health, func(o *computev1beta1.ComputeNetworkFirewallPolicyRule) {
			o.Spec.Match.SrcNetworkContext = nil
		})
		settle(t, health, func(got *pb.FirewallPolicyRule) bool {
			// Cleared with UNSPECIFIED; the API may answer either.
			c := got.GetMatch().GetSrcNetworkContext()
			return c == "" || c == "UNSPECIFIED"
		}, 1000)
		before = writes()
		resync(t, health)
		time.Sleep(3 * time.Second)
		env.WaitReady(ruleGVK, namespace, health)
		if n := writes() - before; n != 0 {
			t.Errorf("a resync after clearing wrote %d times", n)
		}
	})

	t.Run("a rule changed out of band is put back", func(t *testing.T) {
		got, _ := cloudRule(t, policyName, 1000)
		got.Action = mockenv.Ptr("deny")
		// A rule just changed is "not ready" to the API for a while, and a
		// patch then answers 400: try again.
		var patchErr error
		for i := 0; i < 60; i++ {
			var op *gcp.Operation
			op, patchErr = policies.PatchRule(env.Ctx, &pb.PatchRuleNetworkFirewallPolicyRequest{
				Project: mockenv.Project, FirewallPolicy: policyName, Priority: mockenv.Ptr(int32(1000)),
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
		settle(t, health, func(got *pb.FirewallPolicyRule) bool { return got.GetAction() == "allow" }, 1000)
	})

	t.Run("another priority is another rule", func(t *testing.T) {
		const egress = "egress-dns"
		r := newRule(egress, 2000)
		r.Spec.Direction = "EGRESS"
		r.Spec.Action = "deny"
		r.Spec.Description = nil
		r.Spec.Match = computev1beta1.NetworkFirewallPolicyRuleMatch{
			DestIPRanges:  []string{"192.0.2.0/24"},
			Layer4Configs: []computev1beta1.NetworkFirewallPolicyRuleLayer4Config{{IPProtocol: "udp", Ports: []string{"53"}}},
		}
		r.Spec.TargetServiceAccounts = []iamrefs.IAMServiceAccountRef{{External: "app@" + mockenv.Project + ".iam.gserviceaccount.com"}}
		r.Spec.ProjectRef = &refsv1beta1.ProjectRef{External: mockenv.Project}
		if err := env.Kube.Create(env.Ctx, r); err != nil {
			t.Fatal(err)
		}
		env.WaitReady(ruleGVK, namespace, egress)
		got, err := cloudRule(t, policyName, 2000)
		if err != nil {
			t.Fatal(err)
		}
		if got.GetDirection() != "EGRESS" || got.GetAction() != "deny" || len(got.GetTargetServiceAccounts()) != 1 || !strings.HasPrefix(got.GetTargetServiceAccounts()[0], "app@") {
			t.Errorf("cloud rule = %v", got)
		}
		if first, err := cloudRule(t, policyName, 1000); err != nil || first.GetDirection() != "INGRESS" {
			t.Errorf("the first rule changed: %v, %v", first, err)
		}
		// Deleting one leaves the other.
		if err := env.Kube.Delete(env.Ctx, r); err != nil {
			t.Fatal(err)
		}
		env.WaitGone(r)
		if _, err := cloudRule(t, policyName, 2000); err == nil {
			t.Error("the deleted rule is still in the policy")
		}
		if _, err := cloudRule(t, policyName, 1000); err != nil {
			t.Errorf("deleting one rule took the other: %v", err)
		}
	})

	t.Run("a projectRef that is not the policy's project is refused", func(t *testing.T) {
		r := newRule("other-project", 2500)
		r.Spec.ProjectRef = &refsv1beta1.ProjectRef{External: "projects/not-" + mockenv.Project}
		if err := env.Kube.Create(env.Ctx, r); err != nil {
			t.Fatal(err)
		}
		if msg := env.WaitNotReady(ruleGVK, namespace, "other-project"); !strings.Contains(msg, "spec.projectRef is project") {
			t.Errorf("Ready message = %q, want the project mismatch", msg)
		}
		if _, err := cloudRule(t, policyName, 2500); err == nil {
			t.Error("a rule was added for a refused object")
		}
		if err := env.Kube.Delete(env.Ctx, r); err != nil {
			t.Fatal(err)
		}
		env.WaitGone(r)
	})

	t.Run("the priority and the policy cannot change", func(t *testing.T) {
		o := getObj(t, health)
		o.Spec.Priority = 1001
		if err := env.Kube.Update(env.Ctx, o); err == nil || !strings.Contains(err.Error(), "priority is immutable") {
			t.Errorf("changing the priority: err = %v", err)
		}
		o = getObj(t, health)
		o.Spec.FirewallPolicyRef = kcccomputev1beta1.ComputeNetworkFirewallPolicyRef{External: "projects/" + mockenv.Project + "/global/firewallPolicies/other"}
		if err := env.Kube.Update(env.Ctx, o); err == nil || !strings.Contains(err.Error(), "firewallPolicyRef is immutable") {
			t.Errorf("changing the policy: err = %v", err)
		}
	})

	t.Run("a Config Connector policy is named by object once it has a selfLink", func(t *testing.T) {
		// Config Connector's ComputeNetworkFirewallPolicy is not run here;
		// the test writes its status as Config Connector would. The object
		// is made before the rule and given its status within the test, so
		// no reference is left unresolved at the end (a watch on it would
		// keep kube-apiserver from stopping).
		byObjPath := newPolicy(t, "by-object")
		byObjName := byObjPath[strings.LastIndex(byObjPath, "/")+1:]
		const kccObj = "kcc-policy"
		kcc := &unstructured.Unstructured{Object: map[string]any{"spec": map[string]any{
			"projectRef": map[string]any{"external": mockenv.Project},
			"resourceID": byObjName,
		}}}
		kcc.SetGroupVersionKind(kcccomputev1beta1.ComputeNetworkFirewallPolicyGVK)
		kcc.SetNamespace(namespace)
		kcc.SetName(kccObj)
		if err := env.Kube.Create(env.Ctx, kcc); err != nil {
			t.Fatal(err)
		}
		r := newRule("by-object", 3000)
		r.Spec.FirewallPolicyRef = kcccomputev1beta1.ComputeNetworkFirewallPolicyRef{Name: kccObj}
		if err := env.Kube.Create(env.Ctx, r); err != nil {
			t.Fatal(err)
		}
		if msg := env.WaitNotReady(ruleGVK, namespace, "by-object"); !strings.Contains(msg, kccObj) {
			t.Errorf("the message does not name the policy that is not ready: %q", msg)
		}
		if err := unstructured.SetNestedField(kcc.Object, "https://www.googleapis.com/compute/v1/"+byObjPath, "status", "selfLink"); err != nil {
			t.Fatal(err)
		}
		if err := unstructured.SetNestedSlice(kcc.Object, []any{map[string]any{
			"type": "Ready", "status": "True", "reason": "UpToDate", "message": "set by the test",
			"lastTransitionTime": time.Now().UTC().Format(time.RFC3339),
		}}, "status", "conditions"); err != nil {
			t.Fatal(err)
		}
		if err := env.Kube.Status().Update(env.Ctx, kcc); err != nil {
			t.Fatal(err)
		}
		resync(t, "by-object")
		u := env.WaitReady(ruleGVK, namespace, "by-object")
		if ref, _, _ := unstructured.NestedString(u.Object, "status", "externalRef"); ref != byObjPath+"/rules/3000" {
			t.Errorf("status.externalRef = %q", ref)
		}
		if got, err := cloudRule(t, byObjName, 3000); err != nil || got.GetAction() != "allow" {
			t.Errorf("the rule in the policy named by object: %v, %v", got, err)
		}
		if err := env.Kube.Delete(env.Ctx, r); err != nil {
			t.Fatal(err)
		}
		env.WaitGone(r)
		if _, err := cloudRule(t, byObjName, 3000); err == nil {
			t.Error("the deleted rule is still in the policy")
		}
		if err := env.Kube.Delete(env.Ctx, kcc); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("delete removes the rule", func(t *testing.T) {
		if err := env.Kube.Delete(env.Ctx, getObj(t, health)); err != nil {
			t.Fatal(err)
		}
		env.WaitGone(&computev1beta1.ComputeNetworkFirewallPolicyRule{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: health}})
		_, err := cloudRule(t, policyName, 1000)
		if err == nil {
			t.Fatal("the rule is still in the policy")
		}
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
		if _, err := cloudRule(t, policyName, 4000); err != nil {
			t.Errorf("the abandoned rule is gone: %v", err)
		}
	})
}
