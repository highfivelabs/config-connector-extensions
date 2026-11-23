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

package monitoring_test

// MonitoringAlertPolicyFields end to end on the fake cloud
// (pkg/test/mockenv, patch 0067) or a real project (docs/live-runs.md). The
// policy is the run's own (display name mockenv.Name), has no notification
// channel, costs nothing, and is deleted at the end.

import (
	"strings"
	"testing"
	"time"

	monitoringv1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/apis/monitoring/v1beta1"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/k8s"
	api "google.golang.org/api/monitoring/v3"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"

	krm "github.com/highfivelabs/config-connector-extensions/apis/monitoring/v1alpha1"
	"github.com/highfivelabs/config-connector-extensions/pkg/test/mockenv"
)

const cpuFilter = `metric.type="compute.googleapis.com/instance/cpu/utilization" AND resource.type="gce_instance"`

func TestMonitoringAlertPolicyFields(t *testing.T) {
	gvk := krm.MonitoringAlertPolicyFieldsGVK
	env := mockenv.New(t, namespace)
	c, err := api.NewService(env.Ctx, env.GCPOptions()...)
	if err != nil {
		t.Fatal(err)
	}
	project := "projects/" + mockenv.Project
	created, err := c.Projects.AlertPolicies.Create(project, &api.AlertPolicy{
		DisplayName: mockenv.Name("ap"),
		Combiner:    "OR",
		Conditions: []*api.Condition{{
			DisplayName:        "cpu",
			ConditionThreshold: &api.MetricThreshold{Filter: cpuFilter, Comparison: "COMPARISON_GT", ThresholdValue: 0.9, Duration: "60s"},
		}},
		Documentation: &api.Documentation{Content: "CPU is high.", MimeType: "text/markdown"},
		AlertStrategy: &api.AlertStrategy{AutoClose: "1800s"},
	}).Context(env.Ctx).Do()
	if err != nil {
		t.Fatal(err)
	}
	policy := created.Name
	t.Cleanup(func() {
		if _, err := c.Projects.AlertPolicies.Delete(policy).Context(env.Ctx).Do(); err != nil && !direct.IsNotFound(err) {
			t.Errorf("deleting alert policy %s: %v", policy, err)
		}
	})
	get := func(t *testing.T) *api.AlertPolicy {
		t.Helper()
		got, err := c.Projects.AlertPolicies.Get(policy).Context(env.Ctx).Do()
		if err != nil {
			t.Fatalf("reading the alert policy: %v", err)
		}
		return got
	}
	patches := func() int {
		n := 0
		for _, req := range env.Requests() {
			if strings.HasPrefix(req, "PATCH ") && strings.Contains(req, policy) {
				n++
			}
		}
		return n
	}
	getObj := func(t *testing.T, name string) *krm.MonitoringAlertPolicyFields {
		t.Helper()
		o := &krm.MonitoringAlertPolicyFields{}
		if err := env.Kube.Get(env.Ctx, types.NamespacedName{Namespace: namespace, Name: name}, o); err != nil {
			t.Fatal(err)
		}
		return o
	}
	update := func(t *testing.T, name string, change func(*krm.MonitoringAlertPolicyFields)) {
		t.Helper()
		o := getObj(t, name)
		change(o)
		if err := env.Kube.Update(env.Ctx, o); err != nil {
			t.Fatal(err)
		}
	}
	resync := func(t *testing.T, name string) {
		t.Helper()
		update(t, name, func(o *krm.MonitoringAlertPolicyFields) {
			o.Annotations = map[string]string{k8s.InternalForceReconcileAnnotation: time.Now().String()}
		})
	}
	resyncWritesNothing := func(t *testing.T, name string) {
		t.Helper()
		before := patches()
		resync(t, name)
		time.Sleep(3 * time.Second)
		env.WaitReady(gvk, namespace, name)
		if n := patches() - before; n != 0 {
			t.Errorf("a resync patched the policy %d times", n)
		}
	}
	settle := func(t *testing.T, name string, ok func(*api.AlertPolicy) bool) {
		t.Helper()
		deadline := time.Now().Add(mockenv.Timeout())
		for !ok(get(t)) {
			if time.Now().After(deadline) {
				got := get(t)
				t.Fatalf("the policy did not change: documentation %+v, strategy %+v", got.Documentation, got.AlertStrategy)
			}
			time.Sleep(time.Second)
		}
		env.WaitReady(gvk, namespace, name)
	}
	newFields := func(name string, ref *monitoringv1beta1.MonitoringAlertPolicyRef) *krm.MonitoringAlertPolicyFields {
		return &krm.MonitoringAlertPolicyFields{
			ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: name, Labels: mockenv.Labels()},
			Spec: krm.MonitoringAlertPolicyFieldsSpec{
				ResourceRef: ref,
				Documentation: &krm.AlertPolicy_DocumentationFields{
					Subject: mockenv.Ptr("CPU high on ${resource.label.instance_id}"),
					Links:   []krm.AlertPolicy_Documentation_Link{{DisplayName: mockenv.Ptr("playbook"), URL: mockenv.Ptr("https://example.com/playbooks/cpu")}},
				},
				AlertStrategy: &krm.AlertPolicy_AlertStrategyFields{NotificationPrompts: []string{"OPENED", "CLOSED"}},
			},
		}
	}
	ours := func(got *api.AlertPolicy) bool {
		d, s := got.Documentation, got.AlertStrategy
		return d != nil && strings.HasPrefix(d.Subject, "CPU high") && len(d.Links) == 1 && s != nil && len(s.NotificationPrompts) == 2
	}

	// Config Connector's MonitoringAlertPolicy for it (not reconciled here:
	// read to resolve resourceRef by name, from its spec and project
	// annotation).
	cc := &unstructured.Unstructured{Object: map[string]any{"spec": map[string]any{
		"resourceID":  policy[strings.LastIndex(policy, "/")+1:],
		"displayName": mockenv.Name("ap"),
		"combiner":    "OR",
		"conditions": []any{map[string]any{
			"displayName":        "cpu",
			"conditionThreshold": map[string]any{"filter": cpuFilter, "comparison": "COMPARISON_GT", "thresholdValue": 0.9, "duration": "60s"},
		}},
	}}}
	cc.SetGroupVersionKind(schema.GroupVersionKind{Group: "monitoring.cnrm.cloud.google.com", Version: "v1beta1", Kind: "MonitoringAlertPolicy"})
	cc.SetNamespace(namespace)
	cc.SetName("policy")
	cc.SetAnnotations(map[string]string{"cnrm.cloud.google.com/project-id": mockenv.Project})
	if err := env.Kube.Create(env.Ctx, cc); err != nil {
		t.Fatal(err)
	}

	obj := newFields("fields", &monitoringv1beta1.MonitoringAlertPolicyRef{Name: "policy"})
	if err := env.Kube.Create(env.Ctx, obj); err != nil {
		t.Fatal(err)
	}

	t.Run("create sets the fields, and nothing else", func(t *testing.T) {
		u := env.WaitReady(gvk, namespace, "fields")
		if ref, _, _ := unstructured.NestedString(u.Object, "status", "externalRef"); ref != policy {
			t.Errorf("status.externalRef = %q, want %q", ref, policy)
		}
		got := get(t)
		if !ours(got) {
			t.Errorf("documentation %+v, strategy %+v", got.Documentation, got.AlertStrategy)
		}
		if got.Documentation.Content != "CPU is high." || got.Documentation.MimeType != "text/markdown" || got.AlertStrategy.AutoClose != "1800s" || got.DisplayName != mockenv.Name("ap") || len(got.Conditions) != 1 {
			t.Errorf("the policy's other fields changed: %+v, documentation %+v, strategy %+v", got, got.Documentation, got.AlertStrategy)
		}
	})

	t.Run("a resync writes nothing", func(t *testing.T) {
		resyncWritesNothing(t, "fields")
	})

	t.Run("Terraform's patch drops them, and the next reconcile puts them back", func(t *testing.T) {
		// What Config Connector's MonitoringAlertPolicy (Terraform) sends
		// when its documentation or strategy changes: each whole, without
		// ours (the partial case).
		if _, err := c.Projects.AlertPolicies.Patch(policy, &api.AlertPolicy{
			Documentation: &api.Documentation{Content: "CPU is very high.", MimeType: "text/markdown"},
			AlertStrategy: &api.AlertStrategy{AutoClose: "3600s"},
			DisplayName:   mockenv.Name("ap"),
		}).UpdateMask("documentation,alertStrategy,displayName").Context(env.Ctx).Do(); err != nil {
			t.Fatal(err)
		}
		if got := get(t); ours(got) {
			t.Errorf("Terraform's whole documentation and strategy kept ours (the fake should replace them): %+v, %+v", got.Documentation, got.AlertStrategy)
		}
		resync(t, "fields")
		settle(t, "fields", ours)
		got := get(t)
		if got.Documentation.Content != "CPU is very high." || got.AlertStrategy.AutoClose != "3600s" {
			t.Errorf("putting the fields back changed Config Connector's: documentation %+v, strategy %+v", got.Documentation, got.AlertStrategy)
		}
		resyncWritesNothing(t, "fields")
	})

	t.Run("a change is patched", func(t *testing.T) {
		update(t, "fields", func(o *krm.MonitoringAlertPolicyFields) {
			o.Spec.Documentation.Links = append(o.Spec.Documentation.Links, krm.AlertPolicy_Documentation_Link{DisplayName: mockenv.Ptr("dashboard"), URL: mockenv.Ptr("https://example.com/d/cpu")})
			o.Spec.AlertStrategy.NotificationPrompts = []string{"OPENED"}
		})
		settle(t, "fields", func(got *api.AlertPolicy) bool {
			return len(got.Documentation.Links) == 2 && got.AlertStrategy != nil && len(got.AlertStrategy.NotificationPrompts) == 1
		})
		resyncWritesNothing(t, "fields")
	})

	t.Run("the CRD refuses no field, an empty documentation and four links", func(t *testing.T) {
		bad := newFields("bad", &monitoringv1beta1.MonitoringAlertPolicyRef{External: policy})
		bad.Spec.Documentation, bad.Spec.AlertStrategy = nil, nil
		if err := env.Kube.Create(env.Ctx, bad); err == nil || !strings.Contains(err.Error(), "at least one of documentation and alertStrategy") {
			t.Errorf("creating no field: %v, want refused", err)
		}
		bad = newFields("bad", &monitoringv1beta1.MonitoringAlertPolicyRef{External: policy})
		bad.Spec.Documentation = &krm.AlertPolicy_DocumentationFields{}
		if err := env.Kube.Create(env.Ctx, bad); err == nil || !strings.Contains(err.Error(), "at least one of subject and links") {
			t.Errorf("creating an empty documentation: %v, want refused", err)
		}
		bad = newFields("bad", &monitoringv1beta1.MonitoringAlertPolicyRef{External: policy})
		l := bad.Spec.Documentation.Links[0]
		bad.Spec.Documentation.Links = []krm.AlertPolicy_Documentation_Link{l, l, l, l}
		if err := env.Kube.Create(env.Ctx, bad); err == nil || !strings.Contains(err.Error(), "must have at most 3 items") {
			t.Errorf("creating four links: %v, want refused", err)
		}
	})

	t.Run("a second object for the policy is refused", func(t *testing.T) {
		dup := newFields("dup", &monitoringv1beta1.MonitoringAlertPolicyRef{External: policy})
		if err := env.Kube.Create(env.Ctx, dup); err != nil {
			t.Fatal(err)
		}
		if msg := env.WaitNotReady(gvk, namespace, "dup"); !strings.Contains(msg, "already sets the fields of") {
			t.Errorf("Ready message = %q, want the second object refused", msg)
		}
		if err := env.Kube.Delete(env.Ctx, dup); err != nil {
			t.Fatal(err)
		}
		env.WaitGone(dup)
	})

	t.Run("a missing policy is waited for, never made", func(t *testing.T) {
		missing := project + "/alertPolicies/" + mockenv.Name("never-made")
		w := newFields("waits", &monitoringv1beta1.MonitoringAlertPolicyRef{External: missing})
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

	t.Run("delete clears the fields and keeps the policy", func(t *testing.T) {
		if err := env.Kube.Delete(env.Ctx, obj); err != nil {
			t.Fatal(err)
		}
		env.WaitGone(obj)
		got := get(t)
		if got.Documentation.Subject != "" || len(got.Documentation.Links) != 0 || (got.AlertStrategy != nil && len(got.AlertStrategy.NotificationPrompts) > 1) {
			t.Errorf("after delete: documentation %+v, strategy %+v", got.Documentation, got.AlertStrategy)
		}
		if got.Documentation.Content != "CPU is very high." || got.AlertStrategy == nil || got.AlertStrategy.AutoClose != "3600s" {
			t.Errorf("delete changed the policy's other fields: documentation %+v, strategy %+v", got.Documentation, got.AlertStrategy)
		}
	})

	t.Run("abandon keeps the fields", func(t *testing.T) {
		k := newFields("kept", &monitoringv1beta1.MonitoringAlertPolicyRef{Name: "policy"})
		k.Annotations = map[string]string{"cnrm.cloud.google.com/deletion-policy": "abandon"}
		if err := env.Kube.Create(env.Ctx, k); err != nil {
			t.Fatal(err)
		}
		env.WaitReady(gvk, namespace, "kept")
		if err := env.Kube.Delete(env.Ctx, k); err != nil {
			t.Fatal(err)
		}
		env.WaitGone(k)
		if got := get(t); !ours(got) {
			t.Errorf("after an abandoning delete: documentation %+v, strategy %+v", got.Documentation, got.AlertStrategy)
		}
	})
}
