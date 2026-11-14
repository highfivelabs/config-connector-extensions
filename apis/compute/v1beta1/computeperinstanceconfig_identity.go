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

// Modified by High Five Labs for config-connector-extensions: adapted from
// our ComputeRegionPerInstanceConfig identity (computeregionperinstanceconfig_identity.go,
// itself copied from Config Connector's
// apis/compute/v1alpha1/computeregionperinstanceconfig_identity.go, UPSTREAM.md)
// for the zonal per-instance config, which Config Connector has only as a
// Terraform-based alpha kind: zones and a zonal instance group manager for
// regions and a regional one.

package v1beta1

import (
	"context"
	"fmt"
	"strings"

	"github.com/GoogleCloudPlatform/k8s-config-connector/apis/common/identity"
	computev1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/apis/compute/v1beta1"
	apirefs "github.com/GoogleCloudPlatform/k8s-config-connector/apis/refs"
	refs "github.com/GoogleCloudPlatform/k8s-config-connector/apis/refs/v1beta1"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/gcpurls"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	_ identity.IdentityV2 = &ComputePerInstanceConfigIdentity{}
	_ identity.Resource   = &ComputePerInstanceConfig{}
)

var ComputePerInstanceConfigIdentityFormat = gcpurls.Template[ComputePerInstanceConfigIdentity](
	"compute.googleapis.com",
	"projects/{project}/zones/{zone}/instanceGroupManagers/{instanceGroupManager}/{name}",
)

// ComputePerInstanceConfigIdentity is the identity of a GCP ComputePerInstanceConfig resource.
// +k8s:deepcopy-gen=false
type ComputePerInstanceConfigIdentity struct {
	Project              string
	Zone                 string
	InstanceGroupManager string
	Name                 string
}

func (i *ComputePerInstanceConfigIdentity) String() string {
	return ComputePerInstanceConfigIdentityFormat.ToString(*i)
}

func (i *ComputePerInstanceConfigIdentity) FromExternal(ref string) error {
	trimmedRef := apirefs.TrimComputeURIPrefix(ref)
	parsed, match, err := ComputePerInstanceConfigIdentityFormat.Parse(trimmedRef)
	if err != nil {
		return fmt.Errorf("format of ComputePerInstanceConfig external=%q was not known (use %s): %w", ref, ComputePerInstanceConfigIdentityFormat.CanonicalForm(), err)
	}
	if !match {
		return fmt.Errorf("format of ComputePerInstanceConfig external=%q was not known (use %s)", ref, ComputePerInstanceConfigIdentityFormat.CanonicalForm())
	}

	*i = *parsed
	return nil
}

func (i *ComputePerInstanceConfigIdentity) Host() string {
	return ComputePerInstanceConfigIdentityFormat.Host()
}

func (i *ComputePerInstanceConfigIdentity) ParentString() string {
	return fmt.Sprintf("projects/%s/zones/%s/instanceGroupManagers/%s", i.Project, i.Zone, i.InstanceGroupManager)
}

func ParseComputePerInstanceConfigExternal(external string) (*ComputePerInstanceConfigIdentity, error) {
	if external == "" {
		return nil, fmt.Errorf("empty ComputePerInstanceConfig external value")
	}
	id := &ComputePerInstanceConfigIdentity{}
	if err := id.FromExternal(external); err != nil {
		return nil, err
	}
	return id, nil
}

func parseZonalInstanceGroupManager(ctx context.Context, reader client.Reader, obj *ComputePerInstanceConfig) (string, error) {
	ref := obj.Spec.InstanceGroupManagerRef
	if ref.External != "" {
		external := apirefs.TrimComputeURIPrefix(ref.External)
		if strings.Contains(external, "instanceGroupManagers/") {
			parts := strings.Split(external, "instanceGroupManagers/")
			if len(parts) == 2 && parts[1] != "" {
				subparts := strings.Split(parts[1], "/")
				if subparts[0] != "" {
					return subparts[0], nil
				}
			}
			return "", fmt.Errorf("invalid ComputeInstanceGroupManager external reference format: %q", ref.External)
		}
		if strings.Contains(external, "/") {
			return "", fmt.Errorf("invalid ComputeInstanceGroupManager external reference format: %q", ref.External)
		}
		return external, nil
	}
	if ref.Name != "" {
		ns := ref.Namespace
		if ns == "" {
			ns = obj.Namespace
		}
		if reader != nil {
			u := &unstructured.Unstructured{}
			u.SetGroupVersionKind(computev1beta1.ComputeInstanceGroupManagerGVK)
			if err := reader.Get(ctx, types.NamespacedName{Name: ref.Name, Namespace: ns}, u); err != nil {
				return "", fmt.Errorf("cannot get referenced ComputeInstanceGroupManager %s/%s: %w", ns, ref.Name, err)
			}
			igm := &computev1beta1.ComputeInstanceGroupManager{}
			if err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, igm); err != nil {
				return "", fmt.Errorf("reading referenced ComputeInstanceGroupManager %s/%s: %w", ns, ref.Name, err)
			}
			id, err := igm.GetIdentity(ctx, reader)
			if err != nil {
				return "", fmt.Errorf("cannot get identity of referenced ComputeInstanceGroupManager %s/%s: %w", ns, ref.Name, err)
			}
			if igmId, ok := id.(*computev1beta1.ComputeInstanceGroupManagerIdentity); ok && igmId.InstanceGroupManager != "" {
				return igmId.InstanceGroupManager, nil
			}
			resourceID, err := refs.GetResourceID(igm)
			if err != nil {
				return "", fmt.Errorf("cannot get resource ID of referenced ComputeInstanceGroupManager %s/%s: %w", ns, ref.Name, err)
			}
			if resourceID != "" {
				return resourceID, nil
			}
			return ref.Name, nil
		}
		return ref.Name, nil
	}
	return "", fmt.Errorf("cannot resolve instanceGroupManager: neither external nor name is set")
}

func getIdentityFromComputePerInstanceConfigSpec(ctx context.Context, reader client.Reader, obj *ComputePerInstanceConfig) (*ComputePerInstanceConfigIdentity, error) {
	resourceID, err := refs.GetResourceID(obj)
	if err != nil {
		return nil, fmt.Errorf("cannot resolve resource ID: %w", err)
	}

	projectID, err := refs.ResolveProjectID(ctx, reader, obj)
	if err != nil {
		return nil, fmt.Errorf("cannot resolve project: %w", err)
	}

	zone := obj.Spec.Zone
	if zone == "" {
		return nil, fmt.Errorf("cannot resolve zone: spec.zone is empty")
	}

	igm, err := parseZonalInstanceGroupManager(ctx, reader, obj)
	if err != nil {
		return nil, fmt.Errorf("cannot resolve instanceGroupManager: %w", err)
	}

	identity := &ComputePerInstanceConfigIdentity{
		Project:              projectID,
		Zone:                 zone,
		InstanceGroupManager: igm,
		Name:                 resourceID,
	}
	return identity, nil
}

func (obj *ComputePerInstanceConfig) GetIdentity(ctx context.Context, reader client.Reader) (identity.Identity, error) {
	specIdentity, err := getIdentityFromComputePerInstanceConfigSpec(ctx, reader, obj)
	if err != nil {
		return nil, err
	}

	return specIdentity, nil
}
