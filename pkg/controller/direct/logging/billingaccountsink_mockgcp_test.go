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

package logging_test

// LoggingBillingAccountSink end to end on the fake cloud (pkg/test/mockenv,
// patch 0029) or a real billing account (docs/live-runs.md). The case: a
// landing zone exports its billing account's audit logs (who changed
// budgets, who linked projects) to a Pub/Sub topic in the platform's
// project, leaving out a noisy log and later pausing the export.
//
// A live run needs a billing account the live-test service account
// administers logging on (roles/logging.configWriter on the billing
// account), passed as E2E_BILLING_ACCOUNT=billingAccounts/{id}; without it
// a live run skips the test (live: pending, design.md decision 15).

import (
	"os"
	"strings"
	"testing"
	"time"

	gcp "cloud.google.com/go/logging/apiv2"
	pb "cloud.google.com/go/logging/apiv2/loggingpb"
	billingv1alpha1 "github.com/GoogleCloudPlatform/k8s-config-connector/apis/billing/v1alpha1"
	loggingv1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/apis/logging/v1beta1"
	pubsubv1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/apis/pubsub/v1beta1"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/k8s"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"

	loggingv1alpha1 "github.com/highfivelabs/config-connector-extensions/apis/logging/v1alpha1"
	"github.com/highfivelabs/config-connector-extensions/pkg/test/mockenv"
)

func TestLoggingBillingAccountSink(t *testing.T) {
	account := "billingAccounts/012345-6789AB-CDEF01"
	if mockenv.Live() {
		account = os.Getenv("E2E_BILLING_ACCOUNT")
		if account == "" {
			t.Skip("a live LoggingBillingAccountSink needs a billing account the live-test service account administers logging on; set E2E_BILLING_ACCOUNT=billingAccounts/{id} (docs/live-runs.md)")
		}
	}
	sinkGVK := loggingv1alpha1.LoggingBillingAccountSinkGVK
	env := mockenv.New(t, namespace)
	cloud, err := gcp.NewConfigRESTClient(env.Ctx, env.GCPOptions()...)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cloud.Close() })

	sinkName := func(base string) string { return account + "/sinks/" + mockenv.Name(base) }
	topic := "projects/" + mockenv.Project + "/topics/" + mockenv.Name("billing-audit")
	get := func(name string) (*pb.LogSink, error) {
		return cloud.GetSink(env.Ctx, &pb.GetSinkRequest{SinkName: name})
	}
	mustGet := func(t *testing.T, name string) *pb.LogSink {
		t.Helper()
		s, err := get(name)
		if err != nil {
			t.Fatalf("reading %s: %v", name, err)
		}
		return s
	}
	// writes are the requests that change a sink of this billing account.
	writes := func() []string {
		var out []string
		for _, r := range env.Requests() {
			if strings.HasPrefix(r, "GET ") || strings.HasPrefix(r, "DELETE ") {
				continue
			}
			if strings.Contains(r, "/v2/"+account+"/sinks") {
				out = append(out, r)
			}
		}
		return out
	}
	getObj := func(t *testing.T, name string) *loggingv1alpha1.LoggingBillingAccountSink {
		t.Helper()
		obj := &loggingv1alpha1.LoggingBillingAccountSink{}
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
	// Delete what the test made, the abandoned sink included, and what a
	// failed step left.
	t.Cleanup(func() {
		for _, base := range []string{"billing-audit", "kept"} {
			err := cloud.DeleteSink(env.Ctx, &pb.DeleteSinkRequest{SinkName: sinkName(base)})
			if err != nil && !direct.IsNotFound(err) {
				t.Errorf("cleaning up sink %s: %v", sinkName(base), err)
			}
		}
	})

	newSink := func(name string) *loggingv1alpha1.LoggingBillingAccountSink {
		return &loggingv1alpha1.LoggingBillingAccountSink{
			ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: name, Labels: mockenv.Labels()},
			Spec: loggingv1alpha1.LoggingBillingAccountSinkSpec{
				BillingAccountRef: billingv1alpha1.BillingAccountRef{External: account},
				ResourceID:        mockenv.Ptr(mockenv.Name(name)),
				Destination: loggingv1beta1.LogSinkDestination{
					PubSubTopicRef: &pubsubv1beta1.PubSubTopicRef{External: topic},
				},
				Filter:      mockenv.Ptr(`logName:"cloudaudit.googleapis.com"`),
				Description: mockenv.Ptr("billing account audit logs for the platform"),
			},
		}
	}

	sink := newSink("billing-audit")
	sink.Spec.Exclusions = []loggingv1beta1.LogSinkExclusions{{
		Name:   "no-reads",
		Filter: `protoPayload.methodName:"Get"`,
	}}
	if err := env.Kube.Create(env.Ctx, sink); err != nil {
		t.Fatal(err)
	}

	t.Run("create makes the sink under the billing account", func(t *testing.T) {
		u := env.WaitReady(sinkGVK, namespace, "billing-audit")
		if ref, _, _ := unstructured.NestedString(u.Object, "status", "externalRef"); ref != sinkName("billing-audit") {
			t.Errorf("status.externalRef = %q, want %q", ref, sinkName("billing-audit"))
		}
		got := mustGet(t, sinkName("billing-audit"))
		if got.Destination != "pubsub.googleapis.com/"+topic {
			t.Errorf("destination = %q", got.Destination)
		}
		if got.Filter != `logName:"cloudaudit.googleapis.com"` || got.Disabled {
			t.Errorf("filter %q, disabled %v", got.Filter, got.Disabled)
		}
		if len(got.Exclusions) != 1 || got.Exclusions[0].Name != "no-reads" {
			t.Errorf("exclusions = %v", got.Exclusions)
		}
		wi, _, _ := unstructured.NestedString(u.Object, "status", "writerIdentity")
		if wi == "" || wi != got.WriterIdentity {
			t.Errorf("status.writerIdentity = %q, cloud %q", wi, got.WriterIdentity)
		}
		var create string
		for _, r := range writes() {
			if strings.HasPrefix(r, "POST ") {
				create = r
			}
		}
		if !strings.Contains(create, "uniqueWriterIdentity=true") {
			t.Errorf("create request %q, want uniqueWriterIdentity=true", create)
		}
	})

	t.Run("a resync writes nothing", func(t *testing.T) {
		before := len(writes())
		o := getObj(t, "billing-audit")
		o.Annotations = map[string]string{k8s.InternalForceReconcileAnnotation: "1"}
		if err := env.Kube.Update(env.Ctx, o); err != nil {
			t.Fatal(err)
		}
		time.Sleep(3 * time.Second)
		env.WaitReady(sinkGVK, namespace, "billing-audit")
		if w := writes()[before:]; len(w) != 0 {
			t.Errorf("a resync wrote %v", w)
		}
	})

	t.Run("pausing and a second exclusion are one update of those fields", func(t *testing.T) {
		before := len(writes())
		o := getObj(t, "billing-audit")
		o.Spec.Disabled = mockenv.Ptr(true)
		o.Spec.Exclusions = append(o.Spec.Exclusions, loggingv1beta1.LogSinkExclusions{
			Name:        "no-lists",
			Description: mockenv.Ptr("list calls are noise"),
			Filter:      `protoPayload.methodName:"List"`,
		})
		if err := env.Kube.Update(env.Ctx, o); err != nil {
			t.Fatal(err)
		}
		waitFor(t, "the sink to be disabled", func() bool {
			s, err := get(sinkName("billing-audit"))
			return err == nil && s.Disabled && len(s.Exclusions) == 2
		})
		env.WaitReady(sinkGVK, namespace, "billing-audit")
		w := writes()[before:]
		if len(w) != 1 {
			t.Fatalf("writes = %v, want one update", w)
		}
		if !strings.Contains(w[0], "disabled") || !strings.Contains(w[0], "exclusions") || strings.Contains(w[0], "destination") || strings.Contains(w[0], "filter") {
			t.Errorf("update %q, want a mask of disabled and exclusions only", w[0])
		}
		got := mustGet(t, sinkName("billing-audit"))
		if got.Destination != "pubsub.googleapis.com/"+topic || got.Exclusions[1].Description != "list calls are noise" {
			t.Errorf("sink after update = %v", got)
		}
	})

	t.Run("the CRD refuses a change of billing account", func(t *testing.T) {
		o := getObj(t, "billing-audit")
		o.Spec.BillingAccountRef.External = "billingAccounts/000000-000000-000000"
		err := env.Kube.Update(env.Ctx, o)
		if err == nil || !strings.Contains(err.Error(), "billingAccountRef is immutable") {
			t.Errorf("changing billingAccountRef: got %v, want the API server to refuse it", err)
		}
	})

	t.Run("the CRD refuses a sink without a billing account", func(t *testing.T) {
		o := newSink("no-account")
		o.Spec.BillingAccountRef = billingv1alpha1.BillingAccountRef{}
		err := env.Kube.Create(env.Ctx, o)
		if err == nil {
			_ = env.Kube.Delete(env.Ctx, o)
			t.Errorf("creating a sink with an empty billingAccountRef: accepted, want the API server to refuse it")
		}
	})

	t.Run("delete deletes the sink", func(t *testing.T) {
		if err := env.Kube.Delete(env.Ctx, getObj(t, "billing-audit")); err != nil {
			t.Fatal(err)
		}
		env.WaitGone(&loggingv1alpha1.LoggingBillingAccountSink{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: "billing-audit"}})
		if _, err := get(sinkName("billing-audit")); !direct.IsNotFound(err) {
			t.Errorf("after delete, reading the sink: %v, want not found", err)
		}
	})

	t.Run("abandon keeps the sink", func(t *testing.T) {
		kept := newSink("kept")
		if err := env.Kube.Create(env.Ctx, kept); err != nil {
			t.Fatal(err)
		}
		env.WaitReady(sinkGVK, namespace, "kept")
		kept = getObj(t, "kept")
		kept.Annotations = map[string]string{"cnrm.cloud.google.com/deletion-policy": "abandon"}
		if err := env.Kube.Update(env.Ctx, kept); err != nil {
			t.Fatal(err)
		}
		if err := env.Kube.Delete(env.Ctx, kept); err != nil {
			t.Fatal(err)
		}
		env.WaitGone(&loggingv1alpha1.LoggingBillingAccountSink{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: "kept"}})
		if _, err := get(sinkName("kept")); err != nil {
			t.Errorf("after abandon, reading the sink: %v, want it kept", err)
		}
	})
}
