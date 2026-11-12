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

package controller

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"sigs.k8s.io/yaml"

	"github.com/highfivelabs/config-connector-extensions/pkg/slotguard"
)

// slotLabelled is every kind whose cloud resource carries labels, so the
// slot guard can mark it, and where they come from. Every other kind is in
// docs/design.md "Version slots" as one the guard cannot mark. A new kind
// with labels changes this list, its adapter gets CloudLabels, and the
// design's list changes with it.
var slotLabelled = map[string]slotguard.LabelSource{
	"SecretManagerSecret":                            slotguard.Metadata,
	"ArtifactRegistryRepository":                     slotguard.Metadata,
	"DNSResponsePolicy":                              slotguard.Metadata,
	"RunService":                                     slotguard.Spec,
	"ComputeVPNTunnel":                               slotguard.Spec,
	"NetworkSecurityAddressGroup":                    slotguard.Spec,
	"NetworkSecuritySecurityProfile":                 slotguard.Spec,
	"NetworkSecuritySecurityProfileGroup":            slotguard.Spec,
	"GKEHubMembershipRBACRoleBinding":                slotguard.Spec,
	"GKEHubRolloutSequence":                          slotguard.Spec,
	"NetworkServicesAgentGateway":                    slotguard.Spec,
	"NetworkServicesAgentConnectivityTemplate":       slotguard.Spec,
	"NetworkConnectivityPscAuthorizationPolicy":      slotguard.Spec,
	"NetworkConnectivityAutomatedDnsRecord":          slotguard.Spec,
	"NetworkConnectivitySpokeGatewayAdvertisedRoute": slotguard.Spec,
	"CertificateManagerCertificateIssuanceConfig":    slotguard.Spec,
	"BackupDRManagementServer":                       slotguard.Spec,
	"NetworkSecurityDNSThreatDetector":               slotguard.Spec,
	"GKEBackupBackupPlan":                            slotguard.Spec,
	"GKEBackupRestorePlan":                           slotguard.Spec,
	"GKEBackupBackupChannel":                         slotguard.Spec,
	"GKEBackupRestoreChannel":                        slotguard.Spec,
	"APIGatewayAPIConfig":                            slotguard.Spec,
	"APIGatewayGateway":                              slotguard.Spec,
	"CloudDeployTarget":                              slotguard.Spec,
	"CloudDeployDeployPolicy":                        slotguard.Spec,
	"CloudDeployAutomation":                          slotguard.Spec,
	"CloudFunctions2Function":                        slotguard.Spec,
	"DataplexLake":                                   slotguard.Spec,
	"DataplexZone":                                   slotguard.Spec,
	"DataplexAsset":                                  slotguard.Spec,
	"DataplexAspectType":                             slotguard.Spec,
	"DataplexDataScan":                               slotguard.Spec,
	"AIPlatformReasoningEngine":                      slotguard.Spec,
}

func TestSlotLabelSources(t *testing.T) {
	seen := map[string]bool{}
	for _, k := range Kinds {
		plural, group, _ := strings.Cut(k.crdName, ".")
		path := filepath.Join("..", "..", "config", "crds", group+"_"+plural+".yaml")
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("%s: %v", k.gvk.Kind, err)
		}
		crd := &apiextensionsv1.CustomResourceDefinition{}
		if err := yaml.Unmarshal(data, crd); err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		got := LabelSource(k, crd)
		want := slotLabelled[k.gvk.Kind]
		if got != want {
			t.Errorf("%s: label source %d, want %d (slotLabelled)", k.gvk.Kind, got, want)
		}
		seen[k.gvk.Kind] = true
	}
	for kind := range slotLabelled {
		if !seen[kind] {
			t.Errorf("slotLabelled names %s, which is not a kind", kind)
		}
	}
}
