// Copyright 2026 Google LLC
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//    http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// SPDX-License-Identifier: Apache-2.0

// Modified by High Five Labs for config-connector-extensions: copied from Config Connector
// apis/compute/v1beta1/computevpntunnel_types.go (UPSTREAM.md), the direct
// KRM types of the Terraform-based ComputeVPNTunnel, as a whole-object kind
// of ours for a field gap (docs/design.md decision 19 and "Field gaps on
// large Terraform-based kinds"). Added: cipherSuite, labels, and
// status.externalRef. Changed: immutable fields are CEL rules (decision 20)
// rather than Terraform's ForceNew; routerRef is ours, which finds our
// ComputeRouter or Config Connector's; group, labels and short names are
// ours. The rest, comments included, is upstream's.

package v1beta1

import (
	computev1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/apis/compute/v1beta1"
	refsv1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/apis/refs/v1beta1"
	"github.com/GoogleCloudPlatform/k8s-config-connector/apis/refs/v1beta1/secret"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/apis/k8s/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	computev1alpha1 "github.com/highfivelabs/config-connector-extensions/apis/compute/v1alpha1"
)

var ComputeVPNTunnelGVK = GroupVersion.WithKind("ComputeVPNTunnel")

// ComputeVPNTunnelSpec defines the desired state of ComputeVPNTunnel
// +kcc:spec:proto=google.cloud.compute.v1.VpnTunnel
// +kubebuilder:validation:XValidation:rule="has(self.vpnGatewayRef) != has(self.targetVPNGatewayRef)",message="exactly one of vpnGatewayRef and targetVPNGatewayRef is required"
type ComputeVPNTunnelSpec struct {
	/* Immutable. An optional description of this resource. */
	// +optional
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="description is immutable"
	Description *string `json:"description,omitempty"`

	/* Immutable. IKE protocol version to use when establishing the VPN tunnel with
	peer VPN gateway.
	Acceptable IKE versions are 1 or 2. Default version is 2. */
	// +optional
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="ikeVersion is immutable"
	IkeVersion *int64 `json:"ikeVersion,omitempty"`

	/* Immutable. Local traffic selector to use when establishing the VPN tunnel with
	peer VPN gateway. The value should be a CIDR formatted string,
	for example '192.168.0.0/16'. The ranges should be disjoint.
	Only IPv4 is supported. */
	// +optional
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="localTrafficSelector is immutable"
	LocalTrafficSelector []string `json:"localTrafficSelector,omitempty"`

	/* Immutable. The interface ID of the external VPN gateway to which this VPN tunnel is connected. */
	// +optional
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="peerExternalGatewayInterface is immutable"
	PeerExternalGatewayInterface *int64 `json:"peerExternalGatewayInterface,omitempty"`

	/* The peer side external VPN gateway to which this VPN tunnel
	is connected. */
	// +optional
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="peerExternalGatewayRef is immutable"
	PeerExternalGatewayRef *computev1beta1.ComputeExternalVPNGatewayRef `json:"peerExternalGatewayRef,omitempty"`

	/* The peer side HA GCP VPN gateway to which this VPN tunnel is
	connected. If provided, the VPN tunnel will automatically use the
	same VPN gateway interface ID in the peer GCP VPN gateway. */
	// +optional
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="peerGCPGatewayRef is immutable"
	PeerGCPGatewayRef *computev1beta1.ComputeVPNGatewayRef `json:"peerGCPGatewayRef,omitempty"`

	/* Immutable. IP address of the peer VPN gateway. Only IPv4 is supported. */
	// +optional
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="peerIp is immutable"
	PeerIp *string `json:"peerIp,omitempty"`

	/* The project of the tunnel. Optional: the namespace's project
	(the cnrm.cloud.google.com/project-id annotation) by default. Immutable. */
	// +optional
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="projectRef is immutable"
	ProjectRef *refsv1beta1.ProjectRef `json:"projectRef,omitempty"`

	/* Immutable. The region where the tunnel is located. If unset, is set to the region of 'target_vpn_gateway'. */
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="region is immutable"
	Region string `json:"region"`

	/* Immutable. Remote traffic selector to use when establishing the VPN tunnel with
	peer VPN gateway. The value should be a CIDR formatted string,
	for example '192.168.0.0/16'. The ranges should be disjoint.
	Only IPv4 is supported. */
	// +optional
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="remoteTrafficSelector is immutable"
	RemoteTrafficSelector []string `json:"remoteTrafficSelector,omitempty"`

	/* Immutable. Optional. The name of the resource. Used for creation and acquisition. When unset, the value of `metadata.name` is used as the default. */
	// +optional
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="resourceID is immutable"
	ResourceID *string `json:"resourceID,omitempty"`

	/* The router to be used for dynamic routing: a ComputeRouter of
	this project's or Config Connector's, by name, or by external. Immutable. */
	// +optional
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="routerRef is immutable"
	RouterRef *computev1alpha1.ComputeRouterRef `json:"routerRef,omitempty"`

	/* Immutable. Shared secret used to set the secure session between the Cloud VPN
	gateway and the peer VPN gateway. The API never returns it: a changed
	Secret is not applied to an existing tunnel (make a new tunnel). */
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="sharedSecret is immutable"
	SharedSecret secret.Legacy `json:"sharedSecret"`

	/* The ComputeTargetVPNGateway with which this VPN tunnel is
	associated (Classic VPN). Immutable. */
	// +optional
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="targetVPNGatewayRef is immutable"
	TargetVPNGatewayRef *refsv1beta1.ComputeTargetVPNGatewayRef `json:"targetVPNGatewayRef,omitempty"`

	/* Immutable. The interface ID of the VPN gateway with which this VPN tunnel is associated. */
	// +optional
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="vpnGatewayInterface is immutable"
	VpnGatewayInterface *int64 `json:"vpnGatewayInterface,omitempty"`

	/* The ComputeVPNGateway with which this VPN tunnel is associated.
	This must be used if a High Availability VPN gateway resource is
	created. Immutable. */
	// +optional
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="vpnGatewayRef is immutable"
	VpnGatewayRef *computev1beta1.ComputeVPNGatewayRef `json:"vpnGatewayRef,omitempty"`

	/* The IKE phase 1 and phase 2 algorithms the tunnel may use. Unset
	means the API's default set. Immutable: the API takes it only when the
	tunnel is made. */
	// +optional
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="cipherSuite is immutable"
	CipherSuite *VPNTunnelCipherSuite `json:"cipherSuite,omitempty"`

	/* Labels on the tunnel. The only field that changes in place. */
	// +optional
	Labels map[string]string `json:"labels,omitempty"`
}

// VPNTunnelCipherSuite is the API's VpnTunnelCipherSuite: the algorithms
// each IKE phase may use.
type VPNTunnelCipherSuite struct {
	// +optional
	Phase1 *VPNTunnelPhase1Algorithms `json:"phase1,omitempty"`
	// +optional
	Phase2 *VPNTunnelPhase2Algorithms `json:"phase2,omitempty"`
}

// VPNTunnelPhase1Algorithms is the API's VpnTunnelPhase1Algorithms.
type VPNTunnelPhase1Algorithms struct {
	/* Diffie-Hellman groups, for example Group-14. */
	// +optional
	Dh []string `json:"dh,omitempty"`
	/* Encryption algorithms, for example AES-CBC-256 or AES-GCM-16-256. */
	// +optional
	Encryption []string `json:"encryption,omitempty"`
	/* Integrity algorithms, for example HMAC-SHA2-256-128. */
	// +optional
	Integrity []string `json:"integrity,omitempty"`
	/* Pseudo-random functions, for example PRF-HMAC-SHA2-256. */
	// +optional
	Prf []string `json:"prf,omitempty"`
}

// VPNTunnelPhase2Algorithms is the API's VpnTunnelPhase2Algorithms.
type VPNTunnelPhase2Algorithms struct {
	/* Encryption algorithms, for example AES-CBC-256. */
	// +optional
	Encryption []string `json:"encryption,omitempty"`
	/* Integrity algorithms, for example HMAC-SHA2-256-128. */
	// +optional
	Integrity []string `json:"integrity,omitempty"`
	/* Perfect forward secrecy groups, for example Group-14. */
	// +optional
	Pfs []string `json:"pfs,omitempty"`
}

// ComputeVPNTunnelStatus defines the config connector machine state of ComputeVPNTunnel
type ComputeVPNTunnelStatus struct {
	/* Conditions represent the latest available observations of the
	   ComputeVPNTunnel's current state. */
	// +optional
	Conditions []v1alpha1.Condition `json:"conditions,omitempty"`

	/* The tunnel, as projects/{project}/regions/{region}/vpnTunnels/{name}. */
	// +optional
	ExternalRef *string `json:"externalRef,omitempty"`

	/* Creation timestamp in RFC3339 text format. */
	// +optional
	CreationTimestamp *string `json:"creationTimestamp,omitempty"`

	/* Detailed status message for the VPN tunnel. */
	// +optional
	DetailedStatus *string `json:"detailedStatus,omitempty"`

	/* The fingerprint used for optimistic locking of this resource.  Used
	internally during updates. */
	// +optional
	LabelFingerprint *string `json:"labelFingerprint,omitempty"`

	/* ObservedGeneration is the generation of the resource that was most recently observed by the Config Connector controller. If this is equal to metadata.generation, then that means that the current reported status reflects the most recent desired state of the resource. */
	// +optional
	ObservedGeneration *int64 `json:"observedGeneration,omitempty"`

	// +optional
	SelfLink *string `json:"selfLink,omitempty"`

	/* Hash of the shared secret. */
	// +optional
	SharedSecretHash *string `json:"sharedSecretHash,omitempty"`

	/* The unique identifier for the resource. This identifier is defined by the server. */
	// +optional
	TunnelId *string `json:"tunnelId,omitempty"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:categories=gcp;hfl,shortName=hflcomputevpntunnel;hflcomputevpntunnels
// +kubebuilder:subresource:status
// +kubebuilder:metadata:labels="gcp.hfl.dev/gap=true"
// +kubebuilder:printcolumn:name="Age",JSONPath=".metadata.creationTimestamp",type="date"
// +kubebuilder:printcolumn:name="Ready",JSONPath=".status.conditions[?(@.type=='Ready')].status",type="string",description="When 'True', the most recent reconcile of the resource succeeded"
// +kubebuilder:printcolumn:name="Status",JSONPath=".status.conditions[?(@.type=='Ready')].reason",type="string",description="The reason for the value in 'Ready'"
// +kubebuilder:printcolumn:name="Status Age",JSONPath=".status.conditions[?(@.type=='Ready')].lastTransitionTime",type="date",description="The last transition time for the value in 'Status'"

// ComputeVPNTunnel is the Schema for the ComputeVPNTunnel API
// +k8s:openapi-gen=true
type ComputeVPNTunnel struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +required
	Spec   ComputeVPNTunnelSpec   `json:"spec,omitempty"`
	Status ComputeVPNTunnelStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// ComputeVPNTunnelList contains a list of ComputeVPNTunnel
type ComputeVPNTunnelList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ComputeVPNTunnel `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ComputeVPNTunnel{}, &ComputeVPNTunnelList{})
}
