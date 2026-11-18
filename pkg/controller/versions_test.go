// Copyright 2026 High Five Labs
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"sigs.k8s.io/yaml"
)

// promoted are our own kinds that moved from v1alpha1 to v1beta1 once they
// passed live (docs/design.md decisions 24 and 27). Each CRD serves
// v1beta1 only: v1alpha1 is no longer served, and the manager watches
// v1beta1.
var promoted = []string{
	"IAMBinding",
	"ComputeRouter", "LoggingSettings", "SecurityCenterMuteConfig",
	"ArtifactRegistryTag",
	"ComputeProjectMetadataItem",
	"ComputeRegionNetworkEndpoint",
	"ComputeRegionNetworkFirewallPolicy",
	"ComputeRegionNetworkFirewallPolicyAssociation",
	"ComputeRegionNetworkFirewallPolicyRule",
	"ComputeRouterRoutePolicy",
	"IAMMember",
	"IAMWorkloadIdentityPoolNamespace",
	"IAMWorkloadIdentityPoolManagedIdentity",
	"IAMWorkloadIdentityPoolProviderKey",
	"NetworkSecurityAddressGroup",
	"NetworkSecuritySecurityProfile",
	"NetworkSecuritySecurityProfileGroup",
	"SQLAdminBackup",
	"StorageBucketObject",
	// A field-gap kind (section 10 question 9, answered 2026-11-15).
	"RunService",
	// Passed live 2026-11-15 (cce-11152310-b63f).
	"ServiceIdentity",
	// Generated kinds that passed live with their CRDs unchanged since
	// (decision 34, 2026-11-15).
	"ComputeRegionHealthAggregationPolicy", "LoggingSavedQuery",
	"ComputeRegionBackendBucket", "LoggingLink",
	"GKEHubRolloutSequence", "IAMOauthClient",
	// Field kinds that passed live (run cce-11160848-7b8d, 2026-11-16).
	"ContainerClusterFields", "ContainerNodePoolFields",
	// Passed live 2026-11-18 (run cce-11181701-a6f8) as live-tests@, CRD unchanged.
	"LoggingBillingAccountSink",
	// Passed live at 78644ec (run cce-11120932-1df3), CRD unchanged since.
	"ComputeNetworkFirewallPolicyPacketMirroringRule",
}

func TestPromotedKindsServeOnlyV1beta1(t *testing.T) {
	byKind := map[string]kind{}
	for _, k := range Kinds {
		byKind[k.gvk.Kind] = k
	}
	for _, name := range promoted {
		k, ok := byKind[name]
		if !ok {
			t.Errorf("%s: not in Kinds", name)
			continue
		}
		if k.gvk.Version != "v1beta1" {
			t.Errorf("%s: the manager watches %s, want v1beta1", name, k.gvk.Version)
		}
		plural, group, _ := strings.Cut(k.crdName, ".")
		path := filepath.Join("..", "..", "config", "crds", group+"_"+plural+".yaml")
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		crd := &apiextensionsv1.CustomResourceDefinition{}
		if err := yaml.Unmarshal(data, crd); err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		var served []string
		for _, v := range crd.Spec.Versions {
			if v.Served {
				served = append(served, v.Name)
			}
		}
		if len(served) != 1 || served[0] != "v1beta1" {
			t.Errorf("%s: the CRD serves %v, want only v1beta1", name, served)
		}
	}
}
