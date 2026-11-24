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

package v1alpha1

// Written here, in the shape of our ComputeRegionNetworkFirewallPolicyRule's
// identity and reference: how a regional security policy's rule names its
// policy and itself.

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/GoogleCloudPlatform/k8s-config-connector/apis/common"
	"github.com/GoogleCloudPlatform/k8s-config-connector/apis/common/identity"
	apirefs "github.com/GoogleCloudPlatform/k8s-config-connector/apis/refs"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/k8s"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ComputeRegionSecurityPolicyIdentity is
// projects/{project}/regions/{region}/securityPolicies/{name}.
// +k8s:deepcopy-gen=false
type ComputeRegionSecurityPolicyIdentity struct {
	Project string
	Region  string
	Name    string
}

func (i *ComputeRegionSecurityPolicyIdentity) String() string {
	return fmt.Sprintf("projects/%s/regions/%s/securityPolicies/%s", i.Project, i.Region, i.Name)
}

// ParseComputeRegionSecurityPolicyExternal reads a policy's name, or its
// URL.
func ParseComputeRegionSecurityPolicyExternal(external string) (*ComputeRegionSecurityPolicyIdentity, error) {
	tokens := strings.Split(strings.TrimPrefix(apirefs.TrimComputeURIPrefix(external), "/"), "/")
	if len(tokens) != 6 || tokens[0] != "projects" || tokens[2] != "regions" || tokens[4] != "securityPolicies" || tokens[1] == "" || tokens[3] == "" || tokens[5] == "" {
		return nil, fmt.Errorf("format of a regional security policy %q was not known (use projects/{{project}}/regions/{{region}}/securityPolicies/{{name}})", external)
	}
	return &ComputeRegionSecurityPolicyIdentity{Project: tokens[1], Region: tokens[3], Name: tokens[5]}, nil
}

// Resolve finds the policy the reference names: a ComputeRegionSecurityPolicy
// object's status.externalRef, or external. An object that does not exist
// or is not Ready yet is a reference error, which the reconciler waits on
// and retries.
func (r *ComputeRegionSecurityPolicyRef) Resolve(ctx context.Context, reader client.Reader, defaultNamespace string) (*ComputeRegionSecurityPolicyIdentity, error) {
	if r == nil || (r.Name == "") == (r.External == "") {
		return nil, fmt.Errorf("securityPolicyRef needs exactly one of name and external")
	}
	if r.External != "" {
		return ParseComputeRegionSecurityPolicyExternal(r.External)
	}
	key := types.NamespacedName{Namespace: r.Namespace, Name: r.Name}
	if key.Namespace == "" {
		key.Namespace = defaultNamespace
	}
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(ComputeRegionSecurityPolicyGVK)
	if err := reader.Get(ctx, key, u); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, k8s.NewReferenceNotFoundError(ComputeRegionSecurityPolicyGVK, key)
		}
		return nil, fmt.Errorf("reading referenced ComputeRegionSecurityPolicy %s: %w", key, err)
	}
	ready := false
	conds, _, _ := unstructured.NestedSlice(u.Object, "status", "conditions")
	for _, c := range conds {
		if m, _ := c.(map[string]any); m["type"] == "Ready" && m["status"] == "True" {
			ready = true
		}
	}
	ext, _, _ := unstructured.NestedString(u.Object, "status", "externalRef")
	if !ready || ext == "" {
		return nil, k8s.NewReferenceNotReadyError(ComputeRegionSecurityPolicyGVK, key)
	}
	id, err := ParseComputeRegionSecurityPolicyExternal(ext)
	if err != nil {
		return nil, fmt.Errorf("referenced ComputeRegionSecurityPolicy %s: %w", key, err)
	}
	return id, nil
}

var _ identity.Identity = &ComputeRegionSecurityPolicyRuleIdentity{}

// ComputeRegionSecurityPolicyRuleIdentity is
// projects/{project}/regions/{region}/securityPolicies/{policy}/rules/{priority}.
// The API names a rule only by its priority in the policy; this spelling is
// ours, as for a regional firewall policy's rule.
// +k8s:deepcopy-gen=false
type ComputeRegionSecurityPolicyRuleIdentity struct {
	Policy   ComputeRegionSecurityPolicyIdentity
	Priority int64
}

func (i *ComputeRegionSecurityPolicyRuleIdentity) String() string {
	return fmt.Sprintf("%s/rules/%d", i.Policy.String(), i.Priority)
}

func (i *ComputeRegionSecurityPolicyRuleIdentity) FromExternal(ref string) error {
	policy, rule, ok := strings.Cut(strings.TrimPrefix(ref, "/"), "/rules/")
	if !ok {
		return fmt.Errorf("format of ComputeRegionSecurityPolicyRule external=%q was not known (use projects/{{project}}/regions/{{region}}/securityPolicies/{{policy}}/rules/{{priority}})", ref)
	}
	priority, err := strconv.ParseInt(rule, 10, 64)
	if err != nil {
		return fmt.Errorf("format of ComputeRegionSecurityPolicyRule external=%q was not known: the priority is not a number", ref)
	}
	p, err := ParseComputeRegionSecurityPolicyExternal(policy)
	if err != nil {
		return err
	}
	*i = ComputeRegionSecurityPolicyRuleIdentity{Policy: *p, Priority: priority}
	return nil
}

// GetIdentity is the rule of the referenced policy at spec.priority. A
// rule whose policy resolves differently from the one it was made in (a
// referenced policy made anew elsewhere) is refused.
func (obj *ComputeRegionSecurityPolicyRule) GetIdentity(ctx context.Context, reader client.Reader) (identity.Identity, error) {
	policy, err := obj.Spec.SecurityPolicyRef.Resolve(ctx, reader, obj.GetNamespace())
	if err != nil {
		return nil, err
	}
	id := &ComputeRegionSecurityPolicyRuleIdentity{Policy: *policy, Priority: obj.Spec.Priority}
	if externalRef := common.ValueOf(obj.Status.ExternalRef); externalRef != "" && externalRef != id.String() {
		return nil, fmt.Errorf("cannot change ComputeRegionSecurityPolicyRule identity (old=%q, new=%q)", externalRef, id.String())
	}
	return id, nil
}
