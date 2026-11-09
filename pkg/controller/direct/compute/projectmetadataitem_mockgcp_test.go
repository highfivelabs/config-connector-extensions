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

// ComputeProjectMetadataItem end to end on the fake cloud (pkg/test/mockenv;
// mockgcp already serves projects.get and setCommonInstanceMetadata) or a
// real project (docs/live-runs.md). The case: a project module's metadata
// keys (OS Login on, a team owner) set one by one beside a key nobody here
// manages. Every key is mockenv.Name(...), so a live run's keys carry its
// prefix; the test removes them all at the end, the abandoned one
// included.

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

func TestComputeProjectMetadataItem(t *testing.T) {
	const namespace = "example"
	itemGVK := computev1alpha1.ComputeProjectMetadataItemGVK
	env := mockenv.New(t, namespace)
	projects, err := gcp.NewProjectsRESTClient(env.Ctx, env.GCPOptions()...)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { projects.Close() })

	bases := []string{"other", "oslogin", "owner", "region", "kept"}
	// metadata is the project's metadata as key to value.
	metadata := func(t *testing.T) (map[string]string, *pb.Metadata) {
		t.Helper()
		p, err := projects.Get(env.Ctx, &pb.GetProjectRequest{Project: mockenv.Project})
		if err != nil {
			t.Fatalf("reading the project: %v", err)
		}
		out := map[string]string{}
		for _, item := range p.GetCommonInstanceMetadata().GetItems() {
			out[item.GetKey()] = item.GetValue()
		}
		return out, p.GetCommonInstanceMetadata()
	}
	// write sets (value non-nil) or removes keys directly, as another
	// writer would, retrying while the fingerprint is stale.
	write := func(t *testing.T, change map[string]*string) {
		t.Helper()
		for attempt := 0; ; attempt++ {
			_, md := metadata(t)
			var items []*pb.Items
			for _, item := range md.GetItems() {
				if _, ok := change[item.GetKey()]; !ok {
					items = append(items, item)
				}
			}
			for k, v := range change {
				if v != nil {
					items = append(items, &pb.Items{Key: mockenv.Ptr(k), Value: v})
				}
			}
			op, err := projects.SetCommonInstanceMetadata(env.Ctx, &pb.SetCommonInstanceMetadataProjectRequest{
				Project: mockenv.Project, MetadataResource: &pb.Metadata{Fingerprint: md.Fingerprint, Items: items},
			})
			if err == nil {
				err = op.Wait(env.Ctx)
			}
			if err == nil {
				return
			}
			if attempt == 4 {
				t.Fatalf("writing the project's metadata: %v", err)
			}
			time.Sleep(time.Second)
		}
	}
	t.Cleanup(func() {
		change := map[string]*string{}
		for _, b := range bases {
			change[mockenv.Name(b)] = nil
		}
		write(t, change)
	})
	writes := func() int {
		n := 0
		for _, r := range env.Requests() {
			if strings.HasPrefix(r, "POST ") && strings.Contains(r, "/setCommonInstanceMetadata") {
				n++
			}
		}
		return n
	}
	waitFor := func(t *testing.T, cond func(map[string]string) bool) {
		t.Helper()
		deadline := time.Now().Add(mockenv.Timeout())
		for {
			md, _ := metadata(t)
			if cond(md) {
				return
			}
			if time.Now().After(deadline) {
				t.Fatalf("timed out waiting for the project's metadata; it is %v", md)
			}
			time.Sleep(500 * time.Millisecond)
		}
	}
	newItem := func(base, value string) *computev1alpha1.ComputeProjectMetadataItem {
		return &computev1alpha1.ComputeProjectMetadataItem{
			ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: base, Labels: mockenv.Labels()},
			Spec: computev1alpha1.ComputeProjectMetadataItemSpec{
				ProjectRef: &refsv1beta1.ProjectRef{External: mockenv.Project},
				ResourceID: mockenv.Ptr(mockenv.Name(base)),
				Value:      value,
			},
		}
	}
	getObj := func(t *testing.T, name string) *computev1alpha1.ComputeProjectMetadataItem {
		t.Helper()
		obj := &computev1alpha1.ComputeProjectMetadataItem{}
		if err := env.Kube.Get(env.Ctx, types.NamespacedName{Namespace: namespace, Name: name}, obj); err != nil {
			t.Fatal(err)
		}
		return obj
	}

	// A key nobody here manages.
	write(t, map[string]*string{mockenv.Name("other"): mockenv.Ptr("left alone")})

	if err := env.Kube.Create(env.Ctx, newItem("oslogin", "TRUE")); err != nil {
		t.Fatal(err)
	}

	t.Run("create adds the key and leaves the others", func(t *testing.T) {
		u := env.WaitReady(itemGVK, namespace, "oslogin")
		want := "projects/" + mockenv.Project + "/commonInstanceMetadata/" + mockenv.Name("oslogin")
		if ref, _, _ := unstructured.NestedString(u.Object, "status", "externalRef"); ref != want {
			t.Errorf("status.externalRef = %q, want %q", ref, want)
		}
		md, _ := metadata(t)
		if md[mockenv.Name("oslogin")] != "TRUE" || md[mockenv.Name("other")] != "left alone" {
			t.Errorf("project metadata = %v", md)
		}
	})

	t.Run("a resync writes nothing", func(t *testing.T) {
		before := writes()
		o := getObj(t, "oslogin")
		o.Annotations = map[string]string{k8s.InternalForceReconcileAnnotation: "1"}
		if err := env.Kube.Update(env.Ctx, o); err != nil {
			t.Fatal(err)
		}
		time.Sleep(3 * time.Second)
		env.WaitReady(itemGVK, namespace, "oslogin")
		if n := writes() - before; n != 0 {
			t.Errorf("a resync wrote the metadata %d times", n)
		}
	})

	t.Run("a new value is written once", func(t *testing.T) {
		before := writes()
		o := getObj(t, "oslogin")
		o.Spec.Value = "FALSE"
		if err := env.Kube.Update(env.Ctx, o); err != nil {
			t.Fatal(err)
		}
		waitFor(t, func(md map[string]string) bool { return md[mockenv.Name("oslogin")] == "FALSE" })
		env.WaitReady(itemGVK, namespace, "oslogin")
		if n := writes() - before; n != 1 {
			t.Errorf("wrote the metadata %d times, want once", n)
		}
	})

	t.Run("a value changed outside is put back", func(t *testing.T) {
		write(t, map[string]*string{mockenv.Name("oslogin"): mockenv.Ptr("changed by hand")})
		o := getObj(t, "oslogin")
		o.Annotations = map[string]string{k8s.InternalForceReconcileAnnotation: "2"}
		if err := env.Kube.Update(env.Ctx, o); err != nil {
			t.Fatal(err)
		}
		waitFor(t, func(md map[string]string) bool { return md[mockenv.Name("oslogin")] == "FALSE" })
	})

	t.Run("two keys at once both land", func(t *testing.T) {
		for _, o := range []*computev1alpha1.ComputeProjectMetadataItem{newItem("owner", "team-a"), newItem("region", "europe-west1")} {
			if err := env.Kube.Create(env.Ctx, o); err != nil {
				t.Fatal(err)
			}
		}
		env.WaitReady(itemGVK, namespace, "owner")
		env.WaitReady(itemGVK, namespace, "region")
		md, _ := metadata(t)
		if md[mockenv.Name("owner")] != "team-a" || md[mockenv.Name("region")] != "europe-west1" || md[mockenv.Name("oslogin")] != "FALSE" || md[mockenv.Name("other")] != "left alone" {
			t.Errorf("project metadata = %v", md)
		}
	})

	t.Run("delete removes only its key", func(t *testing.T) {
		if err := env.Kube.Delete(env.Ctx, getObj(t, "owner")); err != nil {
			t.Fatal(err)
		}
		env.WaitGone(&computev1alpha1.ComputeProjectMetadataItem{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: "owner"}})
		md, _ := metadata(t)
		if _, ok := md[mockenv.Name("owner")]; ok {
			t.Errorf("the deleted key is still there: %v", md)
		}
		if md[mockenv.Name("region")] != "europe-west1" || md[mockenv.Name("other")] != "left alone" {
			t.Errorf("other keys changed: %v", md)
		}
	})

	t.Run("abandon keeps the key", func(t *testing.T) {
		k := newItem("kept", "yes")
		k.Annotations = map[string]string{"cnrm.cloud.google.com/deletion-policy": "abandon"}
		if err := env.Kube.Create(env.Ctx, k); err != nil {
			t.Fatal(err)
		}
		env.WaitReady(itemGVK, namespace, "kept")
		if err := env.Kube.Delete(env.Ctx, k); err != nil {
			t.Fatal(err)
		}
		env.WaitGone(k)
		if md, _ := metadata(t); md[mockenv.Name("kept")] != "yes" {
			t.Errorf("the abandoned key is gone: %v", md)
		}
	})
}
