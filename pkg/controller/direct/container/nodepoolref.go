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

package container

// Written here: Config Connector's ContainerNodePoolRef.Normalize resolves
// a ContainerNodePool by name through its status.externalRef (the
// Terraform-based ContainerNodePool never sets one) and then through its
// clusterRef, which it reads as a full cluster id only. A clusterRef.external
// that is a bare cluster name (what Terraform's `cluster` field takes, and
// what the catalog's gke-nodepool chart writes, because a full id breaks
// Config Connector's own delete of the node pool) fails to parse, the
// fallback answers nothing, and the reference stays "not ready" although
// the node pool is Ready. resolveNodePool reads the clusterRef itself: a
// full id as it is, a bare name in the node pool's project and location,
// a name through the ContainerCluster object.

import (
	"context"
	"fmt"
	"strings"

	containerv1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/apis/container/v1beta1"
	refsv1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/apis/refs/v1beta1"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/k8s"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// resolveNodePool turns a node pool reference into the node pool's
// identity. An external reference is parsed as it is.
func resolveNodePool(ctx context.Context, reader client.Reader, defaultNamespace string, ref *containerv1beta1.ContainerNodePoolRef) (*containerv1beta1.ContainerNodePoolIdentity, error) {
	id := &containerv1beta1.ContainerNodePoolIdentity{}
	if ref.External != "" {
		if err := id.FromExternal(ref.External); err != nil {
			return nil, err
		}
		return id, nil
	}
	key := types.NamespacedName{Namespace: ref.Namespace, Name: ref.Name}
	if key.Namespace == "" {
		key.Namespace = defaultNamespace
	}
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(containerv1beta1.ContainerNodePoolGVK)
	if err := reader.Get(ctx, key, u); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, k8s.NewReferenceNotFoundError(u.GroupVersionKind(), key)
		}
		return nil, fmt.Errorf("reading referenced ContainerNodePool %s: %w", key, err)
	}
	if ext, _, _ := unstructured.NestedString(u.Object, "status", "externalRef"); ext != "" {
		if err := id.FromExternal(ext); err == nil {
			return id, nil
		}
	}
	pool, err := refsv1beta1.GetResourceID(u)
	if err != nil {
		return nil, err
	}
	location, _, _ := unstructured.NestedString(u.Object, "spec", "location")
	clusterExt, _, _ := unstructured.NestedString(u.Object, "spec", "clusterRef", "external")
	clusterName, _, _ := unstructured.NestedString(u.Object, "spec", "clusterRef", "name")
	clusterNS, _, _ := unstructured.NestedString(u.Object, "spec", "clusterRef", "namespace")

	cluster := &containerv1beta1.ContainerClusterIdentity{}
	switch {
	case clusterExt != "" && !strings.Contains(clusterExt, "/"):
		// A bare cluster name: the cluster is in the node pool's project and
		// location, as Terraform reads its `cluster` field.
		project, err := nodePoolProject(u)
		if err != nil {
			return nil, err
		}
		if location == "" {
			return nil, fmt.Errorf("ContainerNodePool %s: spec.location is needed to resolve clusterRef.external %q", key, clusterExt)
		}
		cluster.Project, cluster.Location, cluster.Cluster = project, location, clusterExt
	case clusterExt != "":
		if err := cluster.FromExternal(clusterExt); err != nil {
			return nil, fmt.Errorf("ContainerNodePool %s: clusterRef.external: %w", key, err)
		}
	case clusterName != "":
		cr := &containerv1beta1.ContainerClusterRef{Name: clusterName, Namespace: clusterNS}
		if err := cr.Normalize(ctx, reader, key.Namespace); err != nil {
			return nil, err
		}
		if err := cluster.FromExternal(cr.External); err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("ContainerNodePool %s has no spec.clusterRef", key)
	}
	if location == "" {
		location = cluster.Location
	}
	id.Project, id.Location, id.Cluster, id.NodePool = cluster.Project, location, cluster.Cluster, pool
	return id, nil
}

// nodePoolProject is a Terraform-based object's project: its
// cnrm.cloud.google.com/project-id annotation, which Config Connector's
// admission webhook always sets (from the namespace's annotation, else the
// namespace's name). The managers do not read namespaces (no RBAC for it in
// namespaced mode).
func nodePoolProject(u *unstructured.Unstructured) (string, error) {
	if p := u.GetAnnotations()[k8s.ProjectIDAnnotation]; p != "" {
		return p, nil
	}
	return "", fmt.Errorf("ContainerNodePool %s/%s has no %s annotation to resolve its bare clusterRef.external", u.GetNamespace(), u.GetName(), k8s.ProjectIDAnnotation)
}
