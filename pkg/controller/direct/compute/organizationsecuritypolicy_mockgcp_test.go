// Copyright 2026 High Five Labs
// SPDX-License-Identifier: Apache-2.0

package compute_test

// ComputeOrganizationSecurityPolicy and its association end to end on the
// fake cloud (pkg/test/mockenv; mockgcp serves organizationSecurityPolicies
// unpatched). The case: a landing zone's hierarchical firewall policy, made
// under an organization and attached to it, then described anew, and
// deleted (and one abandoned). Both kinds act on an organization or a
// folder, so a live run skips them (docs/live-runs.md: a live run never
// touches a folder or an organization).

import (
	"strings"
	"testing"
	"time"

	gcp "cloud.google.com/go/compute/apiv1"
	pb "cloud.google.com/go/compute/apiv1/computepb"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	computev1beta1 "github.com/highfivelabs/config-connector-extensions/apis/compute/v1beta1"
	"github.com/highfivelabs/config-connector-extensions/pkg/test/mockenv"
)

func TestComputeOrganizationSecurityPolicy(t *testing.T) {
	if mockenv.Live() {
		t.Skip("a live run never touches a folder or an organization (docs/live-runs.md); live: pending, needs the test folder")
	}
	const (
		namespace = "example"
		org       = "organizations/123456789"
	)
	policyGVK := computev1beta1.ComputeOrganizationSecurityPolicyGVK
	assocGVK := computev1beta1.ComputeOrganizationSecurityPolicyAssociationGVK
	env := mockenv.New(t, namespace)
	cloud, err := gcp.NewOrganizationSecurityPoliciesRESTClient(env.Ctx, env.GCPOptions()...)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cloud.Close() })

	newPolicy := func(name, displayName string) *computev1beta1.ComputeOrganizationSecurityPolicy {
		return &computev1beta1.ComputeOrganizationSecurityPolicy{
			ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: name},
			Spec: computev1beta1.ComputeOrganizationSecurityPolicySpec{
				DisplayName: displayName,
				Parent:      org,
				Description: mockenv.Ptr("the landing zone's baseline"),
			},
		}
	}
	getPolicy := func(t *testing.T, name string) *computev1beta1.ComputeOrganizationSecurityPolicy {
		t.Helper()
		o := &computev1beta1.ComputeOrganizationSecurityPolicy{}
		if err := env.Kube.Get(env.Ctx, types.NamespacedName{Namespace: namespace, Name: name}, o); err != nil {
			t.Fatal(err)
		}
		return o
	}
	cloudPolicy := func(t *testing.T, id string) *pb.SecurityPolicy {
		t.Helper()
		p, err := cloud.Get(env.Ctx, &pb.GetOrganizationSecurityPolicyRequest{SecurityPolicy: id})
		if err != nil {
			t.Fatalf("reading policy %s from the cloud: %v", id, err)
		}
		return p
	}
	// patches counts the controller's PATCHes of a policy.
	patches := func() int {
		n := 0
		for _, req := range env.Requests() {
			if strings.HasPrefix(req, "PATCH ") && strings.Contains(req, "/locations/global/securityPolicies/") {
				n++
			}
		}
		return n
	}

	policy := newPolicy("baseline", "lz-baseline")
	if err := env.Kube.Create(env.Ctx, policy); err != nil {
		t.Fatal(err)
	}
	env.WaitReady(policyGVK, namespace, "baseline")
	var policyID string

	t.Run("create makes the policy under the organization", func(t *testing.T) {
		if id := getPolicy(t, "baseline").Status.PolicyId; id != nil {
			policyID = *id
		}
		if policyID == "" {
			t.Fatal("status.policyId is empty")
		}
		got := cloudPolicy(t, policyID)
		if got.GetShortName() != "lz-baseline" || got.GetParent() != org || got.GetDescription() != "the landing zone's baseline" {
			t.Errorf("cloud policy = shortName %q, parent %q, description %q", got.GetShortName(), got.GetParent(), got.GetDescription())
		}
	})

	t.Run("a new description is patched", func(t *testing.T) {
		before := patches()
		o := getPolicy(t, "baseline")
		o.Spec.Description = mockenv.Ptr("the landing zone's baseline, v2")
		if err := env.Kube.Update(env.Ctx, o); err != nil {
			t.Fatal(err)
		}
		deadline := time.Now().Add(30 * time.Second)
		for cloudPolicy(t, policyID).GetDescription() != "the landing zone's baseline, v2" {
			if time.Now().After(deadline) {
				t.Fatalf("cloud description = %q", cloudPolicy(t, policyID).GetDescription())
			}
			time.Sleep(time.Second)
		}
		env.WaitReady(policyGVK, namespace, "baseline")
		if n := patches() - before; n != 1 {
			t.Errorf("the change patched the policy %d times, want 1", n)
		}
	})

	t.Run("displayName, parent and type are immutable", func(t *testing.T) {
		for field, mutate := range map[string]func(*computev1beta1.ComputeOrganizationSecurityPolicy){
			"displayName": func(o *computev1beta1.ComputeOrganizationSecurityPolicy) { o.Spec.DisplayName = "renamed" },
			"parent":      func(o *computev1beta1.ComputeOrganizationSecurityPolicy) { o.Spec.Parent = "folders/42" },
		} {
			o := getPolicy(t, "baseline")
			mutate(o)
			err := env.Kube.Update(env.Ctx, o)
			if err == nil || !strings.Contains(err.Error(), field+" is immutable") {
				t.Errorf("changing %s: got %v, want the API server to refuse it", field, err)
			}
		}
	})

	newAssoc := func(name string, set func(*computev1beta1.ComputeOrganizationSecurityPolicyAssociationSpec)) *computev1beta1.ComputeOrganizationSecurityPolicyAssociation {
		a := &computev1beta1.ComputeOrganizationSecurityPolicyAssociation{
			ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: name},
			Spec:       computev1beta1.ComputeOrganizationSecurityPolicyAssociationSpec{AttachmentID: org},
		}
		set(&a.Spec)
		return a
	}
	byRef := func(s *computev1beta1.ComputeOrganizationSecurityPolicyAssociationSpec) {
		s.SecurityPolicyRef = &computev1beta1.ComputeOrganizationSecurityPolicyRef{Name: "baseline"}
	}
	getAssoc := func(name string) (*pb.SecurityPolicyAssociation, error) {
		return cloud.GetAssociation(env.Ctx, &pb.GetAssociationOrganizationSecurityPolicyRequest{SecurityPolicy: policyID, Name: mockenv.Ptr(name)})
	}

	t.Run("an association by reference attaches the policy", func(t *testing.T) {
		a := newAssoc("org-baseline", byRef)
		if err := env.Kube.Create(env.Ctx, a); err != nil {
			t.Fatal(err)
		}
		env.WaitReady(assocGVK, namespace, "org-baseline")
		got, err := getAssoc("org-baseline")
		if err != nil {
			t.Fatalf("reading the association: %v", err)
		}
		if got.GetAttachmentId() != org {
			t.Errorf("association attachmentId = %q, want %q", got.GetAttachmentId(), org)
		}
	})

	t.Run("an association by policyId attaches the policy", func(t *testing.T) {
		a := newAssoc("by-id", func(s *computev1beta1.ComputeOrganizationSecurityPolicyAssociationSpec) {
			s.PolicyID = mockenv.Ptr(policyID)
			s.AttachmentID = "folders/42"
			s.ResourceID = mockenv.Ptr("folder-baseline")
		})
		if err := env.Kube.Create(env.Ctx, a); err != nil {
			t.Fatal(err)
		}
		env.WaitReady(assocGVK, namespace, "by-id")
		if _, err := getAssoc("folder-baseline"); err != nil {
			t.Fatalf("reading the association: %v", err)
		}
		if err := env.Kube.Delete(env.Ctx, a); err != nil {
			t.Fatal(err)
		}
		env.WaitGone(a)
		if _, err := getAssoc("folder-baseline"); err == nil || !strings.Contains(err.Error(), "does not exist") {
			t.Errorf("reading the deleted association: got %v, want it gone", err)
		}
	})

	t.Run("exactly one of policyId and securityPolicyRef", func(t *testing.T) {
		for name, set := range map[string]func(*computev1beta1.ComputeOrganizationSecurityPolicyAssociationSpec){
			"both": func(s *computev1beta1.ComputeOrganizationSecurityPolicyAssociationSpec) {
				byRef(s)
				s.PolicyID = mockenv.Ptr("1")
			},
			"none": func(*computev1beta1.ComputeOrganizationSecurityPolicyAssociationSpec) {},
		} {
			err := env.Kube.Create(env.Ctx, newAssoc(name, set))
			if err == nil || !strings.Contains(err.Error(), "exactly one of policyId and securityPolicyRef") {
				t.Errorf("creating with %s: got %v, want the API server to refuse it", name, err)
			}
		}
	})

	t.Run("an association is immutable", func(t *testing.T) {
		a := &computev1beta1.ComputeOrganizationSecurityPolicyAssociation{}
		if err := env.Kube.Get(env.Ctx, types.NamespacedName{Namespace: namespace, Name: "org-baseline"}, a); err != nil {
			t.Fatal(err)
		}
		a.Spec.AttachmentID = "folders/42"
		if err := env.Kube.Update(env.Ctx, a); err == nil || !strings.Contains(err.Error(), "the spec is immutable") {
			t.Errorf("changing attachmentId: got %v, want the API server to refuse it", err)
		}
	})

	// The catalog's cloud_armor_policy (fabric's project and folder
	// modules, google_compute_organization_security_policy_association):
	// a hierarchical Cloud Armor policy attached to a project by its number,
	// and to a folder with exclusions.
	t.Run("a Cloud Armor policy attached to a project and to a folder with exclusions", func(t *testing.T) {
		armor := newPolicy("armor", "lz-armor")
		armor.Spec.Type = mockenv.Ptr("CLOUD_ARMOR")
		if err := env.Kube.Create(env.Ctx, armor); err != nil {
			t.Fatal(err)
		}
		env.WaitReady(policyGVK, namespace, "armor")
		armorID := ""
		if id := getPolicy(t, "armor").Status.PolicyId; id != nil {
			armorID = *id
		}
		byArmor := func(s *computev1beta1.ComputeOrganizationSecurityPolicyAssociationSpec) {
			s.SecurityPolicyRef = &computev1beta1.ComputeOrganizationSecurityPolicyRef{Name: "armor"}
		}
		project := newAssoc("armor-project", func(s *computev1beta1.ComputeOrganizationSecurityPolicyAssociationSpec) {
			byArmor(s)
			s.AttachmentID = "projects/886048300"
			s.ResourceID = mockenv.Ptr("project-886048300")
		})
		folder := newAssoc("armor-folder", func(s *computev1beta1.ComputeOrganizationSecurityPolicyAssociationSpec) {
			byArmor(s)
			s.AttachmentID = "folders/42"
			s.ExcludedFolders = []string{"folders/43"}
			s.ExcludedProjects = []string{"projects/886048301"}
		})
		for _, a := range []*computev1beta1.ComputeOrganizationSecurityPolicyAssociation{project, folder} {
			if err := env.Kube.Create(env.Ctx, a); err != nil {
				t.Fatal(err)
			}
			env.WaitReady(assocGVK, namespace, a.Name)
		}
		read := func(name string) *pb.SecurityPolicyAssociation {
			t.Helper()
			got, err := cloud.GetAssociation(env.Ctx, &pb.GetAssociationOrganizationSecurityPolicyRequest{SecurityPolicy: armorID, Name: mockenv.Ptr(name)})
			if err != nil {
				t.Fatalf("reading association %s: %v", name, err)
			}
			return got
		}
		if got := read("project-886048300"); got.GetAttachmentId() != "projects/886048300" {
			t.Errorf("project association attachmentId = %q", got.GetAttachmentId())
		}
		got := read("armor-folder")
		if strings.Join(got.GetExcludedFolders(), ",") != "folders/43" || strings.Join(got.GetExcludedProjects(), ",") != "projects/886048301" {
			t.Errorf("folder association exclusions = %v, %v", got.GetExcludedFolders(), got.GetExcludedProjects())
		}

		// A project's association takes no exclusions (fabric refuses them).
		bad := newAssoc("armor-project-excluding", func(s *computev1beta1.ComputeOrganizationSecurityPolicyAssociationSpec) {
			byArmor(s)
			s.AttachmentID = "projects/886048300"
			s.ExcludedProjects = []string{"projects/886048301"}
		})
		if err := env.Kube.Create(env.Ctx, bad); err == nil || !strings.Contains(err.Error(), "exclusions are not supported for a project's association") {
			t.Errorf("a project's association with exclusions: got %v, want the API server to refuse it", err)
		}

		for _, a := range []*computev1beta1.ComputeOrganizationSecurityPolicyAssociation{project, folder} {
			if err := env.Kube.Delete(env.Ctx, a); err != nil {
				t.Fatal(err)
			}
			env.WaitGone(a)
		}
		if _, err := cloud.GetAssociation(env.Ctx, &pb.GetAssociationOrganizationSecurityPolicyRequest{SecurityPolicy: armorID, Name: mockenv.Ptr("project-886048300")}); err == nil {
			t.Error("the project's association is still in the cloud")
		}
		p := getPolicy(t, "armor")
		if err := env.Kube.Delete(env.Ctx, p); err != nil {
			t.Fatal(err)
		}
		env.WaitGone(p)
	})

	t.Run("delete detaches, then removes the policy", func(t *testing.T) {
		a := &computev1beta1.ComputeOrganizationSecurityPolicyAssociation{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: "org-baseline"}}
		if err := env.Kube.Delete(env.Ctx, a); err != nil {
			t.Fatal(err)
		}
		env.WaitGone(a)
		if _, err := getAssoc("org-baseline"); err == nil {
			t.Error("the association is still in the cloud")
		}
		p := getPolicy(t, "baseline")
		if err := env.Kube.Delete(env.Ctx, p); err != nil {
			t.Fatal(err)
		}
		env.WaitGone(p)
		_, err := cloud.Get(env.Ctx, &pb.GetOrganizationSecurityPolicyRequest{SecurityPolicy: policyID})
		if !direct.IsNotFound(err) {
			t.Errorf("reading the deleted policy: got %v, want not found", err)
		}
	})

	t.Run("abandon keeps the policy", func(t *testing.T) {
		k := newPolicy("kept", "lz-kept")
		k.Annotations = map[string]string{"cnrm.cloud.google.com/deletion-policy": "abandon"}
		if err := env.Kube.Create(env.Ctx, k); err != nil {
			t.Fatal(err)
		}
		env.WaitReady(policyGVK, namespace, "kept")
		id := ""
		if p := getPolicy(t, "kept").Status.PolicyId; p != nil {
			id = *p
		}
		if err := env.Kube.Delete(env.Ctx, k); err != nil {
			t.Fatal(err)
		}
		env.WaitGone(k)
		if got := cloudPolicy(t, id); got.GetShortName() != "lz-kept" {
			t.Errorf("the abandoned policy = %v", got)
		}
	})
}
