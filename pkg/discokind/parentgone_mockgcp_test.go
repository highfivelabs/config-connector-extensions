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

package discokind_test

import (
	"net/http"
	"testing"
	"time"

	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/k8s"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/highfivelabs/config-connector-extensions/pkg/discokind"
	"github.com/highfivelabs/config-connector-extensions/pkg/test/mockenv"
)

// TestComputeNetworkAttachmentWithItsParent deletes a generated kind
// (pkg/discokind) together with the parent object its projectRef names:
// when everything is deleted at once, the parent object often goes first.
// The delete takes the resource's name from status.externalRef and does not
// resolve the parent, so it neither waits on the gone parent nor leaves the
// resource behind. The fake only: a Project object needs no cloud call
// (nothing here runs Project's controller), but the attachment's live
// subnet is made by the live kind test's Setup, not here.
func TestComputeNetworkAttachmentWithItsParent(t *testing.T) {
	if mockenv.Live() {
		t.Skip("fake only: TestComputeNetworkAttachment covers the kind live")
	}
	const ns = "parentgone"
	env := mockenv.New(t, ns)
	k := discokind.ByKind("ComputeNetworkAttachment")
	api := &discokind.API{HTTP: env.HTTP, BaseURL: k.BaseURL, PollInterval: 200 * time.Millisecond}
	projectGVK := schema.GroupVersionKind{Group: "resourcemanager.cnrm.cloud.google.com", Version: "v1beta1", Kind: "Project"}

	project := func(name string) *unstructured.Unstructured {
		u := &unstructured.Unstructured{}
		u.SetGroupVersionKind(projectGVK)
		u.SetNamespace(ns)
		u.SetName(name)
		_ = unstructured.SetNestedField(u.Object, mockenv.Project, "spec", "resourceID")
		_ = unstructured.SetNestedField(u.Object, "mock project", "spec", "name")
		_ = unstructured.SetNestedField(u.Object, "123456789", "spec", "organizationRef", "external")
		if err := env.Kube.Create(env.Ctx, u); err != nil {
			t.Fatal(err)
		}
		return u
	}
	attachment := func(name, projectObj string, abandon bool) *unstructured.Unstructured {
		u := &unstructured.Unstructured{}
		u.SetGroupVersionKind(k.GVK)
		u.SetNamespace(ns)
		u.SetName(name)
		if abandon {
			u.SetAnnotations(map[string]string{k8s.DeletionPolicyAnnotation: k8s.DeletionPolicyAbandon})
		}
		u.Object["spec"] = map[string]any{
			"projectRef":           map[string]any{"name": projectObj},
			"region":               "us-central1",
			"connectionPreference": "ACCEPT_MANUAL",
			"subnetworks":          []any{"projects/" + mockenv.Project + "/regions/us-central1/subnetworks/" + name + "-subnet"},
		}
		if err := env.Kube.Create(env.Ctx, u); err != nil {
			t.Fatal(err)
		}
		got := env.WaitReady(k.GVK, ns, name)
		ext, _, _ := unstructured.NestedString(got.Object, "status", "externalRef")
		if ext == "" {
			t.Fatalf("%s: status.externalRef is not set", name)
		}
		t.Cleanup(func() {
			if _, err := api.Get(env.Ctx, ext); err == nil {
				op, err := api.Do(env.Ctx, http.MethodDelete, ext, nil, nil)
				if err == nil && k.DeleteLRO {
					_, err = api.Wait(env.Ctx, op)
				}
				if err != nil && !direct.IsNotFound(err) {
					t.Errorf("cleanup: deleting %s: %v", ext, err)
				}
			}
		})
		return got
	}
	inCloud := func(name string) bool {
		_, err := api.Get(env.Ctx, name)
		if direct.IsNotFound(err) {
			return false
		}
		if err != nil {
			t.Fatalf("reading %s: %v", name, err)
		}
		return true
	}
	externalRef := func(u *unstructured.Unstructured) string {
		s, _, _ := unstructured.NestedString(u.Object, "status", "externalRef")
		return s
	}
	del := func(objs ...client.Object) {
		for _, o := range objs {
			if err := env.Kube.Delete(env.Ctx, o); err != nil {
				t.Fatal(err)
			}
		}
	}

	t.Run("deleted together with its parent, both go and so does the resource", func(t *testing.T) {
		p := project(mockenv.Name("together-project"))
		a := attachment(mockenv.Name("together"), p.GetName(), false)
		name := externalRef(a)
		del(p, a)
		env.WaitGone(p)
		env.WaitGone(a)
		if inCloud(name) {
			t.Errorf("%s is still in the cloud", name)
		}
	})

	t.Run("its parent already gone, it still deletes the resource", func(t *testing.T) {
		p := project(mockenv.Name("first-project"))
		a := attachment(mockenv.Name("first"), p.GetName(), false)
		name := externalRef(a)
		del(p)
		env.WaitGone(p)
		del(a)
		env.WaitGone(a)
		if inCloud(name) {
			t.Errorf("%s is still in the cloud", name)
		}
	})

	t.Run("abandoned with its parent gone, it keeps the resource", func(t *testing.T) {
		p := project(mockenv.Name("kept-project"))
		a := attachment(mockenv.Name("kept"), p.GetName(), true)
		name := externalRef(a)
		del(p)
		env.WaitGone(p)
		del(a)
		env.WaitGone(a)
		if !inCloud(name) {
			t.Errorf("the abandoned %s is gone from the cloud", name)
		}
	})
}
