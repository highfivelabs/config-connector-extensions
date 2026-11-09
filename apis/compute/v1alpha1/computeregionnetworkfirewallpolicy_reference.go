// Copyright 2026 High Five Labs
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

// Written here: how the kinds that live in a regional network firewall
// policy (its rules, its associations) name the policy.

import (
	"context"
	"fmt"

	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/k8s"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ComputeRegionNetworkFirewallPolicyRef names a regional network firewall
// policy. Exactly one of name and external is required.
// +kubebuilder:validation:XValidation:rule="has(self.name) != has(self.external)",message="exactly one of name and external is required"
type ComputeRegionNetworkFirewallPolicyRef struct {
	// The name of a ComputeRegionNetworkFirewallPolicy object.
	// +optional
	Name string `json:"name,omitempty"`

	// The namespace of that object; the referring object's by default.
	// +optional
	Namespace string `json:"namespace,omitempty"`

	// A policy not managed by an object, as
	// projects/{project}/regions/{region}/firewallPolicies/{name}.
	// +optional
	External string `json:"external,omitempty"`
}

// Resolve finds the policy the reference names. A policy object that does
// not exist or is not Ready yet is a reference error, which the reconciler
// waits on and retries.
func (r *ComputeRegionNetworkFirewallPolicyRef) Resolve(ctx context.Context, reader client.Reader, defaultNamespace string) (*ComputeRegionNetworkFirewallPolicyIdentity, error) {
	if r == nil || (r.Name == "") == (r.External == "") {
		return nil, fmt.Errorf("firewallPolicyRef needs exactly one of name and external")
	}
	if r.External != "" {
		return ParseComputeRegionNetworkFirewallPolicyExternal(r.External)
	}
	key := types.NamespacedName{Namespace: r.Namespace, Name: r.Name}
	if key.Namespace == "" {
		key.Namespace = defaultNamespace
	}
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(ComputeRegionNetworkFirewallPolicyGVK)
	if err := reader.Get(ctx, key, u); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, k8s.NewReferenceNotFoundError(ComputeRegionNetworkFirewallPolicyGVK, key)
		}
		return nil, fmt.Errorf("reading referenced ComputeRegionNetworkFirewallPolicy %s: %w", key, err)
	}
	if conds, _, _ := unstructured.NestedSlice(u.Object, "status", "conditions"); len(conds) > 0 {
		for _, c := range conds {
			if m, _ := c.(map[string]any); m["type"] == "Ready" && m["status"] != "True" {
				return nil, k8s.NewReferenceNotReadyError(ComputeRegionNetworkFirewallPolicyGVK, key)
			}
		}
	}
	policy := &ComputeRegionNetworkFirewallPolicy{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, policy); err != nil {
		return nil, fmt.Errorf("reading referenced ComputeRegionNetworkFirewallPolicy %s: %w", key, err)
	}
	id, err := getIdentityFromComputeRegionNetworkFirewallPolicySpec(ctx, reader, policy)
	if err != nil {
		return nil, fmt.Errorf("referenced ComputeRegionNetworkFirewallPolicy %s: %w", key, err)
	}
	return id, nil
}
