// Copyright 2026 High Five Labs
// SPDX-License-Identifier: Apache-2.0

package iam_test

// IAMPolicyBinding end to end on the fake cloud (pkg/test/mockenv,
// mockgcp patch 0044). The case: an organization's principal access
// boundary policy bound to a project's principals, with a condition that
// narrows it to one principal, and a folder's binding of a policy given by
// its resource name.
//
// Live, a binding needs a principal access boundary policy, which lives
// under an organization, and the right to bind it
// (iam.principalaccessboundarypolicies.bind on the organization's policy;
// iam.policyBindings.* on the project): a run never makes either
// (docs/live-runs.md). The test skips live unless
// E2E_IAM_POLICY_BINDING_POLICY names such a policy; then it binds that
// policy to the run's project with a condition no real principal meets, so
// the boundary changes nothing.

import (
	"os"
	"strings"
	"testing"
	"time"

	gcp "cloud.google.com/go/iam/apiv3"
	pb "cloud.google.com/go/iam/apiv3/iampb"
	refsv1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/apis/refs/v1beta1"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/k8s"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"

	iamv1alpha1 "github.com/highfivelabs/config-connector-extensions/apis/iam/v1alpha1"
	iamv1beta1 "github.com/highfivelabs/config-connector-extensions/apis/iam/v1beta1"
	"github.com/highfivelabs/config-connector-extensions/pkg/test/mockenv"
)

func TestIAMPolicyBinding(t *testing.T) {
	livePolicy := os.Getenv("E2E_IAM_POLICY_BINDING_POLICY")
	if mockenv.Live() && livePolicy == "" {
		t.Skip("a live IAMPolicyBinding needs a principal access boundary policy (it lives under an organization, which a live run never touches) that cce-live-tests may bind; set E2E_IAM_POLICY_BINDING_POLICY=organizations/{org}/locations/global/principalAccessBoundaryPolicies/{id} where it has one (docs/live-runs.md)")
	}
	const org = "123456789"
	gvk := iamv1alpha1.IAMPolicyBindingGVK
	env := mockenv.New(t, namespace)
	cloud, err := gcp.NewPolicyBindingsRESTClient(env.Ctx, env.GCPOptions()...)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cloud.Close() })

	project := mockenv.Project
	bindingID := mockenv.Name("pab-binding")
	cloudName := func(parent, id string) string { return parent + "/locations/global/policyBindings/" + id }
	get := func(name string) (*pb.PolicyBinding, error) {
		return cloud.GetPolicyBinding(env.Ctx, &pb.GetPolicyBindingRequest{Name: name})
	}
	patches := func(id string) int {
		n := 0
		for _, r := range env.Requests() {
			if strings.HasPrefix(r, "PATCH ") && strings.Contains(r, "/policyBindings/"+id) {
				n++
			}
		}
		return n
	}
	getObj := func(t *testing.T, name string) *iamv1alpha1.IAMPolicyBinding {
		t.Helper()
		obj := &iamv1alpha1.IAMPolicyBinding{}
		if err := env.Kube.Get(env.Ctx, types.NamespacedName{Namespace: namespace, Name: name}, obj); err != nil {
			t.Fatal(err)
		}
		return obj
	}
	// update reads, changes and writes the object, again on a conflict
	// with the controller's own writes.
	update := func(t *testing.T, name string, change func(*iamv1alpha1.IAMPolicyBinding)) {
		t.Helper()
		if err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
			o := getObj(t, name)
			change(o)
			return env.Kube.Update(env.Ctx, o)
		}); err != nil {
			t.Fatal(err)
		}
	}
	// tryUpdate is update that returns the API server's refusal.
	tryUpdate := func(t *testing.T, name string, change func(*iamv1alpha1.IAMPolicyBinding)) error {
		t.Helper()
		return retry.RetryOnConflict(retry.DefaultRetry, func() error {
			o := getObj(t, name)
			change(o)
			return env.Kube.Update(env.Ctx, o)
		})
	}

	// The policy: ours by name on the fake cloud, the given one live.
	policyName := "organizations/" + org + "/locations/global/principalAccessBoundaryPolicies/org-boundary"
	policyRef := &iamv1alpha1.PolicyBindingPolicyRef{Name: "org-boundary"}
	if mockenv.Live() {
		policyName = livePolicy
		policyRef = &iamv1alpha1.PolicyBindingPolicyRef{External: livePolicy}
	} else {
		pab := &iamv1beta1.IAMPrincipalAccessBoundaryPolicy{
			ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: "org-boundary"},
			Spec: iamv1beta1.IAMPrincipalAccessBoundaryPolicySpec{
				OrganizationRef: &refsv1beta1.OrganizationRef{External: "organizations/" + org},
				Location:        mockenv.Ptr("global"),
				Details: &iamv1beta1.PrincipalAccessBoundaryPolicyDetails{
					Rules: []iamv1beta1.PrincipalAccessBoundaryPolicyRule{{
						Resources: []string{"//cloudresourcemanager.googleapis.com/organizations/" + org},
						Effect:    mockenv.Ptr("ALLOW"),
					}},
				},
			},
		}
		if err := env.Kube.Create(env.Ctx, pab); err != nil {
			t.Fatal(err)
		}
		env.WaitReady(iamv1beta1.IAMPrincipalAccessBoundaryPolicyGVK, namespace, "org-boundary")
	}

	const subject = "principal.subject == 'nobody@example.com'"
	binding := &iamv1alpha1.IAMPolicyBinding{
		ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: "project-binding"},
		Spec: iamv1alpha1.IAMPolicyBindingSpec{
			ProjectRef:  &refsv1beta1.ProjectRef{External: "projects/" + project},
			ResourceID:  mockenv.Ptr(bindingID),
			DisplayName: mockenv.Ptr("Boundary for one principal"),
			Target:      &iamv1alpha1.PolicyBinding_Target{PrincipalSet: mockenv.Ptr("//cloudresourcemanager.googleapis.com/projects/" + project)},
			PolicyKind:  mockenv.Ptr("PRINCIPAL_ACCESS_BOUNDARY"),
			PolicyRef:   policyRef,
			Condition:   &iamv1alpha1.PolicyBindingExpr{Expression: mockenv.Ptr(subject), Title: mockenv.Ptr("one principal")},
		},
	}
	if err := env.Kube.Create(env.Ctx, binding); err != nil {
		t.Fatal(err)
	}
	projectBinding := cloudName("projects/"+project, bindingID)
	t.Cleanup(func() {
		if mockenv.Live() {
			if op, err := cloud.DeletePolicyBinding(env.Ctx, &pb.DeletePolicyBindingRequest{Name: projectBinding}); err == nil {
				_ = op.Wait(env.Ctx)
			}
		}
	})

	t.Run("create binds the policy to the project's principals", func(t *testing.T) {
		u := env.WaitReady(gvk, namespace, "project-binding")
		got, err := get(projectBinding)
		if err != nil {
			t.Fatalf("reading the binding: %v", err)
		}
		if got.GetPolicy() != policyName {
			t.Errorf("policy = %q, want %q", got.GetPolicy(), policyName)
		}
		if got.GetTarget().GetPrincipalSet() != "//cloudresourcemanager.googleapis.com/projects/"+project {
			t.Errorf("target = %v", got.GetTarget())
		}
		if got.GetPolicyKind() != pb.PolicyBinding_PRINCIPAL_ACCESS_BOUNDARY {
			t.Errorf("policy kind = %v", got.GetPolicyKind())
		}
		if got.GetCondition().GetExpression() != subject || got.GetDisplayName() != "Boundary for one principal" {
			t.Errorf("condition = %v, display name = %q", got.GetCondition(), got.GetDisplayName())
		}
		if ref, _, _ := unstructured.NestedString(u.Object, "status", "externalRef"); ref != projectBinding {
			t.Errorf("status.externalRef = %q, want %q", ref, projectBinding)
		}
		for _, f := range []string{"uid", "etag", "policyUid"} {
			if v, _, _ := unstructured.NestedString(u.Object, "status", "observedState", f); v == "" {
				t.Errorf("status.observedState.%s is not set", f)
			}
		}
	})

	t.Run("a resync writes nothing", func(t *testing.T) {
		before := patches(bindingID)
		update(t, "project-binding", func(o *iamv1alpha1.IAMPolicyBinding) {
			o.Annotations = map[string]string{k8s.InternalForceReconcileAnnotation: time.Now().Format(time.RFC3339Nano)}
		})
		time.Sleep(3 * time.Second)
		env.WaitReady(gvk, namespace, "project-binding")
		if n := patches(bindingID) - before; n != 0 {
			t.Errorf("a resync patched %d times", n)
		}
	})

	t.Run("a changed name, annotations and condition are patched", func(t *testing.T) {
		const other = "principal.subject == 'nobody-else@example.com'"
		update(t, "project-binding", func(o *iamv1alpha1.IAMPolicyBinding) {
			o.Spec.DisplayName = mockenv.Ptr("Boundary for another principal")
			o.Spec.Annotations = map[string]string{"owner": "platform"}
			o.Spec.Condition.Expression = mockenv.Ptr(other)
		})
		deadline := time.Now().Add(mockenv.Timeout())
		for {
			got, err := get(projectBinding)
			if err == nil && got.GetCondition().GetExpression() == other && got.GetDisplayName() == "Boundary for another principal" && got.GetAnnotations()["owner"] == "platform" {
				break
			}
			if time.Now().After(deadline) {
				t.Fatalf("the binding was not patched: %v, %v", got, err)
			}
			time.Sleep(500 * time.Millisecond)
		}
		env.WaitReady(gvk, namespace, "project-binding")
	})

	t.Run("the parent, location, target, policy and kind cannot change", func(t *testing.T) {
		for _, c := range []struct {
			field  string
			change func(*iamv1alpha1.IAMPolicyBinding)
		}{
			{"projectRef", func(o *iamv1alpha1.IAMPolicyBinding) {
				o.Spec.ProjectRef = &refsv1beta1.ProjectRef{External: "projects/other-project"}
			}},
			{"resourceID", func(o *iamv1alpha1.IAMPolicyBinding) { o.Spec.ResourceID = mockenv.Ptr("other") }},
			{"target", func(o *iamv1alpha1.IAMPolicyBinding) {
				o.Spec.Target = &iamv1alpha1.PolicyBinding_Target{PrincipalSet: mockenv.Ptr("//cloudresourcemanager.googleapis.com/projects/other-project")}
			}},
			{"policyRef", func(o *iamv1alpha1.IAMPolicyBinding) {
				o.Spec.PolicyRef = &iamv1alpha1.PolicyBindingPolicyRef{External: "organizations/" + org + "/locations/global/principalAccessBoundaryPolicies/other"}
			}},
			{"policyKind", func(o *iamv1alpha1.IAMPolicyBinding) { o.Spec.PolicyKind = mockenv.Ptr("ACCESS") }},
		} {
			err := tryUpdate(t, "project-binding", c.change)
			if err == nil || !strings.Contains(err.Error(), c.field+" is immutable") {
				t.Errorf("changing %s: err = %v", c.field, err)
			}
		}
		// Setting location where it was left out is a change CEL on an
		// existing value cannot catch; the identity check refuses it.
		if err := tryUpdate(t, "project-binding", func(o *iamv1alpha1.IAMPolicyBinding) { o.Spec.Location = mockenv.Ptr("us-central1") }); err == nil {
			if msg := env.WaitNotReady(gvk, namespace, "project-binding"); !strings.Contains(msg, "cannot change") {
				t.Errorf("setting location: Ready message = %q", msg)
			}
			update(t, "project-binding", func(o *iamv1alpha1.IAMPolicyBinding) { o.Spec.Location = nil })
			env.WaitReady(gvk, namespace, "project-binding")
		}
	})

	t.Run("a binding takes exactly one parent", func(t *testing.T) {
		two := binding.DeepCopy()
		two.ObjectMeta = metav1.ObjectMeta{Namespace: namespace, Name: "two-parents"}
		two.Spec.FolderRef = &refsv1beta1.FolderRefDeprecated{External: "folders/456"}
		if err := env.Kube.Create(env.Ctx, two); err == nil || !strings.Contains(err.Error(), "exactly one of organizationRef, folderRef and projectRef") {
			t.Errorf("creating a binding with two parents: err = %v", err)
		}
		none := binding.DeepCopy()
		none.ObjectMeta = metav1.ObjectMeta{Namespace: namespace, Name: "no-parent"}
		none.Spec.ProjectRef = nil
		if err := env.Kube.Create(env.Ctx, none); err == nil || !strings.Contains(err.Error(), "exactly one of organizationRef, folderRef and projectRef") {
			t.Errorf("creating a binding with no parent: err = %v", err)
		}
	})

	if !mockenv.Live() {
		t.Run("a folder's binding takes a policy by its resource name", func(t *testing.T) {
			folder := &iamv1alpha1.IAMPolicyBinding{
				ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: "folder-binding"},
				Spec: iamv1alpha1.IAMPolicyBindingSpec{
					FolderRef: &refsv1beta1.FolderRefDeprecated{External: "folders/456"},
					Location:  mockenv.Ptr("global"),
					Target:    &iamv1alpha1.PolicyBinding_Target{PrincipalSet: mockenv.Ptr("//cloudresourcemanager.googleapis.com/folders/456")},
					PolicyRef: &iamv1alpha1.PolicyBindingPolicyRef{External: policyName},
				},
			}
			if err := env.Kube.Create(env.Ctx, folder); err != nil {
				t.Fatal(err)
			}
			env.WaitReady(gvk, namespace, "folder-binding")
			got, err := get(cloudName("folders/456", "folder-binding"))
			if err != nil {
				t.Fatalf("reading the folder's binding: %v", err)
			}
			if got.GetPolicy() != policyName || got.GetPolicyKind() != pb.PolicyBinding_PRINCIPAL_ACCESS_BOUNDARY {
				t.Errorf("folder binding = %v", got)
			}
			if err := env.Kube.Delete(env.Ctx, getObj(t, "folder-binding")); err != nil {
				t.Fatal(err)
			}
			env.WaitGone(folder)
			if _, err := get(cloudName("folders/456", "folder-binding")); err == nil {
				t.Error("the folder's binding is still there")
			}
		})
	}

	t.Run("delete removes the binding", func(t *testing.T) {
		if err := env.Kube.Delete(env.Ctx, getObj(t, "project-binding")); err != nil {
			t.Fatal(err)
		}
		env.WaitGone(&iamv1alpha1.IAMPolicyBinding{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: "project-binding"}})
		if _, err := get(projectBinding); err == nil {
			t.Error("the binding is still there")
		}
	})

	t.Run("abandon keeps the binding", func(t *testing.T) {
		keptID := mockenv.Name("pab-kept")
		kept := binding.DeepCopy()
		kept.ObjectMeta = metav1.ObjectMeta{Namespace: namespace, Name: "kept",
			Annotations: map[string]string{"cnrm.cloud.google.com/deletion-policy": "abandon"}}
		kept.Spec.ResourceID = mockenv.Ptr(keptID)
		if err := env.Kube.Create(env.Ctx, kept); err != nil {
			t.Fatal(err)
		}
		keptName := cloudName("projects/"+project, keptID)
		t.Cleanup(func() {
			if op, err := cloud.DeletePolicyBinding(env.Ctx, &pb.DeletePolicyBindingRequest{Name: keptName}); err == nil {
				_ = op.Wait(env.Ctx)
			}
		})
		env.WaitReady(gvk, namespace, "kept")
		if err := env.Kube.Delete(env.Ctx, kept); err != nil {
			t.Fatal(err)
		}
		env.WaitGone(kept)
		if _, err := get(keptName); err != nil {
			t.Errorf("the abandoned binding is gone: %v", err)
		}
	})
}
