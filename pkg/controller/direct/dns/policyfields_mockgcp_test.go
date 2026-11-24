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

// DNSPolicyFields end to end on the fake cloud (pkg/test/mockenv, patch
// 0057) or a real project (docs/live-runs.md, with E2E_DNS_ADMIN=true). The
// DNS server policy is the run's own (mockenv.Name), bound to no network,
// costs nothing and is deleted at the end.

import (
	"strings"
	"testing"
	"time"

	dnsv1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/apis/dns/v1beta1"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/k8s"
	api "google.golang.org/api/dns/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"

	krm "github.com/highfivelabs/config-connector-extensions/apis/dns/v1alpha1"
	"github.com/highfivelabs/config-connector-extensions/pkg/test/mockenv"
)

func TestDNSPolicyFields(t *testing.T) {
	skipLiveWithoutDNS(t)
	gvk := krm.DNSPolicyFieldsGVK
	env := mockenv.New(t, namespace)
	c, err := api.NewService(env.Ctx, env.GCPOptions()...)
	if err != nil {
		t.Fatal(err)
	}
	policy := mockenv.Name("pf")
	if _, err := c.Policies.Create(mockenv.Project, &api.Policy{Name: policy, Description: "made by the test"}).Context(env.Ctx).Do(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := c.Policies.Delete(mockenv.Project, policy).Context(env.Ctx).Do(); err != nil && !direct.IsNotFound(err) {
			t.Errorf("deleting DNS policy %s: %v", policy, err)
		}
	})
	policyName := "projects/" + mockenv.Project + "/policies/" + policy
	get := func(t *testing.T) *api.Policy {
		t.Helper()
		p, err := c.Policies.Get(mockenv.Project, policy).Context(env.Ctx).Do()
		if err != nil {
			t.Fatalf("reading the policy: %v", err)
		}
		return p
	}
	allQueries := func(p *api.Policy) bool {
		return p.Dns64Config != nil && p.Dns64Config.Scope != nil && p.Dns64Config.Scope.AllQueries
	}
	patches := func() []string {
		var out []string
		for _, req := range env.Requests() {
			if strings.HasPrefix(req, "PATCH ") && strings.Contains(req, "/policies/"+policy) {
				out = append(out, req)
			}
		}
		return out
	}

	// Config Connector's DNSPolicy for it (not reconciled here: read to
	// resolve resourceRef by name, from its spec and project annotation).
	cc := &unstructured.Unstructured{Object: map[string]any{"spec": map[string]any{"resourceID": policy}}}
	cc.SetGroupVersionKind(schema.GroupVersionKind{Group: "dns.cnrm.cloud.google.com", Version: "v1beta1", Kind: "DNSPolicy"})
	cc.SetNamespace(namespace)
	cc.SetName("policy")
	cc.SetAnnotations(map[string]string{"cnrm.cloud.google.com/project-id": mockenv.Project})
	if err := env.Kube.Create(env.Ctx, cc); err != nil {
		t.Fatal(err)
	}

	getObj := func(t *testing.T, name string) *krm.DNSPolicyFields {
		t.Helper()
		o := &krm.DNSPolicyFields{}
		if err := env.Kube.Get(env.Ctx, types.NamespacedName{Namespace: namespace, Name: name}, o); err != nil {
			t.Fatal(err)
		}
		return o
	}
	resync := func(t *testing.T, name string) {
		t.Helper()
		o := getObj(t, name)
		o.Annotations = map[string]string{k8s.InternalForceReconcileAnnotation: time.Now().String()}
		if err := env.Kube.Update(env.Ctx, o); err != nil {
			t.Fatal(err)
		}
		time.Sleep(3 * time.Second)
		env.WaitReady(gvk, namespace, name)
	}
	newFields := func(name string, ref *dnsv1beta1.DNSPolicyRef, all bool) *krm.DNSPolicyFields {
		return &krm.DNSPolicyFields{
			ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: name, Labels: mockenv.Labels()},
			Spec: krm.DNSPolicyFieldsSpec{ResourceRef: ref, Dns64Config: &krm.PolicyDns64Config{
				Scope: &krm.PolicyDns64ConfigScope{AllQueries: mockenv.Ptr(all)},
			}},
		}
	}

	obj := newFields("fields", &dnsv1beta1.DNSPolicyRef{Name: "policy"}, true)
	if err := env.Kube.Create(env.Ctx, obj); err != nil {
		t.Fatal(err)
	}

	t.Run("create turns DNS64 on, and nothing else", func(t *testing.T) {
		u := env.WaitReady(gvk, namespace, "fields")
		if ref, _, _ := unstructured.NestedString(u.Object, "status", "externalRef"); ref != policyName {
			t.Errorf("status.externalRef = %q, want %q", ref, policyName)
		}
		p := get(t)
		if !allQueries(p) {
			t.Errorf("dns64Config = %+v, want allQueries", p.Dns64Config)
		}
		if p.Description != "made by the test" {
			t.Errorf("description = %q, changed", p.Description)
		}
		if n := len(patches()); n != 1 {
			t.Errorf("%d patches, want one", n)
		}
	})

	t.Run("Terraform's patch keeps it", func(t *testing.T) {
		// What Config Connector's DNSPolicy (Terraform) sends when its
		// fields change: those fields, never dns64Config.
		if _, err := c.Policies.Patch(mockenv.Project, policy, &api.Policy{
			Description: "changed by config connector", EnableLogging: true, ForceSendFields: []string{"EnableInboundForwarding"},
		}).Context(env.Ctx).Do(); err != nil {
			t.Fatal(err)
		}
		p := get(t)
		if p.Description != "changed by config connector" || !p.EnableLogging {
			t.Errorf("after Terraform's patch description %q, enableLogging %v", p.Description, p.EnableLogging)
		}
		if !allQueries(p) {
			t.Errorf("Terraform's patch turned DNS64 off")
		}
	})

	t.Run("a resync writes nothing", func(t *testing.T) {
		before := len(patches())
		resync(t, "fields")
		if n := len(patches()) - before; n != 0 {
			t.Errorf("a resync patched the policy %d times", n)
		}
	})

	t.Run("a change is patched, and one outside is set again", func(t *testing.T) {
		o := getObj(t, "fields")
		o.Spec.Dns64Config.Scope.AllQueries = mockenv.Ptr(false)
		if err := env.Kube.Update(env.Ctx, o); err != nil {
			t.Fatal(err)
		}
		deadline := time.Now().Add(mockenv.Timeout())
		for allQueries(get(t)) {
			if time.Now().After(deadline) {
				t.Fatal("DNS64 is still on")
			}
			time.Sleep(time.Second)
		}
		env.WaitReady(gvk, namespace, "fields")
		o = getObj(t, "fields")
		o.Spec.Dns64Config.Scope.AllQueries = mockenv.Ptr(true)
		if err := env.Kube.Update(env.Ctx, o); err != nil {
			t.Fatal(err)
		}
		for !allQueries(get(t)) {
			if time.Now().After(deadline) {
				t.Fatal("DNS64 is still off")
			}
			time.Sleep(time.Second)
		}
		env.WaitReady(gvk, namespace, "fields")
		if _, err := c.Policies.Patch(mockenv.Project, policy, &api.Policy{Dns64Config: &api.PolicyDns64Config{
			Scope: &api.PolicyDns64ConfigScope{AllQueries: false, ForceSendFields: []string{"AllQueries"}},
		}}).Context(env.Ctx).Do(); err != nil {
			t.Fatal(err)
		}
		resync(t, "fields")
		if !allQueries(get(t)) {
			t.Errorf("after a resync DNS64 is off")
		}
	})

	t.Run("a second object for the policy is refused", func(t *testing.T) {
		dup := newFields("dup", &dnsv1beta1.DNSPolicyRef{External: policyName}, false)
		if err := env.Kube.Create(env.Ctx, dup); err != nil {
			t.Fatal(err)
		}
		if msg := env.WaitNotReady(gvk, namespace, "dup"); !strings.Contains(msg, "already sets the fields of") {
			t.Errorf("Ready message = %q, want the second object refused", msg)
		}
		if !allQueries(get(t)) {
			t.Errorf("the refused object changed the policy")
		}
		if err := env.Kube.Delete(env.Ctx, dup); err != nil {
			t.Fatal(err)
		}
		env.WaitGone(dup)
	})

	t.Run("a missing policy is waited for, never made", func(t *testing.T) {
		missing := "projects/" + mockenv.Project + "/policies/" + mockenv.Name("never-made")
		w := newFields("waits", &dnsv1beta1.DNSPolicyRef{External: missing}, true)
		if err := env.Kube.Create(env.Ctx, w); err != nil {
			t.Fatal(err)
		}
		if msg := env.WaitNotReady(gvk, namespace, "waits"); !strings.Contains(msg, "does not exist yet") {
			t.Errorf("Ready message = %q, want the missing policy waited for", msg)
		}
		if err := env.Kube.Delete(env.Ctx, w); err != nil {
			t.Fatal(err)
		}
		env.WaitGone(w)
	})

	t.Run("delete turns DNS64 off and keeps the policy", func(t *testing.T) {
		if err := env.Kube.Delete(env.Ctx, obj); err != nil {
			t.Fatal(err)
		}
		env.WaitGone(obj)
		p := get(t)
		if allQueries(p) {
			t.Errorf("after delete DNS64 is on")
		}
		if p.Description != "changed by config connector" {
			t.Errorf("delete changed the policy's other fields: description %q", p.Description)
		}
	})

	t.Run("abandon keeps DNS64", func(t *testing.T) {
		k := newFields("kept", &dnsv1beta1.DNSPolicyRef{Name: "policy"}, true)
		k.Annotations = map[string]string{"cnrm.cloud.google.com/deletion-policy": "abandon"}
		if err := env.Kube.Create(env.Ctx, k); err != nil {
			t.Fatal(err)
		}
		env.WaitReady(gvk, namespace, "kept")
		if err := env.Kube.Delete(env.Ctx, k); err != nil {
			t.Fatal(err)
		}
		env.WaitGone(k)
		if !allQueries(get(t)) {
			t.Errorf("after an abandoning delete DNS64 is off")
		}
	})
}
