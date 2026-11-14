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

// ChronicleRule (chronicle.gcp.hfl.dev/v1beta1) end to end on the fake
// cloud (the Chronicle fake, pkg/controller/direct/chronicle/fake) or a
// real Google SecOps instance (docs/live-runs.md). The case: the catalog's
// secops-rules module, a rule made in an instance with the ID the API
// assigns, its text changed, a change made by hand put back, a rule that
// exists acquired by spec.resourceID, a ChronicleRuleDeployment that names
// the rule by object, and the rule and its deployment deleted at once.

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	kccchroniclev1alpha1 "github.com/GoogleCloudPlatform/k8s-config-connector/apis/chronicle/v1alpha1"
	refsv1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/apis/refs/v1beta1"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/k8s"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"

	deploymentkrm "github.com/highfivelabs/config-connector-extensions/apis/chronicle/v1alpha1"
	krm "github.com/highfivelabs/config-connector-extensions/apis/chronicle/v1beta1"
	"github.com/highfivelabs/config-connector-extensions/pkg/discokind"
	"github.com/highfivelabs/config-connector-extensions/pkg/test/mockenv"
)

func TestChronicleRule(t *testing.T) {
	location, inst := instance(t)
	const ns = "rules"
	env := mockenv.New(t, ns)
	api := &discokind.API{HTTP: env.HTTP, BaseURL: "https://" + location + "-chronicle.googleapis.com/v1/"}
	parent := fmt.Sprintf("projects/%s/locations/%s/instances/%s", mockenv.Project, location, inst)
	ruleGVK := krm.ChronicleRuleGVK

	// text is a rule that matches nothing, named after base.
	text := func(base, hostname string) string {
		return fmt.Sprintf(`rule %s {
  meta:
    description = "config-connector-extensions ChronicleRule test"
  events:
    $e.metadata.event_type = "USER_LOGIN"
    $e.principal.hostname = "%s"
  condition:
    $e
}`, strings.ReplaceAll(mockenv.Name(base), "-", "_"), hostname)
	}
	// deleteRule deletes a rule at the end, whatever the test did.
	deleteRule := func(t *testing.T, name string) {
		t.Cleanup(func() {
			if _, err := api.Do(context.Background(), http.MethodDelete, name, map[string]string{"force": "true"}, nil); err != nil && !strings.Contains(err.Error(), "404") {
				t.Errorf("deleting rule %s: %v", name, err)
			}
		})
	}
	cloud := func(t *testing.T, name string) (map[string]any, bool) {
		t.Helper()
		got, err := api.Get(env.Ctx, name)
		if err != nil {
			if strings.Contains(err.Error(), "404") {
				return nil, false
			}
			t.Fatalf("reading %s: %v", name, err)
		}
		return got, true
	}
	// requests are the method and query of the requests to a path so far.
	requests := func(path string) []string {
		var out []string
		for _, r := range env.Requests() {
			method, raw, _ := strings.Cut(r, " ")
			u, err := url.Parse(raw)
			if err != nil || !strings.HasSuffix(u.Path, "/v1/"+path) {
				continue
			}
			q := u.Query()
			q.Del("$alt")
			q.Del("view")
			out = append(out, strings.TrimSuffix(method+" "+q.Encode(), " "))
		}
		return out
	}
	writes := func(path string) []string {
		var out []string
		for _, r := range requests(path) {
			if !strings.HasPrefix(r, http.MethodGet) {
				out = append(out, r)
			}
		}
		return out
	}
	getObj := func(t *testing.T, name string) *krm.ChronicleRule {
		t.Helper()
		obj := &krm.ChronicleRule{}
		if err := env.Kube.Get(env.Ctx, types.NamespacedName{Namespace: ns, Name: name}, obj); err != nil {
			t.Fatal(err)
		}
		return obj
	}
	tryUpdate := func(t *testing.T, name string, change func(*krm.ChronicleRule)) error {
		t.Helper()
		return retry.RetryOnConflict(retry.DefaultRetry, func() error {
			o := getObj(t, name)
			change(o)
			return env.Kube.Update(env.Ctx, o)
		})
	}
	update := func(t *testing.T, name string, change func(*krm.ChronicleRule)) {
		t.Helper()
		if err := tryUpdate(t, name, change); err != nil {
			t.Fatal(err)
		}
	}
	resync := func(t *testing.T, name string) {
		t.Helper()
		update(t, name, func(o *krm.ChronicleRule) {
			if o.Annotations == nil {
				o.Annotations = map[string]string{}
			}
			o.Annotations[k8s.InternalForceReconcileAnnotation] = time.Now().Format(time.RFC3339Nano)
		})
	}
	waitCloud := func(t *testing.T, name string, check func(map[string]any) bool) map[string]any {
		t.Helper()
		deadline := time.Now().Add(mockenv.Timeout())
		for {
			got, _ := cloud(t, name)
			if got != nil && check(got) {
				return got
			}
			if time.Now().After(deadline) {
				t.Fatalf("%s is %v", name, got)
			}
			time.Sleep(time.Second)
		}
	}
	newObj := func(name, ruleText string) *krm.ChronicleRule {
		return &krm.ChronicleRule{
			ObjectMeta: metav1.ObjectMeta{Namespace: ns, Name: name, Labels: mockenv.Labels()},
			Spec: krm.ChronicleRuleSpec{
				ProjectRef: &refsv1beta1.ProjectRef{External: mockenv.Project},
				Location:   location,
				Instance:   inst,
				Text:       mockenv.Ptr(ruleText),
			},
		}
	}
	externalRef := func(u *unstructured.Unstructured) string {
		s, _, _ := unstructured.NestedString(u.Object, "status", "externalRef")
		return s
	}

	first := text("login", "cce-test-never-matches.invalid")
	obj := newObj("login", first)
	if err := env.Kube.Create(env.Ctx, obj); err != nil {
		t.Fatal(err)
	}
	var rule string
	outer := t

	t.Run("create makes the rule with the ID the API assigns", func(t *testing.T) {
		u := env.WaitReady(ruleGVK, ns, "login")
		rule = externalRef(u)
		if !strings.HasPrefix(rule, parent+"/rules/ru_") {
			t.Fatalf("status.externalRef = %q, want a rule of %s", rule, parent)
		}
		deleteRule(outer, rule)
		got, ok := cloud(t, rule)
		if !ok || got["text"] != first {
			t.Errorf("the rule is %v, want its text", got)
		}
		if n := len(writes(parent + "/rules")); n != 1 {
			t.Errorf("%d creates, want 1", n)
		}
		if rev, _, _ := unstructured.NestedString(u.Object, "status", "observedState", "revisionID"); rev == "" {
			t.Errorf("status.observedState.revisionID is not set: %v", u.Object["status"])
		}
		if dn, _, _ := unstructured.NestedString(u.Object, "status", "observedState", "displayName"); dn != strings.ReplaceAll(mockenv.Name("login"), "-", "_") {
			t.Errorf("status.observedState.displayName = %q", dn)
		}
	})
	if rule == "" {
		t.FailNow()
	}

	t.Run("a resync with nothing changed writes nothing", func(t *testing.T) {
		before := len(writes(rule)) + len(writes(parent+"/rules"))
		resync(t, "login")
		time.Sleep(3 * time.Second)
		env.WaitReady(ruleGVK, ns, "login")
		if n := len(writes(rule)) + len(writes(parent+"/rules")); n != before {
			t.Errorf("a resync wrote (%d writes, was %d)", n, before)
		}
	})

	second := text("login", "cce-test-never-matches-either.invalid")
	t.Run("a changed text is patched alone", func(t *testing.T) {
		before := len(writes(rule))
		update(t, "login", func(o *krm.ChronicleRule) { o.Spec.Text = mockenv.Ptr(second) })
		waitCloud(t, rule, func(r map[string]any) bool { return r["text"] == second })
		env.WaitReady(ruleGVK, ns, "login")
		if w := writes(rule)[before:]; len(w) != 1 || w[0] != "PATCH updateMask=text" {
			t.Errorf("writes = %q, want one PATCH of text", w)
		}
	})

	t.Run("a change made by hand is put back", func(t *testing.T) {
		if _, err := api.Do(env.Ctx, http.MethodPatch, rule, map[string]string{"updateMask": "text"}, map[string]any{"text": first}); err != nil {
			t.Fatal(err)
		}
		resync(t, "login")
		waitCloud(t, rule, func(r map[string]any) bool { return r["text"] == second })
	})

	t.Run("the CRD refuses a new instance, location or rule ID", func(t *testing.T) {
		for field, change := range map[string]func(*krm.ChronicleRule){
			"instance":   func(o *krm.ChronicleRule) { o.Spec.Instance = "00000000-0000-4000-8000-0000000000ff" },
			"location":   func(o *krm.ChronicleRule) { o.Spec.Location = "elsewhere" },
			"resourceID": func(o *krm.ChronicleRule) { o.Spec.ResourceID = mockenv.Ptr("ru_other") },
		} {
			if field == "resourceID" {
				// Set it to the assigned ID first (allowed), then change it.
				id := rule[strings.LastIndex(rule, "/")+1:]
				update(t, "login", func(o *krm.ChronicleRule) { o.Spec.ResourceID = mockenv.Ptr(id) })
			}
			if err := tryUpdate(t, "login", change); err == nil || !strings.Contains(err.Error(), field+" is immutable") {
				t.Errorf("changing %s: %v", field, err)
			}
		}
		env.WaitReady(ruleGVK, ns, "login")
	})

	t.Run("spec.resourceID acquires a rule that exists, and abandon keeps it", func(t *testing.T) {
		made, err := api.Do(env.Ctx, http.MethodPost, parent+"/rules", nil, map[string]any{"text": text("kept", "cce-test-kept.invalid")})
		if err != nil {
			t.Fatal(err)
		}
		name, _ := made["name"].(string)
		deleteRule(t, name)
		creates := len(writes(parent + "/rules"))
		kept := newObj("kept", text("kept", "cce-test-kept.invalid"))
		kept.Spec.ResourceID = mockenv.Ptr(name[strings.LastIndex(name, "/")+1:])
		kept.Annotations = map[string]string{k8s.DeletionPolicyAnnotation: k8s.DeletionPolicyAbandon}
		if err := env.Kube.Create(env.Ctx, kept); err != nil {
			t.Fatal(err)
		}
		if got := externalRef(env.WaitReady(ruleGVK, ns, "kept")); got != name {
			t.Errorf("status.externalRef = %q, want %q", got, name)
		}
		if n := len(writes(parent + "/rules")); n != creates {
			t.Errorf("acquiring created a rule")
		}
		if err := env.Kube.Delete(env.Ctx, kept); err != nil {
			t.Fatal(err)
		}
		env.WaitGone(kept)
		if _, ok := cloud(t, name); !ok {
			t.Errorf("the abandoned %s is gone", name)
		}
	})

	t.Run("spec.resourceID of a rule that is not there is not ready, and says so", func(t *testing.T) {
		missing := newObj("missing", text("missing", "cce-test-missing.invalid"))
		missing.Spec.ResourceID = mockenv.Ptr("ru_00000000-0000-4000-8000-00000000dead")
		if err := env.Kube.Create(env.Ctx, missing); err != nil {
			t.Fatal(err)
		}
		if msg := env.WaitNotReady(ruleGVK, ns, "missing"); !strings.Contains(msg, "acquires a rule that exists") {
			t.Errorf("the message does not say why: %q", msg)
		}
		if err := env.Kube.Delete(env.Ctx, missing); err != nil {
			t.Fatal(err)
		}
		env.WaitGone(missing)
	})

	t.Run("a ChronicleRuleDeployment names the rule by object", func(t *testing.T) {
		d := &deploymentkrm.ChronicleRuleDeployment{
			ObjectMeta: metav1.ObjectMeta{Namespace: ns, Name: "login", Labels: mockenv.Labels()},
			Spec: deploymentkrm.ChronicleRuleDeploymentSpec{
				RuleRef:  &kccchroniclev1alpha1.ChronicleRuleRef{Name: "login"},
				Enabled:  mockenv.Ptr(true),
				Alerting: mockenv.Ptr(true),
			},
		}
		if err := env.Kube.Create(env.Ctx, d); err != nil {
			t.Fatal(err)
		}
		u := env.WaitReady(deploymentkrm.ChronicleRuleDeploymentGVK, ns, "login")
		if got := externalRef(u); got != rule+"/deployment" {
			t.Errorf("the deployment's status.externalRef = %q, want %q", got, rule+"/deployment")
		}
		if got, _ := cloud(t, rule+"/deployment"); got["enabled"] != true || got["alerting"] != true {
			t.Errorf("the deployment is %v", got)
		}
	})

	t.Run("the rule and its deployment deleted at once both go, and so does the rule", func(t *testing.T) {
		d := &deploymentkrm.ChronicleRuleDeployment{}
		if err := env.Kube.Get(env.Ctx, types.NamespacedName{Namespace: ns, Name: "login"}, d); err != nil {
			t.Fatal(err)
		}
		o := getObj(t, "login")
		if err := env.Kube.Delete(env.Ctx, o); err != nil {
			t.Fatal(err)
		}
		if err := env.Kube.Delete(env.Ctx, d); err != nil {
			t.Fatal(err)
		}
		env.WaitGone(o)
		env.WaitGone(d)
		if _, ok := cloud(t, rule); ok {
			t.Errorf("%s is still there", rule)
		}
		if w := writes(rule); len(w) == 0 || w[len(w)-1] != "DELETE" {
			t.Errorf("the last write = %q, want a DELETE without force (deletionPolicy left out)", w)
		}
	})

	t.Run("deletionPolicy FORCE deletes with force", func(t *testing.T) {
		forced := newObj("forced", text("forced", "cce-test-forced.invalid"))
		forced.Spec.DeletionPolicy = mockenv.Ptr("FORCE")
		if err := env.Kube.Create(env.Ctx, forced); err != nil {
			t.Fatal(err)
		}
		name := externalRef(env.WaitReady(ruleGVK, ns, "forced"))
		deleteRule(t, name)
		if err := env.Kube.Delete(env.Ctx, getObj(t, "forced")); err != nil {
			t.Fatal(err)
		}
		env.WaitGone(forced)
		if w := writes(name); len(w) == 0 || w[len(w)-1] != "DELETE force=true" {
			t.Errorf("the last write = %q, want DELETE force=true", w)
		}
	})
}
