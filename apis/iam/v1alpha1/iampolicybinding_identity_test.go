// Copyright 2026 High Five Labs
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import "testing"

func TestIAMPolicyBindingIdentity(t *testing.T) {
	for _, ref := range []string{
		"organizations/123/locations/global/policyBindings/b",
		"folders/456/locations/global/policyBindings/b",
		"projects/p/locations/global/policyBindings/b",
	} {
		id := &IAMPolicyBindingIdentity{}
		if err := id.FromExternal(ref); err != nil {
			t.Errorf("FromExternal(%q): %v", ref, err)
			continue
		}
		if id.String() != ref {
			t.Errorf("String() = %q, want %q", id.String(), ref)
		}
		if long := "//iam.googleapis.com/" + ref; id.FromExternal(long) != nil || id.String() != ref {
			t.Errorf("FromExternal(%q) = %q", long, id.String())
		}
	}
	if got := (&IAMPolicyBindingIdentity{Parent: "projects/p", Location: "global", PolicyBinding: "b"}).ParentString(); got != "projects/p/locations/global" {
		t.Errorf("ParentString() = %q", got)
	}
	for _, bad := range []string{
		"",
		"billingAccounts/1/locations/global/policyBindings/b",
		"projects/p/locations/global/principalAccessBoundaryPolicies/b",
		"projects/p/locations/global/policyBindings",
		"projects//locations/global/policyBindings/b",
	} {
		if err := (&IAMPolicyBindingIdentity{}).FromExternal(bad); err == nil {
			t.Errorf("FromExternal(%q) took it", bad)
		}
	}
}
