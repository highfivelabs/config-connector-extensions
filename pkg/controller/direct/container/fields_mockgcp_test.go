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

package container_test

// ContainerClusterFields and ContainerNodePoolFields end to end on the fake
// cloud (pkg/test/mockenv, patch 0053) or a real project
// (docs/live-runs.md). One zonal cluster and one node pool, the run's own
// (mockenv.Name), made by the test and deleted at the end. Live, it skips
// unless E2E_GKE=true: a cluster takes minutes to make and each node pool
// kubelet change recreates its nodes.

import (
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	gcp "cloud.google.com/go/container/apiv1"
	pb "cloud.google.com/go/container/apiv1/containerpb"
	containerv1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/apis/container/v1beta1"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/k8s"
	"google.golang.org/protobuf/types/known/wrapperspb"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hflcontainer "github.com/highfivelabs/config-connector-extensions/apis/container/v1alpha1"
	"github.com/highfivelabs/config-connector-extensions/pkg/test/mockenv"
)

var (
	configConnectorClusterGVK  = schema.GroupVersionKind{Group: "container.cnrm.cloud.google.com", Version: "v1beta1", Kind: "ContainerCluster"}
	configConnectorNodePoolGVK = schema.GroupVersionKind{Group: "container.cnrm.cloud.google.com", Version: "v1beta1", Kind: "ContainerNodePool"}
)

func TestContainerFields(t *testing.T) {
	if mockenv.Live() && os.Getenv("E2E_GKE") != "true" {
		t.Skip("a live ContainerClusterFields and ContainerNodePoolFields run makes a GKE cluster (minutes, and node recreation per kubelet change); set E2E_GKE=true to run it (docs/live-runs.md)")
	}
	const (
		namespace = "example"
		zone      = "us-central1-a"
	)
	clusterID := mockenv.Name("fields")
	poolID := "pool"
	env := mockenv.New(t, namespace)
	c, err := gcp.NewClusterManagerRESTClient(env.Ctx, env.GCPOptions()...)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { c.Close() })
	parent := "projects/" + mockenv.Project + "/locations/" + zone
	clusterName := parent + "/clusters/" + clusterID
	poolName := clusterName + "/nodePools/" + poolID

	wait := func(t *testing.T, op *pb.Operation) {
		t.Helper()
		name := op.GetName()
		if !strings.HasPrefix(name, "projects/") {
			name = parent + "/operations/" + name
		}
		deadline := time.Now().Add(30 * time.Minute)
		for {
			got, err := c.GetOperation(env.Ctx, &pb.GetOperationRequest{Name: name})
			if err != nil {
				t.Fatal(err)
			}
			if got.GetStatus() == pb.Operation_DONE {
				if got.GetError().GetCode() != 0 {
					t.Fatalf("operation %s: %v", name, got.GetError())
				}
				return
			}
			if time.Now().After(deadline) {
				t.Fatalf("operation %s still %v", name, got.GetStatus())
			}
			time.Sleep(5 * time.Second)
		}
	}
	// labels are cloud labels: mockenv.Labels() plus nothing else.
	labels := mockenv.Labels()

	// The cluster, small: one e2-small node in the default pool, HTTP load
	// balancing off (an add-on the field kind must leave as it is).
	op, err := c.CreateCluster(env.Ctx, &pb.CreateClusterRequest{Parent: parent, Cluster: &pb.Cluster{
		Name:           clusterID,
		ResourceLabels: labels,
		NodePools: []*pb.NodePool{{
			Name: "default-pool", InitialNodeCount: 1,
			Config: &pb.NodeConfig{MachineType: "e2-small", DiskSizeGb: 20, ResourceLabels: labels},
		}},
		AddonsConfig: &pb.AddonsConfig{HttpLoadBalancing: &pb.HttpLoadBalancing{Disabled: true}},
		// The Secret Manager add-on needs Workload Identity (live run
		// cce-11160547-aae3: 400 "Secret_manager_config.enabled cannot be
		// true if workload_identity_config.workload_pool is not set").
		WorkloadIdentityConfig: &pb.WorkloadIdentityConfig{WorkloadPool: mockenv.Project + ".svc.id.goog"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		// A cluster with an operation running refuses the delete (400
		// "Cluster is running incompatible operation", run
		// cce-11160623-50f0): try again until it is done.
		for deadline := time.Now().Add(30 * time.Minute); ; time.Sleep(15 * time.Second) {
			op, err := c.DeleteCluster(env.Ctx, &pb.DeleteClusterRequest{Name: clusterName})
			if err == nil {
				wait(t, op)
				return
			}
			if direct.IsNotFound(err) {
				return
			}
			if !strings.Contains(err.Error(), "incompatible operation") || time.Now().After(deadline) {
				t.Errorf("deleting cluster %s: %v", clusterName, err)
				return
			}
		}
	})
	wait(t, op)
	op, err = c.CreateNodePool(env.Ctx, &pb.CreateNodePoolRequest{Parent: clusterName, NodePool: &pb.NodePool{
		Name: poolID, InitialNodeCount: 1,
		Config: &pb.NodeConfig{
			MachineType: "e2-small", DiskSizeGb: 20, ResourceLabels: labels,
			// Config Connector's kubelet fields: they must survive.
			KubeletConfig: &pb.NodeKubeletConfig{CpuCfsQuota: wrapperspb.Bool(true), PodPidsLimit: 4096},
		},
	}})
	if err != nil {
		t.Fatal(err)
	}
	wait(t, op)

	// Config Connector's objects (not reconciled here; read to resolve
	// resourceRef by name from their spec, as their reference does).
	cc := &unstructured.Unstructured{Object: map[string]any{"spec": map[string]any{
		"location": zone, "resourceID": clusterID, "initialNodeCount": int64(1),
	}}}
	cc.SetGroupVersionKind(configConnectorClusterGVK)
	cc.SetNamespace(namespace)
	cc.SetName("cluster")
	cc.SetAnnotations(map[string]string{"cnrm.cloud.google.com/project-id": mockenv.Project})
	np := &unstructured.Unstructured{Object: map[string]any{"spec": map[string]any{
		"location": zone, "resourceID": poolID, "clusterRef": map[string]any{"name": "cluster"},
	}}}
	np.SetGroupVersionKind(configConnectorNodePoolGVK)
	np.SetNamespace(namespace)
	np.SetName("pool")
	np.SetAnnotations(map[string]string{"cnrm.cloud.google.com/project-id": mockenv.Project})
	for _, o := range []client.Object{cc, np} {
		if err := env.Kube.Create(env.Ctx, o); err != nil {
			t.Fatal(err)
		}
	}

	getCluster := func(t *testing.T) *pb.Cluster {
		t.Helper()
		cl, err := c.GetCluster(env.Ctx, &pb.GetClusterRequest{Name: clusterName})
		if err != nil {
			t.Fatal(err)
		}
		return cl
	}
	getPool := func(t *testing.T) *pb.NodePool {
		t.Helper()
		p, err := c.GetNodePool(env.Ctx, &pb.GetNodePoolRequest{Name: poolName})
		if err != nil {
			t.Fatal(err)
		}
		return p
	}
	count := func(method, suffix string) int {
		n := 0
		for _, req := range env.Requests() {
			if strings.HasPrefix(req, method+" ") && strings.Contains(req, suffix) {
				n++
			}
		}
		return n
	}
	// clusters.update is a PUT on the cluster; nodePools.update a PUT on
	// the node pool.
	clusterUpdates := func() int { return count("PUT", "/clusters/"+clusterID+"?") }
	poolUpdates := func() int { return count("PUT", "/nodePools/"+poolID+"?") }
	// waitReady is env.WaitReady with GKE's time: live, each clusters.update
	// takes minutes (about 7 to turn on the Secret Manager add-on, run
	// cce-11160623-50f0) and a kubelet change recreates the nodes, longer
	// than mockenv.Timeout().
	waitReady := func(t *testing.T, gvk schema.GroupVersionKind, name string) *unstructured.Unstructured {
		t.Helper()
		timeout := mockenv.Timeout()
		if mockenv.Live() {
			timeout = 75 * time.Minute
		}
		var last *unstructured.Unstructured
		for deadline := time.Now().Add(timeout); time.Now().Before(deadline); time.Sleep(time.Second) {
			u := &unstructured.Unstructured{}
			u.SetGroupVersionKind(gvk)
			if err := env.Kube.Get(env.Ctx, types.NamespacedName{Namespace: namespace, Name: name}, u); err != nil {
				t.Fatal(err)
			}
			last = u
			if ready, _ := mockenv.Ready(u); ready {
				return u
			}
		}
		t.Fatalf("%s %s/%s did not become Ready in %v; last status: %v", gvk.Kind, namespace, name, timeout, last.Object["status"])
		return nil
	}
	resync := func(t *testing.T, gvk schema.GroupVersionKind, obj client.Object) {
		t.Helper()
		obj.SetAnnotations(map[string]string{k8s.InternalForceReconcileAnnotation: time.Now().String()})
		if err := env.Kube.Update(env.Ctx, obj); err != nil {
			t.Fatal(err)
		}
		time.Sleep(3 * time.Second)
		waitReady(t, gvk, obj.GetName())
	}

	t.Run("ContainerClusterFields", func(t *testing.T) {
		gvk := hflcontainer.ContainerClusterFieldsGVK
		obj := &hflcontainer.ContainerClusterFields{
			ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: "cluster", Labels: mockenv.Labels()},
			Spec: hflcontainer.ContainerClusterFieldsSpec{
				ResourceRef:                   &containerv1beta1.ContainerClusterRef{Name: "cluster"},
				SecretManagerConfig:           &hflcontainer.SecretManagerConfig{Enabled: mockenv.Ptr(true)},
				RBACBindingConfig:             &hflcontainer.RBACBindingConfig{EnableInsecureBindingSystemUnauthenticated: mockenv.Ptr(false), EnableInsecureBindingSystemAuthenticated: mockenv.Ptr(false)},
				AnonymousAuthenticationConfig: &hflcontainer.AnonymousAuthenticationConfig{Mode: "LIMITED"},
				AddonsConfig:                  &hflcontainer.AddonsConfig{StatefulHAConfig: &hflcontainer.StatefulHAConfig{Enabled: true}},
				NodePoolAutoConfig:            &hflcontainer.NodePoolAutoConfig{NodeKubeletConfig: &hflcontainer.AutoConfigKubeletConfig{InsecureKubeletReadonlyPortEnabled: mockenv.Ptr(false)}},
			},
		}
		before := clusterUpdates()
		if err := env.Kube.Create(env.Ctx, obj); err != nil {
			t.Fatal(err)
		}

		t.Run("create sets each field with its own update", func(t *testing.T) {
			u := waitReady(t, gvk, "cluster")
			if ref, _, _ := unstructured.NestedString(u.Object, "status", "externalRef"); ref != clusterName {
				t.Errorf("status.externalRef = %q, want %q", ref, clusterName)
			}
			cl := getCluster(t)
			if !cl.GetSecretManagerConfig().GetEnabled() {
				t.Errorf("secretManagerConfig = %v, want enabled", cl.GetSecretManagerConfig())
			}
			if r := cl.GetRbacBindingConfig(); r == nil || r.GetEnableInsecureBindingSystemUnauthenticated() || r.GetEnableInsecureBindingSystemAuthenticated() {
				t.Errorf("rbacBindingConfig = %v, want both false", r)
			}
			if m := cl.GetAnonymousAuthenticationConfig().GetMode(); m != pb.AnonymousAuthenticationConfig_LIMITED {
				t.Errorf("anonymousAuthenticationConfig.mode = %v, want LIMITED", m)
			}
			if !cl.GetAddonsConfig().GetStatefulHaConfig().GetEnabled() {
				t.Errorf("addonsConfig.statefulHaConfig = %v, want enabled", cl.GetAddonsConfig().GetStatefulHaConfig())
			}
			if !cl.GetAddonsConfig().GetHttpLoadBalancing().GetDisabled() {
				t.Errorf("addonsConfig.httpLoadBalancing = %v: the add-on update changed an add-on it does not own", cl.GetAddonsConfig().GetHttpLoadBalancing())
			}
			if k := cl.GetNodePoolAutoConfig().GetNodeKubeletConfig(); k == nil || k.InsecureKubeletReadonlyPortEnabled == nil || k.GetInsecureKubeletReadonlyPortEnabled() {
				t.Errorf("nodePoolAutoConfig.nodeKubeletConfig = %v, want the read-only port off", k)
			}
			// A new cluster already has some of these values (the fake's
			// defaults include one), so only "at most one each" holds.
			if n := clusterUpdates() - before; n == 0 || n > 5 {
				t.Errorf("clusters.update called %d times, want one per field that differed (1 to 5)", n)
			}
		})

		t.Run("a resync writes nothing", func(t *testing.T) {
			before := clusterUpdates()
			o := &hflcontainer.ContainerClusterFields{}
			if err := env.Kube.Get(env.Ctx, types.NamespacedName{Namespace: namespace, Name: "cluster"}, o); err != nil {
				t.Fatal(err)
			}
			resync(t, gvk, o)
			if n := clusterUpdates() - before; n != 0 {
				t.Errorf("a resync updated the cluster %d times", n)
			}
		})

		t.Run("a changed field is written alone", func(t *testing.T) {
			before := clusterUpdates()
			o := &hflcontainer.ContainerClusterFields{}
			if err := env.Kube.Get(env.Ctx, types.NamespacedName{Namespace: namespace, Name: "cluster"}, o); err != nil {
				t.Fatal(err)
			}
			o.Spec.AnonymousAuthenticationConfig.Mode = "ENABLED"
			if err := env.Kube.Update(env.Ctx, o); err != nil {
				t.Fatal(err)
			}
			deadline := time.Now().Add(30 * time.Minute)
			for getCluster(t).GetAnonymousAuthenticationConfig().GetMode() != pb.AnonymousAuthenticationConfig_ENABLED {
				if time.Now().After(deadline) {
					t.Fatal("anonymousAuthenticationConfig.mode did not become ENABLED")
				}
				time.Sleep(2 * time.Second)
			}
			waitReady(t, gvk, "cluster")
			if n := clusterUpdates() - before; n != 1 {
				t.Errorf("clusters.update called %d times, want 1", n)
			}
		})

		t.Run("delete leaves the fields", func(t *testing.T) {
			if err := env.Kube.Delete(env.Ctx, obj); err != nil {
				t.Fatal(err)
			}
			env.WaitGone(obj)
			if !getCluster(t).GetSecretManagerConfig().GetEnabled() {
				t.Error("deleting the object changed the cluster")
			}
		})
	})

	t.Run("ContainerNodePoolFields", func(t *testing.T) {
		gvk := hflcontainer.ContainerNodePoolFieldsGVK
		newObj := func(name string) *hflcontainer.ContainerNodePoolFields {
			return &hflcontainer.ContainerNodePoolFields{
				ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: name, Labels: mockenv.Labels()},
				Spec: hflcontainer.ContainerNodePoolFieldsSpec{
					ResourceRef: &containerv1beta1.ContainerNodePoolRef{Name: "pool"},
					Config: &hflcontainer.NodePoolFieldsConfig{KubeletConfig: &hflcontainer.NodePoolKubeletConfig{
						InsecureKubeletReadonlyPortEnabled: mockenv.Ptr(false),
						ContainerLogMaxSize:                mockenv.Ptr("20Mi"),
						ContainerLogMaxFiles:               mockenv.Ptr(int32(3)),
						AllowedUnsafeSysctls:               []string{"kernel.shm*"},
					}},
				},
			}
		}
		obj := newObj("pool")
		if err := env.Kube.Create(env.Ctx, obj); err != nil {
			t.Fatal(err)
		}
		ours := func(t *testing.T, k *pb.NodeKubeletConfig) {
			t.Helper()
			if k.GetContainerLogMaxSize() != "20Mi" || k.GetContainerLogMaxFiles() != 3 || !slices.Equal(k.GetAllowedUnsafeSysctls(), []string{"kernel.shm*"}) || k.InsecureKubeletReadonlyPortEnabled == nil || k.GetInsecureKubeletReadonlyPortEnabled() {
				t.Errorf("kubeletConfig = %v, want this object's fields", k)
			}
		}

		t.Run("create sets the fields and keeps Config Connector's", func(t *testing.T) {
			u := waitReady(t, gvk, "pool")
			if ref, _, _ := unstructured.NestedString(u.Object, "status", "externalRef"); ref != poolName {
				t.Errorf("status.externalRef = %q, want %q", ref, poolName)
			}
			k := getPool(t).GetConfig().GetKubeletConfig()
			ours(t, k)
			if !k.GetCpuCfsQuota().GetValue() || k.GetPodPidsLimit() != 4096 {
				t.Errorf("kubeletConfig = %v: Config Connector's cpuCfsQuota and podPidsLimit changed", k)
			}
		})

		t.Run("a resync writes nothing", func(t *testing.T) {
			before := poolUpdates()
			o := &hflcontainer.ContainerNodePoolFields{}
			if err := env.Kube.Get(env.Ctx, types.NamespacedName{Namespace: namespace, Name: "pool"}, o); err != nil {
				t.Fatal(err)
			}
			resync(t, gvk, o)
			if n := poolUpdates() - before; n != 0 {
				t.Errorf("a resync updated the node pool %d times", n)
			}
		})

		t.Run("Terraform's kubelet update resets them until the next reconcile", func(t *testing.T) {
			// What Config Connector's Terraform-based ContainerNodePool sends
			// when one of its kubelet fields changes: the whole config from
			// Terraform's schema, which lacks this object's fields.
			op, err := c.UpdateNodePool(env.Ctx, &pb.UpdateNodePoolRequest{Name: poolName, KubeletConfig: &pb.NodeKubeletConfig{
				CpuCfsQuota: wrapperspb.Bool(true), PodPidsLimit: 2048,
			}})
			if err != nil {
				t.Fatal(err)
			}
			wait(t, op)
			if got := getPool(t).GetConfig().GetKubeletConfig().GetContainerLogMaxSize(); got == "20Mi" {
				t.Logf("the API kept containerLogMaxSize through a whole-config update (the fake replaces it)")
			}
			o := &hflcontainer.ContainerNodePoolFields{}
			if err := env.Kube.Get(env.Ctx, types.NamespacedName{Namespace: namespace, Name: "pool"}, o); err != nil {
				t.Fatal(err)
			}
			resync(t, gvk, o)
			k := getPool(t).GetConfig().GetKubeletConfig()
			ours(t, k)
			if k.GetPodPidsLimit() != 2048 {
				t.Errorf("podPidsLimit = %d, want Config Connector's 2048 kept", k.GetPodPidsLimit())
			}
		})

		t.Run("delete clears the fields and keeps Config Connector's", func(t *testing.T) {
			if err := env.Kube.Delete(env.Ctx, obj); err != nil {
				t.Fatal(err)
			}
			env.WaitGone(obj)
			k := getPool(t).GetConfig().GetKubeletConfig()
			if k.GetContainerLogMaxSize() == "20Mi" || k.GetContainerLogMaxFiles() == 3 || len(k.GetAllowedUnsafeSysctls()) != 0 {
				t.Errorf("after delete kubeletConfig = %v, want this object's fields unset", k)
			}
			if !k.GetCpuCfsQuota().GetValue() || k.GetPodPidsLimit() != 2048 {
				t.Errorf("after delete kubeletConfig = %v: Config Connector's fields changed", k)
			}
		})

		t.Run("abandon keeps the fields", func(t *testing.T) {
			k := newObj("kept")
			k.Annotations = map[string]string{"cnrm.cloud.google.com/deletion-policy": "abandon"}
			if err := env.Kube.Create(env.Ctx, k); err != nil {
				t.Fatal(err)
			}
			waitReady(t, gvk, "kept")
			if err := env.Kube.Delete(env.Ctx, k); err != nil {
				t.Fatal(err)
			}
			env.WaitGone(k)
			ours(t, getPool(t).GetConfig().GetKubeletConfig())
		})
	})
}
