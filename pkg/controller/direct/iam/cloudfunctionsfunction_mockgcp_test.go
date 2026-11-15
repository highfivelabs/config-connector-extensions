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

package iam_test

// IAMBinding and IAMMember on a 1st gen Cloud Function (Config Connector's
// CloudFunctionsFunction), named by the object's name: the catalog's
// cloud-function-v1 chart gives exact roles on its function (the builder's
// RECPF answer, 2026-11-15). The function object gives the project, its
// spec.region and its resourceID, which make
// projects/{project}/locations/{region}/functions/{id}, and the policy is
// the Cloud Functions v1 API's :getIamPolicy and :setIamPolicy.
//
// Fake only: the fake cloud's policy store does not need the function to
// exist, and nothing reconciles the Config Connector object here. A live
// run would need a deployed 1st gen function (a source archive and a
// build); the policy verbs are the google.iam.v1 mixin IAMBinding's live
// run already covers (docs/fake-cloud.md).

import (
	"slices"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	iamv1beta1 "github.com/highfivelabs/config-connector-extensions/apis/iam/v1beta1"
	"github.com/highfivelabs/config-connector-extensions/pkg/iampolicy"
	"github.com/highfivelabs/config-connector-extensions/pkg/test/mockenv"
)

var cloudFunctionsFunctionGVK = schema.GroupVersionKind{Group: "cloudfunctions.cnrm.cloud.google.com", Version: "v1beta1", Kind: "CloudFunctionsFunction"}

func TestIAMOnCloudFunctionsFunction(t *testing.T) {
	if mockenv.Live() {
		t.Skip("needs a deployed 1st gen function; fake only")
	}
	env := mockenv.New(t, namespace)
	policies := &iampolicy.Client{HTTP: env.HTTP}
	fnType, ok := iampolicy.Lookup("CloudFunctionsFunction")
	if !ok {
		t.Fatal("CloudFunctionsFunction has no row in the IAM resource table")
	}
	const (
		region    = "us-central1"
		id        = "orders-handler"
		invoker   = "roles/cloudfunctions.invoker"
		developer = "roles/cloudfunctions.developer"
	)
	function := "projects/" + mockenv.Project + "/locations/" + region + "/functions/" + id
	account := func(name string) string {
		return "serviceAccount:" + name + "@" + mockenv.Project + ".iam.gserviceaccount.com"
	}

	// The function as Config Connector's object, reported Ready as Config
	// Connector would.
	fn := &unstructured.Unstructured{Object: map[string]any{"spec": map[string]any{
		"resourceID":        id,
		"region":            region,
		"runtime":           "python312",
		"projectRef":        map[string]any{"external": "projects/" + mockenv.Project},
		"sourceArchiveUrl":  "gs://example-source/orders.zip",
		"httpsTrigger":      map[string]any{},
		"entryPoint":        "handle",
		"availableMemoryMb": int64(256),
	}}}
	fn.SetGroupVersionKind(cloudFunctionsFunctionGVK)
	fn.SetNamespace(namespace)
	fn.SetName("orders")
	if err := env.Kube.Create(env.Ctx, fn); err != nil {
		t.Fatalf("creating the CloudFunctionsFunction object: %v", err)
	}
	if err := unstructured.SetNestedSlice(fn.Object, []any{map[string]any{
		"type": "Ready", "status": "True", "reason": "UpToDate", "lastTransitionTime": time.Now().UTC().Format(time.RFC3339),
	}}, "status", "conditions"); err != nil {
		t.Fatal(err)
	}
	if err := env.Kube.Status().Update(env.Ctx, fn); err != nil {
		t.Fatal(err)
	}

	members := func(t *testing.T, role string) []string {
		t.Helper()
		p, err := policies.Get(env.Ctx, fnType, function)
		if err != nil {
			t.Fatalf("reading the function's policy: %v", err)
		}
		m, _ := iampolicy.Members(p, role, nil)
		return m
	}
	ref := iamv1beta1.IAMResourceReference{Kind: "CloudFunctionsFunction", Name: "orders"}

	t.Run("IAMBinding sets a role's members on the function", func(t *testing.T) {
		b := &iamv1beta1.IAMBinding{
			ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: "orders-invokers"},
			Spec: iamv1beta1.IAMBindingSpec{
				ResourceRef: ref,
				Role:        invoker,
				Members:     []string{account("frontend"), account("scheduler")},
			},
		}
		if err := env.Kube.Create(env.Ctx, b); err != nil {
			t.Fatal(err)
		}
		u := env.WaitReady(bindingGVK, namespace, b.Name)
		if got, _, _ := unstructured.NestedString(u.Object, "status", "externalRef"); got != function+"#"+invoker {
			t.Errorf("status.externalRef = %q, want %q", got, function+"#"+invoker)
		}
		got := members(t, invoker)
		slices.Sort(got)
		if want := []string{account("frontend"), account("scheduler")}; !slices.Equal(got, want) {
			t.Errorf("members of %s = %v, want %v", invoker, got, want)
		}
		if err := env.Kube.Delete(env.Ctx, b); err != nil {
			t.Fatal(err)
		}
		env.WaitGone(b)
		if got := members(t, invoker); len(got) != 0 {
			t.Errorf("members of %s after the delete = %v, want none", invoker, got)
		}
	})

	t.Run("IAMMember adds one member to the function", func(t *testing.T) {
		m := &iamv1beta1.IAMMember{
			ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: "orders-developer"},
			Spec: iamv1beta1.IAMMemberSpec{
				ResourceRef: ref,
				Role:        developer,
				Member:      mockenv.Ptr(account("deployer")),
			},
		}
		if err := env.Kube.Create(env.Ctx, m); err != nil {
			t.Fatal(err)
		}
		env.WaitReady(memberGVK, namespace, m.Name)
		if got := members(t, developer); !slices.Equal(got, []string{account("deployer")}) {
			t.Errorf("members of %s = %v, want [%s]", developer, got, account("deployer"))
		}
		if err := env.Kube.Delete(env.Ctx, m); err != nil {
			t.Fatal(err)
		}
		env.WaitGone(m)
		if got := members(t, developer); len(got) != 0 {
			t.Errorf("members of %s after the delete = %v, want none", developer, got)
		}
	})
}
