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

// Written here, in the shape of Config Connector's identities.

import (
	"context"
	"fmt"
	"strings"

	"github.com/GoogleCloudPlatform/k8s-config-connector/apis/common"
	"github.com/GoogleCloudPlatform/k8s-config-connector/apis/common/identity"
	refs "github.com/GoogleCloudPlatform/k8s-config-connector/apis/refs/v1beta1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// IAMWorkloadIdentityPoolProviderGVK is Config Connector's (DCL-based)
// workload identity pool provider.
var IAMWorkloadIdentityPoolProviderGVK = schema.GroupVersionKind{Group: "iam.cnrm.cloud.google.com", Version: "v1beta1", Kind: "IAMWorkloadIdentityPoolProvider"}

// IAMWorkloadIdentityPoolProviderRef names a workload identity pool provider.
type IAMWorkloadIdentityPoolProviderRef struct {
	// An existing provider, as
	//  "projects/{{projectID}}/locations/{{location}}/workloadIdentityPools/{{poolID}}/providers/{{providerID}}".
	External string `json:"external,omitempty"`

	// The name of a Config Connector IAMWorkloadIdentityPoolProvider.
	Name string `json:"name,omitempty"`

	// The namespace of that IAMWorkloadIdentityPoolProvider; the referring object's by default.
	Namespace string `json:"namespace,omitempty"`
}

var _ identity.Identity = &IAMWorkloadIdentityPoolProviderKeyIdentity{}

// IAMWorkloadIdentityPoolProviderKeyIdentity is
// projects/{project}/locations/{location}/workloadIdentityPools/{pool}/providers/{provider}/keys/{key}.
type IAMWorkloadIdentityPoolProviderKeyIdentity struct {
	Project, Location, Pool, Provider, Key string
}

func (i *IAMWorkloadIdentityPoolProviderKeyIdentity) ParentString() string {
	return fmt.Sprintf("projects/%s/locations/%s/workloadIdentityPools/%s/providers/%s", i.Project, i.Location, i.Pool, i.Provider)
}

func (i *IAMWorkloadIdentityPoolProviderKeyIdentity) String() string {
	return i.ParentString() + "/keys/" + i.Key
}

func (i *IAMWorkloadIdentityPoolProviderKeyIdentity) FromExternal(ref string) error {
	t := strings.Split(strings.TrimPrefix(ref, "/"), "/")
	if len(t) != 10 || t[0] != "projects" || t[2] != "locations" || t[4] != "workloadIdentityPools" || t[6] != "providers" || t[8] != "keys" {
		return fmt.Errorf("format of IAMWorkloadIdentityPoolProviderKey external=%q was not known (use projects/{{projectID}}/locations/{{location}}/workloadIdentityPools/{{poolID}}/providers/{{providerID}}/keys/{{keyID}})", ref)
	}
	*i = IAMWorkloadIdentityPoolProviderKeyIdentity{Project: t[1], Location: t[3], Pool: t[5], Provider: t[7], Key: t[9]}
	return nil
}

// resolveProvider returns the pool and provider IDs of the referenced
// provider, refusing one in another project or location.
func resolveProvider(ctx context.Context, reader client.Reader, obj *IAMWorkloadIdentityPoolProviderKey, projectID, location string) (string, string, error) {
	ref := obj.Spec.WorkloadIdentityPoolProviderRef
	if ref == nil || (ref.External == "" && ref.Name == "") {
		return "", "", fmt.Errorf("spec.workloadIdentityPoolProviderRef is required")
	}
	var project, loc, pool, provider string
	if ref.External != "" {
		t := strings.Split(strings.TrimPrefix(ref.External, "/"), "/")
		if len(t) != 8 || t[0] != "projects" || t[2] != "locations" || t[4] != "workloadIdentityPools" || t[6] != "providers" {
			return "", "", fmt.Errorf("format of workloadIdentityPoolProviderRef external=%q was not known (use projects/{{projectID}}/locations/{{location}}/workloadIdentityPools/{{poolID}}/providers/{{providerID}})", ref.External)
		}
		project, loc, pool, provider = t[1], t[3], t[5], t[7]
	} else {
		key := types.NamespacedName{Namespace: ref.Namespace, Name: ref.Name}
		if key.Namespace == "" {
			key.Namespace = obj.GetNamespace()
		}
		p := &unstructured.Unstructured{}
		p.SetGroupVersionKind(IAMWorkloadIdentityPoolProviderGVK)
		if err := reader.Get(ctx, key, p); err != nil {
			if apierrors.IsNotFound(err) {
				return "", "", fmt.Errorf("referenced IAMWorkloadIdentityPoolProvider %v not found", key)
			}
			return "", "", fmt.Errorf("reading referenced IAMWorkloadIdentityPoolProvider %v: %w", key, err)
		}
		var err error
		if provider, err = refs.GetResourceID(p); err != nil {
			return "", "", err
		}
		if project, err = refs.ResolveProjectID(ctx, reader, p); err != nil {
			return "", "", err
		}
		if loc, err = refs.GetLocation(p); err != nil {
			return "", "", err
		}
		// The provider's own pool, by name or external, as its spec has it.
		poolRef, _, _ := unstructured.NestedStringMap(p.Object, "spec", "workloadIdentityPoolRef")
		switch {
		case poolRef["external"] != "":
			pool = poolRef["external"][strings.LastIndex(poolRef["external"], "/")+1:]
		case poolRef["name"] != "":
			ns := poolRef["namespace"]
			if ns == "" {
				ns = p.GetNamespace()
			}
			pu := &unstructured.Unstructured{}
			pu.SetGroupVersionKind(IAMWorkloadIdentityPoolGVK)
			if err := reader.Get(ctx, types.NamespacedName{Namespace: ns, Name: poolRef["name"]}, pu); err != nil {
				return "", "", fmt.Errorf("reading the pool of IAMWorkloadIdentityPoolProvider %v: %w", key, err)
			}
			if pool, err = refs.GetResourceID(pu); err != nil {
				return "", "", err
			}
		default:
			return "", "", fmt.Errorf("IAMWorkloadIdentityPoolProvider %v has no spec.workloadIdentityPoolRef", key)
		}
	}
	if project != projectID || loc != location {
		return "", "", fmt.Errorf("workloadIdentityPoolProviderRef is in projects/%s/locations/%s, but the key is in projects/%s/locations/%s", project, loc, projectID, location)
	}
	return pool, provider, nil
}

func (obj *IAMWorkloadIdentityPoolProviderKey) GetIdentity(ctx context.Context, reader client.Reader) (identity.Identity, error) {
	keyID, err := refs.GetResourceID(obj)
	if err != nil {
		return nil, err
	}
	projectID, err := refs.ResolveProjectID(ctx, reader, obj)
	if err != nil {
		return nil, fmt.Errorf("cannot resolve project: %w", err)
	}
	location, err := refs.GetLocation(obj)
	if err != nil {
		return nil, err
	}
	pool, provider, err := resolveProvider(ctx, reader, obj, projectID, location)
	if err != nil {
		return nil, err
	}
	id := &IAMWorkloadIdentityPoolProviderKeyIdentity{Project: projectID, Location: location, Pool: pool, Provider: provider, Key: keyID}
	if externalRef := common.ValueOf(obj.Status.ExternalRef); externalRef != "" && externalRef != id.String() {
		return nil, fmt.Errorf("cannot change IAMWorkloadIdentityPoolProviderKey identity (old=%q, new=%q)", externalRef, id.String())
	}
	return id, nil
}
