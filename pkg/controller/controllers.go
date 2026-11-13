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

// Package controller starts the config-connector-extensions kinds on Config Connector's
// direct-controller reconciler (pkg/controller/direct/directbase) and its
// deletion defender (pkg/controller/deletiondefender), both unchanged. The
// direct reconciler adds the deletion-defender finalizer and will not delete
// until the defender has removed it; the defender abandons the cloud
// resource instead when the kind's CRD itself is being deleted, so an
// uninstall never deletes cloud resources.
// Config Connector chooses its reconcilers from a generated table of its own
// kinds (pkg/controller/resourceconfig/static_config.go) and skips any other
// CRD, so our kinds are registered here instead, in our own manager.
package controller

import (
	"context"
	"fmt"

	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/config"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/deletiondefender"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct/directbase"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/jitter"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiextensionsclient "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	crcontroller "sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	aiplatformv1beta1 "github.com/highfivelabs/config-connector-extensions/apis/aiplatform/v1beta1"
	artifactregistryv1alpha1 "github.com/highfivelabs/config-connector-extensions/apis/artifactregistry/v1alpha1"
	artifactregistryv1beta1 "github.com/highfivelabs/config-connector-extensions/apis/artifactregistry/v1beta1"
	computev1alpha1 "github.com/highfivelabs/config-connector-extensions/apis/compute/v1alpha1"
	computev1beta1 "github.com/highfivelabs/config-connector-extensions/apis/compute/v1beta1"
	datacatalogv1beta1 "github.com/highfivelabs/config-connector-extensions/apis/datacatalog/v1beta1"
	dnsv1beta1 "github.com/highfivelabs/config-connector-extensions/apis/dns/v1beta1"
	firestorev1beta1 "github.com/highfivelabs/config-connector-extensions/apis/firestore/v1beta1"
	iamv1alpha1 "github.com/highfivelabs/config-connector-extensions/apis/iam/v1alpha1"
	iamv1beta1 "github.com/highfivelabs/config-connector-extensions/apis/iam/v1beta1"
	loggingv1alpha1 "github.com/highfivelabs/config-connector-extensions/apis/logging/v1alpha1"
	networksecurityv1alpha1 "github.com/highfivelabs/config-connector-extensions/apis/networksecurity/v1alpha1"
	resourcemanagerv1alpha1 "github.com/highfivelabs/config-connector-extensions/apis/resourcemanager/v1alpha1"
	runv1alpha1 "github.com/highfivelabs/config-connector-extensions/apis/run/v1alpha1"
	secretmanagerv1beta1 "github.com/highfivelabs/config-connector-extensions/apis/secretmanager/v1beta1"
	securitycenterv1alpha1 "github.com/highfivelabs/config-connector-extensions/apis/securitycenter/v1alpha1"
	serviceusagev1alpha1 "github.com/highfivelabs/config-connector-extensions/apis/serviceusage/v1alpha1"
	spannerv1beta1 "github.com/highfivelabs/config-connector-extensions/apis/spanner/v1beta1"
	sqlv1beta1 "github.com/highfivelabs/config-connector-extensions/apis/sql/v1beta1"
	sqladminv1alpha1 "github.com/highfivelabs/config-connector-extensions/apis/sqladmin/v1alpha1"
	storagev1alpha1 "github.com/highfivelabs/config-connector-extensions/apis/storage/v1alpha1"
	"github.com/highfivelabs/config-connector-extensions/pkg/controller/direct/aiplatform"
	"github.com/highfivelabs/config-connector-extensions/pkg/controller/direct/artifactregistry"
	"github.com/highfivelabs/config-connector-extensions/pkg/controller/direct/compute"
	"github.com/highfivelabs/config-connector-extensions/pkg/controller/direct/datacatalog"
	"github.com/highfivelabs/config-connector-extensions/pkg/controller/direct/dns"
	"github.com/highfivelabs/config-connector-extensions/pkg/controller/direct/firestore"
	"github.com/highfivelabs/config-connector-extensions/pkg/controller/direct/iam"
	"github.com/highfivelabs/config-connector-extensions/pkg/controller/direct/logging"
	"github.com/highfivelabs/config-connector-extensions/pkg/controller/direct/networksecurity"
	"github.com/highfivelabs/config-connector-extensions/pkg/controller/direct/resourcemanager"
	"github.com/highfivelabs/config-connector-extensions/pkg/controller/direct/run"
	"github.com/highfivelabs/config-connector-extensions/pkg/controller/direct/secretmanager"
	"github.com/highfivelabs/config-connector-extensions/pkg/controller/direct/securitycenter"
	"github.com/highfivelabs/config-connector-extensions/pkg/controller/direct/serviceusage"
	"github.com/highfivelabs/config-connector-extensions/pkg/controller/direct/spanner"
	"github.com/highfivelabs/config-connector-extensions/pkg/controller/direct/sql"
	"github.com/highfivelabs/config-connector-extensions/pkg/controller/direct/sqladmin"
	"github.com/highfivelabs/config-connector-extensions/pkg/controller/direct/storage"
	"github.com/highfivelabs/config-connector-extensions/pkg/slotguard"
)

type kind struct {
	gvk     schema.GroupVersionKind
	crdName string
	model   func(context.Context, *config.ControllerConfig) (directbase.Model, error)
	// labels is where the cloud resource's labels come from, for the slot
	// guard (pkg/slotguard); unset, a CRD with spec.labels means
	// slotguard.Spec, any other none.
	labels slotguard.LabelSource
}

// Kinds is every kind config-connector-extensions reconciles.
var Kinds = []kind{
	{gvk: sqlv1beta1.SQLUserGVK, crdName: "sqlusers.sql.gcp.hfl.dev", model: sql.NewSQLUserModel},
	{gvk: sqladminv1alpha1.SQLAdminBackupGVK, crdName: "sqladminbackups.sqladmin.gcp.hfl.dev", model: sqladmin.NewSQLAdminBackupModel},
	{gvk: secretmanagerv1beta1.SecretManagerSecretGVK, crdName: "secretmanagersecrets.secretmanager.gcp.hfl.dev", model: secretmanager.NewModel, labels: slotguard.Metadata},
	{gvk: secretmanagerv1beta1.SecretManagerSecretVersionGVK, crdName: "secretmanagersecretversions.secretmanager.gcp.hfl.dev", model: secretmanager.NewSecretVersionModel},
	{gvk: artifactregistryv1beta1.ArtifactRegistryRepositoryGVK, crdName: "artifactregistryrepositories.artifactregistry.gcp.hfl.dev", model: artifactregistry.NewArtifactRegistryRepositoryModel, labels: slotguard.Metadata},
	{gvk: artifactregistryv1alpha1.ArtifactRegistryTagGVK, crdName: "artifactregistrytags.artifactregistry.gcp.hfl.dev", model: artifactregistry.NewArtifactRegistryTagModel},
	{gvk: iamv1alpha1.IAMWorkloadIdentityPoolNamespaceGVK, crdName: "iamworkloadidentitypoolnamespaces.iam.gcp.hfl.dev", model: iam.NewWorkloadIdentityPoolNamespaceModel},
	{gvk: iamv1alpha1.IAMWorkloadIdentityPoolManagedIdentityGVK, crdName: "iamworkloadidentitypoolmanagedidentities.iam.gcp.hfl.dev", model: iam.NewWorkloadIdentityPoolManagedIdentityModel},
	{gvk: iamv1alpha1.IAMWorkloadIdentityPoolProviderKeyGVK, crdName: "iamworkloadidentitypoolproviderkeys.iam.gcp.hfl.dev", model: iam.NewWorkloadIdentityPoolProviderKeyModel},
	{gvk: iamv1beta1.IAMBindingGVK, crdName: "iambindings.iam.gcp.hfl.dev", model: iam.NewBindingModel},
	{gvk: iamv1alpha1.IAMMemberGVK, crdName: "iammembers.iam.gcp.hfl.dev", model: iam.NewMemberModel},
	{gvk: runv1alpha1.RunServiceGVK, crdName: "runservices.run.gcp.hfl.dev", model: run.NewRunServiceModel, labels: slotguard.Spec},
	{gvk: computev1alpha1.ComputeRegionNetworkEndpointGVK, crdName: "computeregionnetworkendpoints.compute.gcp.hfl.dev", model: compute.NewRegionNetworkEndpointModel},
	{gvk: computev1beta1.ComputeNetworkEndpointGVK, crdName: "computenetworkendpoints.compute.gcp.hfl.dev", model: compute.NewComputeNetworkEndpointModel},
	{gvk: computev1beta1.ComputeAutoscalerGVK, crdName: "computeautoscalers.compute.gcp.hfl.dev", model: compute.NewComputeAutoscalerModel},
	{gvk: computev1beta1.ComputeDiskResourcePolicyAttachmentGVK, crdName: "computediskresourcepolicyattachments.compute.gcp.hfl.dev", model: compute.NewComputeDiskResourcePolicyAttachmentModel},
	{gvk: computev1beta1.ComputeRegionPerInstanceConfigGVK, crdName: "computeregionperinstanceconfigs.compute.gcp.hfl.dev", model: compute.NewComputeRegionPerInstanceConfigModel},
	{gvk: computev1alpha1.ComputeRouterRoutePolicyGVK, crdName: "computerouterroutepolicies.compute.gcp.hfl.dev", model: compute.NewRouterRoutePolicyModel},
	{gvk: computev1alpha1.ComputeProjectMetadataItemGVK, crdName: "computeprojectmetadataitems.compute.gcp.hfl.dev", model: compute.NewProjectMetadataItemModel},
	{gvk: storagev1alpha1.StorageBucketObjectGVK, crdName: "storagebucketobjects.storage.gcp.hfl.dev", model: storage.NewStorageBucketObjectModel},
	{gvk: computev1alpha1.ComputeRegionNetworkFirewallPolicyGVK, crdName: "computeregionnetworkfirewallpolicies.compute.gcp.hfl.dev", model: compute.NewRegionNetworkFirewallPolicyModel},
	{gvk: computev1alpha1.ComputeRegionNetworkFirewallPolicyRuleGVK, crdName: "computeregionnetworkfirewallpolicyrules.compute.gcp.hfl.dev", model: compute.NewRegionNetworkFirewallPolicyRuleModel},
	{gvk: computev1alpha1.ComputeRegionNetworkFirewallPolicyAssociationGVK, crdName: "computeregionnetworkfirewallpolicyassociations.compute.gcp.hfl.dev", model: compute.NewRegionNetworkFirewallPolicyAssociationModel},
	{gvk: loggingv1alpha1.LoggingSettingsGVK, crdName: "loggingsettings.logging.gcp.hfl.dev", model: logging.NewSettingsModel},
	{gvk: loggingv1alpha1.LoggingBillingAccountSinkGVK, crdName: "loggingbillingaccountsinks.logging.gcp.hfl.dev", model: logging.NewBillingAccountSinkModel},
	{gvk: serviceusagev1alpha1.ServiceIdentityGVK, crdName: "serviceidentities.serviceusage.gcp.hfl.dev", model: serviceusage.NewServiceIdentityModel},
	{gvk: computev1alpha1.ComputeProjectDefaultNetworkTierGVK, crdName: "computeprojectdefaultnetworktiers.compute.gcp.hfl.dev", model: compute.NewProjectDefaultNetworkTierModel},
	{gvk: resourcemanagerv1alpha1.ProjectDefaultServiceAccountsGVK, crdName: "projectdefaultserviceaccounts.resourcemanager.gcp.hfl.dev", model: resourcemanager.NewProjectDefaultServiceAccountsModel},
	{gvk: networksecurityv1alpha1.NetworkSecurityAddressGroupGVK, crdName: "networksecurityaddressgroups.networksecurity.gcp.hfl.dev", model: networksecurity.NewAddressGroupModel, labels: slotguard.Spec},
	{gvk: networksecurityv1alpha1.NetworkSecuritySecurityProfileGVK, crdName: "networksecuritysecurityprofiles.networksecurity.gcp.hfl.dev", model: networksecurity.NewSecurityProfileModel, labels: slotguard.Spec},
	{gvk: networksecurityv1alpha1.NetworkSecuritySecurityProfileGroupGVK, crdName: "networksecuritysecurityprofilegroups.networksecurity.gcp.hfl.dev", model: networksecurity.NewSecurityProfileGroupModel, labels: slotguard.Spec},
	{gvk: computev1alpha1.ComputeRouterGVK, crdName: "computerouters.compute.gcp.hfl.dev", model: compute.NewRouterModel},
	{gvk: iamv1beta1.IAMDenyPolicyGVK, crdName: "iamdenypolicies.iam.gcp.hfl.dev", model: iam.NewDenyPolicyModel},
	{gvk: iamv1beta1.IAMPrincipalAccessBoundaryPolicyGVK, crdName: "iamprincipalaccessboundarypolicies.iam.gcp.hfl.dev", model: iam.NewPrincipalAccessBoundaryPolicyModel},
	{gvk: computev1beta1.ComputeOrganizationSecurityPolicyGVK, crdName: "computeorganizationsecuritypolicies.compute.gcp.hfl.dev", model: compute.NewComputeOrganizationSecurityPolicyModel},
	{gvk: computev1beta1.ComputeOrganizationSecurityPolicyAssociationGVK, crdName: "computeorganizationsecuritypolicyassociations.compute.gcp.hfl.dev", model: compute.NewComputeOrganizationSecurityPolicyAssociationModel},
	{gvk: computev1beta1.ComputeOrganizationSecurityPolicyRuleGVK, crdName: "computeorganizationsecuritypolicyrules.compute.gcp.hfl.dev", model: compute.NewOrganizationSecurityPolicyRuleModel},
	{gvk: computev1beta1.ComputeNetworkFirewallPolicyRuleGVK, crdName: "computenetworkfirewallpolicyrules.compute.gcp.hfl.dev", model: compute.NewNetworkFirewallPolicyRuleModel},
	{gvk: securitycenterv1alpha1.SecurityCenterMuteConfigGVK, crdName: "securitycentermuteconfigs.securitycenter.gcp.hfl.dev", model: securitycenter.NewMuteConfigModel},
	{gvk: dnsv1beta1.DNSResponsePolicyGVK, crdName: "dnsresponsepolicies.dns.gcp.hfl.dev", model: dns.NewResponsePolicyModel, labels: slotguard.Metadata},
	{gvk: dnsv1beta1.DNSResponsePolicyRuleGVK, crdName: "dnsresponsepolicyrules.dns.gcp.hfl.dev", model: dns.NewResponsePolicyRuleModel},
	{gvk: computev1alpha1.ComputeNetworkFirewallPolicyPacketMirroringRuleGVK, crdName: "computenetworkfirewallpolicypacketmirroringrules.compute.gcp.hfl.dev", model: compute.NewNetworkFirewallPolicyPacketMirroringRuleModel},
	{gvk: computev1beta1.ComputeVPNTunnelGVK, crdName: "computevpntunnels.compute.gcp.hfl.dev", model: compute.NewVPNTunnelModel, labels: slotguard.Spec},
	{gvk: firestorev1beta1.FirestoreBackupScheduleGVK, crdName: "firestorebackupschedules.firestore.gcp.hfl.dev", model: firestore.NewFirestoreBackupScheduleModel},
	{gvk: spannerv1beta1.SpannerInstanceConfigGVK, crdName: "spannerinstanceconfigs.spanner.gcp.hfl.dev", model: spanner.NewInstanceConfigModel, labels: slotguard.Spec},
	{gvk: datacatalogv1beta1.DataCatalogTagGVK, crdName: "datacatalogtags.datacatalog.gcp.hfl.dev", model: datacatalog.NewTagModel},
	{gvk: aiplatformv1beta1.AIPlatformReasoningEngineGVK, crdName: "aiplatformreasoningengines.aiplatform.gcp.hfl.dev", model: aiplatform.NewAIPlatformReasoningEngineModel, labels: slotguard.Spec},
}

// AddToScheme adds every config-connector-extensions API group to a scheme.
func AddToScheme(scheme *runtime.Scheme) error {
	for _, add := range []func(*runtime.Scheme) error{
		sqlv1beta1.AddToScheme,
		sqladminv1alpha1.AddToScheme,
		secretmanagerv1beta1.AddToScheme,
		artifactregistryv1beta1.AddToScheme,
		artifactregistryv1alpha1.AddToScheme,
		iamv1alpha1.AddToScheme,
		iamv1beta1.AddToScheme,
		runv1alpha1.AddToScheme,
		computev1alpha1.AddToScheme,
		computev1beta1.AddToScheme,
		dnsv1beta1.AddToScheme,
		firestorev1beta1.AddToScheme,
		spannerv1beta1.AddToScheme,
		datacatalogv1beta1.AddToScheme,
		aiplatformv1beta1.AddToScheme,
		storagev1alpha1.AddToScheme,
		loggingv1alpha1.AddToScheme,
		serviceusagev1alpha1.AddToScheme,
		resourcemanagerv1alpha1.AddToScheme,
		networksecurityv1alpha1.AddToScheme,
		securitycenterv1alpha1.AddToScheme,
	} {
		if err := add(scheme); err != nil {
			return err
		}
	}
	// The discokind kinds' groups (zz_discokinds.go); adding a group twice
	// is harmless.
	for _, add := range discoSchemes {
		if err := add(scheme); err != nil {
			return err
		}
	}
	return nil
}

// Options are the manager's choices that change how every kind reconciles.
type Options struct {
	// Slot is the version slot this manager is (--slot, HFL_SLOT); empty
	// turns the slot guard off (pkg/slotguard).
	Slot string
}

// AddToManager adds a direct controller and a deletion defender for every
// kind in Kinds. The kinds' CRDs must be installed.
func AddToManager(ctx context.Context, mgr manager.Manager, cfg *config.ControllerConfig, opts Options) error {
	crds, err := apiextensionsclient.NewForConfig(mgr.GetConfig())
	if err != nil {
		return fmt.Errorf("building apiextensions client: %w", err)
	}
	deps := directbase.Deps{
		// No Terraform service mappings or DCL metadata: the reconcile
		// interval is Config Connector's default, or the object's
		// cnrm.cloud.google.com/reconcile-interval-in-seconds annotation.
		JitterGenerator:    jitter.NewDefaultGenerator(nil, nil),
		SkipNameValidation: cfg.SkipNameValidation,
	}
	var guard *slotguard.Guard
	if opts.Slot != "" {
		if err := slotguard.Validate(opts.Slot); err != nil {
			return err
		}
		guard = &slotguard.Guard{Slot: opts.Slot, Recorder: mgr.GetEventRecorderFor("config-connector-extensions-slot-guard")}
	}
	for _, k := range Kinds {
		model, err := k.model(ctx, cfg)
		if err != nil {
			return fmt.Errorf("building model for %v: %w", k.gvk, err)
		}
		crd, err := crds.ApiextensionsV1().CustomResourceDefinitions().Get(ctx, k.crdName, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("reading CRD %s: %w", k.crdName, err)
		}
		if guard != nil {
			if model, err = guard.Wrap(k.gvk, model, LabelSource(k, crd)); err != nil {
				return err
			}
		}
		if err := directbase.AddController(mgr, k.gvk, model, deps); err != nil {
			return fmt.Errorf("adding controller for %v: %w", k.gvk, err)
		}
		if err := deletiondefender.Add(mgr, crd, crcontroller.Options{SkipNameValidation: &cfg.SkipNameValidation}); err != nil {
			return fmt.Errorf("adding deletion defender for %v: %w", k.gvk, err)
		}
	}
	return nil
}

// LabelSource is where k's cloud labels come from: the kind's own choice,
// else spec.labels when its CRD (served version) has it, else none.
func LabelSource(k kind, crd *apiextensionsv1.CustomResourceDefinition) slotguard.LabelSource {
	if k.labels != slotguard.None {
		return k.labels
	}
	for _, v := range crd.Spec.Versions {
		if v.Name != k.gvk.Version || v.Schema == nil || v.Schema.OpenAPIV3Schema == nil {
			continue
		}
		if _, ok := v.Schema.OpenAPIV3Schema.Properties["spec"].Properties["labels"]; ok {
			return slotguard.Spec
		}
	}
	return slotguard.None
}
