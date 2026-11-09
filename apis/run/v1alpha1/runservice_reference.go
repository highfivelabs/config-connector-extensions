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
// apis/run/v1alpha1/runworkerpool_reference.go (UPSTREAM.md): a reference to this
// project's RunService (run.gcp.hfl.dev) instead of a worker pool.

package v1alpha1

import (
	"context"

	"github.com/GoogleCloudPlatform/k8s-config-connector/apis/common/identity"
	refsv1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/apis/refs/v1beta1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ refsv1beta1.Ref = &RunServiceRef{}

func init() {
	refsv1beta1.Register(&RunServiceRef{}, &RunService{})
}

// RunServiceRef is a reference to a GCP RunService.
type RunServiceRef struct {
	/* A reference to an externally managed RunService resource.
	   Should be in the format "projects/{{projectID}}/locations/{{location}}/services/{{serviceID}}". */
	External string `json:"external,omitempty"`

	// The name of a RunService resource.
	Name string `json:"name,omitempty"`

	// The namespace of a RunService resource.
	Namespace string `json:"namespace,omitempty"`
}

func (r *RunServiceRef) GetGVK() schema.GroupVersionKind {
	return RunServiceGVK
}

func (r *RunServiceRef) GetNamespacedName() types.NamespacedName {
	return types.NamespacedName{
		Name:      r.Name,
		Namespace: r.Namespace,
	}
}

func (r *RunServiceRef) GetExternal() string {
	return r.External
}

func (r *RunServiceRef) SetExternal(ref string) {
	r.External = ref
	r.Name = ""
	r.Namespace = ""
}

func (r *RunServiceRef) ValidateExternal(ref string) error {
	id := &RunServiceIdentity{}
	if err := id.FromExternal(ref); err != nil {
		return err
	}
	return nil
}

func (r *RunServiceRef) ParseExternalToIdentity() (identity.Identity, error) {
	id := &RunServiceIdentity{}
	if err := id.FromExternal(r.External); err != nil {
		return nil, err
	}
	return id, nil
}

func (r *RunServiceRef) Normalize(ctx context.Context, reader client.Reader, defaultNamespace string) error {
	return refsv1beta1.Normalize(ctx, reader, r, defaultNamespace)
}
