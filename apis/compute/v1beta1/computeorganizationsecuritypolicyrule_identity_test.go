// Copyright 2026 Google LLC
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

// SPDX-License-Identifier: Apache-2.0

// Modified by High Five Labs for config-connector-extensions: copied from Config Connector
// apis/compute/v1alpha1/computeorganizationsecuritypolicyrule_identity_test.go (UPSTREAM.md); the package is v1beta1; spec.policyId is a pointer, the error cases set a numeric priority, the objects are in our group; ours: TestComputeOrganizationSecurityPolicyRule_GetIdentityOurs.

package v1beta1

import (
	"context"
	"strings"
	"testing"

	"github.com/GoogleCloudPlatform/k8s-config-connector/apis/common"
	"github.com/GoogleCloudPlatform/k8s-config-connector/apis/common/identity"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/k8s"
	"github.com/google/go-cmp/cmp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestComputeOrganizationSecurityPolicyRuleIdentity_Conformance(t *testing.T) {
	identity.AssertConformance(t, identity.ConformanceCase[ComputeOrganizationSecurityPolicyRuleIdentity]{
		SampleExternal: "locations/global/securityPolicies/1234567890/rules/1000",
		WantHost:       "compute.googleapis.com",
		WantParent:     "locations/global/securityPolicies/1234567890",
		WantIdentity: &ComputeOrganizationSecurityPolicyRuleIdentity{
			SecurityPolicy: "1234567890",
			Priority:       "1000",
		},
		Ref: &ComputeOrganizationSecurityPolicyRuleRef{},
	})
}

func TestComputeOrganizationSecurityPolicyRuleIdentity_FromExternal(t *testing.T) {
	tests := []struct {
		name    string
		ref     string
		wantErr bool
		want    *ComputeOrganizationSecurityPolicyRuleIdentity
	}{
		{
			name: "valid relative reference",
			ref:  "locations/global/securityPolicies/1234567890/rules/1000",
			want: &ComputeOrganizationSecurityPolicyRuleIdentity{
				SecurityPolicy: "1234567890",
				Priority:       "1000",
			},
		},
		{
			name: "valid full URL with compute v1 prefix",
			ref:  "https://compute.googleapis.com/compute/v1/locations/global/securityPolicies/1234567890/rules/1000",
			want: &ComputeOrganizationSecurityPolicyRuleIdentity{
				SecurityPolicy: "1234567890",
				Priority:       "1000",
			},
		},
		{
			name: "valid full URL without compute prefix",
			ref:  "https://compute.googleapis.com/locations/global/securityPolicies/1234567890/rules/1000",
			want: &ComputeOrganizationSecurityPolicyRuleIdentity{
				SecurityPolicy: "1234567890",
				Priority:       "1000",
			},
		},
		{
			name: "valid with // prefix",
			ref:  "//compute.googleapis.com/locations/global/securityPolicies/1234567890/rules/1000",
			want: &ComputeOrganizationSecurityPolicyRuleIdentity{
				SecurityPolicy: "1234567890",
				Priority:       "1000",
			},
		},
		{
			name:    "invalid format",
			ref:     "invalid/format",
			wantErr: true,
		},
		{
			name:    "empty reference",
			ref:     "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			i := &ComputeOrganizationSecurityPolicyRuleIdentity{}
			err := i.FromExternal(tt.ref)
			if (err != nil) != tt.wantErr {
				t.Fatalf("FromExternal() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr {
				if diff := cmp.Diff(tt.want, i); diff != "" {
					t.Errorf("FromExternal() mismatch (-want +got):\n%s", diff)
				}
			}
		})
	}
}

func TestComputeOrganizationSecurityPolicyRuleRef_ValidateExternal(t *testing.T) {
	tests := []struct {
		name    string
		ref     string
		wantErr bool
	}{
		{
			name:    "valid relative reference",
			ref:     "locations/global/securityPolicies/1234567890/rules/1000",
			wantErr: false,
		},
		{
			name:    "valid full URL",
			ref:     "https://compute.googleapis.com/compute/v1/locations/global/securityPolicies/1234567890/rules/1000",
			wantErr: false,
		},
		{
			name:    "invalid format",
			ref:     "locations/global/securityPolicies/1234567890",
			wantErr: true,
		},
		{
			name:    "empty string",
			ref:     "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &ComputeOrganizationSecurityPolicyRuleRef{}
			err := r.ValidateExternal(tt.ref)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateExternal() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestComputeOrganizationSecurityPolicyRule_GetIdentity(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = SchemeBuilder.AddToScheme(scheme)
	client := fake.NewClientBuilder().WithScheme(scheme).Build()

	tests := []struct {
		name         string
		obj          *ComputeOrganizationSecurityPolicyRule
		wantIdentity *ComputeOrganizationSecurityPolicyRuleIdentity
		wantErr      bool
	}{
		{
			name: "spec resourceID set and plain policyID",
			obj: &ComputeOrganizationSecurityPolicyRule{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "rule-k8s-name",
					Namespace: "default",
				},
				Spec: ComputeOrganizationSecurityPolicyRuleSpec{
					Action:     "allow",
					PolicyID:   common.LazyPtr("1234567890"),
					ResourceID: common.LazyPtr("1000"),
				},
			},
			wantIdentity: &ComputeOrganizationSecurityPolicyRuleIdentity{
				SecurityPolicy: "1234567890",
				Priority:       "1000",
			},
		},
		{
			name: "spec resourceID unset, fallback to metadata.name and full url policyID",
			obj: &ComputeOrganizationSecurityPolicyRule{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "2000",
					Namespace: "default",
				},
				Spec: ComputeOrganizationSecurityPolicyRuleSpec{
					Action:   "allow",
					PolicyID: common.LazyPtr("locations/global/securityPolicies/9876543210"),
				},
			},
			wantIdentity: &ComputeOrganizationSecurityPolicyRuleIdentity{
				SecurityPolicy: "9876543210",
				Priority:       "2000",
			},
		},
		{
			name: "spec policyID with compute v1 prefix",
			obj: &ComputeOrganizationSecurityPolicyRule{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "rule-1",
					Namespace: "default",
				},
				Spec: ComputeOrganizationSecurityPolicyRuleSpec{
					Action:     "deny",
					PolicyID:   common.LazyPtr("https://compute.googleapis.com/compute/v1/locations/global/securityPolicies/5555555555"),
					ResourceID: common.LazyPtr("3000"),
				},
			},
			wantIdentity: &ComputeOrganizationSecurityPolicyRuleIdentity{
				SecurityPolicy: "5555555555",
				Priority:       "3000",
			},
		},
		{
			name: "spec policyID with securityPolicies/ prefix",
			obj: &ComputeOrganizationSecurityPolicyRule{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "rule-2",
					Namespace: "default",
				},
				Spec: ComputeOrganizationSecurityPolicyRuleSpec{
					Action:     "deny",
					PolicyID:   common.LazyPtr("securityPolicies/7777777777"),
					ResourceID: common.LazyPtr("4000"),
				},
			},
			wantIdentity: &ComputeOrganizationSecurityPolicyRuleIdentity{
				SecurityPolicy: "7777777777",
				Priority:       "4000",
			},
		},
		{
			name: "empty policyId",
			obj: &ComputeOrganizationSecurityPolicyRule{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "rule-empty-policy",
					Namespace: "default",
				},
				Spec: ComputeOrganizationSecurityPolicyRuleSpec{
					Action:     "allow",
					PolicyID:   common.LazyPtr(""),
					ResourceID: common.LazyPtr("1000"),
				},
			},
			wantErr: true,
		},
		{
			name: "invalid policyId format",
			obj: &ComputeOrganizationSecurityPolicyRule{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "rule-invalid-policy",
					Namespace: "default",
				},
				Spec: ComputeOrganizationSecurityPolicyRuleSpec{
					Action:     "allow",
					PolicyID:   common.LazyPtr("invalid/policy/format"),
					ResourceID: common.LazyPtr("1000"),
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id, err := tt.obj.GetIdentity(context.Background(), client)
			if (err != nil) != tt.wantErr {
				t.Fatalf("GetIdentity() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr {
				ruleID, ok := id.(*ComputeOrganizationSecurityPolicyRuleIdentity)
				if !ok {
					t.Fatalf("expected *ComputeOrganizationSecurityPolicyRuleIdentity, got %T", id)
				}
				if diff := cmp.Diff(tt.wantIdentity, ruleID); diff != "" {
					t.Errorf("GetIdentity() mismatch (-want +got):\n%s", diff)
				}
			}
		})
	}
}

func TestComputeOrganizationSecurityPolicyRuleRef_Normalize(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = SchemeBuilder.AddToScheme(scheme)

	targetObj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "compute.gcp.hfl.dev/v1beta1",
			"kind":       "ComputeOrganizationSecurityPolicyRule",
			"metadata": map[string]interface{}{
				"name":      "rule-target",
				"namespace": "default",
			},
			"spec": map[string]interface{}{
				"action":     "allow",
				"policyId":   "1234567890",
				"resourceID": "1000",
			},
			"status": map[string]interface{}{
				"conditions": []interface{}{
					map[string]interface{}{
						"type":   "Ready",
						"status": "True",
					},
				},
			},
		},
	}

	unreadyObj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "compute.gcp.hfl.dev/v1beta1",
			"kind":       "ComputeOrganizationSecurityPolicyRule",
			"metadata": map[string]interface{}{
				"name":      "rule-unready",
				"namespace": "default",
			},
			"spec": map[string]interface{}{
				"action":     "allow",
				"policyId":   "1234567890",
				"resourceID": "1000",
			},
			"status": map[string]interface{}{
				"conditions": []interface{}{
					map[string]interface{}{
						"type":   "Ready",
						"status": "False",
					},
				},
			},
		},
	}

	noConditionsObj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "compute.gcp.hfl.dev/v1beta1",
			"kind":       "ComputeOrganizationSecurityPolicyRule",
			"metadata": map[string]interface{}{
				"name":      "rule-no-conditions",
				"namespace": "default",
			},
			"spec": map[string]interface{}{
				"action":     "allow",
				"policyId":   "1234567890",
				"resourceID": "1000",
			},
		},
	}

	client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(targetObj, unreadyObj, noConditionsObj).Build()

	tests := []struct {
		name         string
		ref          *ComputeOrganizationSecurityPolicyRuleRef
		wantExternal string
		wantErr      bool
		errCheck     func(err error) bool
	}{
		{
			name: "external already set, normalized",
			ref: &ComputeOrganizationSecurityPolicyRuleRef{
				External: "https://compute.googleapis.com/compute/v1/locations/global/securityPolicies/1234567890/rules/1000",
			},
			wantExternal: "locations/global/securityPolicies/1234567890/rules/1000",
		},
		{
			name: "resolve from name and fallback",
			ref: &ComputeOrganizationSecurityPolicyRuleRef{
				Name:      "rule-target",
				Namespace: "default",
			},
			wantExternal: "locations/global/securityPolicies/1234567890/rules/1000",
		},
		{
			name: "resolve from name fails when target has Ready: False condition",
			ref: &ComputeOrganizationSecurityPolicyRuleRef{
				Name:      "rule-unready",
				Namespace: "default",
			},
			wantErr: true,
			errCheck: func(err error) bool {
				return k8s.IsReferenceNotReadyError(err)
			},
		},
		{
			name: "resolve from name fails when target has no conditions",
			ref: &ComputeOrganizationSecurityPolicyRuleRef{
				Name:      "rule-no-conditions",
				Namespace: "default",
			},
			wantErr: true,
			errCheck: func(err error) bool {
				return k8s.IsReferenceNotReadyError(err)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.ref.Normalize(context.Background(), client, "default")
			if (err != nil) != tt.wantErr {
				t.Fatalf("Normalize() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr && tt.errCheck != nil {
				if !tt.errCheck(err) {
					t.Errorf("Normalize() unexpected error type: %v", err)
				}
			}
			if !tt.wantErr && tt.ref.External != tt.wantExternal {
				t.Errorf("Normalize() external = %v, want %v", tt.ref.External, tt.wantExternal)
			}
		})
	}
}

// Ours: a priority that is not a number, a policy by reference, and a
// changed identity.
func TestComputeOrganizationSecurityPolicyRule_GetIdentityOurs(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = SchemeBuilder.AddToScheme(scheme)
	policy := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "compute.gcp.hfl.dev/v1beta1",
		"kind":       "ComputeOrganizationSecurityPolicy",
		"metadata":   map[string]interface{}{"name": "baseline", "namespace": "default"},
		"spec":       map[string]interface{}{"displayName": "baseline", "parent": "organizations/1"},
		"status":     map[string]interface{}{"policyId": "4242"},
	}}
	client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(policy).Build()
	rule := func(name string, set func(*ComputeOrganizationSecurityPolicyRule)) *ComputeOrganizationSecurityPolicyRule {
		r := &ComputeOrganizationSecurityPolicyRule{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"},
			Spec:       ComputeOrganizationSecurityPolicyRuleSpec{Action: "allow", PolicyID: common.LazyPtr("1")},
		}
		if set != nil {
			set(r)
		}
		return r
	}
	for name, tc := range map[string]struct {
		obj     *ComputeOrganizationSecurityPolicyRule
		want    string
		wantErr string
	}{
		"a name that is not a number": {obj: rule("allow-ssh", nil), wantErr: "is not a number"},
		"a negative priority":         {obj: rule("r", func(r *ComputeOrganizationSecurityPolicyRule) { r.Spec.ResourceID = common.LazyPtr("-1") }), wantErr: "is not a number"},
		"a priority past the maximum": {obj: rule("r", func(r *ComputeOrganizationSecurityPolicyRule) { r.Spec.ResourceID = common.LazyPtr("2147483648") }), wantErr: "is not a number"},
		"the maximum priority":        {obj: rule("2147483647", nil), want: "locations/global/securityPolicies/1/rules/2147483647"},
		"a policy by reference": {obj: rule("1000", func(r *ComputeOrganizationSecurityPolicyRule) {
			r.Spec.PolicyID = nil
			r.Spec.SecurityPolicyRef = &ComputeOrganizationSecurityPolicyRef{Name: "baseline"}
		}), want: "locations/global/securityPolicies/4242/rules/1000"},
		"a reference not yet resolved": {obj: rule("1000", func(r *ComputeOrganizationSecurityPolicyRule) {
			r.Spec.PolicyID = nil
			r.Spec.SecurityPolicyRef = &ComputeOrganizationSecurityPolicyRef{Name: "missing"}
		}), wantErr: "securityPolicyRef"},
		"the identity it was made with": {obj: rule("1000", func(r *ComputeOrganizationSecurityPolicyRule) {
			r.Status.ExternalRef = common.LazyPtr("locations/global/securityPolicies/1/rules/1000")
		}), want: "locations/global/securityPolicies/1/rules/1000"},
		"a changed identity": {obj: rule("1000", func(r *ComputeOrganizationSecurityPolicyRule) {
			r.Status.ExternalRef = common.LazyPtr("locations/global/securityPolicies/2/rules/1000")
		}), wantErr: "cannot change"},
	} {
		t.Run(name, func(t *testing.T) {
			id, err := tc.obj.GetIdentity(context.Background(), client)
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("GetIdentity() error = %v, want one containing %q", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got := id.String(); got != tc.want {
				t.Errorf("GetIdentity() = %q, want %q", got, tc.want)
			}
		})
	}
}
