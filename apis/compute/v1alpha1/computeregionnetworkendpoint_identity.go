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

// ComputeRegionNetworkEndpointGroupGVK is Config Connector's (Terraform-based)
// regional network endpoint group.
var ComputeRegionNetworkEndpointGroupGVK = schema.GroupVersionKind{Group: "compute.cnrm.cloud.google.com", Version: "v1beta1", Kind: "ComputeRegionNetworkEndpointGroup"}

// ComputeRegionNetworkEndpointGroupRef names a regional network endpoint group.
type ComputeRegionNetworkEndpointGroupRef struct {
	// An existing group, as
	//  "projects/{{projectID}}/regions/{{region}}/networkEndpointGroups/{{name}}".
	External string `json:"external,omitempty"`

	// The name of a Config Connector ComputeRegionNetworkEndpointGroup.
	Name string `json:"name,omitempty"`

	// The namespace of that ComputeRegionNetworkEndpointGroup; the referring object's by default.
	Namespace string `json:"namespace,omitempty"`
}

var _ identity.Identity = &ComputeRegionNetworkEndpointIdentity{}

// ComputeRegionNetworkEndpointIdentity is an endpoint in a regional group:
// projects/{project}/regions/{region}/networkEndpointGroups/{group}/networkEndpoints/{address}:{port},
// where the address is the IP address or the domain name.
type ComputeRegionNetworkEndpointIdentity struct {
	Project, Region, Group, Address string
	Port                            int32
}

func (i *ComputeRegionNetworkEndpointIdentity) GroupName() string {
	return fmt.Sprintf("projects/%s/regions/%s/networkEndpointGroups/%s", i.Project, i.Region, i.Group)
}

func (i *ComputeRegionNetworkEndpointIdentity) String() string {
	return fmt.Sprintf("%s/networkEndpoints/%s:%d", i.GroupName(), i.Address, i.Port)
}

func (i *ComputeRegionNetworkEndpointIdentity) FromExternal(ref string) error {
	t := strings.Split(strings.TrimPrefix(ref, "/"), "/")
	if len(t) != 8 || t[0] != "projects" || t[2] != "regions" || t[4] != "networkEndpointGroups" || t[6] != "networkEndpoints" {
		return fmt.Errorf("format of ComputeRegionNetworkEndpoint external=%q was not known", ref)
	}
	j := strings.LastIndex(t[7], ":")
	if j < 0 {
		return fmt.Errorf("format of ComputeRegionNetworkEndpoint external=%q was not known (no port)", ref)
	}
	var port int32
	if _, err := fmt.Sscanf(t[7][j+1:], "%d", &port); err != nil {
		return fmt.Errorf("format of ComputeRegionNetworkEndpoint external=%q was not known: %w", ref, err)
	}
	*i = ComputeRegionNetworkEndpointIdentity{Project: t[1], Region: t[3], Group: t[5], Address: t[7][:j], Port: port}
	return nil
}

// resolveGroup returns the group's name, refusing one in another project or
// region than the endpoint's.
func resolveGroup(ctx context.Context, reader client.Reader, obj *ComputeRegionNetworkEndpoint, projectID string) (string, error) {
	ref := obj.Spec.RegionNetworkEndpointGroupRef
	if ref == nil || (ref.External == "" && ref.Name == "") {
		return "", fmt.Errorf("spec.regionNetworkEndpointGroupRef is required")
	}
	var project, region, group string
	if ref.External != "" {
		t := strings.Split(strings.TrimPrefix(ref.External, "/"), "/")
		if len(t) != 6 || t[0] != "projects" || t[2] != "regions" || t[4] != "networkEndpointGroups" {
			return "", fmt.Errorf("format of regionNetworkEndpointGroupRef external=%q was not known (use projects/{{projectID}}/regions/{{region}}/networkEndpointGroups/{{name}})", ref.External)
		}
		project, region, group = t[1], t[3], t[5]
	} else {
		key := types.NamespacedName{Namespace: ref.Namespace, Name: ref.Name}
		if key.Namespace == "" {
			key.Namespace = obj.GetNamespace()
		}
		u := &unstructured.Unstructured{}
		u.SetGroupVersionKind(ComputeRegionNetworkEndpointGroupGVK)
		if err := reader.Get(ctx, key, u); err != nil {
			if apierrors.IsNotFound(err) {
				return "", fmt.Errorf("referenced ComputeRegionNetworkEndpointGroup %v not found", key)
			}
			return "", fmt.Errorf("reading referenced ComputeRegionNetworkEndpointGroup %v: %w", key, err)
		}
		var err error
		if group, err = refs.GetResourceID(u); err != nil {
			return "", err
		}
		if project, err = refs.ResolveProjectID(ctx, reader, u); err != nil {
			return "", err
		}
		region, _, _ = unstructured.NestedString(u.Object, "spec", "region")
	}
	if project != projectID || region != obj.Spec.Region {
		return "", fmt.Errorf("regionNetworkEndpointGroupRef is in projects/%s/regions/%s, but the endpoint is in projects/%s/regions/%s", project, region, projectID, obj.Spec.Region)
	}
	return group, nil
}

func (obj *ComputeRegionNetworkEndpoint) GetIdentity(ctx context.Context, reader client.Reader) (identity.Identity, error) {
	projectID, err := refs.ResolveProjectID(ctx, reader, obj)
	if err != nil {
		return nil, fmt.Errorf("cannot resolve project: %w", err)
	}
	if obj.Spec.Region == "" {
		return nil, fmt.Errorf("spec.region is required")
	}
	group, err := resolveGroup(ctx, reader, obj, projectID)
	if err != nil {
		return nil, err
	}
	address := common.ValueOf(obj.Spec.IPAddress)
	if fqdn := common.ValueOf(obj.Spec.FQDN); fqdn != "" {
		if address != "" {
			address += "," + fqdn
		} else {
			address = fqdn
		}
	}
	if address == "" {
		return nil, fmt.Errorf("one of spec.ipAddress and spec.fqdn is required")
	}
	id := &ComputeRegionNetworkEndpointIdentity{Project: projectID, Region: obj.Spec.Region, Group: group, Address: address, Port: obj.Spec.Port}
	if externalRef := common.ValueOf(obj.Status.ExternalRef); externalRef != "" && externalRef != id.String() {
		return nil, fmt.Errorf("cannot change ComputeRegionNetworkEndpoint identity (old=%q, new=%q): an endpoint is immutable; make a new one instead", externalRef, id.String())
	}
	return id, nil
}
