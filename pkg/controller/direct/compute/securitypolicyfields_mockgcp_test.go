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

// ComputeSecurityPolicyFields end to end on the fake cloud
// (pkg/test/mockenv) or a real project (docs/live-runs.md). The policy is
// the run's own (mockenv.Name), global, attached to nothing; the test
// deletes it. A live run needs E2E_CLOUD_ARMOR=true: Cloud Armor bills a
// policy by the month, prorated, and threshold configs may need Cloud
// Armor Enterprise on the project (to be seen live).

import (
	"os"
	"slices"
	"strings"
	"testing"

	gcp "cloud.google.com/go/compute/apiv1"
	pb "cloud.google.com/go/compute/apiv1/computepb"
	computev1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/apis/compute/v1beta1"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"

	krm "github.com/highfivelabs/config-connector-extensions/apis/compute/v1alpha1"
	"github.com/highfivelabs/config-connector-extensions/pkg/test/mockenv"
)

func TestComputeSecurityPolicyFields(t *testing.T) {
	if mockenv.Live() && os.Getenv("E2E_CLOUD_ARMOR") != "true" {
		t.Skip("a live ComputeSecurityPolicyFields run makes a Cloud Armor policy (billed by the month, prorated) and may need Cloud Armor Enterprise for threshold configs; set E2E_CLOUD_ARMOR=true where that is wanted (docs/live-runs.md)")
	}
	const namespace = "example"
	gvk := krm.ComputeSecurityPolicyFieldsGVK
	env := mockenv.New(t, namespace)
	c, err := gcp.NewSecurityPoliciesRESTClient(env.Ctx, env.GCPOptions()...)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { c.Close() })

	policy := mockenv.Name("spf")
	policyName := "projects/" + mockenv.Project + "/global/securityPolicies/" + policy
	op, err := c.Insert(env.Ctx, &pb.InsertSecurityPolicyRequest{Project: mockenv.Project, SecurityPolicyResource: &pb.SecurityPolicy{
		Name:        direct.LazyPtr(policy),
		Description: direct.LazyPtr("made by the test"),
		AdaptiveProtectionConfig: &pb.SecurityPolicyAdaptiveProtectionConfig{
			Layer7DdosDefenseConfig: &pb.SecurityPolicyAdaptiveProtectionConfigLayer7DdosDefenseConfig{Enable: direct.LazyPtr(true)},
		},
		AdvancedOptionsConfig: &pb.SecurityPolicyAdvancedOptionsConfig{JsonParsing: direct.LazyPtr("STANDARD")},
	}})
	if err == nil {
		err = op.Wait(env.Ctx)
	}
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		op, err := c.Delete(env.Ctx, &pb.DeleteSecurityPolicyRequest{Project: mockenv.Project, SecurityPolicy: policy})
		if err == nil {
			err = op.Wait(env.Ctx)
		}
		if err != nil {
			t.Errorf("deleting security policy %s: %v", policy, err)
		}
	})

	// Config Connector's ComputeSecurityPolicy for the policy (not
	// reconciled here: its reference resolves a name from the spec and the
	// project annotation).
	cc := &unstructured.Unstructured{Object: map[string]any{"spec": map[string]any{"resourceID": policy}}}
	cc.SetGroupVersionKind(schema.GroupVersionKind{Group: "compute.cnrm.cloud.google.com", Version: "v1beta1", Kind: "ComputeSecurityPolicy"})
	cc.SetNamespace(namespace)
	cc.SetName("policy")
	cc.SetAnnotations(map[string]string{"cnrm.cloud.google.com/project-id": mockenv.Project})
	if err := env.Kube.Create(env.Ctx, cc); err != nil {
		t.Fatal(err)
	}

	get := func(t *testing.T) *pb.SecurityPolicy {
		t.Helper()
		p, err := c.Get(env.Ctx, &pb.GetSecurityPolicyRequest{Project: mockenv.Project, SecurityPolicy: policy})
		if err != nil {
			t.Fatalf("reading the policy: %v", err)
		}
		return p
	}
	thresholds := func(p *pb.SecurityPolicy) []string {
		var out []string
		for _, tc := range p.GetAdaptiveProtectionConfig().GetLayer7DdosDefenseConfig().GetThresholdConfigs() {
			out = append(out, tc.GetName())
		}
		slices.Sort(out)
		return out
	}
	patches := func() []string { return patchesOf(env, "/global/securityPolicies/"+policy) }
	getObj := func(t *testing.T, name string) *krm.ComputeSecurityPolicyFields {
		t.Helper()
		o := &krm.ComputeSecurityPolicyFields{}
		if err := env.Kube.Get(env.Ctx, types.NamespacedName{Namespace: namespace, Name: name}, o); err != nil {
			t.Fatal(err)
		}
		return o
	}
	newFields := func(name string, ref *computev1beta1.ComputeSecurityPolicyRef) *krm.ComputeSecurityPolicyFields {
		return &krm.ComputeSecurityPolicyFields{
			ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: name, Labels: mockenv.Labels()},
			Spec:       krm.ComputeSecurityPolicyFieldsSpec{ResourceRef: ref},
		}
	}
	threshold := func(name string, load float32) krm.SecurityPolicyAdaptiveProtectionConfigLayer7DdosDefenseConfigThresholdConfig {
		return krm.SecurityPolicyAdaptiveProtectionConfigLayer7DdosDefenseConfigThresholdConfig{Name: name, DetectionLoadThreshold: mockenv.Ptr(load)}
	}

	t.Run("a spec with no field is refused by the CRD", func(t *testing.T) {
		bad := newFields("bad", &computev1beta1.ComputeSecurityPolicyRef{External: policyName})
		if err := env.Kube.Create(env.Ctx, bad); err == nil || !strings.Contains(err.Error(), "set adaptiveProtectionConfig or advancedOptionsConfig") {
			t.Errorf("creating an empty spec: %v, want refused", err)
		}
	})

	obj := newFields("fields", &computev1beta1.ComputeSecurityPolicyRef{Name: "policy"})
	host := threshold("by-host", 0.8)
	host.AutoDeployLoadThreshold = mockenv.Ptr(float32(0.7))
	host.TrafficGranularityConfigs = []krm.SecurityPolicyAdaptiveProtectionConfigLayer7DdosDefenseConfigThresholdConfigTrafficGranularityConfig{
		{Type: mockenv.Ptr("HTTP_HEADER_HOST"), EnableEachUniqueValue: mockenv.Ptr(true)},
	}
	obj.Spec.AdaptiveProtectionConfig = &krm.SecurityPolicyFieldsAdaptiveProtectionConfig{Layer7DdosDefenseConfig: &krm.SecurityPolicyFieldsLayer7DdosDefenseConfig{
		ThresholdConfigs: []krm.SecurityPolicyAdaptiveProtectionConfigLayer7DdosDefenseConfigThresholdConfig{host, threshold("default", 0.9)},
	}}
	obj.Spec.AdvancedOptionsConfig = &krm.SecurityPolicyFieldsAdvancedOptionsConfig{RequestBodyInspectionSize: "16KB"}
	if err := env.Kube.Create(env.Ctx, obj); err != nil {
		t.Fatal(err)
	}

	t.Run("create sets the fields and keeps the policy's own", func(t *testing.T) {
		u := env.WaitReady(gvk, namespace, "fields")
		if ref, _, _ := unstructured.NestedString(u.Object, "status", "externalRef"); ref != policyName {
			t.Errorf("status.externalRef = %q, want %q", ref, policyName)
		}
		p := get(t)
		if got := thresholds(p); !slices.Equal(got, []string{"by-host", "default"}) {
			t.Errorf("threshold configs = %v", got)
		}
		for _, tc := range p.GetAdaptiveProtectionConfig().GetLayer7DdosDefenseConfig().GetThresholdConfigs() {
			if tc.GetName() == "by-host" && (tc.GetAutoDeployLoadThreshold() != 0.7 || len(tc.GetTrafficGranularityConfigs()) != 1 || tc.GetTrafficGranularityConfigs()[0].GetType() != "HTTP_HEADER_HOST") {
				t.Errorf("by-host = %v", tc)
			}
		}
		if got := p.GetAdvancedOptionsConfig().GetRequestBodyInspectionSize(); got != "16KB" {
			t.Errorf("requestBodyInspectionSize = %q", got)
		}
		if !p.GetAdaptiveProtectionConfig().GetLayer7DdosDefenseConfig().GetEnable() || p.GetAdvancedOptionsConfig().GetJsonParsing() != "STANDARD" || p.GetDescription() != "made by the test" {
			t.Errorf("the policy's own fields changed: %v", p)
		}
		if pp := patches(); len(pp) != 1 || strings.Contains(pp[0], "updateMask") {
			t.Errorf("patches = %v, want one merge patch with no mask", pp)
		}
	})

	t.Run("a resync writes nothing", func(t *testing.T) {
		before := len(patches())
		forceResync(t, env, getObj(t, "fields"), gvk)
		if n := len(patches()) - before; n != 0 {
			t.Errorf("a resync patched the policy %d times", n)
		}
	})

	t.Run("an update of Config Connector's adaptive protection keeps its change, and ours after the next reconcile", func(t *testing.T) {
		// What Terraform's update of a ComputeSecurityPolicy sends when its
		// adaptive protection changes: the sub-object it knows, no mask.
		op, err := c.Patch(env.Ctx, &pb.PatchSecurityPolicyRequest{Project: mockenv.Project, SecurityPolicy: policy, SecurityPolicyResource: &pb.SecurityPolicy{
			Fingerprint: get(t).Fingerprint,
			AdaptiveProtectionConfig: &pb.SecurityPolicyAdaptiveProtectionConfig{Layer7DdosDefenseConfig: &pb.SecurityPolicyAdaptiveProtectionConfigLayer7DdosDefenseConfig{
				Enable: direct.LazyPtr(true), RuleVisibility: direct.LazyPtr("PREMIUM"),
			}},
		}})
		if err == nil {
			err = op.Wait(env.Ctx)
		}
		if err != nil {
			t.Fatal(err)
		}
		forceResync(t, env, getObj(t, "fields"), gvk)
		p := get(t)
		if got := thresholds(p); !slices.Equal(got, []string{"by-host", "default"}) {
			t.Errorf("after a resync the threshold configs = %v", got)
		}
		if got := p.GetAdaptiveProtectionConfig().GetLayer7DdosDefenseConfig().GetRuleVisibility(); got != "PREMIUM" {
			t.Errorf("Config Connector's ruleVisibility = %q, want its change kept", got)
		}
	})

	t.Run("a changed list replaces the policy's", func(t *testing.T) {
		o := getObj(t, "fields")
		o.Spec.AdaptiveProtectionConfig.Layer7DdosDefenseConfig.ThresholdConfigs = []krm.SecurityPolicyAdaptiveProtectionConfigLayer7DdosDefenseConfigThresholdConfig{threshold("default", 0.5)}
		if err := env.Kube.Update(env.Ctx, o); err != nil {
			t.Fatal(err)
		}
		waitFor(t, func() bool { return slices.Equal(thresholds(get(t)), []string{"default"}) })
		env.WaitReady(gvk, namespace, "fields")
	})

	t.Run("a second object for the policy is refused", func(t *testing.T) {
		dup := newFields("dup", &computev1beta1.ComputeSecurityPolicyRef{External: policyName})
		dup.Spec.AdvancedOptionsConfig = &krm.SecurityPolicyFieldsAdvancedOptionsConfig{RequestBodyInspectionSize: "8KB"}
		if err := env.Kube.Create(env.Ctx, dup); err != nil {
			t.Fatal(err)
		}
		if msg := env.WaitNotReady(gvk, namespace, "dup"); !strings.Contains(msg, "already sets the fields of") {
			t.Errorf("Ready message = %q, want the second object refused", msg)
		}
		dup.Annotations = map[string]string{"cnrm.cloud.google.com/deletion-policy": "abandon"}
		if err := env.Kube.Delete(env.Ctx, dup); err != nil {
			t.Fatal(err)
		}
		env.WaitGone(dup)
	})

	t.Run("delete clears the fields with a mask and keeps the policy's own", func(t *testing.T) {
		before := len(patches())
		if err := env.Kube.Delete(env.Ctx, obj); err != nil {
			t.Fatal(err)
		}
		env.WaitGone(&krm.ComputeSecurityPolicyFields{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: "fields"}})
		p := get(t)
		if len(thresholds(p)) != 0 || p.GetAdvancedOptionsConfig().GetRequestBodyInspectionSize() != "" {
			t.Errorf("after delete: threshold configs %v, requestBodyInspectionSize %q; want none", thresholds(p), p.GetAdvancedOptionsConfig().GetRequestBodyInspectionSize())
		}
		if !p.GetAdaptiveProtectionConfig().GetLayer7DdosDefenseConfig().GetEnable() || p.GetAdvancedOptionsConfig().GetJsonParsing() != "STANDARD" {
			t.Errorf("delete changed the policy's own fields: %v", p)
		}
		pp := patches()[before:]
		if len(pp) != 1 || !strings.Contains(pp[0], "updateMask=adaptiveProtectionConfig.layer7DdosDefenseConfig.thresholdConfigs%2CadvancedOptionsConfig.requestBodyInspectionSize") {
			t.Errorf("patches = %v, want one with a mask of the two fields", pp)
		}
	})

	t.Run("abandon keeps the fields", func(t *testing.T) {
		k := newFields("kept", &computev1beta1.ComputeSecurityPolicyRef{Name: "policy"})
		k.Spec.AdvancedOptionsConfig = &krm.SecurityPolicyFieldsAdvancedOptionsConfig{RequestBodyInspectionSize: "32KB"}
		k.Annotations = map[string]string{"cnrm.cloud.google.com/deletion-policy": "abandon"}
		if err := env.Kube.Create(env.Ctx, k); err != nil {
			t.Fatal(err)
		}
		env.WaitReady(gvk, namespace, "kept")
		if err := env.Kube.Delete(env.Ctx, k); err != nil {
			t.Fatal(err)
		}
		env.WaitGone(k)
		if got := get(t).GetAdvancedOptionsConfig().GetRequestBodyInspectionSize(); got != "32KB" {
			t.Errorf("after an abandoning delete requestBodyInspectionSize = %q, want 32KB", got)
		}
	})
}
