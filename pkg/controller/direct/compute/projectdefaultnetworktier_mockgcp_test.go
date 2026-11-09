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

// ComputeProjectDefaultNetworkTier end to end on the fake cloud
// (pkg/test/mockenv, patch 0023) or a real project (docs/live-runs.md). The
// case: a project module puts a cost-sensitive project on the Standard
// tier, later moves it back to Premium, and someone flips it by hand in
// between. The setting changes the whole project, so a live run uses only
// a test project under the test folder (mockenv.FolderProject; skipped
// without one) and puts the tier back as it found it.

import (
	"strings"
	"testing"
	"time"

	gcp "cloud.google.com/go/compute/apiv1"
	pb "cloud.google.com/go/compute/apiv1/computepb"
	refsv1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/apis/refs/v1beta1"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/k8s"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"

	computev1alpha1 "github.com/highfivelabs/config-connector-extensions/apis/compute/v1alpha1"
	"github.com/highfivelabs/config-connector-extensions/pkg/test/mockenv"
)

func TestComputeProjectDefaultNetworkTier(t *testing.T) {
	const namespace = "example"
	tierGVK := computev1alpha1.ComputeProjectDefaultNetworkTierGVK
	env := mockenv.New(t, namespace)
	project := mockenv.FolderProject(t)
	projects, err := gcp.NewProjectsRESTClient(env.Ctx, env.GCPOptions()...)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { projects.Close() })

	tier := func(t *testing.T) string {
		t.Helper()
		p, err := projects.Get(env.Ctx, &pb.GetProjectRequest{Project: project})
		if err != nil {
			t.Fatalf("reading the project: %v", err)
		}
		return p.GetDefaultNetworkTier()
	}
	// set changes the tier directly, as someone with the console would.
	set := func(t *testing.T, to string) {
		t.Helper()
		op, err := projects.SetDefaultNetworkTier(env.Ctx, &pb.SetDefaultNetworkTierProjectRequest{
			Project: project,
			ProjectsSetDefaultNetworkTierRequestResource: &pb.ProjectsSetDefaultNetworkTierRequest{NetworkTier: mockenv.Ptr(to)},
		})
		if err == nil {
			err = op.Wait(env.Ctx)
		}
		if err != nil {
			t.Fatalf("setting the project's default network tier to %s: %v", to, err)
		}
	}
	sets := func() int {
		n := 0
		for _, r := range env.Requests() {
			if strings.HasPrefix(r, "POST ") && strings.Contains(r, "/projects/"+project+"/setDefaultNetworkTier") {
				n++
			}
		}
		return n
	}
	waitTier := func(t *testing.T, want string) {
		t.Helper()
		deadline := time.Now().Add(mockenv.Timeout())
		for got := tier(t); got != want; got = tier(t) {
			if time.Now().After(deadline) {
				t.Fatalf("timed out waiting for the default network tier %s; it is %s", want, got)
			}
			time.Sleep(500 * time.Millisecond)
		}
	}
	getObj := func(t *testing.T) *computev1alpha1.ComputeProjectDefaultNetworkTier {
		t.Helper()
		obj := &computev1alpha1.ComputeProjectDefaultNetworkTier{}
		if err := env.Kube.Get(env.Ctx, types.NamespacedName{Namespace: namespace, Name: "tier"}, obj); err != nil {
			t.Fatal(err)
		}
		return obj
	}

	before := tier(t)
	t.Cleanup(func() {
		if tier(t) != before {
			set(t, before)
		}
	})

	obj := &computev1alpha1.ComputeProjectDefaultNetworkTier{
		ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: "tier", Labels: mockenv.Labels()},
		Spec: computev1alpha1.ComputeProjectDefaultNetworkTierSpec{
			ProjectRef:  &refsv1beta1.ProjectRef{External: project},
			NetworkTier: "STANDARD",
		},
	}
	if err := env.Kube.Create(env.Ctx, obj); err != nil {
		t.Fatal(err)
	}

	t.Run("create sets the tier", func(t *testing.T) {
		u := env.WaitReady(tierGVK, namespace, "tier")
		if got := tier(t); got != "STANDARD" {
			t.Errorf("the project's default network tier is %s, want STANDARD", got)
		}
		if ref, _, _ := unstructured.NestedString(u.Object, "status", "externalRef"); ref != "projects/"+project+"/defaultNetworkTier" {
			t.Errorf("status.externalRef = %q", ref)
		}
		want := 1
		if before == "STANDARD" {
			want = 0
		}
		if n := sets(); n != want {
			t.Errorf("setDefaultNetworkTier was called %d times, want %d", n, want)
		}
	})

	t.Run("a resync sets nothing", func(t *testing.T) {
		n := sets()
		o := getObj(t)
		o.Annotations = map[string]string{k8s.InternalForceReconcileAnnotation: "1"}
		if err := env.Kube.Update(env.Ctx, o); err != nil {
			t.Fatal(err)
		}
		time.Sleep(3 * time.Second)
		env.WaitReady(tierGVK, namespace, "tier")
		if d := sets() - n; d != 0 {
			t.Errorf("a resync called setDefaultNetworkTier %d times", d)
		}
	})

	t.Run("a changed tier is set", func(t *testing.T) {
		n := sets()
		o := getObj(t)
		o.Spec.NetworkTier = "PREMIUM"
		if err := env.Kube.Update(env.Ctx, o); err != nil {
			t.Fatal(err)
		}
		waitTier(t, "PREMIUM")
		env.WaitReady(tierGVK, namespace, "tier")
		if d := sets() - n; d != 1 {
			t.Errorf("setDefaultNetworkTier was called %d times, want 1", d)
		}
	})

	t.Run("a tier changed by hand is put back", func(t *testing.T) {
		set(t, "STANDARD")
		o := getObj(t)
		o.Annotations = map[string]string{k8s.InternalForceReconcileAnnotation: "2"}
		if err := env.Kube.Update(env.Ctx, o); err != nil {
			t.Fatal(err)
		}
		waitTier(t, "PREMIUM")
	})

	t.Run("only PREMIUM or STANDARD", func(t *testing.T) {
		o := &computev1alpha1.ComputeProjectDefaultNetworkTier{
			ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: "fixed"},
			Spec: computev1alpha1.ComputeProjectDefaultNetworkTierSpec{
				ProjectRef:  &refsv1beta1.ProjectRef{External: project},
				NetworkTier: "FIXED_STANDARD",
			},
		}
		if err := env.Kube.Create(env.Ctx, o); err == nil || !strings.Contains(err.Error(), "spec.networkTier") {
			t.Errorf("creating with FIXED_STANDARD: got %v, want the API server to refuse it", err)
		}
	})

	t.Run("delete leaves the tier", func(t *testing.T) {
		n := sets()
		if err := env.Kube.Delete(env.Ctx, getObj(t)); err != nil {
			t.Fatal(err)
		}
		env.WaitGone(&computev1alpha1.ComputeProjectDefaultNetworkTier{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: "tier"}})
		if got := tier(t); got != "PREMIUM" {
			t.Errorf("the tier after delete is %s, want PREMIUM (left as it was)", got)
		}
		if d := sets() - n; d != 0 {
			t.Errorf("delete called setDefaultNetworkTier %d times", d)
		}
	})
}
