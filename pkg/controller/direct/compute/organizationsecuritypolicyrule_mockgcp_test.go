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

// ComputeOrganizationSecurityPolicyRule end to end on the fake cloud
// (pkg/test/mockenv, patch 0028). The case: a landing zone's hierarchical
// firewall baseline, as Magic Modules' organization_security_policy_rule
// example makes it: an organization policy (our kind) and its rules, an
// ingress allow for health checks and an egress deny, by reference and by
// policy ID, changed, put back after an out-of-band change, and deleted
// (one abandoned). The kind acts on an organization or a folder, so a live
// run skips it (docs/live-runs.md: live: pending, needs the test folder).

import (
	"strings"
	"testing"
	"time"

	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/k8s"
	computebeta "google.golang.org/api/compute/v0.beta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"

	computev1beta1 "github.com/highfivelabs/config-connector-extensions/apis/compute/v1beta1"
	"github.com/highfivelabs/config-connector-extensions/pkg/test/mockenv"
)

func TestComputeOrganizationSecurityPolicyRule(t *testing.T) {
	if mockenv.Live() {
		t.Skip("a live run never touches a folder or an organization (docs/live-runs.md); live: pending, needs the test folder")
	}
	const (
		namespace = "example"
		org       = "organizations/123456789"
		policyObj = "baseline"
	)
	policyGVK := computev1beta1.ComputeOrganizationSecurityPolicyGVK
	ruleGVK := computev1beta1.ComputeOrganizationSecurityPolicyRuleGVK
	env := mockenv.New(t, namespace)
	cloud, err := computebeta.NewService(env.Ctx, env.GCPOptions()...)
	if err != nil {
		t.Fatal(err)
	}

	policy := &computev1beta1.ComputeOrganizationSecurityPolicy{
		ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: policyObj},
		Spec: computev1beta1.ComputeOrganizationSecurityPolicySpec{
			DisplayName: "lz-baseline",
			Parent:      org,
			Type:        mockenv.Ptr("FIREWALL"),
		},
	}
	if err := env.Kube.Create(env.Ctx, policy); err != nil {
		t.Fatal(err)
	}
	u := env.WaitReady(policyGVK, namespace, policyObj)
	policyID, _, _ := unstructured.NestedString(u.Object, "status", "policyId")
	if policyID == "" {
		t.Fatal("the policy's status.policyId is empty")
	}

	cloudRule := func(t *testing.T, priority int64) (*computebeta.SecurityPolicyRule, error) {
		t.Helper()
		return cloud.OrganizationSecurityPolicies.GetRule(policyID).Priority(priority).Context(env.Ctx).Do()
	}
	writes := func() int {
		n := 0
		for _, r := range env.Requests() {
			if strings.HasPrefix(r, "POST ") && strings.Contains(r, "/securityPolicies/"+policyID+"/") &&
				(strings.Contains(r, "/addRule") || strings.Contains(r, "/patchRule") || strings.Contains(r, "/removeRule")) {
				n++
			}
		}
		return n
	}
	getObj := func(t *testing.T, name string) *computev1beta1.ComputeOrganizationSecurityPolicyRule {
		t.Helper()
		obj := &computev1beta1.ComputeOrganizationSecurityPolicyRule{}
		if err := env.Kube.Get(env.Ctx, types.NamespacedName{Namespace: namespace, Name: name}, obj); err != nil {
			t.Fatal(err)
		}
		return obj
	}
	update := func(t *testing.T, name string, change func(*computev1beta1.ComputeOrganizationSecurityPolicyRule)) {
		t.Helper()
		o := getObj(t, name)
		change(o)
		if err := env.Kube.Update(env.Ctx, o); err != nil {
			t.Fatal(err)
		}
	}
	resync := func(t *testing.T, name string) {
		t.Helper()
		update(t, name, func(o *computev1beta1.ComputeOrganizationSecurityPolicyRule) {
			o.Annotations = map[string]string{k8s.InternalForceReconcileAnnotation: time.Now().Format(time.RFC3339Nano)}
		})
	}
	newRule := func(name string, set func(*computev1beta1.ComputeOrganizationSecurityPolicyRuleSpec)) *computev1beta1.ComputeOrganizationSecurityPolicyRule {
		r := &computev1beta1.ComputeOrganizationSecurityPolicyRule{
			ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: name},
			Spec: computev1beta1.ComputeOrganizationSecurityPolicyRuleSpec{
				SecurityPolicyRef: &computev1beta1.ComputeOrganizationSecurityPolicyRef{Name: policyObj},
				Action:            "allow",
				Match: &computev1beta1.OrganizationSecurityPolicyRuleMatch{
					Config: &computev1beta1.OrganizationSecurityPolicyRuleMatchConfig{
						SrcIPRanges: []string{"130.211.0.0/22", "35.191.0.0/16"},
						Layer4Config: []computev1beta1.OrganizationSecurityPolicyRuleMatchConfigLayer4Config{
							{IPProtocol: "tcp", Ports: []string{"80", "443"}},
						},
					},
				},
			},
		}
		if set != nil {
			set(&r.Spec)
		}
		return r
	}

	const health = "health-checks"
	if err := env.Kube.Create(env.Ctx, newRule(health, func(s *computev1beta1.ComputeOrganizationSecurityPolicyRuleSpec) {
		s.ResourceID = mockenv.Ptr("1000")
		s.Description = mockenv.Ptr("health checks")
		s.Direction = mockenv.Ptr("INGRESS")
		s.EnableLogging = mockenv.Ptr(true)
		s.TargetResources = []string{"https://www.googleapis.com/compute/v1/projects/" + mockenv.Project + "/global/networks/shared-vpc"}
		s.TargetServiceAccounts = []string{"web@" + mockenv.Project + ".iam.gserviceaccount.com"}
	})); err != nil {
		t.Fatal(err)
	}

	t.Run("create adds the rule at its priority, with the beta fields", func(t *testing.T) {
		u := env.WaitReady(ruleGVK, namespace, health)
		got, err := cloudRule(t, 1000)
		if err != nil {
			t.Fatalf("reading the rule from the cloud: %v", err)
		}
		if got.Action != "allow" || got.Direction != "INGRESS" || got.Description != "health checks" || !got.EnableLogging || got.Preview {
			t.Errorf("cloud rule = action %q, direction %q, description %q, logging %t, preview %t", got.Action, got.Direction, got.Description, got.EnableLogging, got.Preview)
		}
		if got.Match == nil || got.Match.VersionedExpr != "FIREWALL" || got.Match.Config == nil {
			t.Fatalf("cloud match = %+v", got.Match)
		}
		if r := got.Match.Config.SrcIpRanges; len(r) != 2 || r[0] != "130.211.0.0/22" {
			t.Errorf("cloud source ranges = %v", r)
		}
		if l := got.Match.Config.Layer4Configs; len(l) != 1 || l[0].IpProtocol != "tcp" || len(l[0].Ports) != 2 {
			t.Errorf("cloud layer 4 configs = %v", l)
		}
		if len(got.TargetResources) != 1 || len(got.TargetServiceAccounts) != 1 {
			t.Errorf("cloud targets = %v, %v", got.TargetResources, got.TargetServiceAccounts)
		}
		want := "locations/global/securityPolicies/" + policyID + "/rules/1000"
		if ref, _, _ := unstructured.NestedString(u.Object, "status", "externalRef"); ref != want {
			t.Errorf("status.externalRef = %q, want %q", ref, want)
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
		update(t, health, func(o *computev1beta1.ComputeOrganizationSecurityPolicyRule) {
			o.Spec.Description = mockenv.Ptr("health checks and the proxy")
			o.Spec.Preview = mockenv.Ptr(true)
			o.Spec.Match.Config.SrcIPRanges = []string{"130.211.0.0/22", "35.191.0.0/16", "10.0.0.0/8"}
			o.Spec.Match.Config.Layer4Config[0].Ports = []string{"80", "443", "8080-8090"}
		})
		waitFor(t, func() bool {
			got, err := cloudRule(t, 1000)
			return err == nil && got.Preview && len(got.Match.Config.SrcIpRanges) == 3
		})
		env.WaitReady(ruleGVK, namespace, health)
		got, _ := cloudRule(t, 1000)
		if got.Description != "health checks and the proxy" || len(got.Match.Config.Layer4Configs[0].Ports) != 3 || !got.EnableLogging {
			t.Errorf("cloud rule = %+v", got)
		}
		if n := writes() - before; n != 1 {
			t.Errorf("wrote %d times, want once", n)
		}
	})

	t.Run("taking fields off the spec puts their defaults back", func(t *testing.T) {
		update(t, health, func(o *computev1beta1.ComputeOrganizationSecurityPolicyRule) {
			o.Spec.Description = nil
			o.Spec.EnableLogging = nil
			o.Spec.Preview = nil
			o.Spec.TargetResources = nil
			o.Spec.TargetServiceAccounts = nil
			o.Spec.Match.Config.Layer4Config[0].Ports = nil
		})
		waitFor(t, func() bool {
			got, err := cloudRule(t, 1000)
			return err == nil && !got.EnableLogging && !got.Preview && got.Description == "" && len(got.TargetResources) == 0
		})
		env.WaitReady(ruleGVK, namespace, health)
		got, _ := cloudRule(t, 1000)
		if len(got.TargetServiceAccounts) != 0 || len(got.Match.Config.Layer4Configs[0].Ports) != 0 {
			t.Errorf("cloud rule kept what the spec dropped: %+v", got)
		}
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
		got.Action = "deny"
		got.Direction = "EGRESS"
		// The fake's operations are done when they are returned.
		if _, err := cloud.OrganizationSecurityPolicies.PatchRule(policyID, got).Priority(1000).Context(env.Ctx).Do(); err != nil {
			t.Fatal(err)
		}
		if got, _ := cloudRule(t, 1000); got.Action != "deny" {
			t.Fatalf("the out-of-band change did not take: %+v", got)
		}
		resync(t, health)
		waitFor(t, func() bool {
			got, err := cloudRule(t, 1000)
			return err == nil && got.Action == "allow" && got.Direction == "INGRESS"
		})
		env.WaitReady(ruleGVK, namespace, health)
	})

	t.Run("a rule by policyId, its priority from metadata.name", func(t *testing.T) {
		egress := newRule("2000", func(s *computev1beta1.ComputeOrganizationSecurityPolicyRuleSpec) {
			s.SecurityPolicyRef = nil
			s.PolicyID = mockenv.Ptr(policyID)
			s.Action = "deny"
			s.Direction = mockenv.Ptr("EGRESS")
			s.Match.Config.SrcIPRanges = nil
			s.Match.Config.DestIPRanges = []string{"0.0.0.0/0"}
			s.Match.Config.Layer4Config = []computev1beta1.OrganizationSecurityPolicyRuleMatchConfigLayer4Config{{IPProtocol: "all"}}
		})
		if err := env.Kube.Create(env.Ctx, egress); err != nil {
			t.Fatal(err)
		}
		env.WaitReady(ruleGVK, namespace, "2000")
		got, err := cloudRule(t, 2000)
		if err != nil {
			t.Fatalf("reading the rule: %v", err)
		}
		if got.Action != "deny" || got.Direction != "EGRESS" || len(got.Match.Config.DestIpRanges) != 1 || len(got.Match.Config.SrcIpRanges) != 0 {
			t.Errorf("cloud rule = %+v, config %+v", got, got.Match.Config)
		}
		if other, err := cloudRule(t, 1000); err != nil || other.Action != "allow" {
			t.Errorf("the other rule changed: %+v, %v", other, err)
		}
	})

	t.Run("the API server refuses what the API cannot take", func(t *testing.T) {
		for name, c := range map[string]struct {
			set  func(*computev1beta1.ComputeOrganizationSecurityPolicyRuleSpec)
			want string
		}{
			"both policyId and securityPolicyRef": {func(s *computev1beta1.ComputeOrganizationSecurityPolicyRuleSpec) { s.PolicyID = mockenv.Ptr("1") }, "exactly one of policyId and securityPolicyRef"},
			"neither":                             {func(s *computev1beta1.ComputeOrganizationSecurityPolicyRuleSpec) { s.SecurityPolicyRef = nil }, "exactly one of policyId and securityPolicyRef"},
			"match.description": {func(s *computev1beta1.ComputeOrganizationSecurityPolicyRuleSpec) {
				s.Match.Description = mockenv.Ptr("x")
			}, "match.description is not in the Compute Engine API"},
			"another direction": {func(s *computev1beta1.ComputeOrganizationSecurityPolicyRuleSpec) { s.Direction = mockenv.Ptr("BOTH") }, "direction"},
			"no layer 4 config": {func(s *computev1beta1.ComputeOrganizationSecurityPolicyRuleSpec) { s.Match.Config.Layer4Config = nil }, "layer4Config"},
		} {
			err := env.Kube.Create(env.Ctx, newRule("refused", func(s *computev1beta1.ComputeOrganizationSecurityPolicyRuleSpec) {
				s.ResourceID = mockenv.Ptr("3000")
				c.set(s)
			}))
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Errorf("creating with %s: got %v, want the API server to refuse it with %q", name, err, c.want)
			}
		}
		for field, mutate := range map[string]func(*computev1beta1.ComputeOrganizationSecurityPolicyRule){
			"resourceID":        func(o *computev1beta1.ComputeOrganizationSecurityPolicyRule) { o.Spec.ResourceID = mockenv.Ptr("1001") },
			"securityPolicyRef": func(o *computev1beta1.ComputeOrganizationSecurityPolicyRule) { o.Spec.SecurityPolicyRef.Name = "other" },
		} {
			o := getObj(t, health)
			mutate(o)
			if err := env.Kube.Update(env.Ctx, o); err == nil || !strings.Contains(err.Error(), field+" is immutable") {
				t.Errorf("changing %s: got %v, want the API server to refuse it", field, err)
			}
		}
		o := getObj(t, "2000")
		o.Spec.PolicyID = mockenv.Ptr("1")
		if err := env.Kube.Update(env.Ctx, o); err == nil || !strings.Contains(err.Error(), "policyId is immutable") {
			t.Errorf("changing policyId: got %v, want the API server to refuse it", err)
		}
	})

	t.Run("a priority that is not a number is not ready, and never written", func(t *testing.T) {
		before := writes()
		bad := newRule("allow-ssh", nil)
		if err := env.Kube.Create(env.Ctx, bad); err != nil {
			t.Fatal(err)
		}
		if msg := env.WaitNotReady(ruleGVK, namespace, "allow-ssh"); !strings.Contains(msg, "is not a number") {
			t.Errorf("the message does not say the priority is not a number: %q", msg)
		}
		if err := env.Kube.Delete(env.Ctx, bad); err != nil {
			t.Fatal(err)
		}
		env.WaitGone(bad)
		if n := writes() - before; n != 0 {
			t.Errorf("wrote %d times", n)
		}
	})

	t.Run("delete removes the rule", func(t *testing.T) {
		for _, name := range []string{health, "2000"} {
			if err := env.Kube.Delete(env.Ctx, getObj(t, name)); err != nil {
				t.Fatal(err)
			}
			env.WaitGone(&computev1beta1.ComputeOrganizationSecurityPolicyRule{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: name}})
		}
		for _, priority := range []int64{1000, 2000} {
			if _, err := cloudRule(t, priority); err == nil || !strings.Contains(err.Error(), "does not contain a rule at priority") {
				t.Errorf("reading the deleted rule at %d: got %v, want it gone", priority, err)
			}
		}
	})

	t.Run("abandon keeps the rule", func(t *testing.T) {
		kept := newRule("kept", func(s *computev1beta1.ComputeOrganizationSecurityPolicyRuleSpec) { s.ResourceID = mockenv.Ptr("4000") })
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

	t.Run("the policy's delete takes its rules", func(t *testing.T) {
		p := &computev1beta1.ComputeOrganizationSecurityPolicy{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: policyObj}}
		if err := env.Kube.Delete(env.Ctx, p); err != nil {
			t.Fatal(err)
		}
		env.WaitGone(p)
		if _, err := cloudRule(t, 4000); !direct.IsNotFound(err) {
			t.Errorf("reading a rule of the deleted policy: got %v, want not found", err)
		}
	})
}
