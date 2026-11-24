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

// Written here: a field kind (docs/design.md section 6.6, pattern 2;
// decision 37 names it). Config Connector's DNSPolicy (Terraform, or its
// direct controller) has no dns64Config; this kind owns that field of a
// DNS server policy Config Connector's kind (or anything else) made. The
// field keeps the Cloud DNS API's name (Policy in dns v1).

import (
	dnsv1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/apis/dns/v1beta1"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/apis/k8s/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var DNSPolicyFieldsGVK = GroupVersion.WithKind("DNSPolicyFields")

// DNSPolicyFieldsSpec is the fields of a DNS server policy this object
// owns.
type DNSPolicyFieldsSpec struct {
	// Immutable. The policy: Config Connector's DNSPolicy by name
	// (dns.cnrm.cloud.google.com), or any policy by external, as
	// "projects/{{projectID}}/policies/{{policy}}". The policy must exist;
	// this object never creates or deletes it.
	// +required
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="resourceRef is immutable"
	ResourceRef *dnsv1beta1.DNSPolicyRef `json:"resourceRef"`

	// DNS64 for the policy's networks: synthesized AAAA records, so that
	// IPv6-only workloads reach IPv4-only names through NAT64.
	// +required
	Dns64Config *PolicyDns64Config `json:"dns64Config"`
}

// PolicyDns64Config is Cloud DNS's PolicyDns64Config.
type PolicyDns64Config struct {
	// Which queries DNS64 answers.
	// +required
	Scope *PolicyDns64ConfigScope `json:"scope"`
}

// PolicyDns64ConfigScope is Cloud DNS's PolicyDns64ConfigScope.
type PolicyDns64ConfigScope struct {
	// Answer every query of the policy's networks with DNS64 (true), or
	// none (false).
	// +required
	AllQueries *bool `json:"allQueries"`
}

// DNSPolicyFieldsStatus defines the observed state.
type DNSPolicyFieldsStatus struct {
	/* Conditions represent the latest available observations of the
	   object's current state. */
	Conditions []v1alpha1.Condition `json:"conditions,omitempty"`

	// ObservedGeneration is the generation of the resource that was most recently observed by the Config Connector controller. If this is equal to metadata.generation, then that means that the current reported status reflects the most recent desired state of the resource.
	ObservedGeneration *int64 `json:"observedGeneration,omitempty"`

	// The DNS server policy whose fields this object sets, as
	// projects/{project}/policies/{policy}.
	ExternalRef *string `json:"externalRef,omitempty"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:categories=gcp;hfl,shortName=hfldnspolicyfields
// +kubebuilder:subresource:status
// +kubebuilder:metadata:labels="gcp.hfl.dev/gap=true"
// +kubebuilder:printcolumn:name="Age",JSONPath=".metadata.creationTimestamp",type="date"
// +kubebuilder:printcolumn:name="Ready",JSONPath=".status.conditions[?(@.type=='Ready')].status",type="string",description="When 'True', the most recent reconcile of the resource succeeded"
// +kubebuilder:printcolumn:name="Status",JSONPath=".status.conditions[?(@.type=='Ready')].reason",type="string",description="The reason for the value in 'Ready'"
// +kubebuilder:printcolumn:name="Status Age",JSONPath=".status.conditions[?(@.type=='Ready')].lastTransitionTime",type="date",description="The last transition time for the value in 'Status'"

// DNSPolicyFields sets fields of a Cloud DNS server policy that Config
// Connector's DNSPolicy has no field for (dns64Config), and only those: it
// patches the policy with a body naming them. Deleting it turns DNS64 off.
// +k8s:openapi-gen=true
type DNSPolicyFields struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +required
	Spec   DNSPolicyFieldsSpec   `json:"spec,omitempty"`
	Status DNSPolicyFieldsStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// DNSPolicyFieldsList contains a list of DNSPolicyFields
type DNSPolicyFieldsList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []DNSPolicyFields `json:"items"`
}

func init() {
	SchemeBuilder.Register(&DNSPolicyFields{}, &DNSPolicyFieldsList{})
}
