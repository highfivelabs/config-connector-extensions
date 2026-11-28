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

// Package v1alpha1 holds types the generated kinds share; it is no API
// group of its own.
//
// +kubebuilder:object:generate=true
package v1alpha1

// ResourceRef is a reference a generated kind's spec takes beside a string
// field the API fills from another resource (hack/discokind kinds.yaml
// refs, docs/design.md section 2.4): Config Connector's shape, either
// external, or the name (and namespace) of an object in the cluster. The
// controller resolves it to the string before every write.
type ResourceRef struct {
	// The value itself, as the string field beside the reference takes it:
	// the resource's name, or for a service account its email.
	// +optional
	External string `json:"external,omitempty"`

	// The name of an object of the referenced kind: this repository's
	// kind of that name first, then Config Connector's. The controller
	// takes the value from the object's status.
	// +optional
	Name string `json:"name,omitempty"`

	// The object's namespace; the referencing object's when unset.
	// +optional
	Namespace string `json:"namespace,omitempty"`
}
