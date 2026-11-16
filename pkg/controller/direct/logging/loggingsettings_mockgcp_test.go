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

// LoggingSettings end to end on the fake cloud (pkg/test/mockenv, patch
// 0017) or a real project (docs/live-runs.md). The case: a landing zone's
// folder and organization keep their logs in Europe, encrypted with their
// own key, with the _Default sink off because an aggregated sink already
// collects everything. Settings cannot be deleted, so the test puts the
// folder's back as it found them; against a real project it uses a test
// folder (mockenv.LoggingFolder) and never changes an organization's.

import (
	"strings"
	"testing"
	"time"

	kmsv1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/apis/kms/v1beta1"
	refsv1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/apis/refs/v1beta1"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/k8s"
	api "google.golang.org/api/logging/v2"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"

	loggingv1beta1 "github.com/highfivelabs/config-connector-extensions/apis/logging/v1beta1"
	"github.com/highfivelabs/config-connector-extensions/pkg/test/mockenv"
)

const (
	namespace = "example"
	org       = "organizations/987654"
)

var gvk = loggingv1beta1.LoggingSettingsGVK

func TestLoggingSettings(t *testing.T) {
	env := mockenv.New(t, namespace)
	svc, err := api.NewService(env.Ctx, env.GCPOptions()...)
	if err != nil {
		t.Fatal(err)
	}
	folder, key := mockenv.LoggingFolder(t), mockenv.LoggingKMSKey(t)

	cloud := func(t *testing.T, parent string) *api.Settings {
		t.Helper()
		var s *api.Settings
		var err error
		if strings.HasPrefix(parent, "folders/") {
			s, err = svc.Folders.GetSettings(parent).Context(env.Ctx).Do()
		} else {
			s, err = svc.Organizations.GetSettings(parent).Context(env.Ctx).Do()
		}
		if err != nil {
			t.Fatalf("reading %s/settings from the fake cloud: %v", parent, err)
		}
		return s
	}
	patches := func(parent string) []string {
		var out []string
		for _, r := range env.Requests() {
			if strings.HasPrefix(r, "PATCH ") && strings.Contains(r, "/v2/"+parent+"/settings") {
				out = append(out, r)
			}
		}
		return out
	}
	getObj := func(t *testing.T, name string) *loggingv1beta1.LoggingSettings {
		t.Helper()
		obj := &loggingv1beta1.LoggingSettings{}
		if err := env.Kube.Get(env.Ctx, types.NamespacedName{Namespace: namespace, Name: name}, obj); err != nil {
			t.Fatal(err)
		}
		return obj
	}
	waitFor := func(t *testing.T, cond func() bool) {
		t.Helper()
		deadline := time.Now().Add(30 * time.Second)
		for !cond() {
			if time.Now().After(deadline) {
				t.Fatal("timed out waiting for the fake cloud")
			}
			time.Sleep(200 * time.Millisecond)
		}
	}

	// Put the folder's settings back as they were when the test ends.
	before := cloud(t, folder)
	t.Cleanup(func() {
		restore := &api.Settings{
			KmsKeyName:         before.KmsKeyName,
			StorageLocation:    before.StorageLocation,
			DisableDefaultSink: before.DisableDefaultSink,
			ForceSendFields:    []string{"KmsKeyName", "StorageLocation", "DisableDefaultSink"},
		}
		if _, err := svc.Folders.UpdateSettings(folder, restore).UpdateMask("disableDefaultSink,kmsKeyName,storageLocation").Context(env.Ctx).Do(); err != nil {
			t.Errorf("putting %s/settings back: %v", folder, err)
		}
	})

	fs := &loggingv1beta1.LoggingSettings{
		ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: "landing-zone-folder", Labels: mockenv.Labels()},
		Spec: loggingv1beta1.LoggingSettingsSpec{
			FolderRef:          &refsv1beta1.FolderRefDeprecated{External: folder},
			KMSKeyRef:          &kmsv1beta1.KMSCryptoKeyRef{External: key},
			StorageLocation:    mockenv.Ptr("europe-west1"),
			DisableDefaultSink: mockenv.Ptr(true),
		},
	}
	if err := env.Kube.Create(env.Ctx, fs); err != nil {
		t.Fatal(err)
	}

	// The kind moved to v1beta1 (docs/design.md decision 27): v1alpha1 is
	// not served, so a manifest that still names it is refused.
	t.Run("v1alpha1 is not served", func(t *testing.T) {
		alpha := &unstructured.Unstructured{}
		alpha.SetGroupVersionKind(gvk.GroupKind().WithVersion("v1alpha1"))
		alpha.SetNamespace(namespace)
		alpha.SetName("alpha")
		err := env.Kube.Create(env.Ctx, alpha)
		if !meta.IsNoMatchError(err) {
			t.Fatalf("creating a logging.gcp.hfl.dev/v1alpha1 LoggingSettings: err = %v, want no match for the kind", err)
		}
		if gvk.Version != "v1beta1" {
			t.Errorf("LoggingSettings's version = %q, want v1beta1", gvk.Version)
		}
	})

	t.Run("create sets the folder's settings", func(t *testing.T) {
		u := env.WaitReady(gvk, namespace, "landing-zone-folder")
		if ref, _, _ := unstructured.NestedString(u.Object, "status", "externalRef"); ref != folder+"/settings" {
			t.Errorf("status.externalRef = %q", ref)
		}
		got := cloud(t, folder)
		if got.KmsKeyName != key || got.StorageLocation != "europe-west1" || !got.DisableDefaultSink {
			t.Errorf("cloud settings = kmsKeyName %q, storageLocation %q, disableDefaultSink %v", got.KmsKeyName, got.StorageLocation, got.DisableDefaultSink)
		}
		if sa, _, _ := unstructured.NestedString(u.Object, "status", "observedState", "kmsServiceAccountID"); sa == "" || sa != got.KmsServiceAccountId {
			t.Errorf("status.observedState.kmsServiceAccountID = %q, cloud %q", sa, got.KmsServiceAccountId)
		}
	})

	t.Run("a resync patches nothing", func(t *testing.T) {
		before := len(patches(folder))
		o := getObj(t, "landing-zone-folder")
		o.Annotations = map[string]string{k8s.InternalForceReconcileAnnotation: "1"}
		if err := env.Kube.Update(env.Ctx, o); err != nil {
			t.Fatal(err)
		}
		time.Sleep(3 * time.Second)
		env.WaitReady(gvk, namespace, "landing-zone-folder")
		if n := len(patches(folder)) - before; n != 0 {
			t.Errorf("a resync patched %d times", n)
		}
	})

	t.Run("a changed location is patched alone", func(t *testing.T) {
		before := len(patches(folder))
		o := getObj(t, "landing-zone-folder")
		o.Spec.StorageLocation = mockenv.Ptr("europe-west4")
		if err := env.Kube.Update(env.Ctx, o); err != nil {
			t.Fatal(err)
		}
		waitFor(t, func() bool { return cloud(t, folder).StorageLocation == "europe-west4" })
		env.WaitReady(gvk, namespace, "landing-zone-folder")
		p := patches(folder)[before:]
		if len(p) != 1 || !strings.Contains(p[0], "updateMask=storageLocation") || strings.Contains(p[0], "kmsKeyName") {
			t.Errorf("patches = %v, want one with updateMask=storageLocation", p)
		}
		if got := cloud(t, folder); got.KmsKeyName != key || !got.DisableDefaultSink {
			t.Errorf("the other fields changed: %+v", got)
		}
	})

	t.Run("a field taken off the spec is left as it is", func(t *testing.T) {
		before := len(patches(folder))
		o := getObj(t, "landing-zone-folder")
		o.Spec.DisableDefaultSink = nil
		if err := env.Kube.Update(env.Ctx, o); err != nil {
			t.Fatal(err)
		}
		time.Sleep(3 * time.Second)
		env.WaitReady(gvk, namespace, "landing-zone-folder")
		if n := len(patches(folder)) - before; n != 0 {
			t.Errorf("patched %d times, want none", n)
		}
		if !cloud(t, folder).DisableDefaultSink {
			t.Errorf("disableDefaultSink was reset")
		}
	})

	t.Run("the organization's settings", func(t *testing.T) {
		if mockenv.Live() {
			t.Skip("a live run never changes an organization's settings (docs/live-runs.md)")
		}
		o := &loggingv1beta1.LoggingSettings{
			ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: "organization"},
			Spec: loggingv1beta1.LoggingSettingsSpec{
				OrganizationRef: &refsv1beta1.OrganizationRef{External: org},
				StorageLocation: mockenv.Ptr("europe-west1"),
			},
		}
		if err := env.Kube.Create(env.Ctx, o); err != nil {
			t.Fatal(err)
		}
		u := env.WaitReady(gvk, namespace, "organization")
		if ref, _, _ := unstructured.NestedString(u.Object, "status", "externalRef"); ref != org+"/settings" {
			t.Errorf("status.externalRef = %q", ref)
		}
		if got := cloud(t, org); got.StorageLocation != "europe-west1" || got.KmsKeyName != "" {
			t.Errorf("cloud settings = %+v", got)
		}
	})

	t.Run("exactly one parent", func(t *testing.T) {
		o := &loggingv1beta1.LoggingSettings{
			ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: "both"},
			Spec: loggingv1beta1.LoggingSettingsSpec{
				OrganizationRef: &refsv1beta1.OrganizationRef{External: org},
				FolderRef:       &refsv1beta1.FolderRefDeprecated{External: folder},
			},
		}
		err := env.Kube.Create(env.Ctx, o)
		if err == nil || !strings.Contains(err.Error(), "exactly one of organizationRef and folderRef") {
			t.Errorf("creating with both parents: got %v, want the API server to refuse it", err)
		}
	})

	// The case: everything deleted at once (a namespace, a release, the
	// catalog's folder chart), and the Folder object the folderRef names
	// goes first. The delete makes no call, so it resolves nothing and the
	// object goes. Nothing here runs Folder's controller, so the Folder
	// object is gone as soon as it is deleted. The settings objects set no
	// field, so they only read the folder's settings, live too.
	folderObject := func(t *testing.T, name string) *unstructured.Unstructured {
		t.Helper()
		u := &unstructured.Unstructured{Object: map[string]any{"spec": map[string]any{
			"resourceID":      strings.TrimPrefix(folder, "folders/"),
			"displayName":     "landing",
			"organizationRef": map[string]any{"external": org},
		}}}
		u.SetGroupVersionKind(schema.GroupVersionKind{Group: "resourcemanager.cnrm.cloud.google.com", Version: "v1beta1", Kind: "Folder"})
		u.SetNamespace(namespace)
		u.SetName(name)
		if err := env.Kube.Create(env.Ctx, u); err != nil {
			t.Fatal(err)
		}
		return u
	}
	onFolderObject := func(t *testing.T, name, folderObj string, annotations map[string]string) {
		t.Helper()
		o := &loggingv1beta1.LoggingSettings{
			ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: name, Labels: mockenv.Labels(), Annotations: annotations},
			Spec:       loggingv1beta1.LoggingSettingsSpec{FolderRef: &refsv1beta1.FolderRefDeprecated{Name: folderObj}},
		}
		if err := env.Kube.Create(env.Ctx, o); err != nil {
			t.Fatal(err)
		}
	}
	deleteCallsNothing := func(t *testing.T, name string) {
		t.Helper()
		before := len(env.Requests())
		if err := env.Kube.Delete(env.Ctx, getObj(t, name)); err != nil {
			t.Fatal(err)
		}
		env.WaitGone(&loggingv1beta1.LoggingSettings{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: name}})
		for _, r := range env.Requests()[before:] {
			if strings.Contains(r, "/settings") {
				t.Errorf("delete called %s", r)
			}
		}
	}
	madeThenFolderGone := func(t *testing.T, name string, annotations map[string]string) {
		t.Helper()
		f := folderObject(t, name+"-folder")
		onFolderObject(t, name, name+"-folder", annotations)
		u := env.WaitReady(gvk, namespace, name)
		if ref, _, _ := unstructured.NestedString(u.Object, "status", "externalRef"); ref != folder+"/settings" {
			t.Errorf("status.externalRef = %q", ref)
		}
		if err := env.Kube.Delete(env.Ctx, f); err != nil {
			t.Fatal(err)
		}
		env.WaitGone(f)
		deleteCallsNothing(t, name)
	}

	t.Run("made, then its folder object deleted first, it still deletes", func(t *testing.T) {
		madeThenFolderGone(t, "folder-gone", nil)
	})

	t.Run("never made, its folder object never existed, it deletes", func(t *testing.T) {
		name := "folder-never"
		onFolderObject(t, name, "no-such-folder", nil)
		if msg := env.WaitNotReady(gvk, namespace, name); !strings.Contains(msg, "not found") {
			t.Errorf("not-Ready message = %q, want the missing Folder", msg)
		}
		deleteCallsNothing(t, name)
	})

	t.Run("abandoned, its folder object deleted first, it still goes", func(t *testing.T) {
		madeThenFolderGone(t, "folder-abandon", map[string]string{k8s.DeletionPolicyAnnotation: k8s.DeletionPolicyAbandon})
	})

	t.Run("delete leaves the settings", func(t *testing.T) {
		before := len(patches(folder))
		if err := env.Kube.Delete(env.Ctx, getObj(t, "landing-zone-folder")); err != nil {
			t.Fatal(err)
		}
		env.WaitGone(&loggingv1beta1.LoggingSettings{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: "landing-zone-folder"}})
		if got := cloud(t, folder); got.StorageLocation != "europe-west4" || got.KmsKeyName != key {
			t.Errorf("settings after delete = %+v, want them unchanged", got)
		}
		if n := len(patches(folder)) - before; n != 0 {
			t.Errorf("delete patched %d times", n)
		}
	})
}
