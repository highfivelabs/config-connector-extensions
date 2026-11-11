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
// apis/dns/v1alpha1/dnsresponsepolicy_types.go (UPSTREAM.md) as a stable kind
// (v1beta1, docs/design.md decision 13). Changed: projectRef and resourceID
// are immutable by CEL rules (decision 20; upstream says so only in the
// description); status.externalRef, which upstream's status lacks; the
// object's metadata.labels are the policy's labels (the controller; upstream
// leaves them out, docs/design.md decision 19); group, labels and short names
// are ours.

package v1beta1

import (
	refs "github.com/GoogleCloudPlatform/k8s-config-connector/apis/refs"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/apis/k8s/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var DNSResponsePolicyGVK = GroupVersion.WithKind("DNSResponsePolicy")

// +openapi:ResponsePolicyGKECluster
type ResponsepolicyGkeClusters struct {
	/* The resource name of the cluster to bind this ManagedZone to.
	This should be specified in the format like
	'projects/* /locations/* /clusters/*'. */
	GkeClusterName string `json:"gkeClusterName"`
}

// +openapi:ResponsePolicyNetwork
type ResponsepolicyNetworks struct {
	/* The fully qualified URL of the VPC network to bind to.
	This should be formatted like
	'https://www.googleapis.com/compute/v1/projects/{project}/global/networks/{network}'. */
	NetworkUrl string `json:"networkUrl"`
}

// +openapi:ResponsePolicy
type DNSResponsePolicySpec struct {
	/* The description of the response policy, such as 'My new response policy'. */
	// +optional
	Description *string `json:"description,omitempty"`

	/* The list of Google Kubernetes Engine clusters that can see this zone. */
	// +optional
	GkeClusters []ResponsepolicyGkeClusters `json:"gkeClusters,omitempty"`

	/* The list of network names specifying networks to which this policy is applied. */
	// +optional
	Networks []ResponsepolicyNetworks `json:"networks,omitempty"`

	/* The project that this resource belongs to. Immutable. */
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="projectRef is immutable"
	ProjectRef refs.ProjectRef `json:"projectRef"`

	/* Immutable. Optional. The responsePolicyName of the resource. Used for creation and acquisition. When unset, the value of `metadata.name` is used as the default. */
	// +optional
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="resourceID is immutable"
	ResourceID *string `json:"resourceID,omitempty"`
}

type DNSResponsePolicyStatus struct {
	/* Conditions represent the latest available observations of the
	   DNSResponsePolicy's current state. */
	Conditions []v1alpha1.Condition `json:"conditions,omitempty"`
	/* ObservedGeneration is the generation of the resource that was most recently observed by the Config Connector controller. If this is equal to metadata.generation, then that means that the current reported status reflects the most recent desired state of the resource. */
	// +optional
	ObservedGeneration *int64 `json:"observedGeneration,omitempty"`

	/* A unique specifier for the DNSResponsePolicy resource in GCP,
	   projects/{project}/locations/global/responsePolicies/{responsePolicy}. */
	// +optional
	ExternalRef *string `json:"externalRef,omitempty"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:categories=gcp;hfl,shortName=hfldnsresponsepolicy;hfldnsresponsepolicies
// +kubebuilder:subresource:status
// +kubebuilder:metadata:labels="gcp.hfl.dev/gap=true"
// +kubebuilder:printcolumn:name="Age",JSONPath=".metadata.creationTimestamp",type="date"
// +kubebuilder:printcolumn:name="Ready",JSONPath=".status.conditions[?(@.type=='Ready')].status",type="string",description="When 'True', the most recent reconcile of the resource succeeded"
// +kubebuilder:printcolumn:name="Status",JSONPath=".status.conditions[?(@.type=='Ready')].reason",type="string",description="The reason for the value in 'Ready'"
// +kubebuilder:printcolumn:name="Status Age",JSONPath=".status.conditions[?(@.type=='Ready')].lastTransitionTime",type="date",description="The last transition time for the value in 'Status'"

// DNSResponsePolicy is the Schema for the dns API
// +k8s:openapi-gen=true
type DNSResponsePolicy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   DNSResponsePolicySpec   `json:"spec"`
	Status DNSResponsePolicyStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// DNSResponsePolicyList contains a list of DNSResponsePolicy
type DNSResponsePolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []DNSResponsePolicy `json:"items"`
}

func init() {
	SchemeBuilder.Register(&DNSResponsePolicy{}, &DNSResponsePolicyList{})
}
