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

// ComputeRegionSecurityPolicyRule (ours: a regional Cloud Armor policy's
// rule) end to end on the fake cloud (pkg/test/mockenv; mockgcp patch 0063)
// or a real project (docs/live-runs.md). The cases: a CLOUD_ARMOR policy made
// by a ComputeRegionSecurityPolicy object, named by the rules by name; a
// rule by source ranges, then an expression with a rate limit and WAF
// exclusions, set and taken off; the policy's default rule taken over at its
// priority (and left by a delete); immutability; delete and abandon. And,
// on the fake only (Cloud Armor Enterprise), a CLOUD_ARMOR_NETWORK policy's
// rule on user-defined fields.

import (
	"slices"
	"strings"
	"testing"
	"time"

	refs "github.com/GoogleCloudPlatform/k8s-config-connector/apis/refs/v1beta1"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/k8s"
	computeapi "google.golang.org/api/compute/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"

	computev1alpha1 "github.com/highfivelabs/config-connector-extensions/apis/compute/v1alpha1"
	"github.com/highfivelabs/config-connector-extensions/pkg/test/mockenv"
)

// regionSecurityPolicyRuleEnv is what the two tests share: the cluster and
// the cloud, a policy made by an object, and helpers on its rules.
type regionSecurityPolicyRuleEnv struct {
	env    *mockenv.Env
	svc    *computeapi.Service
	policy string // the ComputeRegionSecurityPolicy object's name, and the policy's
}

const securityPolicyRegion = "us-central1"

func newRegionSecurityPolicyRuleEnv(t *testing.T, policyType string, edit func(*computev1alpha1.ComputeRegionSecurityPolicySpec)) *regionSecurityPolicyRuleEnv {
	env := mockenv.New(t, namespace)
	svc, err := computeapi.NewService(env.Ctx, env.GCPOptions()...)
	if err != nil {
		t.Fatal(err)
	}
	e := &regionSecurityPolicyRuleEnv{env: env, svc: svc, policy: mockenv.Name("armor")}
	policy := &computev1alpha1.ComputeRegionSecurityPolicy{
		ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: e.policy, Labels: mockenv.Labels()},
		Spec: computev1alpha1.ComputeRegionSecurityPolicySpec{
			ProjectRef:  &refs.ProjectRef{External: mockenv.Project},
			Region:      securityPolicyRegion,
			Type:        mockenv.Ptr(policyType),
			Description: mockenv.Ptr("regional security policy rules under test"),
		},
	}
	if edit != nil {
		edit(&policy.Spec)
	}
	if err := env.Kube.Create(env.Ctx, policy); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		// Rules go with the policy.
		if err := env.Kube.Delete(env.Ctx, policy); err != nil {
			t.Errorf("deleting the policy object: %v", err)
			return
		}
		env.WaitGone(&computev1alpha1.ComputeRegionSecurityPolicy{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: e.policy}})
	})
	env.WaitReady(computev1alpha1.ComputeRegionSecurityPolicyGVK, namespace, e.policy)
	return e
}

func (e *regionSecurityPolicyRuleEnv) path() string {
	return "projects/" + mockenv.Project + "/regions/" + securityPolicyRegion + "/securityPolicies/" + e.policy
}

func (e *regionSecurityPolicyRuleEnv) cloudRule(priority int64) (*computeapi.SecurityPolicyRule, error) {
	return e.svc.RegionSecurityPolicies.GetRule(mockenv.Project, securityPolicyRegion, e.policy).Priority(priority).Context(e.env.Ctx).Do()
}

func (e *regionSecurityPolicyRuleEnv) writes(method string) int {
	n := 0
	for _, r := range e.env.Requests() {
		if strings.HasPrefix(r, "POST ") && strings.Contains(r, "/securityPolicies/"+e.policy+"/"+method) {
			n++
		}
	}
	return n
}

func (e *regionSecurityPolicyRuleEnv) get(t *testing.T, name string) *computev1alpha1.ComputeRegionSecurityPolicyRule {
	t.Helper()
	obj := &computev1alpha1.ComputeRegionSecurityPolicyRule{}
	if err := e.env.Kube.Get(e.env.Ctx, types.NamespacedName{Namespace: namespace, Name: name}, obj); err != nil {
		t.Fatal(err)
	}
	return obj
}

func (e *regionSecurityPolicyRuleEnv) update(t *testing.T, name string, change func(*computev1alpha1.ComputeRegionSecurityPolicyRule)) {
	t.Helper()
	o := e.get(t, name)
	change(o)
	if err := e.env.Kube.Update(e.env.Ctx, o); err != nil {
		t.Fatal(err)
	}
}

// resyncWritesNothing forces a reconcile and checks it wrote nothing.
func (e *regionSecurityPolicyRuleEnv) resyncWritesNothing(t *testing.T, name string) {
	t.Helper()
	before := e.writes("patchRule") + e.writes("addRule")
	e.update(t, name, func(o *computev1alpha1.ComputeRegionSecurityPolicyRule) {
		if o.Annotations == nil {
			o.Annotations = map[string]string{}
		}
		o.Annotations[k8s.InternalForceReconcileAnnotation] = time.Now().Format(time.RFC3339Nano)
	})
	time.Sleep(3 * time.Second)
	e.env.WaitReady(computev1alpha1.ComputeRegionSecurityPolicyRuleGVK, namespace, name)
	if n := e.writes("patchRule") + e.writes("addRule") - before; n != 0 {
		t.Errorf("a resync of %s wrote %d times", name, n)
	}
}

// settle waits until the cloud's rule at priority satisfies ok and the
// object is Ready again.
func (e *regionSecurityPolicyRuleEnv) settle(t *testing.T, name string, priority int64, ok func(*computeapi.SecurityPolicyRule) bool) {
	t.Helper()
	deadline := time.Now().Add(mockenv.Timeout())
	for {
		got, err := e.cloudRule(priority)
		if err == nil && ok(got) {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("the cloud rule at %d did not change: %+v, %v", priority, got, err)
		}
		time.Sleep(time.Second)
	}
	e.env.WaitReady(computev1alpha1.ComputeRegionSecurityPolicyRuleGVK, namespace, name)
}

func (e *regionSecurityPolicyRuleEnv) newRule(name string, priority int64) *computev1alpha1.ComputeRegionSecurityPolicyRule {
	return &computev1alpha1.ComputeRegionSecurityPolicyRule{
		ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: name, Labels: mockenv.Labels()},
		Spec: computev1alpha1.ComputeRegionSecurityPolicyRuleSpec{
			SecurityPolicyRef: computev1alpha1.ComputeRegionSecurityPolicyRef{Name: e.policy},
			Priority:          priority,
		},
	}
}

func TestComputeRegionSecurityPolicyRule(t *testing.T) {
	e := newRegionSecurityPolicyRuleEnv(t, "CLOUD_ARMOR", nil)
	gvk := computev1alpha1.ComputeRegionSecurityPolicyRuleGVK
	env := e.env

	const blocked = "blocked-ranges"
	r := e.newRule(blocked, 1000)
	r.Spec.Action = "deny(403)"
	r.Spec.Description = mockenv.Ptr("two documentation ranges are refused")
	r.Spec.Match = &computev1alpha1.SecurityPolicyRuleMatcher{
		VersionedExpr: mockenv.Ptr("SRC_IPS_V1"),
		Config:        &computev1alpha1.SecurityPolicyRuleMatcherConfig{SrcIPRanges: []string{"192.0.2.0/24", "198.51.100.0/24"}},
	}
	if err := env.Kube.Create(env.Ctx, r); err != nil {
		t.Fatal(err)
	}

	t.Run("create adds the rule at its priority", func(t *testing.T) {
		u := env.WaitReady(gvk, namespace, blocked)
		got, err := e.cloudRule(1000)
		if err != nil {
			t.Fatalf("reading the rule from the cloud: %v", err)
		}
		if got.Action != "deny(403)" || got.Description != "two documentation ranges are refused" || got.Preview {
			t.Errorf("cloud rule = %+v", got)
		}
		if got.Match == nil || got.Match.VersionedExpr != "SRC_IPS_V1" || got.Match.Config == nil ||
			!slices.Equal(got.Match.Config.SrcIpRanges, []string{"192.0.2.0/24", "198.51.100.0/24"}) {
			t.Errorf("cloud match = %+v", got.Match)
		}
		want := e.path() + "/rules/1000"
		if ref, _, _ := unstructured.NestedString(u.Object, "status", "externalRef"); ref != want {
			t.Errorf("status.externalRef = %q, want %q", ref, want)
		}
		if n := e.writes("patchRule"); n != 0 {
			t.Errorf("create patched the rule %d times, want 0", n)
		}
	})

	t.Run("a resync writes nothing", func(t *testing.T) {
		e.resyncWritesNothing(t, blocked)
	})

	t.Run("a match by expression with a rate limit and WAF exclusions is set and taken off", func(t *testing.T) {
		e.update(t, blocked, func(o *computev1alpha1.ComputeRegionSecurityPolicyRule) {
			o.Spec.Action = "throttle"
			o.Spec.Description = nil
			o.Spec.Preview = mockenv.Ptr(true)
			o.Spec.Match = &computev1alpha1.SecurityPolicyRuleMatcher{
				Expr: &computev1alpha1.SecurityPolicyRuleMatcherExpr{Expression: mockenv.Ptr("request.path.matches('/login') && evaluatePreconfiguredWaf('sqli-v33-stable')")},
			}
			o.Spec.RateLimitOptions = &computev1alpha1.SecurityPolicyRuleRateLimitOptions{
				ConformAction:      mockenv.Ptr("allow"),
				ExceedAction:       mockenv.Ptr("deny(429)"),
				RateLimitThreshold: &computev1alpha1.SecurityPolicyRuleRateLimitOptionsThreshold{Count: mockenv.Ptr(int32(100)), IntervalSec: mockenv.Ptr(int32(60))},
				EnforceOnKeyConfigs: []computev1alpha1.SecurityPolicyRuleRateLimitOptionsEnforceOnKeyConfig{
					{EnforceOnKeyType: mockenv.Ptr("IP")},
					{EnforceOnKeyType: mockenv.Ptr("HTTP_HEADER"), EnforceOnKeyName: mockenv.Ptr("x-tenant")},
				},
			}
			o.Spec.PreconfiguredWafConfig = &computev1alpha1.SecurityPolicyRulePreconfiguredWafConfig{
				Exclusion: []computev1alpha1.SecurityPolicyRulePreconfiguredWafConfigExclusion{{
					TargetRuleSet: mockenv.Ptr("sqli-v33-stable"),
					TargetRuleIds: []string{"owasp-crs-v030301-id942110-sqli"},
					RequestHeader: []computev1alpha1.SecurityPolicyRulePreconfiguredWafConfigExclusionFieldParams{
						{Operator: mockenv.Ptr("EQUALS"), Value: mockenv.Ptr("x-session")},
					},
				}},
			}
		})
		e.settle(t, blocked, 1000, func(got *computeapi.SecurityPolicyRule) bool {
			return got.Action == "throttle" && got.Preview && got.Description == "" && got.Match != nil && got.Match.Expr != nil &&
				got.Match.Config == nil && got.RateLimitOptions != nil && len(got.RateLimitOptions.EnforceOnKeyConfigs) == 2 &&
				got.PreconfiguredWafConfig != nil && len(got.PreconfiguredWafConfig.Exclusions) == 1
		})
		got, _ := e.cloudRule(1000)
		x := got.PreconfiguredWafConfig.Exclusions[0]
		if x.TargetRuleSet != "sqli-v33-stable" || len(x.RequestHeadersToExclude) != 1 || x.RequestHeadersToExclude[0].Op != "EQUALS" || x.RequestHeadersToExclude[0].Val != "x-session" {
			t.Errorf("cloud WAF exclusion = %+v", x)
		}
		if rl := got.RateLimitOptions; rl.RateLimitThreshold == nil || rl.RateLimitThreshold.Count != 100 || rl.RateLimitThreshold.IntervalSec != 60 || rl.ExceedAction != "deny(429)" {
			t.Errorf("cloud rate limit = %+v", rl)
		}
		e.resyncWritesNothing(t, blocked)

		e.update(t, blocked, func(o *computev1alpha1.ComputeRegionSecurityPolicyRule) {
			o.Spec.Action = "deny(403)"
			o.Spec.Preview = nil
			o.Spec.RateLimitOptions = nil
			o.Spec.PreconfiguredWafConfig = nil
		})
		e.settle(t, blocked, 1000, func(got *computeapi.SecurityPolicyRule) bool {
			return got.Action == "deny(403)" && !got.Preview && got.RateLimitOptions == nil && got.PreconfiguredWafConfig == nil
		})
		e.resyncWritesNothing(t, blocked)
	})

	t.Run("the default rule is taken over at its priority and left by a delete", func(t *testing.T) {
		before, err := e.cloudRule(2147483647)
		if err != nil {
			t.Fatalf("the policy's default rule: %v", err)
		}
		adds := e.writes("addRule")
		def := e.newRule("default-deny", 2147483647)
		def.Spec.Action = "deny(404)"
		def.Spec.Description = mockenv.Ptr("default rule")
		def.Spec.Match = &computev1alpha1.SecurityPolicyRuleMatcher{
			VersionedExpr: mockenv.Ptr("SRC_IPS_V1"),
			Config:        &computev1alpha1.SecurityPolicyRuleMatcherConfig{SrcIPRanges: []string{"*"}},
		}
		if err := env.Kube.Create(env.Ctx, def); err != nil {
			t.Fatal(err)
		}
		e.settle(t, "default-deny", 2147483647, func(got *computeapi.SecurityPolicyRule) bool { return got.Action == "deny(404)" })
		if n := e.writes("addRule") - adds; n != 0 {
			t.Errorf("taking the default rule over added %d rules, want 0", n)
		}
		removes := e.writes("removeRule")
		if err := env.Kube.Delete(env.Ctx, e.get(t, "default-deny")); err != nil {
			t.Fatal(err)
		}
		env.WaitGone(&computev1alpha1.ComputeRegionSecurityPolicyRule{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: "default-deny"}})
		if n := e.writes("removeRule") - removes; n != 0 {
			t.Errorf("deleting the default rule's object called removeRule %d times, want 0", n)
		}
		got, err := e.cloudRule(2147483647)
		if err != nil || got.Action != "deny(404)" {
			t.Errorf("after the delete, the default rule = %+v, %v (was %s); want it kept as it was", got, err, before.Action)
		}
		// Put the default back, so the live policy is as it was made.
		op, err := e.svc.RegionSecurityPolicies.PatchRule(mockenv.Project, securityPolicyRegion, e.policy, &computeapi.SecurityPolicyRule{Action: "allow"}).
			Priority(2147483647).UpdateMask("action").Context(env.Ctx).Do()
		if err == nil {
			_, err = e.svc.RegionOperations.Wait(mockenv.Project, securityPolicyRegion, op.Name).Context(env.Ctx).Do()
		}
		if err != nil {
			t.Errorf("putting the default rule back: %v", err)
		}
	})

	t.Run("the CRD refuses a rule without a match, and changes of the priority and the policy", func(t *testing.T) {
		bad := e.newRule("no-match", 1100)
		bad.Spec.Action = "allow"
		if err := env.Kube.Create(env.Ctx, bad); err == nil || !strings.Contains(err.Error(), "exactly one of match") {
			t.Errorf("creating a rule without a match: %v, want refused", err)
		}
		o := e.get(t, blocked)
		o.Spec.Priority = 1001
		if err := env.Kube.Update(env.Ctx, o); err == nil || !strings.Contains(err.Error(), "priority is immutable") {
			t.Errorf("changing the priority: %v, want refused", err)
		}
		o = e.get(t, blocked)
		o.Spec.SecurityPolicyRef = computev1alpha1.ComputeRegionSecurityPolicyRef{External: e.path()}
		if err := env.Kube.Update(env.Ctx, o); err == nil || !strings.Contains(err.Error(), "securityPolicyRef is immutable") {
			t.Errorf("changing the policy: %v, want refused", err)
		}
	})

	t.Run("abandon keeps the rule; an object by external takes it over", func(t *testing.T) {
		e.update(t, blocked, func(o *computev1alpha1.ComputeRegionSecurityPolicyRule) {
			o.Annotations = map[string]string{"cnrm.cloud.google.com/deletion-policy": "abandon"}
		})
		if err := env.Kube.Delete(env.Ctx, e.get(t, blocked)); err != nil {
			t.Fatal(err)
		}
		env.WaitGone(&computev1alpha1.ComputeRegionSecurityPolicyRule{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: blocked}})
		if _, err := e.cloudRule(1000); err != nil {
			t.Fatalf("after abandon, reading the rule at 1000: %v, want it kept", err)
		}
		adds := e.writes("addRule")
		again := e.newRule("blocked-again", 1000)
		again.Spec.SecurityPolicyRef = computev1alpha1.ComputeRegionSecurityPolicyRef{External: e.path()}
		again.Spec.Action = "deny(403)"
		again.Spec.Description = mockenv.Ptr("two documentation ranges are refused")
		again.Spec.Match = &computev1alpha1.SecurityPolicyRuleMatcher{
			VersionedExpr: mockenv.Ptr("SRC_IPS_V1"),
			Config:        &computev1alpha1.SecurityPolicyRuleMatcherConfig{SrcIPRanges: []string{"192.0.2.0/24"}},
		}
		if err := env.Kube.Create(env.Ctx, again); err != nil {
			t.Fatal(err)
		}
		e.settle(t, "blocked-again", 1000, func(got *computeapi.SecurityPolicyRule) bool {
			return got.Match != nil && got.Match.Config != nil && slices.Equal(got.Match.Config.SrcIpRanges, []string{"192.0.2.0/24"})
		})
		if n := e.writes("addRule") - adds; n != 0 {
			t.Errorf("taking the rule over added %d rules, want 0", n)
		}
	})

	t.Run("delete removes the rule", func(t *testing.T) {
		if err := env.Kube.Delete(env.Ctx, e.get(t, "blocked-again")); err != nil {
			t.Fatal(err)
		}
		env.WaitGone(&computev1alpha1.ComputeRegionSecurityPolicyRule{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: "blocked-again"}})
		if _, err := e.cloudRule(1000); err == nil {
			t.Errorf("after delete, the rule at 1000 is still in the policy")
		}
	})
}

func TestComputeRegionSecurityPolicyRuleNetwork(t *testing.T) {
	if mockenv.Live() {
		t.Skip("a CLOUD_ARMOR_NETWORK policy needs Cloud Armor Enterprise, which the live-test project does not have (as TestComputeRegionSecurityPolicyNetwork)")
	}
	e := newRegionSecurityPolicyRuleEnv(t, "CLOUD_ARMOR_NETWORK", func(s *computev1alpha1.ComputeRegionSecurityPolicySpec) {
		s.UserDefinedFields = []computev1alpha1.ComputeRegionSecurityPolicySecurityPolicyUserDefinedField{{
			Name:   mockenv.Ptr("gtp-teid"),
			Base:   mockenv.Ptr("UDP"),
			Offset: mockenv.Ptr(int32(12)),
			Size:   mockenv.Ptr(int32(4)),
		}}
	})
	env := e.env
	const udf = "gtp-tunnels"
	r := e.newRule(udf, 500)
	r.Spec.Action = "deny"
	r.Spec.NetworkMatch = &computev1alpha1.SecurityPolicyRuleNetworkMatcher{
		IPProtocols: []string{"udp"},
		DestPorts:   []string{"2152"},
		UserDefinedFields: []computev1alpha1.SecurityPolicyRuleNetworkMatcherUserDefinedFieldMatch{
			{Name: "gtp-teid", Values: []string{"0x100-0x1ff"}},
		},
	}
	if err := env.Kube.Create(env.Ctx, r); err != nil {
		t.Fatal(err)
	}
	e.settle(t, udf, 500, func(got *computeapi.SecurityPolicyRule) bool {
		return got.NetworkMatch != nil && len(got.NetworkMatch.UserDefinedFields) == 1 && got.NetworkMatch.UserDefinedFields[0].Name == "gtp-teid"
	})
	e.resyncWritesNothing(t, udf)

	e.update(t, udf, func(o *computev1alpha1.ComputeRegionSecurityPolicyRule) {
		o.Spec.NetworkMatch.UserDefinedFields = nil
		o.Spec.NetworkMatch.SrcRegionCodes = []string{"AQ"}
		o.Spec.NetworkMatch.SrcAsns = []int64{64512}
	})
	e.settle(t, udf, 500, func(got *computeapi.SecurityPolicyRule) bool {
		n := got.NetworkMatch
		return n != nil && len(n.UserDefinedFields) == 0 && slices.Equal(n.SrcRegionCodes, []string{"AQ"}) && slices.Equal(n.SrcAsns, []int64{64512}) &&
			slices.Equal(n.DestPorts, []string{"2152"})
	})
	e.resyncWritesNothing(t, udf)

	if err := env.Kube.Delete(env.Ctx, e.get(t, udf)); err != nil {
		t.Fatal(err)
	}
	env.WaitGone(&computev1alpha1.ComputeRegionSecurityPolicyRule{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: udf}})
	if _, err := e.cloudRule(500); err == nil {
		t.Errorf("after delete, the rule at 500 is still in the policy")
	}
}
