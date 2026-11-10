// Copyright 2026 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// SPDX-License-Identifier: Apache-2.0

// Modified by High Five Labs for config-connector-extensions: copied from Config Connector
// apis/compute/v1beta1/computerouter_types.go (UPSTREAM.md; PR #13960 leaves
// the types as they are). Our ComputeRouter owns the whole router
// (docs/design.md decision 14): written here are bgp.identifierRange and the
// four lists upstream's direct kind leaves to the Terraform-based
// ComputeRouterInterface, ComputeRouterPeer and ComputeRouterNAT:
//   - md5AuthenticationKeys, whose values come from Secrets only;
//   - interfaces, the fields of upstream's ComputeRouterInterfaceSpec (its
//     reference types) without the router, region and resource ID, plus
//     ipVersion; privateIpAddress and redundantInterface are the API's
//     strings (an IP address and an interface name on this router);
//   - bgpPeers, the API's RouterBgpPeer (the field set controllerbuilder
//     generated) named as Config Connector's ComputeRouterPeer names them,
//     with Terraform's google_compute_router_peer as the behaviour;
//   - nats, the fields of upstream's ComputeRouterNATSpec (its types)
//     without the router, region and resource ID, plus name,
//     autoNetworkTier, nat64Subnetworks and sourceSubnetworkIpRangesToNat64.
// Group, labels and short names are ours.

package v1alpha1

import (
	computerefs "github.com/GoogleCloudPlatform/k8s-config-connector/apis/compute/refs"
	computev1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/apis/compute/v1beta1"
	"github.com/GoogleCloudPlatform/k8s-config-connector/apis/refs/v1beta1/secret"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/apis/k8s/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var ComputeRouterGVK = GroupVersion.WithKind("ComputeRouter")

// +kcc:proto=google.cloud.compute.v1.RouterAdvertisedIpRange
type RouterAdvertisedIPRange struct {
	/* User-specified description for the IP range. */
	// +optional
	Description *string `json:"description,omitempty"`

	/* The IP range to advertise. The value must be a
	CIDR-formatted string. */
	// +required
	Range string `json:"range"`
}

// +kcc:proto=google.cloud.compute.v1.RouterBgp
type RouterBGP struct {
	/* User-specified flag to indicate which mode to use for advertisement. Default value: "DEFAULT" Possible values: ["DEFAULT", "CUSTOM"]. */
	// +optional
	AdvertiseMode *string `json:"advertiseMode,omitempty"`

	/* User-specified list of prefix groups to advertise in custom mode.
	This field can only be populated if advertiseMode is CUSTOM and
	is advertised to all peers of the router. These groups will be
	advertised in addition to any specified prefixes. Leave this field
	blank to advertise no custom groups.

	This enum field has the one valid value: ALL_SUBNETS. */
	// +optional
	AdvertisedGroups []string `json:"advertisedGroups,omitempty"`

	/* User-specified list of individual IP ranges to advertise in
	custom mode. This field can only be populated if advertiseMode
	is CUSTOM and is advertised to all peers of the router. These IP
	ranges will be advertised in addition to any specified groups.
	Leave this field blank to advertise no custom IP ranges. */
	// +optional
	AdvertisedIPRanges []RouterAdvertisedIPRange `json:"advertisedIpRanges,omitempty"`

	/* Local BGP Autonomous System Number (ASN). Must be an RFC6996
	private ASN, either 16-bit or 32-bit. The value will be fixed for
	this router resource. All VPN tunnels that link to this router
	will have the same local ASN. */
	// +required
	Asn int64 `json:"asn"`

	/* Explicitly specifies a range of valid BGP Identifiers for this
	Router. It is provided as a link-local IPv4 range (from
	169.254.0.0/16), of size at least /30, even if the BGP sessions are
	over IPv6. It must not overlap with any IPv4 BGP session ranges.
	Other vendors commonly call this router ID. The API chooses one when
	it is not set. */
	// +optional
	IdentifierRange *string `json:"identifierRange,omitempty"`

	/* The interval in seconds between BGP keepalive messages that are sent
	to the peer. Hold time is three times the interval at which keepalive
	messages are sent, and the hold time is the maximum number of seconds
	allowed to elapse between successive keepalive messages that BGP
	receives from a peer.

	BGP will use the smaller of either the local hold time value or the
	peer's hold time value as the hold time for the BGP connection
	between the two peers. If set, this value must be between 20 and 60.
	The default is 20. */
	// +optional
	KeepaliveInterval *int64 `json:"keepaliveInterval,omitempty"`
}

// RouterMD5AuthenticationKey is a key a BGP peer authenticates with. Its
// value is read from a Secret in the router's namespace and never kept in
// the spec or the status; the API does not return it.
// +kcc:proto=google.cloud.compute.v1.RouterMd5AuthenticationKey
type RouterMD5AuthenticationKey struct {
	/* Name used to identify the key. Must be unique within a router. Must
	be referenced by exactly one bgpPeer (its md5AuthenticationKeyName).
	Must comply with RFC1035. */
	// +required
	Name string `json:"name"`

	/* The Secret key that holds the value of the key used for MD5
	authentication. */
	// +required
	KeyRef secret.LegacyValueFromSecretKeyRef `json:"keyRef"`
}

// +kcc:proto=google.cloud.compute.v1.RouterInterface
type RouterInterface struct {
	/* Name of this interface entry. The name must be 1-63 characters long
	and comply with RFC1035. Immutable. */
	// +required
	Name string `json:"name"`

	/* The interconnect attachment (VLAN attachment) this interface is
	linked to. Only one of vpnTunnelRef, interconnectAttachmentRef and
	subnetworkRef can be set. Immutable. */
	// +optional
	InterconnectAttachmentRef *computev1beta1.ComputeInterconnectAttachmentRef `json:"interconnectAttachmentRef,omitempty"`

	/* IP address and range of the interface. The IP range must be in the
	RFC3927 link-local IP space. Changing this forces a new interface to
	be created. Immutable. */
	// +optional
	IPRange *string `json:"ipRange,omitempty"`

	/* IP version of this interface: IPV4 or IPV6. The API sets IPV4 when
	it is not given. Immutable. */
	// +optional
	IPVersion *string `json:"ipVersion,omitempty"`

	/* The regional private internal IP address that is used to establish
	BGP sessions to a VM instance acting as a third-party Router
	Appliance. Immutable. */
	// +optional
	PrivateIPAddress *string `json:"privateIpAddress,omitempty"`

	/* The name of another interface on this router that is redundant
	with this one (a Router Appliance interface pair). Immutable. */
	// +optional
	RedundantInterface *string `json:"redundantInterface,omitempty"`

	/* The subnetwork of a Router Appliance this interface is in. Only one
	of vpnTunnelRef, interconnectAttachmentRef and subnetworkRef can be
	set. Immutable. */
	// +optional
	SubnetworkRef *computev1beta1.ComputeSubnetworkRef `json:"subnetworkRef,omitempty"`

	/* The VPN tunnel this interface is linked to. Only one of
	vpnTunnelRef, interconnectAttachmentRef and subnetworkRef can be set.
	Immutable. */
	// +optional
	VPNTunnelRef *computev1beta1.ComputeVPNTunnelRef `json:"vpnTunnelRef,omitempty"`
}

// +kcc:proto=google.cloud.compute.v1.RouterBgpPeer
type RouterBGPPeer struct {
	/* Name of this BGP peer. The name must be 1-63 characters long and
	comply with RFC1035. Immutable. */
	// +required
	Name string `json:"name"`

	/* Name of the interface the BGP peer is associated with. Immutable. */
	// +required
	InterfaceName string `json:"interfaceName"`

	/* Peer BGP Autonomous System Number (ASN). Each BGP interface may use
	a different value. */
	// +required
	PeerAsn int64 `json:"peerAsn"`

	/* User-specified flag to indicate which mode to use for advertisement:
	DEFAULT or CUSTOM. */
	// +optional
	AdvertiseMode *string `json:"advertiseMode,omitempty"`

	/* User-specified list of prefix groups to advertise in custom mode,
	which currently supports ALL_SUBNETS. This field can only be set if
	advertiseMode is CUSTOM; it overrides the router's list. */
	// +optional
	AdvertisedGroups []string `json:"advertisedGroups,omitempty"`

	/* User-specified list of individual IP ranges to advertise in custom
	mode. This field can only be set if advertiseMode is CUSTOM; it
	overrides the router's list. */
	// +optional
	AdvertisedIPRanges []RouterAdvertisedIPRange `json:"advertisedIpRanges,omitempty"`

	/* The priority of routes advertised to this BGP peer. Where there is
	more than one matching route of maximum length, the routes with the
	lowest priority value win. */
	// +optional
	AdvertisedRoutePriority *int64 `json:"advertisedRoutePriority,omitempty"`

	/* BFD configuration for the BGP peering. */
	// +optional
	Bfd *RouterBGPPeerBfd `json:"bfd,omitempty"`

	/* The custom learned route IP address ranges for this BGP session. */
	// +optional
	CustomLearnedIPRanges []RouterBGPPeerCustomLearnedIPRange `json:"customLearnedIpRanges,omitempty"`

	/* The priority of the custom learned routes, 0 to 65335. The API uses
	100 when it is not given. */
	// +optional
	CustomLearnedRoutePriority *int64 `json:"customLearnedRoutePriority,omitempty"`

	/* The status of the BGP peer connection. If false, any active session
	with the peer is terminated and all associated routing information is
	removed. If true, the peer connection can be established with routing
	information. The default is true. */
	// +optional
	Enable *bool `json:"enable,omitempty"`

	/* Enable IPv4 traffic over BGP Peer. It is enabled by default if the
	peerIpAddress is version 4. */
	// +optional
	EnableIPv4 *bool `json:"enableIpv4,omitempty"`

	/* Enable IPv6 traffic over BGP Peer. If not specified, it is disabled
	by default. */
	// +optional
	EnableIPv6 *bool `json:"enableIpv6,omitempty"`

	/* Names of the export route policies of this router applied to this
	peer, in the order they are evaluated. */
	// +optional
	ExportPolicies []string `json:"exportPolicies,omitempty"`

	/* Names of the import route policies of this router applied to this
	peer, in the order they are evaluated. */
	// +optional
	ImportPolicies []string `json:"importPolicies,omitempty"`

	/* IP address of the interface inside Google Cloud Platform. Only IPv4
	is supported. The API chooses it from the interface's range when it
	is not given. */
	// +optional
	IPAddress *string `json:"ipAddress,omitempty"`

	/* IPv4 address of the interface inside Google Cloud Platform. */
	// +optional
	IPv4NexthopAddress *string `json:"ipv4NexthopAddress,omitempty"`

	/* IPv6 address of the interface inside Google Cloud Platform. The
	address must be in the range 2600:2d00:0:2::/64 or
	2600:2d00:0:3::/64. */
	// +optional
	IPv6NexthopAddress *string `json:"ipv6NexthopAddress,omitempty"`

	/* The name of one of this router's md5AuthenticationKeys; present if
	MD5 authentication is enabled for the peering. */
	// +optional
	MD5AuthenticationKeyName *string `json:"md5AuthenticationKeyName,omitempty"`

	/* IP address of the BGP interface outside Google Cloud Platform. Only
	IPv4 is supported. */
	// +optional
	PeerIPAddress *string `json:"peerIpAddress,omitempty"`

	/* IPv4 address of the BGP interface outside Google Cloud Platform. */
	// +optional
	PeerIPv4NexthopAddress *string `json:"peerIpv4NexthopAddress,omitempty"`

	/* IPv6 address of the BGP interface outside Google Cloud Platform. */
	// +optional
	PeerIPv6NexthopAddress *string `json:"peerIpv6NexthopAddress,omitempty"`

	/* The VM instance that is the BGP peer, for a Router Appliance; its
	subnetwork must be the one of the interface this peer is on. */
	// +optional
	RouterApplianceInstanceRef *computev1beta1.InstanceRef `json:"routerApplianceInstanceRef,omitempty"`
}

// RouterNAT is a Cloud NAT gateway on the router. The fields are those of
// upstream's ComputeRouterNATSpec, with the NAT's name in place of its
// resource ID, and the router and region the router's own.
// +kcc:proto=google.cloud.compute.v1.RouterNat
type RouterNAT struct {
	/* Name of the NAT service. The name must be 1-63 characters long and
	comply with RFC1035. Immutable. */
	// +required
	Name string `json:"name"`

	/* The network tier to use when automatically reserving NAT IP
	addresses: PREMIUM or STANDARD. Must be one of the network tiers the
	project allows (the project's default by default). */
	// +optional
	AutoNetworkTier *string `json:"autoNetworkTier,omitempty"`

	/* A list of URLs of the IP resources to be drained. These IPs must be
	valid static external IPs that have been assigned to the NAT. */
	// +optional
	DrainNatIps []computev1beta1.ComputeAddressRef `json:"drainNatIps,omitempty"`

	/* Enable Dynamic Port Allocation. If minPortsPerVm is set, it must be
	a power of two between 32 and 32768 (default 32). If maxPortsPerVm is
	set, it must be a power of two greater than minPortsPerVm (default
	65536). Mutually exclusive with enableEndpointIndependentMapping. */
	// +optional
	EnableDynamicPortAllocation *bool `json:"enableDynamicPortAllocation,omitempty"`

	/* Enable endpoint independent mapping. For more information see the
	[official documentation](https://cloud.google.com/nat/docs/overview#specs-rfcs). */
	// +optional
	EnableEndpointIndependentMapping *bool `json:"enableEndpointIndependentMapping,omitempty"`

	/* Specifies the endpoint types supported by the NAT gateway:
	ENDPOINT_TYPE_VM, ENDPOINT_TYPE_SWG, ENDPOINT_TYPE_MANAGED_PROXY_LB.
	The API uses ENDPOINT_TYPE_VM when it is not given. Immutable. */
	// +optional
	EndpointTypes []string `json:"endpointTypes,omitempty"`

	/* Timeout (in seconds) for ICMP connections. Defaults to 30s if not set. */
	// +optional
	IcmpIdleTimeoutSec *int64 `json:"icmpIdleTimeoutSec,omitempty"`

	/* Configuration for logging on NAT. */
	// +optional
	LogConfig *computev1beta1.RouternatLogConfig `json:"logConfig,omitempty"`

	/* Maximum number of ports allocated to a VM from this NAT. This is
	only valid if enableDynamicPortAllocation is true. */
	// +optional
	MaxPortsPerVm *int64 `json:"maxPortsPerVm,omitempty"`

	/* Minimum number of ports allocated to a VM from this NAT. Defaults to
	64 for static port allocation and 32 for dynamic port allocation if
	not set. */
	// +optional
	MinPortsPerVm *int64 `json:"minPortsPerVm,omitempty"`

	/* List of subnetworks to which NAT64 is applied, when
	sourceSubnetworkIpRangesToNat64 is LIST_OF_IPV6_SUBNETWORKS. */
	// +optional
	Nat64Subnetworks []RouterNATSubnetworkToNat64 `json:"nat64Subnetworks,omitempty"`

	/* How external IPs should be allocated for this NAT. Valid values are
	'AUTO_ONLY' for only allowing NAT IPs allocated by Google Cloud
	Platform, or 'MANUAL_ONLY' for only user-allocated NAT IP addresses.
	Possible values: ["MANUAL_ONLY", "AUTO_ONLY"]. */
	// +optional
	NatIpAllocateOption *string `json:"natIpAllocateOption,omitempty"`

	/* Self-links of NAT IPs. Only valid if natIpAllocateOption is set to
	MANUAL_ONLY. */
	// +optional
	NatIps []computev1beta1.ComputeAddressRef `json:"natIps,omitempty"`

	/* A list of rules associated with this NAT. */
	// +optional
	Rules []computev1beta1.RouternatRules `json:"rules,omitempty"`

	/* How NAT should be configured per Subnetwork. If
	'ALL_SUBNETWORKS_ALL_IP_RANGES', all of the IP ranges in every
	Subnetwork are allowed to Nat. If
	'ALL_SUBNETWORKS_ALL_PRIMARY_IP_RANGES', all of the primary IP ranges
	in every Subnetwork are allowed to Nat. 'LIST_OF_SUBNETWORKS': A list
	of Subnetworks are allowed to Nat (specified in the field subnetwork
	below). Note that if this field contains
	ALL_SUBNETWORKS_ALL_IP_RANGES or
	ALL_SUBNETWORKS_ALL_PRIMARY_IP_RANGES, then there should not be any
	other RouterNat section in any Router for this network in this
	region. Possible values: ["ALL_SUBNETWORKS_ALL_IP_RANGES",
	"ALL_SUBNETWORKS_ALL_PRIMARY_IP_RANGES", "LIST_OF_SUBNETWORKS"]. */
	// +required
	SourceSubnetworkIpRangesToNat string `json:"sourceSubnetworkIpRangesToNat"`

	/* How NAT64 should be configured: ALL_IPV6_SUBNETWORKS or
	LIST_OF_IPV6_SUBNETWORKS (the subnetworks in nat64Subnetworks). */
	// +optional
	SourceSubnetworkIpRangesToNat64 *string `json:"sourceSubnetworkIpRangesToNat64,omitempty"`

	/* One or more subnetwork NAT configurations. Only used if
	'source_subnetwork_ip_ranges_to_nat' is set to 'LIST_OF_SUBNETWORKS'. */
	// +optional
	Subnetwork []computev1beta1.RouternatSubnetwork `json:"subnetwork,omitempty"`

	/* Timeout (in seconds) for TCP established connections. Defaults to
	1200s if not set. */
	// +optional
	TcpEstablishedIdleTimeoutSec *int64 `json:"tcpEstablishedIdleTimeoutSec,omitempty"`

	/* Timeout (in seconds) for TCP connections that are in TIME_WAIT
	state. Defaults to 120s if not set. */
	// +optional
	TcpTimeWaitTimeoutSec *int64 `json:"tcpTimeWaitTimeoutSec,omitempty"`

	/* Timeout (in seconds) for TCP transitory connections. Defaults to 30s
	if not set. */
	// +optional
	TcpTransitoryIdleTimeoutSec *int64 `json:"tcpTransitoryIdleTimeoutSec,omitempty"`

	/* Indicates whether this NAT is used for public or private IP
	translation. If unspecified, it defaults to PUBLIC. If 'PUBLIC' NAT
	used for public IP translation. If 'PRIVATE' NAT used for private IP
	translation. Default value: "PUBLIC" Possible values: ["PUBLIC",
	"PRIVATE"]. Immutable. */
	// +optional
	Type *string `json:"type,omitempty"`

	/* Timeout (in seconds) for UDP connections. Defaults to 30s if not set. */
	// +optional
	UdpIdleTimeoutSec *int64 `json:"udpIdleTimeoutSec,omitempty"`
}

// ComputeRouterSpec defines the desired state of ComputeRouter
// +kcc:spec:proto=google.cloud.compute.v1.Router
type ComputeRouterSpec struct {
	/* BGP information specific to this router. */
	// +optional
	BGP *RouterBGP `json:"bgp,omitempty"`

	/* BGP peers of this router: the whole list. A peer the spec leaves out
	is removed, but for one the API manages for an interconnect attachment
	(managementType MANAGED_BY_ATTACHMENT). */
	// +optional
	BGPPeers []RouterBGPPeer `json:"bgpPeers,omitempty"`

	/* An optional description of this resource. */
	// +optional
	Description *string `json:"description,omitempty"`

	/* Immutable. Indicates if a router is dedicated for use with encrypted VLAN
	attachments (interconnectAttachments). */
	// +optional
	EncryptedInterconnectRouter *bool `json:"encryptedInterconnectRouter,omitempty"`

	/* Router interfaces: the whole list. An interface the spec leaves out
	is removed, but for one the API manages for an interconnect attachment
	(managementType MANAGED_BY_ATTACHMENT). */
	// +optional
	Interfaces []RouterInterface `json:"interfaces,omitempty"`

	/* Keys used for MD5 authentication of BGP peers: the whole list. */
	// +optional
	MD5AuthenticationKeys []RouterMD5AuthenticationKey `json:"md5AuthenticationKeys,omitempty"`

	/* Cloud NAT gateways on this router: the whole list. */
	// +optional
	NATs []RouterNAT `json:"nats,omitempty"`

	/* A reference to the network to which this router belongs. */
	// +required
	NetworkRef computerefs.ComputeNetworkRef `json:"networkRef"`

	/* Immutable. Region where the router resides. */
	// +required
	Region string `json:"region"`

	/* Immutable. Optional. The name of the resource. Used for creation and acquisition. When unset, the value of `metadata.name` is used as the default. */
	// +optional
	ResourceID *string `json:"resourceID,omitempty"`
}

// ComputeRouterStatus defines the config connector machine state of ComputeRouter
// +kcc:status:proto=google.cloud.compute.v1.Router
type ComputeRouterStatus struct {
	/* Conditions represent the latest available observations of the
	   ComputeRouter's current state. */
	Conditions []v1alpha1.Condition `json:"conditions,omitempty"`

	/* Creation timestamp in RFC3339 text format. */
	// +optional
	// +kcc:proto:field=google.cloud.compute.v1.Router.creation_timestamp
	CreationTimestamp *string `json:"creationTimestamp,omitempty"`

	/* Which Secret versions the MD5 authentication keys were last written
	from: a digest of each key's name and its Secret's name, key and
	resourceVersion, never of the key itself. The API does not return
	key values, so a changed Secret is noticed by this. */
	// +optional
	MD5AuthenticationKeysVersion *string `json:"md5AuthenticationKeysVersion,omitempty"`

	// ObservedGeneration is the generation of the resource that was most recently observed by the Config Connector controller. If this is equal to metadata.generation, then that means that the current reported status reflects the most recent desired state of the resource.
	// +optional
	ObservedGeneration *int64 `json:"observedGeneration,omitempty"`

	// +optional
	// +kcc:proto:field=google.cloud.compute.v1.Router.self_link
	SelfLink *string `json:"selfLink,omitempty"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:categories=gcp;hfl,shortName=hflcomputerouter;hflcomputerouters
// +kubebuilder:subresource:status
// +kubebuilder:metadata:labels="gcp.hfl.dev/gap=true"
// +kubebuilder:printcolumn:name="Age",JSONPath=".metadata.creationTimestamp",type="date"
// +kubebuilder:printcolumn:name="Ready",JSONPath=".status.conditions[?(@.type=='Ready')].status",type="string",description="When 'True', the most recent reconcile of the resource succeeded"
// +kubebuilder:printcolumn:name="Status",JSONPath=".status.conditions[?(@.type=='Ready')].reason",type="string",description="The reason for the value in 'Ready'"
// +kubebuilder:printcolumn:name="Status Age",JSONPath=".status.conditions[?(@.type=='Ready')].lastTransitionTime",type="date",description="The last transition time for the value in 'Status'"

// ComputeRouter is the Schema for the compute API
// +k8s:openapi-gen=true
type ComputeRouter struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +required
	Spec   ComputeRouterSpec   `json:"spec,omitempty"`
	Status ComputeRouterStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ComputeRouterList contains a list of ComputeRouter
type ComputeRouterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ComputeRouter `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ComputeRouter{}, &ComputeRouterList{})
}
