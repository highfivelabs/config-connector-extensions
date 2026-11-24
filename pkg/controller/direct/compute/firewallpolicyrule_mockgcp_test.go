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

// ComputeFirewallPolicyRule (ours: a hierarchical policy's rule with the
// fields Config Connector's lacks) end to end on the fake cloud
// (pkg/test/mockenv; mockgcp patch 0061) or a real folder (docs/live-runs.md:
// the test folder in E2E_TEST_FOLDER, where the live-test service account
// administers firewall policies). The case: a folder's policy, a rule with
// a name and a source network context, then a security profile group, TLS
// inspection, source networks and a target network set and taken off; and a
// rule already in the policy (as Config Connector's kind would have made
// it) taken over by an object of ours at its priority. The policy is made
// in the cloud, under the folder, and named by external; the test deletes
// it.

import (
	"os"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	computerefs "github.com/GoogleCloudPlatform/k8s-config-connector/apis/compute/refs"
	kcccomputev1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/apis/compute/v1beta1"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/k8s"
	computeapi "google.golang.org/api/compute/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"

	computev1alpha1 "github.com/highfivelabs/config-connector-extensions/apis/compute/v1alpha1"
	computev1beta1 "github.com/highfivelabs/config-connector-extensions/apis/compute/v1beta1"
	"github.com/highfivelabs/config-connector-extensions/pkg/test/mockenv"
)

func TestComputeFirewallPolicyRule(t *testing.T) {
	if mockenv.Live() && os.Getenv("E2E_TEST_FOLDER") == "" {
		t.Skip("a live hierarchical firewall policy needs the test folder, E2E_TEST_FOLDER, where the live-test service account administers firewall policies (docs/live-runs.md)")
	}
	gvk := computev1alpha1.ComputeFirewallPolicyRuleGVK
	env := mockenv.New(t, namespace)
	svc, err := computeapi.NewService(env.Ctx, env.GCPOptions()...)
	if err != nil {
		t.Fatal(err)
	}

	// The folder's policy, made in the cloud and deleted when the test ends
	// (with what rules are left in it, the abandoned one included).
	op, err := svc.FirewallPolicies.Insert(&computeapi.FirewallPolicy{
		ShortName:   mockenv.Name("rules"),
		Description: "hierarchical firewall policy rules under test",
	}).ParentId(mockenv.TestFolder(t)).Context(env.Ctx).Do()
	if err == nil {
		op, err = waitOrganizationOperation(env.Ctx, svc, op)
	}
	if err != nil {
		t.Fatalf("making the folder's firewall policy: %v", err)
	}
	policyID := strconv.FormatUint(op.TargetId, 10)
	if op.TargetId == 0 {
		policyID = op.TargetLink[strings.LastIndex(op.TargetLink, "/")+1:]
	}
	t.Cleanup(func() {
		op, err := svc.FirewallPolicies.Delete(policyID).Context(env.Ctx).Do()
		if err == nil {
			_, err = waitOrganizationOperation(env.Ctx, svc, op)
		}
		if err != nil {
			t.Errorf("deleting firewall policy %s: %v", policyID, err)
		}
	})
	policyPath := "locations/global/firewallPolicies/" + policyID

	cloudRule := func(priority int64) (*computeapi.FirewallPolicyRule, error) {
		return svc.FirewallPolicies.GetRule(policyID).Priority(priority).Context(env.Ctx).Do()
	}
	missing := func(err error) bool {
		return err != nil && (strings.Contains(err.Error(), "404") || strings.Contains(err.Error(), "does not contain a rule"))
	}
	writes := func(method string) int {
		n := 0
		for _, r := range env.Requests() {
			if strings.HasPrefix(r, "POST ") && strings.Contains(r, "/firewallPolicies/"+policyID+"/"+method) {
				n++
			}
		}
		return n
	}
	getObj := func(t *testing.T, name string) *computev1alpha1.ComputeFirewallPolicyRule {
		t.Helper()
		obj := &computev1alpha1.ComputeFirewallPolicyRule{}
		if err := env.Kube.Get(env.Ctx, types.NamespacedName{Namespace: namespace, Name: name}, obj); err != nil {
			t.Fatal(err)
		}
		return obj
	}
	update := func(t *testing.T, name string, change func(*computev1alpha1.ComputeFirewallPolicyRule)) {
		t.Helper()
		o := getObj(t, name)
		change(o)
		if err := env.Kube.Update(env.Ctx, o); err != nil {
			t.Fatal(err)
		}
	}
	resync := func(t *testing.T, name string) {
		t.Helper()
		update(t, name, func(o *computev1alpha1.ComputeFirewallPolicyRule) {
			if o.Annotations == nil {
				o.Annotations = map[string]string{}
			}
			o.Annotations[k8s.InternalForceReconcileAnnotation] = time.Now().Format(time.RFC3339Nano)
		})
	}
	// settle waits until the cloud's rule at priority satisfies ok and the
	// object is Ready again.
	settle := func(t *testing.T, name string, priority int64, ok func(*computeapi.FirewallPolicyRule) bool) {
		t.Helper()
		deadline := time.Now().Add(mockenv.Timeout())
		for {
			got, err := cloudRule(priority)
			if err == nil && ok(got) {
				break
			}
			if time.Now().After(deadline) {
				t.Fatalf("the cloud rule at %d did not change: %+v, %v", priority, got, err)
			}
			time.Sleep(time.Second)
		}
		env.WaitReady(gvk, namespace, name)
	}
	newRule := func(name string, priority int64) *computev1alpha1.ComputeFirewallPolicyRule {
		return &computev1alpha1.ComputeFirewallPolicyRule{
			ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: name, Labels: mockenv.Labels()},
			Spec: computev1alpha1.ComputeFirewallPolicyRuleSpec{
				FirewallPolicyRef: kcccomputev1beta1.ComputeFirewallPolicyRef{External: policyPath},
				Priority:          priority,
				Action:            "allow",
				Direction:         "INGRESS",
				Description:       mockenv.Ptr("health checks reach every VM"),
				Match: computev1alpha1.FirewallPolicyRuleMatcher{
					SrcIPRanges: []string{"35.191.0.0/16", "130.211.0.0/22"},
					Layer4Configs: []computev1alpha1.FirewallPolicyRuleMatcherLayer4Config{
						{IPProtocol: "tcp", Ports: []string{"80", "443"}},
					},
				},
			},
		}
	}

	const health = "health-checks"
	r := newRule(health, 1000)
	r.Spec.RuleName = mockenv.Ptr("health-checks")
	r.Spec.Match.SrcNetworkContext = mockenv.Ptr("NON_INTERNET")
	if err := env.Kube.Create(env.Ctx, r); err != nil {
		t.Fatal(err)
	}

	t.Run("create adds the rule at its priority with the fields Config Connector's kind lacks", func(t *testing.T) {
		u := env.WaitReady(gvk, namespace, health)
		got, err := cloudRule(1000)
		if err != nil {
			t.Fatalf("reading the rule from the cloud: %v", err)
		}
		if got.Action != "allow" || got.Direction != "INGRESS" || got.Description != "health checks reach every VM" || got.RuleName != "health-checks" {
			t.Errorf("cloud rule = %+v", got)
		}
		if got.Match == nil || got.Match.SrcNetworkContext != "NON_INTERNET" || !slices.Equal(got.Match.SrcIpRanges, []string{"35.191.0.0/16", "130.211.0.0/22"}) {
			t.Errorf("cloud match = %+v", got.Match)
		}
		want := policyPath + "/rules/1000"
		if ref, _, _ := unstructured.NestedString(u.Object, "status", "externalRef"); ref != want {
			t.Errorf("status.externalRef = %q, want %q (Config Connector's spelling)", ref, want)
		}
		if n, _, _ := unstructured.NestedInt64(u.Object, "status", "ruleTupleCount"); n == 0 {
			t.Errorf("status.ruleTupleCount is not set")
		}
		if n := writes("patchRule"); n != 0 {
			t.Errorf("create patched the rule %d times, want 0", n)
		}
	})

	t.Run("a resync writes nothing", func(t *testing.T) {
		before := writes("patchRule") + writes("addRule")
		resync(t, health)
		time.Sleep(3 * time.Second)
		env.WaitReady(gvk, namespace, health)
		if n := writes("patchRule") + writes("addRule") - before; n != 0 {
			t.Errorf("a resync wrote %d times", n)
		}
	})

	t.Run("a security profile group without its action is refused by the CRD", func(t *testing.T) {
		bad := newRule("bad-group", 1100)
		bad.Spec.SecurityProfileGroup = mockenv.Ptr("//networksecurity.googleapis.com/organizations/123/locations/global/securityProfileGroups/g")
		if err := env.Kube.Create(env.Ctx, bad); err == nil || !strings.Contains(err.Error(), "securityProfileGroup is set exactly when action is apply_security_profile_group") {
			t.Errorf("creating a group on an allow rule: %v, want refused", err)
		}
		bad = newRule("bad-action", 1100)
		bad.Spec.Action = "apply_security_profile_group"
		if err := env.Kube.Create(env.Ctx, bad); err == nil || !strings.Contains(err.Error(), "securityProfileGroup is set exactly when action is apply_security_profile_group") {
			t.Errorf("creating the action without a group: %v, want refused", err)
		}
		bad = newRule("bad-tls", 1100)
		bad.Spec.TLSInspect = mockenv.Ptr(true)
		if err := env.Kube.Create(env.Ctx, bad); err == nil || !strings.Contains(err.Error(), "tlsInspect needs action apply_security_profile_group") {
			t.Errorf("creating TLS inspection on an allow rule: %v, want refused", err)
		}
	})

	t.Run("a security profile group, TLS inspection, source and target networks are set and taken off", func(t *testing.T) {
		// Live, the group must exist: an organization's security profile
		// group a rule under the test folder may use (Cloud NGFW
		// Enterprise), and the networks must be in the live-test project.
		group := "//networksecurity.googleapis.com/organizations/123456789/locations/global/securityProfileGroups/" + mockenv.Name("spg")
		if mockenv.Live() {
			group = os.Getenv("E2E_SECURITY_PROFILE_GROUP")
			if group == "" {
				t.Skip("E2E_SECURITY_PROFILE_GROUP is not set: a live apply_security_profile_group rule needs an existing security profile group (docs/kinds/ComputeFirewallPolicyRule.md)")
			}
		}
		src := "projects/" + mockenv.Project + "/global/networks/" + mockenv.Name("src")
		target := "projects/" + mockenv.Project + "/global/networks/" + mockenv.Name("target")
		update(t, health, func(o *computev1alpha1.ComputeFirewallPolicyRule) {
			o.Spec.Action = "apply_security_profile_group"
			o.Spec.SecurityProfileGroup = mockenv.Ptr(group)
			o.Spec.TLSInspect = mockenv.Ptr(true)
			o.Spec.Match.SrcNetworkContext = mockenv.Ptr("VPC_NETWORKS")
			o.Spec.Match.SrcNetworks = []string{src}
			o.Spec.TargetResources = append(o.Spec.TargetResources, &computerefs.ComputeNetworkRef{External: target})
		})
		settle(t, health, 1000, func(got *computeapi.FirewallPolicyRule) bool {
			return got.SecurityProfileGroup != "" && got.TlsInspect && got.Match != nil && len(got.Match.SrcNetworks) == 1 && len(got.TargetResources) == 1
		})
		got, _ := cloudRule(1000)
		if !strings.HasSuffix(got.TargetResources[0], target) {
			t.Errorf("cloud targetResources = %v, want the network %s", got.TargetResources, target)
		}
		before := writes("patchRule")
		resync(t, health)
		time.Sleep(3 * time.Second)
		env.WaitReady(gvk, namespace, health)
		if n := writes("patchRule") - before; n != 0 {
			t.Errorf("a resync wrote %d times (a URL the API answers for a path sent must compare equal)", n)
		}
		update(t, health, func(o *computev1alpha1.ComputeFirewallPolicyRule) {
			o.Spec.Action = "allow"
			o.Spec.SecurityProfileGroup = nil
			o.Spec.TLSInspect = nil
			o.Spec.Match.SrcNetworkContext = mockenv.Ptr("NON_INTERNET")
			o.Spec.Match.SrcNetworks = nil
			o.Spec.TargetResources = nil
		})
		settle(t, health, 1000, func(got *computeapi.FirewallPolicyRule) bool {
			return got.Action == "allow" && got.SecurityProfileGroup == "" && !got.TlsInspect && got.Match != nil &&
				len(got.Match.SrcNetworks) == 0 && len(got.TargetResources) == 0
		})
	})

	t.Run("secure tags are set and taken off", func(t *testing.T) {
		if mockenv.Live() {
			t.Skip("a live secure tag needs an organization's tag value bound for firewall use, which the test folder has none of")
		}
		update(t, health, func(o *computev1alpha1.ComputeFirewallPolicyRule) {
			o.Spec.TargetSecureTags = []computev1beta1.NetworkFirewallPolicyRuleSecureTag{{Name: "tagValues/111"}}
			o.Spec.Match.SrcSecureTags = []computev1beta1.NetworkFirewallPolicyRuleSecureTag{{Name: "tagValues/222"}}
		})
		settle(t, health, 1000, func(got *computeapi.FirewallPolicyRule) bool {
			return len(got.TargetSecureTags) == 1 && got.TargetSecureTags[0].Name == "tagValues/111" &&
				got.Match != nil && len(got.Match.SrcSecureTags) == 1 && got.Match.SrcSecureTags[0].Name == "tagValues/222"
		})
		update(t, health, func(o *computev1alpha1.ComputeFirewallPolicyRule) {
			o.Spec.TargetSecureTags = nil
			o.Spec.Match.SrcSecureTags = nil
		})
		settle(t, health, 1000, func(got *computeapi.FirewallPolicyRule) bool {
			return len(got.TargetSecureTags) == 0 && got.Match != nil && len(got.Match.SrcSecureTags) == 0
		})
	})

	t.Run("a rule already in the policy is taken over at its priority", func(t *testing.T) {
		// As Config Connector's ComputeFirewallPolicyRule would have made it;
		// moving to ours is an object of ours at the same priority, then
		// Config Connector's deleted with deletion-policy abandon.
		op, err := svc.FirewallPolicies.AddRule(policyID, &computeapi.FirewallPolicyRule{
			Priority:  2000,
			Action:    "deny",
			Direction: "EGRESS",
			Match: &computeapi.FirewallPolicyRuleMatcher{
				DestIpRanges:  []string{"0.0.0.0/0"},
				Layer4Configs: []*computeapi.FirewallPolicyRuleMatcherLayer4Config{{IpProtocol: "all"}},
			},
		}).Context(env.Ctx).Do()
		if err == nil {
			_, err = waitOrganizationOperation(env.Ctx, svc, op)
		}
		if err != nil {
			t.Fatalf("adding the rule at 2000 as Config Connector would: %v", err)
		}
		adds, patches := writes("addRule"), writes("patchRule")
		moved := newRule("egress-denied", 2000)
		moved.Spec.Action = "deny"
		moved.Spec.Direction = "EGRESS"
		moved.Spec.Description = nil
		moved.Spec.RuleName = mockenv.Ptr("egress-denied")
		moved.Spec.Match = computev1alpha1.FirewallPolicyRuleMatcher{
			DestIPRanges:       []string{"0.0.0.0/0"},
			DestNetworkContext: mockenv.Ptr("INTERNET"),
			Layer4Configs:      []computev1alpha1.FirewallPolicyRuleMatcherLayer4Config{{IPProtocol: "all"}},
		}
		if err := env.Kube.Create(env.Ctx, moved); err != nil {
			t.Fatal(err)
		}
		settle(t, "egress-denied", 2000, func(got *computeapi.FirewallPolicyRule) bool {
			return got.RuleName == "egress-denied" && got.Match != nil && got.Match.DestNetworkContext == "INTERNET"
		})
		if n := writes("addRule") - adds; n != 0 {
			t.Errorf("taking the rule over added %d rules, want 0", n)
		}
		if n := writes("patchRule") - patches; n != 1 {
			t.Errorf("taking the rule over patched it %d times, want 1", n)
		}
		if _, err := cloudRule(1000); err != nil {
			t.Errorf("the rule at 1000 after taking over 2000: %v", err)
		}
	})

	t.Run("the priority and the policy are immutable", func(t *testing.T) {
		o := getObj(t, health)
		o.Spec.Priority = 1001
		if err := env.Kube.Update(env.Ctx, o); err == nil || !strings.Contains(err.Error(), "priority is immutable") {
			t.Errorf("changing the priority: got %v, want the API server to refuse it", err)
		}
		o = getObj(t, health)
		o.Spec.FirewallPolicyRef = kcccomputev1beta1.ComputeFirewallPolicyRef{External: policyPath + "0"}
		if err := env.Kube.Update(env.Ctx, o); err == nil || !strings.Contains(err.Error(), "firewallPolicyRef is immutable") {
			t.Errorf("changing the policy: got %v, want the API server to refuse it", err)
		}
	})

	t.Run("delete removes the rule", func(t *testing.T) {
		if err := env.Kube.Delete(env.Ctx, getObj(t, health)); err != nil {
			t.Fatal(err)
		}
		env.WaitGone(&computev1alpha1.ComputeFirewallPolicyRule{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: health}})
		if _, err := cloudRule(1000); !missing(err) {
			t.Errorf("after delete, reading the rule at 1000: %v, want it missing", err)
		}
	})

	t.Run("abandon keeps the rule", func(t *testing.T) {
		update(t, "egress-denied", func(o *computev1alpha1.ComputeFirewallPolicyRule) {
			o.Annotations = map[string]string{"cnrm.cloud.google.com/deletion-policy": "abandon"}
		})
		if err := env.Kube.Delete(env.Ctx, getObj(t, "egress-denied")); err != nil {
			t.Fatal(err)
		}
		env.WaitGone(&computev1alpha1.ComputeFirewallPolicyRule{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: "egress-denied"}})
		if _, err := cloudRule(2000); err != nil {
			t.Errorf("after abandon, reading the rule at 2000: %v, want it kept", err)
		}
	})
}
