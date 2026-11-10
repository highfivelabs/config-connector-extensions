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
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ConfigConnectorComputeRouterGVK is Config Connector's Cloud Router.
var ConfigConnectorComputeRouterGVK = schema.GroupVersionKind{Group: "compute.cnrm.cloud.google.com", Version: "v1beta1", Kind: "ComputeRouter"}

// ComputeRouterRef names a Cloud Router.
type ComputeRouterRef struct {
	// An existing router, as "projects/{{projectID}}/regions/{{region}}/routers/{{name}}".
	External string `json:"external,omitempty"`

	// The name of a ComputeRouter: this project's (compute.gcp.hfl.dev) or
	// Config Connector's (compute.cnrm.cloud.google.com), whichever exists
	// in the namespace; with both, the reference is refused (use external).
	Name string `json:"name,omitempty"`

	// The namespace of that ComputeRouter; the referring object's by default.
	Namespace string `json:"namespace,omitempty"`
}

// referencedRouter reads the ComputeRouter a reference names: ours or
// Config Connector's, never both (a router moving to ours has both objects
// for a while; the reference must not pick one silently).
func referencedRouter(ctx context.Context, reader client.Reader, key types.NamespacedName) (*unstructured.Unstructured, error) {
	var found []*unstructured.Unstructured
	for _, gvk := range []schema.GroupVersionKind{ComputeRouterGVK, ConfigConnectorComputeRouterGVK} {
		u := &unstructured.Unstructured{}
		u.SetGroupVersionKind(gvk)
		if err := reader.Get(ctx, key, u); err != nil {
			if apierrors.IsNotFound(err) || meta.IsNoMatchError(err) {
				continue
			}
			return nil, fmt.Errorf("reading referenced %s %v: %w", gvk.GroupKind(), key, err)
		}
		found = append(found, u)
	}
	switch len(found) {
	case 0:
		return nil, fmt.Errorf("referenced ComputeRouter %v not found", key)
	case 1:
		return found[0], nil
	default:
		return nil, fmt.Errorf("referenced ComputeRouter %v is both a %s and a %s object; name the router by routerRef.external", key, ComputeRouterGVK.Group, ConfigConnectorComputeRouterGVK.Group)
	}
}

// Resolve is the project, region and name of the router the reference
// names: from external, or from the ComputeRouter object (ours or Config
// Connector's, see referencedRouter) in defaultNamespace unless the
// reference names a namespace.
func (ref *ComputeRouterRef) Resolve(ctx context.Context, reader client.Reader, defaultNamespace string) (project, region, router string, err error) {
	if ref.External != "" {
		t := strings.Split(strings.TrimPrefix(ref.External, "/"), "/")
		if len(t) != 6 || t[0] != "projects" || t[2] != "regions" || t[4] != "routers" {
			return "", "", "", fmt.Errorf("format of routerRef external=%q was not known (use projects/{{projectID}}/regions/{{region}}/routers/{{name}})", ref.External)
		}
		return t[1], t[3], t[5], nil
	}
	if ref.Name == "" {
		return "", "", "", fmt.Errorf("routerRef needs external or name")
	}
	key := types.NamespacedName{Namespace: ref.Namespace, Name: ref.Name}
	if key.Namespace == "" {
		key.Namespace = defaultNamespace
	}
	u, err := referencedRouter(ctx, reader, key)
	if err != nil {
		return "", "", "", err
	}
	if router, err = refs.GetResourceID(u); err != nil {
		return "", "", "", err
	}
	if project, err = refs.ResolveProjectID(ctx, reader, u); err != nil {
		return "", "", "", err
	}
	region, _, _ = unstructured.NestedString(u.Object, "spec", "region")
	return project, region, router, nil
}

var _ identity.Identity = &ComputeRouterRoutePolicyIdentity{}

// ComputeRouterRoutePolicyIdentity is
// projects/{project}/regions/{region}/routers/{router}/routePolicies/{policy}.
type ComputeRouterRoutePolicyIdentity struct {
	Project, Region, Router, Policy string
}

func (i *ComputeRouterRoutePolicyIdentity) String() string {
	return fmt.Sprintf("projects/%s/regions/%s/routers/%s/routePolicies/%s", i.Project, i.Region, i.Router, i.Policy)
}

func (i *ComputeRouterRoutePolicyIdentity) FromExternal(ref string) error {
	t := strings.Split(strings.TrimPrefix(ref, "/"), "/")
	if len(t) != 8 || t[0] != "projects" || t[2] != "regions" || t[4] != "routers" || t[6] != "routePolicies" {
		return fmt.Errorf("format of ComputeRouterRoutePolicy external=%q was not known (use projects/{{projectID}}/regions/{{region}}/routers/{{router}}/routePolicies/{{name}})", ref)
	}
	*i = ComputeRouterRoutePolicyIdentity{Project: t[1], Region: t[3], Router: t[5], Policy: t[7]}
	return nil
}

func (obj *ComputeRouterRoutePolicy) GetIdentity(ctx context.Context, reader client.Reader) (identity.Identity, error) {
	projectID, err := refs.ResolveProjectID(ctx, reader, obj)
	if err != nil {
		return nil, fmt.Errorf("cannot resolve project: %w", err)
	}
	policy, err := refs.GetResourceID(obj)
	if err != nil {
		return nil, err
	}
	ref := obj.Spec.RouterRef
	if ref == nil || (ref.External == "" && ref.Name == "") {
		return nil, fmt.Errorf("spec.routerRef is required")
	}
	project, region, router, err := ref.Resolve(ctx, reader, obj.GetNamespace())
	if err != nil {
		return nil, err
	}
	if project != projectID || region != obj.Spec.Region {
		return nil, fmt.Errorf("routerRef is in projects/%s/regions/%s, but the policy is in projects/%s/regions/%s", project, region, projectID, obj.Spec.Region)
	}
	id := &ComputeRouterRoutePolicyIdentity{Project: projectID, Region: region, Router: router, Policy: policy}
	if externalRef := common.ValueOf(obj.Status.ExternalRef); externalRef != "" && externalRef != id.String() {
		return nil, fmt.Errorf("cannot change ComputeRouterRoutePolicy identity (old=%q, new=%q)", externalRef, id.String())
	}
	return id, nil
}
