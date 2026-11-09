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

// Written here, from the Cloud Run v2 API: the scaling settings a service
// and its revisions have, which Config Connector's direct Run kinds do not.

// +kcc:proto=google.cloud.run.v2.ServiceScaling
type ServiceScaling struct {
	// Optional. Total minimum number of instances for the service, across
	//  all its revisions.
	MinInstanceCount *int32 `json:"minInstanceCount,omitempty"`

	// Optional. The scaling mode: AUTOMATIC or MANUAL.
	ScalingMode *string `json:"scalingMode,omitempty"`

	// Optional. Total maximum number of instances for the service.
	MaxInstanceCount *int32 `json:"maxInstanceCount,omitempty"`

	// Optional. Total instance count when the scaling mode is MANUAL.
	ManualInstanceCount *int32 `json:"manualInstanceCount,omitempty"`
}

// +kcc:proto=google.cloud.run.v2.RevisionScaling
type RevisionScaling struct {
	// Optional. Minimum number of serving instances for this revision.
	MinInstanceCount *int32 `json:"minInstanceCount,omitempty"`

	// Optional. Maximum number of serving instances for this revision.
	MaxInstanceCount *int32 `json:"maxInstanceCount,omitempty"`
}
