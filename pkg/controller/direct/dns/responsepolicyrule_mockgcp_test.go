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

// DNSResponsePolicyRule end to end on the fake cloud (pkg/test/mockenv,
// mockgcp with patch 0035) or a real project
// (docs/live-runs.md). The case: the landing zone's DNS firewall answers
// an internal name with a fixed address, then lets it through to Cloud DNS
// (bypassResponsePolicy), and keeps a second rule when its object is
// abandoned.

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

func TestDNSResponsePolicyRule(t *testing.T) {
	skipLiveWithoutDNS(t)
	gvk := dnsv1beta1.DNSResponsePolicyRuleGVK
	env := mockenv.New(t, namespace)
	cloud, err := api.NewService(env.Ctx, env.GCPOptions()...)
	if err != nil {
		t.Fatal(err)
	}

	// The rules' response policy, made through the API; deleted after its
	// rules (cleanups run last first).
	policy := mockenv.Name("rules-firewall")
	if _, err := cloud.ResponsePolicies.Create(mockenv.Project, &api.ResponsePolicy{
		ResponsePolicyName: policy,
		Description:        "the landing zone's DNS firewall",
		Labels:             mockenv.Labels(),
	}).Context(env.Ctx).Do(); err != nil {
		t.Fatalf("making response policy %s: %v", policy, err)
	}
	t.Cleanup(func() {
		if err := cloud.ResponsePolicies.Delete(mockenv.Project, policy).Context(env.Ctx).Do(); err != nil && !direct.IsNotFound(err) {
			t.Errorf("deleting response policy %s: %v", policy, err)
		}
	})
	getRule := func(rule string) (*api.ResponsePolicyRule, error) {
		return cloud.ResponsePolicyRules.Get(mockenv.Project, policy, rule).Context(env.Ctx).Do()
	}
	t.Cleanup(func() {
		for _, base := range []string{"intranet", "kept"} {
			rule := mockenv.Name(base)
			if err := cloud.ResponsePolicyRules.Delete(mockenv.Project, policy, rule).Context(env.Ctx).Do(); err != nil && !direct.IsNotFound(err) {
				t.Errorf("deleting rule %s: %v", rule, err)
			}
		}
	})
	// writes counts the requests with this method to the rules of the policy.
	writes := func(method string) int {
		n := 0
		for _, r := range env.Requests() {
			if strings.HasPrefix(r, method+" ") && strings.Contains(r, "/responsePolicies/"+policy+"/rules") {
				n++
			}
		}
		return n
	}
	getObj := func(t *testing.T, name string) *dnsv1beta1.DNSResponsePolicyRule {
		t.Helper()
		obj := &dnsv1beta1.DNSResponsePolicyRule{}
		if err := env.Kube.Get(env.Ctx, types.NamespacedName{Namespace: namespace, Name: name}, obj); err != nil {
			t.Fatal(err)
		}
		return obj
	}
	waitFor := func(t *testing.T, what string, cond func() bool) {
		t.Helper()
		deadline := time.Now().Add(mockenv.Timeout())
		for !cond() {
			if time.Now().After(deadline) {
				t.Fatalf("timed out waiting for %s", what)
			}
			time.Sleep(500 * time.Millisecond)
		}
	}
	policyPath := "projects/" + mockenv.Project + "/locations/global/responsePolicies/" + policy
	newRule := func(name, dnsName, address string) *dnsv1beta1.DNSResponsePolicyRule {
		return &dnsv1beta1.DNSResponsePolicyRule{
			ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: name, Labels: mockenv.Labels()},
			Spec: dnsv1beta1.DNSResponsePolicyRuleSpec{
				ProjectRef:     refs.ProjectRef{External: "projects/" + mockenv.Project},
				ResourceID:     mockenv.Ptr(mockenv.Name(name)),
				ResponsePolicy: policyPath,
				DnsName:        dnsName,
				LocalData: &dnsv1beta1.ResponsepolicyruleLocalData{LocalDatas: []dnsv1beta1.ResponsepolicyruleLocalDatas{{
					Name: dnsName, Type: "A", Ttl: mockenv.Ptr(int64(300)), Rrdatas: []string{address},
				}}},
			},
		}
	}
	rule := mockenv.Name("intranet")

	if err := env.Kube.Create(env.Ctx, newRule("intranet", "intranet.example.com.", "10.0.0.10")); err != nil {
		t.Fatal(err)
	}

	t.Run("create makes the rule with its local data", func(t *testing.T) {
		u := env.WaitReady(gvk, namespace, "intranet")
		want := policyPath + "/rules/" + rule
		if ref, _, _ := unstructured.NestedString(u.Object, "status", "externalRef"); ref != want {
			t.Errorf("status.externalRef = %q, want %q", ref, want)
		}
		got, err := getRule(rule)
		if err != nil {
			t.Fatalf("reading the rule: %v", err)
		}
		if got.DnsName != "intranet.example.com." || got.LocalData == nil || len(got.LocalData.LocalDatas) != 1 ||
			got.LocalData.LocalDatas[0].Rrdatas[0] != "10.0.0.10" || got.LocalData.LocalDatas[0].Type != "A" {
			t.Errorf("rule = %+v", got)
		}
	})

	t.Run("a resync writes nothing", func(t *testing.T) {
		before := writes("PUT") + writes("PATCH") + writes("POST")
		o := getObj(t, "intranet")
		o.Annotations = map[string]string{k8s.InternalForceReconcileAnnotation: "1"}
		if err := env.Kube.Update(env.Ctx, o); err != nil {
			t.Fatal(err)
		}
		time.Sleep(3 * time.Second)
		env.WaitReady(gvk, namespace, "intranet")
		if after := writes("PUT") + writes("PATCH") + writes("POST"); after != before {
			t.Errorf("a resync wrote %d times", after-before)
		}
	})

	t.Run("a changed address is one update", func(t *testing.T) {
		before := writes("PUT")
		o := getObj(t, "intranet")
		o.Spec.LocalData.LocalDatas[0].Rrdatas = []string{"10.0.0.20"}
		if err := env.Kube.Update(env.Ctx, o); err != nil {
			t.Fatal(err)
		}
		waitFor(t, "the address to change", func() bool {
			got, err := getRule(rule)
			return err == nil && got.LocalData != nil && len(got.LocalData.LocalDatas) == 1 &&
				len(got.LocalData.LocalDatas[0].Rrdatas) == 1 && got.LocalData.LocalDatas[0].Rrdatas[0] == "10.0.0.20"
		})
		env.WaitReady(gvk, namespace, "intranet")
		if n := writes("PUT") - before; n != 1 {
			t.Errorf("updates = %d, want 1", n)
		}
	})

	t.Run("a rule moved to bypassResponsePolicy keeps no local data", func(t *testing.T) {
		o := getObj(t, "intranet")
		o.Spec.LocalData = nil
		o.Spec.Behavior = mockenv.Ptr("bypassResponsePolicy")
		if err := env.Kube.Update(env.Ctx, o); err != nil {
			t.Fatal(err)
		}
		waitFor(t, "the rule to bypass", func() bool {
			got, err := getRule(rule)
			return err == nil && got.Behavior == "bypassResponsePolicy" && (got.LocalData == nil || len(got.LocalData.LocalDatas) == 0)
		})
		env.WaitReady(gvk, namespace, "intranet")
	})

	t.Run("the CRD refuses a change of response policy", func(t *testing.T) {
		o := getObj(t, "intranet")
		o.Spec.ResponsePolicy = "projects/" + mockenv.Project + "/responsePolicies/another"
		err := env.Kube.Update(env.Ctx, o)
		if err == nil || !strings.Contains(err.Error(), "responsePolicy is immutable") {
			t.Errorf("changing responsePolicy: got %v, want the API server to refuse it", err)
		}
	})

	t.Run("delete deletes the rule", func(t *testing.T) {
		if err := env.Kube.Delete(env.Ctx, getObj(t, "intranet")); err != nil {
			t.Fatal(err)
		}
		env.WaitGone(&dnsv1beta1.DNSResponsePolicyRule{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: "intranet"}})
		if _, err := getRule(rule); !direct.IsNotFound(err) {
			t.Errorf("after delete, reading the rule: %v, want not found", err)
		}
	})

	t.Run("abandon keeps the rule", func(t *testing.T) {
		if err := env.Kube.Create(env.Ctx, newRule("kept", "wiki.example.com.", "10.0.0.30")); err != nil {
			t.Fatal(err)
		}
		env.WaitReady(gvk, namespace, "kept")
		o := getObj(t, "kept")
		o.Annotations = map[string]string{"cnrm.cloud.google.com/deletion-policy": "abandon"}
		if err := env.Kube.Update(env.Ctx, o); err != nil {
			t.Fatal(err)
		}
		if err := env.Kube.Delete(env.Ctx, o); err != nil {
			t.Fatal(err)
		}
		env.WaitGone(&dnsv1beta1.DNSResponsePolicyRule{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: "kept"}})
		if _, err := getRule(mockenv.Name("kept")); err != nil {
			t.Errorf("after abandon, reading the rule: %v, want it kept", err)
		}
	})
}
