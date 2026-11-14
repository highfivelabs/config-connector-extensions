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

// IAMAccessPolicy when IAM refuses GetAccessPolicy for quota (429
// RESOURCE_EXHAUSTED, 5 requests a minute per project). The catalog's live
// runs in hfl-dev-514205 found the policy marked failed (UpdateFailed) on
// such a refusal and read again within a second, which the quota refused
// again. Now a refusal requeues with a growing wait, and the object is
// never marked failed (patch 0047, pkg/controller QuotaWait).

import (
	"net/http"
	"slices"
	"strings"
	"testing"

	gcp "cloud.google.com/go/iam/apiv3"
	pb "cloud.google.com/go/iam/apiv3/iampb"
	refsv1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/apis/refs/v1beta1"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	iamv1beta1 "github.com/highfivelabs/config-connector-extensions/apis/iam/v1beta1"
	"github.com/highfivelabs/config-connector-extensions/pkg/test/mockenv"
)

func TestIAMAccessPolicyRefusedForQuota(t *testing.T) {
	if mockenv.Live() {
		t.Skip("an access policy needs iam.accessPolicies.* (roles/iam.accessPolicyAdmin), which the live-test account does not have; live: pending that role")
	}
	gvk := iamv1beta1.IAMAccessPolicyGVK
	env := mockenv.New(t, namespace)
	cloud, err := gcp.NewAccessPoliciesRESTClient(env.Ctx, env.GCPOptions()...)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cloud.Close() })
	const id = "quota-agents"
	name := "projects/" + mockenv.Project + "/locations/global/accessPolicies/" + id
	isGet := func(req *http.Request) bool {
		return req.Method == http.MethodGet && strings.HasSuffix(req.URL.Path, "/"+name)
	}
	failed := func() []string {
		var out []string
		for _, r := range env.Events(namespace, id) {
			if r == "UpdateFailed" || r == "DeleteFailed" {
				out = append(out, r)
			}
		}
		return out
	}

	refused := env.RefuseForQuota(4, isGet)
	if err := env.Kube.Create(env.Ctx, &iamv1beta1.IAMAccessPolicy{
		ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: id},
		Spec: iamv1beta1.IAMAccessPolicySpec{
			ProjectRef:  &refsv1beta1.ProjectRef{External: mockenv.Project},
			DisplayName: mockenv.Ptr("refused for quota first"),
		},
	}); err != nil {
		t.Fatal(err)
	}

	t.Run("refused reads wait, and the policy is made", func(t *testing.T) {
		env.WaitReady(gvk, namespace, id)
		if n := refused(); n != 4 {
			t.Errorf("reads refused = %d, want 4", n)
		}
		if _, err := cloud.GetAccessPolicy(env.Ctx, &pb.GetAccessPolicyRequest{Name: name}); err != nil {
			t.Errorf("reading the policy: %v", err)
		}
		if f := failed(); len(f) > 0 {
			t.Errorf("events %v: a refusal for quota marked the policy failed", f)
		}
		if !slices.Contains(env.Events(namespace, id), "QuotaExhausted") {
			t.Errorf("no QuotaExhausted event, events %v", env.Events(namespace, id))
		}
	})

	t.Run("refused reads wait, and the delete goes on", func(t *testing.T) {
		refused := env.RefuseForQuota(2, isGet)
		obj := &iamv1beta1.IAMAccessPolicy{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: id}}
		if err := env.Kube.Delete(env.Ctx, obj); err != nil {
			t.Fatal(err)
		}
		env.WaitGone(obj)
		if n := refused(); n != 2 {
			t.Errorf("reads refused = %d, want 2", n)
		}
		if _, err := cloud.GetAccessPolicy(env.Ctx, &pb.GetAccessPolicyRequest{Name: name}); !direct.IsNotFound(err) {
			t.Errorf("reading the deleted policy: %v, want not found", err)
		}
		if f := failed(); len(f) > 0 {
			t.Errorf("events %v: a refusal for quota marked the policy failed", f)
		}
	})
}
