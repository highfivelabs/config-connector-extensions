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

package serviceusage_test

// ServiceIdentity end to end on the fake cloud (pkg/test/mockenv, patch
// 0022) or a real project (docs/live-runs.md). The case: a landing zone's
// folder gets OS Config's folder service agent so the folder's patch
// deployments can run, and the organization gets Cloud Asset's so an
// organization feed can publish. The API has only
// services.generateServiceIdentity, which nothing undoes, so delete
// leaves the identity. A live run uses the test folder
// (mockenv.TestFolder) and never the organization.

import (
	"strings"
	"testing"
	"time"

	refsv1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/apis/refs/v1beta1"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/k8s"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"

	serviceusagev1alpha1 "github.com/highfivelabs/config-connector-extensions/apis/serviceusage/v1alpha1"
	"github.com/highfivelabs/config-connector-extensions/pkg/test/mockenv"
)

const (
	namespace = "example"
	org       = "organizations/987654"
)

var gvk = serviceusagev1alpha1.ServiceIdentityGVK

func TestServiceIdentity(t *testing.T) {
	env := mockenv.New(t, namespace)
	folder := mockenv.TestFolder(t)

	generated := func(parent string) int {
		n := 0
		for _, r := range env.Requests() {
			if strings.HasPrefix(r, "POST ") && strings.Contains(r, "/v1beta1/"+parent+"/services/") && strings.Contains(r, ":generateServiceIdentity") {
				n++
			}
		}
		return n
	}
	getObj := func(t *testing.T, name string) *serviceusagev1alpha1.ServiceIdentity {
		t.Helper()
		obj := &serviceusagev1alpha1.ServiceIdentity{}
		if err := env.Kube.Get(env.Ctx, types.NamespacedName{Namespace: namespace, Name: name}, obj); err != nil {
			t.Fatal(err)
		}
		return obj
	}

	osconfig := &serviceusagev1alpha1.ServiceIdentity{
		ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: "folder-osconfig", Labels: mockenv.Labels()},
		Spec: serviceusagev1alpha1.ServiceIdentitySpec{
			ResourceID: mockenv.Ptr("osconfig.googleapis.com"),
			FolderRef:  &refsv1beta1.FolderRefDeprecated{External: folder},
		},
	}
	if err := env.Kube.Create(env.Ctx, osconfig); err != nil {
		t.Fatal(err)
	}

	var email string
	t.Run("create generates the folder's service identity", func(t *testing.T) {
		u := env.WaitReady(gvk, namespace, "folder-osconfig")
		if ref, _, _ := unstructured.NestedString(u.Object, "status", "externalRef"); ref != folder+"/services/osconfig.googleapis.com" {
			t.Errorf("status.externalRef = %q", ref)
		}
		email, _, _ = unstructured.NestedString(u.Object, "status", "email")
		if !strings.HasSuffix(email, ".iam.gserviceaccount.com") {
			t.Errorf("status.email = %q, want a service agent", email)
		}
		if !mockenv.Live() {
			if want := "service-folder-" + strings.TrimPrefix(folder, "folders/") + "@gcp-sa-osconfig.iam.gserviceaccount.com"; email != want {
				t.Errorf("status.email = %q, want %q", email, want)
			}
		}
		if observed, _, _ := unstructured.NestedString(u.Object, "status", "observedState", "email"); observed != email {
			t.Errorf("status.observedState.email = %q, want %q", observed, email)
		}
		if id, _, _ := unstructured.NestedString(u.Object, "status", "observedState", "uniqueID"); id == "" {
			t.Errorf("status.observedState.uniqueID is empty")
		}
		if n := generated(folder); n != 1 {
			t.Errorf("generateServiceIdentity was called %d times for %s, want 1", n, folder)
		}
	})

	t.Run("a resync calls nothing", func(t *testing.T) {
		before := generated(folder)
		o := getObj(t, "folder-osconfig")
		o.Annotations = map[string]string{k8s.InternalForceReconcileAnnotation: "1"}
		if err := env.Kube.Update(env.Ctx, o); err != nil {
			t.Fatal(err)
		}
		time.Sleep(3 * time.Second)
		u := env.WaitReady(gvk, namespace, "folder-osconfig")
		if n := generated(folder) - before; n != 0 {
			t.Errorf("a resync called generateServiceIdentity %d times", n)
		}
		if got, _, _ := unstructured.NestedString(u.Object, "status", "email"); got != email {
			t.Errorf("status.email = %q after a resync, want %q", got, email)
		}
	})

	t.Run("the organization's service identity", func(t *testing.T) {
		if mockenv.Live() {
			t.Skip("a live run never touches an organization (docs/live-runs.md)")
		}
		o := &serviceusagev1alpha1.ServiceIdentity{
			ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: "cloudasset.googleapis.com"},
			Spec: serviceusagev1alpha1.ServiceIdentitySpec{
				OrganizationRef: &refsv1beta1.OrganizationRef{External: org},
			},
		}
		if err := env.Kube.Create(env.Ctx, o); err != nil {
			t.Fatal(err)
		}
		u := env.WaitReady(gvk, namespace, "cloudasset.googleapis.com")
		if ref, _, _ := unstructured.NestedString(u.Object, "status", "externalRef"); ref != org+"/services/cloudasset.googleapis.com" {
			t.Errorf("status.externalRef = %q (the service is metadata.name without resourceID)", ref)
		}
		if got, _, _ := unstructured.NestedString(u.Object, "status", "email"); got != "service-org-987654@gcp-sa-cloudasset.iam.gserviceaccount.com" {
			t.Errorf("status.email = %q", got)
		}
	})

	t.Run("exactly one parent", func(t *testing.T) {
		for name, spec := range map[string]serviceusagev1alpha1.ServiceIdentitySpec{
			"both": {
				OrganizationRef: &refsv1beta1.OrganizationRef{External: org},
				FolderRef:       &refsv1beta1.FolderRefDeprecated{External: folder},
			},
			"neither": {},
		} {
			o := &serviceusagev1alpha1.ServiceIdentity{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: name}, Spec: spec}
			err := env.Kube.Create(env.Ctx, o)
			if err == nil || !strings.Contains(err.Error(), "exactly one of organizationRef and folderRef") {
				t.Errorf("creating with %s: got %v, want the API server to refuse it", name, err)
			}
		}
	})

	t.Run("a changed parent is refused", func(t *testing.T) {
		before := generated("folders/654321")
		o := getObj(t, "folder-osconfig")
		o.Spec.FolderRef = &refsv1beta1.FolderRefDeprecated{External: "folders/654321"}
		if err := env.Kube.Update(env.Ctx, o); err != nil {
			t.Fatal(err)
		}
		if msg := env.WaitNotReady(gvk, namespace, "folder-osconfig"); !strings.Contains(msg, "cannot change ServiceIdentity identity") {
			t.Errorf("Ready message = %q, want the change refused", msg)
		}
		if n := generated("folders/654321") - before; n != 0 {
			t.Errorf("generateServiceIdentity was called %d times for the new folder", n)
		}
		o = getObj(t, "folder-osconfig")
		o.Spec.FolderRef = &refsv1beta1.FolderRefDeprecated{External: folder}
		if err := env.Kube.Update(env.Ctx, o); err != nil {
			t.Fatal(err)
		}
		env.WaitReady(gvk, namespace, "folder-osconfig")
	})

	t.Run("delete leaves the identity and calls nothing", func(t *testing.T) {
		before := len(env.Requests())
		if err := env.Kube.Delete(env.Ctx, getObj(t, "folder-osconfig")); err != nil {
			t.Fatal(err)
		}
		env.WaitGone(&serviceusagev1alpha1.ServiceIdentity{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: "folder-osconfig"}})
		for _, r := range env.Requests()[before:] {
			if strings.Contains(r, "serviceusage") {
				t.Errorf("delete called %s", r)
			}
		}
	})
}
