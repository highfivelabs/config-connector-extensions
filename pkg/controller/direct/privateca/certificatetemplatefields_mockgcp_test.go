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

package privateca_test

// PrivateCACertificateTemplateFields end to end on the fake cloud
// (pkg/test/mockenv) or a real project (docs/live-runs.md). The template
// is the run's own (mockenv.Name), in us-central1; templates cost nothing,
// and the test deletes it. It skips live unless E2E_PRIVATECA=true
// (skipLiveWithoutPrivateCA).

import (
	"slices"
	"strings"
	"testing"
	"time"

	pb "cloud.google.com/go/security/privateca/apiv1/privatecapb"
	privatecav1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/apis/privateca/v1beta1"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"

	krm "github.com/highfivelabs/config-connector-extensions/apis/privateca/v1alpha1"
	"github.com/highfivelabs/config-connector-extensions/pkg/test/mockenv"
)

var configConnectorTemplateGVK = schema.GroupVersionKind{Group: "privateca.cnrm.cloud.google.com", Version: "v1beta1", Kind: "PrivateCACertificateTemplate"}

func TestPrivateCACertificateTemplateFields(t *testing.T) {
	const (
		namespace = "example"
		name      = "fields"
	)
	skipLiveWithoutPrivateCA(t)
	gvk := krm.PrivateCACertificateTemplateFieldsGVK
	env := mockenv.New(t, namespace)
	c := newCAClient(t, env)

	templateID := mockenv.Name("ctf")
	parent := "projects/" + mockenv.Project + "/locations/" + location
	templateName := parent + "/certificateTemplates/" + templateID
	op, err := c.CreateCertificateTemplate(env.Ctx, &pb.CreateCertificateTemplateRequest{
		Parent: parent, CertificateTemplateId: templateID,
		CertificateTemplate: &pb.CertificateTemplate{
			Description: "made by the test",
			Labels:      mockenv.Labels(),
			PredefinedValues: &pb.X509Parameters{
				KeyUsage: &pb.KeyUsage{BaseKeyUsage: &pb.KeyUsage_KeyUsageOptions{DigitalSignature: true}},
			},
		},
	})
	if err == nil {
		_, err = op.Wait(env.Ctx)
	}
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		op, err := c.DeleteCertificateTemplate(env.Ctx, &pb.DeleteCertificateTemplateRequest{Name: templateName})
		if err == nil {
			err = op.Wait(env.Ctx)
		}
		if err != nil {
			t.Errorf("deleting certificate template %s: %v", templateName, err)
		}
	})

	// Config Connector's PrivateCACertificateTemplate for the template (not
	// reconciled here). Its reference resolves a name from the spec only
	// once the object is Ready, so the test marks it Ready as Config
	// Connector would.
	cc := &unstructured.Unstructured{Object: map[string]any{"spec": map[string]any{
		"projectRef": map[string]any{"external": "projects/" + mockenv.Project},
		"location":   location, "resourceID": templateID,
	}}}
	cc.SetGroupVersionKind(configConnectorTemplateGVK)
	cc.SetNamespace(namespace)
	cc.SetName("template")
	if err := env.Kube.Create(env.Ctx, cc); err != nil {
		t.Fatal(err)
	}
	cc.Object["status"] = map[string]any{"conditions": []any{map[string]any{
		"type": "Ready", "status": "True", "reason": "UpToDate", "lastTransitionTime": time.Now().UTC().Format(time.RFC3339),
	}}}
	if err := env.Kube.Status().Update(env.Ctx, cc); err != nil {
		t.Fatal(err)
	}

	getTemplate := func(t *testing.T) *pb.CertificateTemplate {
		t.Helper()
		ct, err := c.GetCertificateTemplate(env.Ctx, &pb.GetCertificateTemplateRequest{Name: templateName})
		if err != nil {
			t.Fatalf("reading the template: %v", err)
		}
		return ct
	}
	patches := func() []string { return patchesOf(env, "/certificateTemplates/"+templateID) }
	getObj := func(t *testing.T, name string) *krm.PrivateCACertificateTemplateFields {
		t.Helper()
		o := &krm.PrivateCACertificateTemplateFields{}
		if err := env.Kube.Get(env.Ctx, types.NamespacedName{Namespace: namespace, Name: name}, o); err != nil {
			t.Fatal(err)
		}
		return o
	}
	newFields := func(name string, ref *privatecav1beta1.PrivateCACertificateTemplateRef) *krm.PrivateCACertificateTemplateFields {
		return &krm.PrivateCACertificateTemplateFields{
			ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: name, Labels: mockenv.Labels()},
			Spec:       krm.PrivateCACertificateTemplateFieldsSpec{ResourceRef: ref},
		}
	}
	checkOwned := func(t *testing.T, ct *pb.CertificateTemplate, lifetime time.Duration, uris []string) {
		t.Helper()
		if got := ct.GetMaximumLifetime().AsDuration(); got != lifetime {
			t.Errorf("maximumLifetime = %v, want %v", got, lifetime)
		}
		if got := ct.GetPredefinedValues().GetNameConstraints().GetPermittedUris(); !slices.Equal(got, uris) {
			t.Errorf("nameConstraints.permittedUris = %v, want %v", got, uris)
		}
	}
	keepsKeyUsage := func(t *testing.T, ct *pb.CertificateTemplate) {
		t.Helper()
		if !ct.GetPredefinedValues().GetKeyUsage().GetBaseKeyUsage().GetDigitalSignature() {
			t.Errorf("the template's own key usage is gone: %v", ct.GetPredefinedValues())
		}
	}

	obj := newFields(name, &privatecav1beta1.PrivateCACertificateTemplateRef{Name: "template"})
	obj.Spec.MaximumLifetime = mockenv.Ptr("2592000s")
	obj.Spec.PredefinedValues = &krm.CertificateTemplateFields_PredefinedValues{NameConstraints: &krm.X509Parameters_NameConstraints{
		PermittedUris:     []string{".example.com"},
		PermittedIPRanges: []string{"10.0.0.0/8"},
	}}
	if err := env.Kube.Create(env.Ctx, obj); err != nil {
		t.Fatal(err)
	}

	t.Run("create sets the fields and keeps the template's own", func(t *testing.T) {
		u := env.WaitReady(gvk, namespace, name)
		if ref, _, _ := unstructured.NestedString(u.Object, "status", "externalRef"); ref != templateName {
			t.Errorf("status.externalRef = %q, want %q", ref, templateName)
		}
		ct := getTemplate(t)
		checkOwned(t, ct, 30*24*time.Hour, []string{".example.com"})
		if got := ct.GetPredefinedValues().GetNameConstraints().GetPermittedIpRanges(); !slices.Equal(got, []string{"10.0.0.0/8"}) {
			t.Errorf("nameConstraints.permittedIpRanges = %v", got)
		}
		keepsKeyUsage(t, ct)
		if ct.GetDescription() != "made by the test" {
			t.Errorf("description = %q", ct.GetDescription())
		}
		if p := patches(); len(p) != 1 || !strings.Contains(p[0], "updateMask=maximumLifetime%2CpredefinedValues") {
			t.Errorf("patches = %v, want one with updateMask=maximumLifetime,predefinedValues", p)
		}
	})

	t.Run("a resync writes nothing", func(t *testing.T) {
		before := len(patches())
		resync(t, env, gvk, getObj(t, name))
		if n := len(patches()) - before; n != 0 {
			t.Errorf("a resync patched the template %d times", n)
		}
	})

	t.Run("an update of Config Connector's fields keeps the lifetime; its predefined values are undone for ours at the next reconcile", func(t *testing.T) {
		// What Config Connector's update of a PrivateCACertificateTemplate
		// sends when its description and predefined values change: the
		// fields it knows, with a top-level mask.
		op, err := c.UpdateCertificateTemplate(env.Ctx, &pb.UpdateCertificateTemplateRequest{
			CertificateTemplate: &pb.CertificateTemplate{
				Name: templateName, Description: "changed by config connector",
				PredefinedValues: &pb.X509Parameters{KeyUsage: &pb.KeyUsage{BaseKeyUsage: &pb.KeyUsage_KeyUsageOptions{DigitalSignature: true}}},
			},
			UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"description", "predefined_values"}},
		})
		if err == nil {
			_, err = op.Wait(env.Ctx)
		}
		if err != nil {
			t.Fatal(err)
		}
		checkOwned(t, getTemplate(t), 30*24*time.Hour, nil)
		before := len(patches())
		resync(t, env, gvk, getObj(t, name))
		ct := getTemplate(t)
		checkOwned(t, ct, 30*24*time.Hour, []string{".example.com"})
		keepsKeyUsage(t, ct)
		if ct.GetDescription() != "changed by config connector" {
			t.Errorf("description = %q, want Config Connector's change kept", ct.GetDescription())
		}
		if p := patches()[before:]; len(p) != 1 || !strings.Contains(p[0], "updateMask=predefinedValues") || strings.Contains(p[0], "maximumLifetime") {
			t.Errorf("patches = %v, want one with updateMask=predefinedValues alone", p)
		}
	})

	t.Run("a second object for the template is refused", func(t *testing.T) {
		dup := newFields("dup", &privatecav1beta1.PrivateCACertificateTemplateRef{External: templateName})
		dup.Spec.MaximumLifetime = mockenv.Ptr("60s")
		if err := env.Kube.Create(env.Ctx, dup); err != nil {
			t.Fatal(err)
		}
		if msg := env.WaitNotReady(gvk, namespace, "dup"); !strings.Contains(msg, "already sets the fields of") {
			t.Errorf("Ready message = %q, want the second object refused", msg)
		}
		dup.Annotations = map[string]string{"cnrm.cloud.google.com/deletion-policy": "abandon"}
		if err := env.Kube.Delete(env.Ctx, dup); err != nil {
			t.Fatal(err)
		}
		env.WaitGone(dup)
	})

	t.Run("delete clears the fields and keeps the template's own", func(t *testing.T) {
		if err := env.Kube.Delete(env.Ctx, obj); err != nil {
			t.Fatal(err)
		}
		env.WaitGone(&krm.PrivateCACertificateTemplateFields{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: name}})
		ct := getTemplate(t)
		checkOwned(t, ct, 0, nil)
		if ct.GetMaximumLifetime() != nil || ct.GetPredefinedValues().GetNameConstraints() != nil {
			t.Errorf("after delete: maximumLifetime %v, name constraints %v; want none", ct.GetMaximumLifetime(), ct.GetPredefinedValues().GetNameConstraints())
		}
		keepsKeyUsage(t, ct)
	})

	t.Run("abandon keeps the fields", func(t *testing.T) {
		k := newFields("kept", &privatecav1beta1.PrivateCACertificateTemplateRef{External: templateName})
		k.Spec.MaximumLifetime = mockenv.Ptr("86400s")
		k.Annotations = map[string]string{"cnrm.cloud.google.com/deletion-policy": "abandon"}
		if err := env.Kube.Create(env.Ctx, k); err != nil {
			t.Fatal(err)
		}
		env.WaitReady(gvk, namespace, "kept")
		if err := env.Kube.Delete(env.Ctx, k); err != nil {
			t.Fatal(err)
		}
		env.WaitGone(k)
		if got := getTemplate(t).GetMaximumLifetime().AsDuration(); got != 24*time.Hour {
			t.Errorf("after an abandoning delete maximumLifetime = %v, want 24h", got)
		}
	})
}
