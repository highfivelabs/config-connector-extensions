// Copyright 2026 Google LLC
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

// SPDX-License-Identifier: Apache-2.0

// Modified by High Five Labs for config-connector-extensions. Copied from Config Connector
// pull request #14209 (head a5e8f5a9), apis/iam/v1alpha1/types.generated.go
// (UPSTREAM.md): only the type the PR adds for IAMWorkloadIdentityPoolNamespace;
// the file's other types belong to kinds this project does not have.

package v1alpha1

// +kcc:observedstate:proto=google.iam.v1.OwnerService
type OwnerServiceObservedState struct {
	// Required. The service agent principal subject, e.g. "serviceAccount:service-1234@gcp-sa-gkehub.iam.gserviceaccount.com".
	// +kcc:proto:field=google.iam.v1.OwnerService.principal_subject
	PrincipalSubject *string `json:"principalSubject,omitempty"`
}
