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

// Written here: a reference to Config Connector's IAMWorkloadIdentityPool
// (iam.cnrm.cloud.google.com/v1beta1, a DCL-based kind with no reference
// type of its own), for the pool a namespace is in. Upstream pull request
// #14209 has no field for the pool.

import (
	"context"
	"fmt"
	"strings"

	refs "github.com/GoogleCloudPlatform/k8s-config-connector/apis/refs/v1beta1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// IAMWorkloadIdentityPoolGVK is Config Connector's workload identity pool.
var IAMWorkloadIdentityPoolGVK = schema.GroupVersionKind{Group: "iam.cnrm.cloud.google.com", Version: "v1beta1", Kind: "IAMWorkloadIdentityPool"}

// IAMWorkloadIdentityPoolRef names a workload identity pool.
type IAMWorkloadIdentityPoolRef struct {
	// An existing pool, as "projects/{{projectID}}/locations/{{location}}/workloadIdentityPools/{{poolID}}".
	External string `json:"external,omitempty"`

	// The name of a Config Connector IAMWorkloadIdentityPool.
	Name string `json:"name,omitempty"`

	// The namespace of that IAMWorkloadIdentityPool; the referring object's by default.
	Namespace string `json:"namespace,omitempty"`
}

// resolveWorkloadIdentityPool returns the pool ID the namespace is in, and
// refuses a pool in another project or location than the namespace's
// spec.projectRef and spec.location.
func resolveWorkloadIdentityPool(ctx context.Context, reader client.Reader, obj *IAMWorkloadIdentityPoolNamespace, projectID, location string) (string, error) {
	ref := obj.Spec.WorkloadIdentityPoolRef
	if ref == nil || (ref.External == "" && ref.Name == "") {
		return "", fmt.Errorf("spec.workloadIdentityPoolRef is required")
	}
	var poolProject, poolLocation, poolID string
	if ref.External != "" {
		tokens := strings.Split(strings.TrimPrefix(ref.External, "/"), "/")
		if len(tokens) != 6 || tokens[0] != "projects" || tokens[2] != "locations" || tokens[4] != "workloadIdentityPools" {
			return "", fmt.Errorf("format of workloadIdentityPoolRef external=%q was not known (use projects/{{projectID}}/locations/{{location}}/workloadIdentityPools/{{poolID}})", ref.External)
		}
		poolProject, poolLocation, poolID = tokens[1], tokens[3], tokens[5]
	} else {
		key := types.NamespacedName{Namespace: ref.Namespace, Name: ref.Name}
		if key.Namespace == "" {
			key.Namespace = obj.GetNamespace()
		}
		pool := &unstructured.Unstructured{}
		pool.SetGroupVersionKind(IAMWorkloadIdentityPoolGVK)
		if err := reader.Get(ctx, key, pool); err != nil {
			if apierrors.IsNotFound(err) {
				return "", fmt.Errorf("referenced IAMWorkloadIdentityPool %v not found", key)
			}
			return "", fmt.Errorf("reading referenced IAMWorkloadIdentityPool %v: %w", key, err)
		}
		var err error
		if poolID, err = refs.GetResourceID(pool); err != nil {
			return "", err
		}
		if poolProject, err = refs.ResolveProjectID(ctx, reader, pool); err != nil {
			return "", err
		}
		if poolLocation, err = refs.GetLocation(pool); err != nil {
			return "", err
		}
	}
	if poolProject != projectID || poolLocation != location {
		return "", fmt.Errorf("workloadIdentityPoolRef is in projects/%s/locations/%s, but the namespace is in projects/%s/locations/%s", poolProject, poolLocation, projectID, location)
	}
	return poolID, nil
}

// resolveManagedIdentityNamespace returns the namespace a managed identity is
// in, from spec.workloadIdentityPoolNamespaceRef (an
// IAMWorkloadIdentityPoolNamespace of ours by name, once it has a
// status.externalRef, or a namespace as external), and refuses one in
// another project or location than the managed identity's.
func resolveManagedIdentityNamespace(ctx context.Context, reader client.Reader, obj *IAMWorkloadIdentityPoolManagedIdentity, projectID, location string) (*IAMWorkloadIdentityPoolNamespaceIdentity, error) {
	ref := obj.Spec.WorkloadIdentityPoolNamespaceRef
	if ref == nil || (ref.External == "" && ref.Name == "") {
		return nil, fmt.Errorf("spec.workloadIdentityPoolNamespaceRef is required")
	}
	if err := ref.Normalize(ctx, reader, obj.GetNamespace()); err != nil {
		return nil, err
	}
	ns := &IAMWorkloadIdentityPoolNamespaceIdentity{}
	if err := ns.FromExternal(ref.External); err != nil {
		return nil, err
	}
	if ns.Project != projectID || ns.Location != location {
		return nil, fmt.Errorf("workloadIdentityPoolNamespaceRef is in projects/%s/locations/%s, but the managed identity is in projects/%s/locations/%s", ns.Project, ns.Location, projectID, location)
	}
	return ns, nil
}
