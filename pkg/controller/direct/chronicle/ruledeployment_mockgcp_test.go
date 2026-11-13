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

package chronicle_test

// ChronicleRuleDeployment end to end on the fake cloud (pkg/test/mockenv,
// with the Chronicle fake, pkg/controller/direct/chronicle/fake) or a real
// Google SecOps instance (docs/live-runs.md). The case: the catalog's
// secops-rules module, a rule deployed live with alerting, its alerting
// turned off, a change made by hand put back, its schedule customized,
// the rule archived and brought back, a rule named by a Config Connector
// ChronicleRule object, and the object deleted with the deployment left
// as it was (Terraform's google_chronicle_rule_deployment has no delete).

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"
	"testing"
	"time"

	kccchroniclev1alpha1 "github.com/GoogleCloudPlatform/k8s-config-connector/apis/chronicle/v1alpha1"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/k8s"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"

	krm "github.com/highfivelabs/config-connector-extensions/apis/chronicle/v1alpha1"
	"github.com/highfivelabs/config-connector-extensions/pkg/discokind"
	"github.com/highfivelabs/config-connector-extensions/pkg/test/mockenv"
)

const namespace = "example"

var gvk = krm.ChronicleRuleDeploymentGVK

// instance is the SecOps instance the test uses, "<location>/<instance>":
// E2E_CHRONICLE_INSTANCE live, a made-up one on the fake.
func instance(t *testing.T) (location, id string) {
	v := os.Getenv("E2E_CHRONICLE_INSTANCE")
	if mockenv.Live() {
		if v == "" {
			t.Skip("a live ChronicleRuleDeployment needs a Google SecOps (Chronicle) instance, which the live-test project does not have, and roles/chronicle.editor on it for the live-test service account; set E2E_CHRONICLE_INSTANCE=<location>/<instance ID> where one exists (docs/live-runs.md)")
		}
	} else if v == "" {
		v = "us/00000000-0000-4000-8000-000000000000"
	}
	location, id, ok := strings.Cut(v, "/")
	if !ok {
		t.Fatalf("E2E_CHRONICLE_INSTANCE=%q is not <location>/<instance ID>", v)
	}
	return location, id
}

func TestChronicleRuleDeployment(t *testing.T) {
	location, inst := instance(t)
	env := mockenv.New(t, namespace)
	api := &discokind.API{HTTP: env.HTTP, BaseURL: "https://" + location + "-chronicle.googleapis.com/v1/"}
	parent := fmt.Sprintf("projects/%s/locations/%s/instances/%s", mockenv.Project, location, inst)

	// newRule makes a rule in the instance (the object never makes one)
	// and deletes it at the end, whatever the test did.
	newRule := func(t *testing.T, base string) string {
		t.Helper()
		ruleName := strings.ReplaceAll(mockenv.Name(base), "-", "_")
		text := fmt.Sprintf(`rule %s {
  meta:
    description = "config-connector-extensions ChronicleRuleDeployment test"
  events:
    $e.metadata.event_type = "USER_LOGIN"
    $e.principal.hostname = "cce-test-never-matches.invalid"
  condition:
    $e
}`, ruleName)
		got, err := api.Do(env.Ctx, http.MethodPost, parent+"/rules", nil, map[string]any{"text": text})
		if err != nil {
			t.Fatalf("creating rule %s: %v", ruleName, err)
		}
		name, _ := got["name"].(string)
		if name == "" {
			t.Fatalf("the created rule has no name: %v", got)
		}
		t.Cleanup(func() {
			if _, err := api.Do(context.Background(), http.MethodDelete, name, map[string]string{"force": "true"}, nil); err != nil && !strings.Contains(err.Error(), "404") {
				t.Errorf("deleting rule %s: %v", name, err)
			}
		})
		return name
	}
	cloud := func(t *testing.T, rule string) map[string]any {
		t.Helper()
		got, err := api.Get(env.Ctx, rule+"/deployment")
		if err != nil {
			t.Fatalf("reading the deployment of %s: %v", rule, err)
		}
		return got
	}
	// patches are the masks of the PATCHes of a rule's deployment so far.
	patches := func(rule string) []string {
		var out []string
		for _, r := range env.Requests() {
			method, raw, _ := strings.Cut(r, " ")
			if method != http.MethodPatch {
				continue
			}
			u, err := url.Parse(raw)
			if err != nil || !strings.HasSuffix(u.Path, "/v1/"+rule+"/deployment") {
				continue
			}
			mask := strings.Split(u.Query().Get("updateMask"), ",")
			sort.Strings(mask)
			out = append(out, strings.Join(mask, ","))
		}
		return out
	}
	getObj := func(t *testing.T, name string) *krm.ChronicleRuleDeployment {
		t.Helper()
		obj := &krm.ChronicleRuleDeployment{}
		if err := env.Kube.Get(env.Ctx, types.NamespacedName{Namespace: namespace, Name: name}, obj); err != nil {
			t.Fatal(err)
		}
		return obj
	}
	tryUpdate := func(t *testing.T, name string, change func(*krm.ChronicleRuleDeployment)) error {
		t.Helper()
		return retry.RetryOnConflict(retry.DefaultRetry, func() error {
			o := getObj(t, name)
			change(o)
			return env.Kube.Update(env.Ctx, o)
		})
	}
	update := func(t *testing.T, name string, change func(*krm.ChronicleRuleDeployment)) {
		t.Helper()
		if err := tryUpdate(t, name, change); err != nil {
			t.Fatal(err)
		}
	}
	resync := func(t *testing.T, name string) {
		t.Helper()
		update(t, name, func(o *krm.ChronicleRuleDeployment) {
			o.Annotations = map[string]string{k8s.InternalForceReconcileAnnotation: time.Now().Format(time.RFC3339Nano)}
		})
	}
	// waitCloud waits until the deployment holds what check wants (live,
	// a resync is done before the cloud has the change).
	waitCloud := func(t *testing.T, rule string, check func(map[string]any) bool) map[string]any {
		t.Helper()
		deadline := time.Now().Add(mockenv.Timeout())
		for {
			got := cloud(t, rule)
			if check(got) {
				return got
			}
			if time.Now().After(deadline) {
				t.Fatalf("the deployment of %s is %v", rule, got)
			}
			time.Sleep(time.Second)
		}
	}
	newObj := func(name string, ref kccchroniclev1alpha1.ChronicleRuleRef) *krm.ChronicleRuleDeployment {
		return &krm.ChronicleRuleDeployment{
			ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: name, Labels: mockenv.Labels()},
			Spec: krm.ChronicleRuleDeploymentSpec{
				RuleRef:      &ref,
				Enabled:      mockenv.Ptr(true),
				Alerting:     mockenv.Ptr(true),
				RunFrequency: mockenv.Ptr("LIVE"),
			},
		}
	}

	rule := newRule(t, "login")
	obj := newObj("login", kccchroniclev1alpha1.ChronicleRuleRef{External: rule})
	if err := env.Kube.Create(env.Ctx, obj); err != nil {
		t.Fatal(err)
	}

	t.Run("create sets the deployment the rule already has, in one patch", func(t *testing.T) {
		u := env.WaitReady(gvk, namespace, "login")
		if ref, _, _ := unstructured.NestedString(u.Object, "status", "externalRef"); ref != rule+"/deployment" {
			t.Errorf("status.externalRef = %q, want %q", ref, rule+"/deployment")
		}
		got := cloud(t, rule)
		if got["enabled"] != true || got["alerting"] != true {
			t.Errorf("the deployment is %v, want enabled and alerting", got)
		}
		if rf, _ := got["runFrequency"].(string); rf != "LIVE" && rf != "LIVE_CUSTOMIZABLE" {
			t.Errorf("runFrequency = %q", rf)
		}
		if p := patches(rule); len(p) != 1 || p[0] != "alerting,enabled,runFrequency" {
			t.Errorf("patches (masks) = %q, want one of alerting,enabled,runFrequency", p)
		}
		if es, _, _ := unstructured.NestedString(u.Object, "status", "observedState", "executionState"); es == "" {
			t.Errorf("status.observedState.executionState is not set: %v", u.Object["status"])
		}
	})

	t.Run("a resync with nothing changed writes nothing", func(t *testing.T) {
		before := len(patches(rule))
		resync(t, "login")
		time.Sleep(3 * time.Second)
		env.WaitReady(gvk, namespace, "login")
		if n := len(patches(rule)); n != before {
			t.Errorf("a resync patched the deployment (%d patches, was %d)", n, before)
		}
	})

	t.Run("a changed field is patched alone", func(t *testing.T) {
		before := len(patches(rule))
		update(t, "login", func(o *krm.ChronicleRuleDeployment) { o.Spec.Alerting = mockenv.Ptr(false) })
		waitCloud(t, rule, func(d map[string]any) bool { return d["alerting"] != true })
		env.WaitReady(gvk, namespace, "login")
		p := patches(rule)[before:]
		if len(p) != 1 || p[0] != "alerting" {
			t.Errorf("patches (masks) = %q, want one of alerting", p)
		}
	})

	t.Run("a change made by hand is put back", func(t *testing.T) {
		if _, err := api.Do(env.Ctx, http.MethodPatch, rule+"/deployment", map[string]string{"updateMask": "enabled"}, map[string]any{"enabled": false}); err != nil {
			t.Fatal(err)
		}
		resync(t, "login")
		waitCloud(t, rule, func(d map[string]any) bool { return d["enabled"] == true })
	})

	t.Run("schedule customizations go with the run frequency", func(t *testing.T) {
		before := len(patches(rule))
		update(t, "login", func(o *krm.ChronicleRuleDeployment) {
			o.Spec.RunFrequency = mockenv.Ptr("LIVE_CUSTOMIZABLE")
			o.Spec.ScheduleCustomizations = &krm.ChronicleRuleDeploymentScheduleCustomizations{
				EnsureEnrichmentCompleteness: mockenv.Ptr(true),
			}
		})
		got := waitCloud(t, rule, func(d map[string]any) bool {
			sc, _ := d["scheduleCustomizations"].(map[string]any)
			return sc["ensureEnrichmentCompleteness"] == true
		})
		if got["runFrequency"] != "LIVE_CUSTOMIZABLE" {
			t.Errorf("runFrequency = %v", got["runFrequency"])
		}
		env.WaitReady(gvk, namespace, "login")
		p := patches(rule)[before:]
		if len(p) != 1 || p[0] != "runFrequency" {
			t.Errorf("patches (masks) = %q, want one of runFrequency", p)
		}
	})

	t.Run("archiving disables first; unarchiving comes before the rest", func(t *testing.T) {
		before := len(patches(rule))
		update(t, "login", func(o *krm.ChronicleRuleDeployment) {
			o.Spec.Enabled = mockenv.Ptr(false)
			o.Spec.Archived = mockenv.Ptr(true)
		})
		waitCloud(t, rule, func(d map[string]any) bool { return d["archived"] == true })
		u := env.WaitReady(gvk, namespace, "login")
		if at, _, _ := unstructured.NestedString(u.Object, "status", "observedState", "archiveTime"); at == "" {
			t.Errorf("status.observedState.archiveTime is not set: %v", u.Object["status"])
		}
		if p := patches(rule)[before:]; len(p) != 2 || p[0] != "enabled" || p[1] != "archived" {
			t.Errorf("patches (masks) = %q, want enabled then archived", p)
		}
		before = len(patches(rule))
		update(t, "login", func(o *krm.ChronicleRuleDeployment) {
			o.Spec.Enabled = mockenv.Ptr(true)
			o.Spec.Archived = mockenv.Ptr(false)
		})
		waitCloud(t, rule, func(d map[string]any) bool { return d["archived"] != true && d["enabled"] == true })
		env.WaitReady(gvk, namespace, "login")
		if p := patches(rule)[before:]; len(p) != 2 || p[0] != "archived" || p[1] != "enabled" {
			t.Errorf("patches (masks) = %q, want archived then enabled", p)
		}
	})

	t.Run("the CRD refuses a new rule and an archived rule that is enabled or alerting", func(t *testing.T) {
		err := tryUpdate(t, "login", func(o *krm.ChronicleRuleDeployment) {
			o.Spec.RuleRef = &kccchroniclev1alpha1.ChronicleRuleRef{External: parent + "/rules/ru_other"}
		})
		if err == nil || !strings.Contains(err.Error(), "ruleRef is immutable") {
			t.Errorf("changing ruleRef: %v", err)
		}
		bad := newObj("archived-enabled", kccchroniclev1alpha1.ChronicleRuleRef{External: rule})
		bad.Spec.Alerting = nil
		bad.Spec.Archived = mockenv.Ptr(true)
		if err := env.Kube.Create(env.Ctx, bad); err == nil || !strings.Contains(err.Error(), "an archived rule is not enabled") {
			t.Errorf("an archived, enabled deployment: %v", err)
		}
		bad = newObj("archived-alerting", kccchroniclev1alpha1.ChronicleRuleRef{External: rule})
		bad.Spec.Enabled = mockenv.Ptr(false)
		bad.Spec.Archived = mockenv.Ptr(true)
		if err := env.Kube.Create(env.Ctx, bad); err == nil || !strings.Contains(err.Error(), "an archived rule does not alert") {
			t.Errorf("an archived, alerting deployment: %v", err)
		}
	})

	t.Run("a rule that does not exist is not ready, and says so", func(t *testing.T) {
		missing := newObj("missing", kccchroniclev1alpha1.ChronicleRuleRef{External: parent + "/rules/ru_00000000-0000-4000-8000-00000000dead"})
		if err := env.Kube.Create(env.Ctx, missing); err != nil {
			t.Fatal(err)
		}
		if msg := env.WaitNotReady(gvk, namespace, "missing"); !strings.Contains(msg, "does the rule exist") {
			t.Errorf("the message does not say the rule is missing: %q", msg)
		}
		if err := env.Kube.Delete(env.Ctx, missing); err != nil {
			t.Fatal(err)
		}
		env.WaitGone(missing)
	})

	t.Run("a Config Connector ChronicleRule is named by object once it has an externalRef", func(t *testing.T) {
		// Config Connector's ChronicleRule is not run here; the test writes
		// its status as Config Connector would, within the test, so no
		// reference is left unresolved at the end.
		byObj := newRule(t, "by-object")
		const kccObj = "kcc-rule"
		kcc := &unstructured.Unstructured{Object: map[string]any{"spec": map[string]any{
			"projectRef": map[string]any{"external": mockenv.Project},
			"location":   location,
			"text":       "rule by_object { condition: $e }",
		}}}
		kcc.SetGroupVersionKind(kccchroniclev1alpha1.ChronicleRuleGVK)
		kcc.SetNamespace(namespace)
		kcc.SetName(kccObj)
		if err := env.Kube.Create(env.Ctx, kcc); err != nil {
			t.Fatal(err)
		}
		d := newObj("by-object", kccchroniclev1alpha1.ChronicleRuleRef{Name: kccObj})
		if err := env.Kube.Create(env.Ctx, d); err != nil {
			t.Fatal(err)
		}
		if msg := env.WaitNotReady(gvk, namespace, "by-object"); !strings.Contains(msg, kccObj) {
			t.Errorf("the message does not name the rule that is not ready: %q", msg)
		}
		if err := unstructured.SetNestedField(kcc.Object, byObj, "status", "externalRef"); err != nil {
			t.Fatal(err)
		}
		if err := env.Kube.Status().Update(env.Ctx, kcc); err != nil {
			t.Fatal(err)
		}
		resync(t, "by-object")
		u := env.WaitReady(gvk, namespace, "by-object")
		if ref, _, _ := unstructured.NestedString(u.Object, "status", "externalRef"); ref != byObj+"/deployment" {
			t.Errorf("status.externalRef = %q", ref)
		}
		if got := cloud(t, byObj); got["enabled"] != true {
			t.Errorf("the deployment of the rule named by object: %v", got)
		}
		if err := env.Kube.Delete(env.Ctx, d); err != nil {
			t.Fatal(err)
		}
		env.WaitGone(d)
		if err := env.Kube.Delete(env.Ctx, kcc); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("delete leaves the deployment as it is", func(t *testing.T) {
		o := getObj(t, "login")
		before := len(patches(rule))
		if err := env.Kube.Delete(env.Ctx, o); err != nil {
			t.Fatal(err)
		}
		env.WaitGone(o)
		if got := cloud(t, rule); got["enabled"] != true {
			t.Errorf("the deployment after delete: %v, want it left enabled", got)
		}
		if n := len(patches(rule)); n != before {
			t.Errorf("delete patched the deployment")
		}
	})
}
