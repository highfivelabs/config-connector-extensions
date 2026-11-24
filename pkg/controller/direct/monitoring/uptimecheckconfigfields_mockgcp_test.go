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

// MonitoringUptimeCheckConfigFields end to end on the fake cloud
// (pkg/test/mockenv) or a real project (docs/live-runs.md). The checks are
// the run's own (display name mockenv.Name), probe example.com every 15
// minutes for the few minutes they exist (uptime checks cost nothing at
// this volume), and are deleted at the end.

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

const namespace = "example"

func TestMonitoringUptimeCheckConfigFields(t *testing.T) {
	gvk := krm.MonitoringUptimeCheckConfigFieldsGVK
	env := mockenv.New(t, namespace)
	c, err := api.NewService(env.Ctx, env.GCPOptions()...)
	if err != nil {
		t.Fatal(err)
	}
	project := "projects/" + mockenv.Project
	// makeCheck makes an uptime check of example.com and deletes it after
	// the test; it returns its name.
	makeCheck := func(t *testing.T, base string, check *api.UptimeCheckConfig) string {
		t.Helper()
		check.DisplayName = mockenv.Name(base)
		check.Period = "900s"
		check.Timeout = "10s"
		check.MonitoredResource = &api.MonitoredResource{Type: "uptime_url", Labels: map[string]string{"project_id": mockenv.Project, "host": "example.com"}}
		got, err := c.Projects.UptimeCheckConfigs.Create(project, check).Context(env.Ctx).Do()
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if _, err := c.Projects.UptimeCheckConfigs.Delete(got.Name).Context(env.Ctx).Do(); err != nil && !direct.IsNotFound(err) {
				t.Errorf("deleting uptime check %s: %v", got.Name, err)
			}
		})
		return got.Name
	}
	get := func(t *testing.T, name string) *api.UptimeCheckConfig {
		t.Helper()
		got, err := c.Projects.UptimeCheckConfigs.Get(name).Context(env.Ctx).Do()
		if err != nil {
			t.Fatalf("reading the uptime check %s: %v", name, err)
		}
		return got
	}
	patches := func(name string) int {
		n := 0
		for _, req := range env.Requests() {
			if strings.HasPrefix(req, "PATCH ") && strings.Contains(req, name) {
				n++
			}
		}
		return n
	}
	getObj := func(t *testing.T, name string) *krm.MonitoringUptimeCheckConfigFields {
		t.Helper()
		o := &krm.MonitoringUptimeCheckConfigFields{}
		if err := env.Kube.Get(env.Ctx, types.NamespacedName{Namespace: namespace, Name: name}, o); err != nil {
			t.Fatal(err)
		}
		return o
	}
	update := func(t *testing.T, name string, change func(*krm.MonitoringUptimeCheckConfigFields)) {
		t.Helper()
		o := getObj(t, name)
		change(o)
		if err := env.Kube.Update(env.Ctx, o); err != nil {
			t.Fatal(err)
		}
	}
	resync := func(t *testing.T, name string) {
		t.Helper()
		update(t, name, func(o *krm.MonitoringUptimeCheckConfigFields) {
			o.Annotations = map[string]string{k8s.InternalForceReconcileAnnotation: time.Now().String()}
		})
	}
	resyncWritesNothing := func(t *testing.T, name, check string) {
		t.Helper()
		before := patches(check)
		resync(t, name)
		time.Sleep(3 * time.Second)
		env.WaitReady(gvk, namespace, name)
		if n := patches(check) - before; n != 0 {
			t.Errorf("a resync patched the check %d times", n)
		}
	}
	settle := func(t *testing.T, name, check string, ok func(*api.UptimeCheckConfig) bool) {
		t.Helper()
		deadline := time.Now().Add(mockenv.Timeout())
		for !ok(get(t, check)) {
			if time.Now().After(deadline) {
				t.Fatalf("the check did not change: %+v (http %+v)", get(t, check), get(t, check).HttpCheck)
			}
			time.Sleep(time.Second)
		}
		env.WaitReady(gvk, namespace, name)
	}
	newFields := func(name string, ref *monitoringv1beta1.MonitoringUptimeCheckConfigRef) *krm.MonitoringUptimeCheckConfigFields {
		return &krm.MonitoringUptimeCheckConfigFields{
			ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: name, Labels: mockenv.Labels()},
			Spec: krm.MonitoringUptimeCheckConfigFieldsSpec{
				ResourceRef: ref,
				UserLabels:  map[string]string{"team": "payments"},
				HTTPCheck: &krm.UptimeCheckConfig_HTTPCheckFields{
					AcceptedResponseStatusCodes: []krm.UptimeCheckConfig_HTTPCheck_ResponseStatusCode{
						{StatusClass: mockenv.Ptr("STATUS_CLASS_2XX")}, {StatusValue: mockenv.Ptr(int32(404))},
					},
					PingConfig:                 &krm.UptimeCheckConfig_PingConfig{PingsCount: mockenv.Ptr(int32(2))},
					ServiceAgentAuthentication: &krm.UptimeCheckConfig_HTTPCheck_ServiceAgentAuthentication{Type: mockenv.Ptr("OIDC_TOKEN")},
				},
			},
		}
	}

	check := makeCheck(t, "uc", &api.UptimeCheckConfig{HttpCheck: &api.HttpCheck{Path: "/", Port: 443, UseSsl: true, RequestMethod: "GET"}})
	checkID := check[strings.LastIndex(check, "/")+1:]
	// Config Connector's MonitoringUptimeCheckConfig for it (not reconciled
	// here: read to resolve resourceRef by name, from its spec).
	cc := &unstructured.Unstructured{Object: map[string]any{"spec": map[string]any{
		"resourceID":  checkID,
		"displayName": mockenv.Name("uc"),
		"timeout":     "10s",
		"projectRef":  map[string]any{"external": mockenv.Project},
	}}}
	cc.SetGroupVersionKind(schema.GroupVersionKind{Group: "monitoring.cnrm.cloud.google.com", Version: "v1beta1", Kind: "MonitoringUptimeCheckConfig"})
	cc.SetNamespace(namespace)
	cc.SetName("check")
	if err := env.Kube.Create(env.Ctx, cc); err != nil {
		t.Fatal(err)
	}

	obj := newFields("fields", &monitoringv1beta1.MonitoringUptimeCheckConfigRef{Name: "check"})
	if err := env.Kube.Create(env.Ctx, obj); err != nil {
		t.Fatal(err)
	}

	t.Run("create sets the fields, and nothing else", func(t *testing.T) {
		u := env.WaitReady(gvk, namespace, "fields")
		if ref, _, _ := unstructured.NestedString(u.Object, "status", "externalRef"); ref != check {
			t.Errorf("status.externalRef = %q, want %q", ref, check)
		}
		got := get(t, check)
		h := got.HttpCheck
		if got.UserLabels["team"] != "payments" || h.PingConfig == nil || h.PingConfig.PingsCount != 2 ||
			len(h.AcceptedResponseStatusCodes) != 2 || h.AcceptedResponseStatusCodes[1].StatusValue != 404 ||
			h.ServiceAgentAuthentication == nil || h.ServiceAgentAuthentication.Type != "OIDC_TOKEN" {
			t.Errorf("the check is %+v, http %+v", got, h)
		}
		if got.DisplayName != mockenv.Name("uc") || h.Path != "/" || h.Port != 443 || !h.UseSsl || got.Period != "900s" {
			t.Errorf("the check's other fields changed: %+v, http %+v", got, h)
		}
	})

	t.Run("DCL's patch keeps them", func(t *testing.T) {
		// What Config Connector's MonitoringUptimeCheckConfig (DCL) sends
		// when its fields change: the paths of those fields, never these.
		if _, err := c.Projects.UptimeCheckConfigs.Patch(check, &api.UptimeCheckConfig{
			DisplayName: mockenv.Name("uc") + "-changed",
			HttpCheck:   &api.HttpCheck{Path: "/health", Port: 443, UseSsl: true, RequestMethod: "GET"},
		}).UpdateMask("displayName,httpCheck.path").Context(env.Ctx).Do(); err != nil {
			t.Fatal(err)
		}
		got := get(t, check)
		if got.HttpCheck.Path != "/health" || got.HttpCheck.PingConfig == nil || got.UserLabels["team"] != "payments" {
			t.Errorf("after DCL's patch: %+v, http %+v", got, got.HttpCheck)
		}
	})

	t.Run("a resync writes nothing", func(t *testing.T) {
		resyncWritesNothing(t, "fields", check)
	})

	t.Run("a change is patched, and one outside is set again", func(t *testing.T) {
		update(t, "fields", func(o *krm.MonitoringUptimeCheckConfigFields) {
			o.Spec.HTTPCheck.PingConfig.PingsCount = mockenv.Ptr(int32(3))
			o.Spec.UserLabels["tier"] = "gold"
		})
		settle(t, "fields", check, func(got *api.UptimeCheckConfig) bool {
			return got.HttpCheck.PingConfig != nil && got.HttpCheck.PingConfig.PingsCount == 3 && got.UserLabels["tier"] == "gold"
		})
		if _, err := c.Projects.UptimeCheckConfigs.Patch(check, &api.UptimeCheckConfig{HttpCheck: &api.HttpCheck{}}).UpdateMask("httpCheck.acceptedResponseStatusCodes").Context(env.Ctx).Do(); err != nil {
			t.Fatal(err)
		}
		resync(t, "fields")
		settle(t, "fields", check, func(got *api.UptimeCheckConfig) bool { return len(got.HttpCheck.AcceptedResponseStatusCodes) == 2 })
		resyncWritesNothing(t, "fields", check)
	})

	t.Run("the CRD refuses no field and a status code with both value and class", func(t *testing.T) {
		bad := newFields("bad", &monitoringv1beta1.MonitoringUptimeCheckConfigRef{External: check})
		bad.Spec.UserLabels, bad.Spec.HTTPCheck = nil, nil
		if err := env.Kube.Create(env.Ctx, bad); err == nil || !strings.Contains(err.Error(), "at least one of") {
			t.Errorf("creating no field: %v, want refused", err)
		}
		bad = newFields("bad", &monitoringv1beta1.MonitoringUptimeCheckConfigRef{External: check})
		bad.Spec.HTTPCheck.AcceptedResponseStatusCodes = []krm.UptimeCheckConfig_HTTPCheck_ResponseStatusCode{{StatusValue: mockenv.Ptr(int32(200)), StatusClass: mockenv.Ptr("STATUS_CLASS_2XX")}}
		if err := env.Kube.Create(env.Ctx, bad); err == nil || !strings.Contains(err.Error(), "one of statusValue and statusClass") {
			t.Errorf("creating a status code with both: %v, want refused", err)
		}
	})

	t.Run("a second object for the check is refused", func(t *testing.T) {
		dup := newFields("dup", &monitoringv1beta1.MonitoringUptimeCheckConfigRef{External: check})
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

	t.Run("HTTP fields on a TCP check are refused", func(t *testing.T) {
		tcp := makeCheck(t, "uc-tcp", &api.UptimeCheckConfig{TcpCheck: &api.TcpCheck{Port: 443}})
		o := newFields("tcp", &monitoringv1beta1.MonitoringUptimeCheckConfigRef{External: tcp})
		if err := env.Kube.Create(env.Ctx, o); err != nil {
			t.Fatal(err)
		}
		if msg := env.WaitNotReady(gvk, namespace, "tcp"); !strings.Contains(msg, "is not an HTTP check") {
			t.Errorf("Ready message = %q, want the TCP check refused", msg)
		}
		if got := get(t, tcp); got.HttpCheck != nil || len(got.UserLabels) != 0 {
			t.Errorf("the TCP check changed: %+v", got)
		}
		if err := env.Kube.Delete(env.Ctx, o); err != nil {
			t.Fatal(err)
		}
		env.WaitGone(o)
	})

	t.Run("a missing check is waited for, never made", func(t *testing.T) {
		missing := project + "/uptimeCheckConfigs/" + mockenv.Name("never-made")
		w := newFields("waits", &monitoringv1beta1.MonitoringUptimeCheckConfigRef{External: missing})
		if err := env.Kube.Create(env.Ctx, w); err != nil {
			t.Fatal(err)
		}
		if msg := env.WaitNotReady(gvk, namespace, "waits"); !strings.Contains(msg, "does not exist yet") {
			t.Errorf("Ready message = %q, want the missing check waited for", msg)
		}
		if err := env.Kube.Delete(env.Ctx, w); err != nil {
			t.Fatal(err)
		}
		env.WaitGone(w)
	})

	t.Run("delete clears the fields and keeps the check", func(t *testing.T) {
		if err := env.Kube.Delete(env.Ctx, obj); err != nil {
			t.Fatal(err)
		}
		env.WaitGone(obj)
		got := get(t, check)
		h := got.HttpCheck
		if len(got.UserLabels) != 0 || len(h.AcceptedResponseStatusCodes) != 0 || (h.PingConfig != nil && h.PingConfig.PingsCount != 0) || (h.ServiceAgentAuthentication != nil && h.ServiceAgentAuthentication.Type != "") {
			t.Errorf("after delete: %+v, http %+v", got, h)
		}
		if h.Path != "/health" || got.DisplayName != mockenv.Name("uc")+"-changed" {
			t.Errorf("delete changed the check's other fields: %+v, http %+v", got, h)
		}
	})

	t.Run("abandon keeps the fields", func(t *testing.T) {
		k := newFields("kept", &monitoringv1beta1.MonitoringUptimeCheckConfigRef{Name: "check"})
		k.Annotations = map[string]string{"cnrm.cloud.google.com/deletion-policy": "abandon"}
		if err := env.Kube.Create(env.Ctx, k); err != nil {
			t.Fatal(err)
		}
		env.WaitReady(gvk, namespace, "kept")
		if err := env.Kube.Delete(env.Ctx, k); err != nil {
			t.Fatal(err)
		}
		env.WaitGone(k)
		if got := get(t, check); got.UserLabels["team"] != "payments" || got.HttpCheck.PingConfig == nil {
			t.Errorf("after an abandoning delete the check is %+v, http %+v", got, got.HttpCheck)
		}
	})
}
