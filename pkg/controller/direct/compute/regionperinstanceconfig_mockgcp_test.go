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

package compute_test

// ComputeRegionPerInstanceConfig end to end on the fake cloud
// (pkg/test/mockenv) or a real project (docs/live-runs.md). The case: a
// stateful regional managed instance group (the catalog's compute-mig
// module) keeps one named VM with preserved metadata. The test makes the
// group itself through the API (a network, a subnet, an e2-micro instance
// template and a regional group of size 0, redistribution off); the
// config then creates its instance (createInstances), and deleting the
// group at the end deletes the instances.

import (
	"fmt"
	"strings"
	"testing"
	"time"

	gcp "cloud.google.com/go/compute/apiv1"
	pb "cloud.google.com/go/compute/apiv1/computepb"
	"github.com/GoogleCloudPlatform/k8s-config-connector/apis/refs"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/k8s"
	"google.golang.org/api/iterator"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	computev1beta1 "github.com/highfivelabs/config-connector-extensions/apis/compute/v1beta1"
	"github.com/highfivelabs/config-connector-extensions/pkg/test/mockenv"
)

const picRegion = "us-central1"

// newStatefulGroup makes a regional managed instance group of size 0 on an
// e2-micro template in a network of its own, and deletes them when the
// test ends (the group first, which deletes its instances). It returns the
// group's name.
func newStatefulGroup(t *testing.T, env *mockenv.Env, base string) string {
	t.Helper()
	network := newNetwork(t, env, base+"-net")
	subnets, err := gcp.NewSubnetworksRESTClient(env.Ctx, env.GCPOptions()...)
	if err != nil {
		t.Fatal(err)
	}
	subnet := mockenv.Name(base) + "-subnet"
	op, err := subnets.Insert(env.Ctx, &pb.InsertSubnetworkRequest{
		Project: mockenv.Project, Region: picRegion,
		SubnetworkResource: &pb.Subnetwork{Name: mockenv.Ptr(subnet), Network: mockenv.Ptr(network), IpCidrRange: mockenv.Ptr("10.20.0.0/24")},
	})
	if err == nil {
		err = op.Wait(env.Ctx)
	}
	if err != nil {
		t.Fatalf("making subnet %s: %v", subnet, err)
	}
	t.Cleanup(func() {
		defer subnets.Close()
		op, err := subnets.Delete(env.Ctx, &pb.DeleteSubnetworkRequest{Project: mockenv.Project, Region: picRegion, Subnetwork: subnet})
		if err == nil {
			err = op.Wait(env.Ctx)
		}
		if err != nil && !direct.IsNotFound(err) {
			t.Errorf("deleting subnet %s: %v", subnet, err)
		}
	})

	templates, err := gcp.NewInstanceTemplatesRESTClient(env.Ctx, env.GCPOptions()...)
	if err != nil {
		t.Fatal(err)
	}
	template := mockenv.Name(base) + "-template"
	op, err = templates.Insert(env.Ctx, &pb.InsertInstanceTemplateRequest{
		Project: mockenv.Project,
		InstanceTemplateResource: &pb.InstanceTemplate{
			Name: mockenv.Ptr(template),
			Properties: &pb.InstanceProperties{
				MachineType: mockenv.Ptr("e2-micro"),
				Labels:      mockenv.Labels(),
				Disks: []*pb.AttachedDisk{{
					Boot: mockenv.Ptr(true), AutoDelete: mockenv.Ptr(true),
					InitializeParams: &pb.AttachedDiskInitializeParams{
						SourceImage: mockenv.Ptr("projects/debian-cloud/global/images/family/debian-12"),
						DiskSizeGb:  mockenv.Ptr(int64(10)),
						DiskType:    mockenv.Ptr("pd-standard"),
					},
				}},
				NetworkInterfaces: []*pb.NetworkInterface{{
					Subnetwork: mockenv.Ptr("projects/" + mockenv.Project + "/regions/" + picRegion + "/subnetworks/" + subnet),
				}},
			},
		},
	})
	if err == nil {
		err = op.Wait(env.Ctx)
	}
	if err != nil {
		t.Fatalf("making instance template %s: %v", template, err)
	}
	t.Cleanup(func() {
		defer templates.Close()
		op, err := templates.Delete(env.Ctx, &pb.DeleteInstanceTemplateRequest{Project: mockenv.Project, InstanceTemplate: template})
		if err == nil {
			err = op.Wait(env.Ctx)
		}
		if err != nil && !direct.IsNotFound(err) {
			t.Errorf("deleting instance template %s: %v", template, err)
		}
	})

	groups, err := gcp.NewRegionInstanceGroupManagersRESTClient(env.Ctx, env.GCPOptions()...)
	if err != nil {
		t.Fatal(err)
	}
	group := mockenv.Name(base)
	op, err = groups.Insert(env.Ctx, &pb.InsertRegionInstanceGroupManagerRequest{
		Project: mockenv.Project, Region: picRegion,
		InstanceGroupManagerResource: &pb.InstanceGroupManager{
			Name:             mockenv.Ptr(group),
			BaseInstanceName: mockenv.Ptr(group),
			InstanceTemplate: mockenv.Ptr("projects/" + mockenv.Project + "/global/instanceTemplates/" + template),
			TargetSize:       mockenv.Ptr(int32(0)),
			// createInstances, which a per-instance config is made with,
			// needs redistribution off (412 CONDITION_NOT_MET otherwise,
			// run cce-11122245-f5ea), as a stateful group has it.
			UpdatePolicy: &pb.InstanceGroupManagerUpdatePolicy{InstanceRedistributionType: mockenv.Ptr("NONE")},
		},
	})
	if err == nil {
		err = op.Wait(env.Ctx)
	}
	if err != nil {
		t.Fatalf("making regional instance group manager %s: %v", group, err)
	}
	t.Cleanup(func() {
		defer groups.Close()
		op, err := groups.Delete(env.Ctx, &pb.DeleteRegionInstanceGroupManagerRequest{Project: mockenv.Project, Region: picRegion, InstanceGroupManager: group})
		if err == nil {
			err = op.Wait(env.Ctx)
		}
		if err != nil && !direct.IsNotFound(err) {
			t.Errorf("deleting regional instance group manager %s: %v", group, err)
		}
	})
	return group
}

func TestComputeRegionPerInstanceConfig(t *testing.T) {
	gvk := computev1beta1.ComputeRegionPerInstanceConfigGVK
	env := mockenv.New(t, "example")
	groups, err := gcp.NewRegionInstanceGroupManagersRESTClient(env.Ctx, env.GCPOptions()...)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { groups.Close() })
	group := newStatefulGroup(t, env, "pic-group")

	configs := func() map[string]*pb.PerInstanceConfig {
		out := map[string]*pb.PerInstanceConfig{}
		it := groups.ListPerInstanceConfigs(env.Ctx, &pb.ListPerInstanceConfigsRegionInstanceGroupManagersRequest{
			Project: mockenv.Project, Region: picRegion, InstanceGroupManager: group,
		})
		for {
			c, err := it.Next()
			if err == iterator.Done {
				break
			}
			if err != nil {
				t.Fatalf("listing per-instance configs: %v", err)
			}
			out[c.GetName()] = c
		}
		return out
	}
	// writes counts the group's write calls: createInstances,
	// updatePerInstanceConfigs, deletePerInstanceConfigs.
	writes := func(call string) int {
		n := 0
		for _, r := range env.Requests() {
			if strings.HasPrefix(r, "POST ") && strings.Contains(r, "/instanceGroupManagers/"+group+"/"+call) {
				n++
			}
		}
		return n
	}
	getObj := func(t *testing.T, name string) *computev1beta1.ComputeRegionPerInstanceConfig {
		t.Helper()
		obj := &computev1beta1.ComputeRegionPerInstanceConfig{}
		if err := env.Kube.Get(env.Ctx, types.NamespacedName{Namespace: "example", Name: name}, obj); err != nil {
			t.Fatal(err)
		}
		return obj
	}
	newConfig := func(name, role string) *computev1beta1.ComputeRegionPerInstanceConfig {
		return &computev1beta1.ComputeRegionPerInstanceConfig{
			ObjectMeta: metav1.ObjectMeta{Namespace: "example", Name: name, Labels: mockenv.Labels()},
			Spec: computev1beta1.ComputeRegionPerInstanceConfigSpec{
				ProjectRef:                    refs.ProjectRef{External: mockenv.Project},
				Region:                        picRegion,
				RegionInstanceGroupManagerRef: computev1beta1.ComputeRegionInstanceGroupManagerRef{External: fmt.Sprintf("projects/%s/regions/%s/instanceGroupManagers/%s", mockenv.Project, picRegion, group)},
				ResourceID:                    mockenv.Ptr(mockenv.Name(name)),
				MinimalAction:                 mockenv.Ptr("NONE"),
				MostDisruptiveAllowedAction:   mockenv.Ptr("REFRESH"),
				PreservedState: &computev1beta1.RegionperinstanceconfigPreservedState{
					Metadata: map[string]string{"role": role},
				},
			},
		}
	}
	db := mockenv.Name("db-0")

	if err := env.Kube.Create(env.Ctx, newConfig("db-0", "primary")); err != nil {
		t.Fatal(err)
	}

	t.Run("create makes the instance with its preserved metadata", func(t *testing.T) {
		env.WaitReady(gvk, "example", "db-0")
		c, ok := configs()[db]
		if !ok {
			t.Fatalf("no per-instance config %s in %s", db, group)
		}
		if got := c.GetPreservedState().GetMetadata()["role"]; got != "primary" {
			t.Errorf("preserved metadata role = %q, want primary", got)
		}
		if n := writes("createInstances"); n != 1 {
			t.Errorf("createInstances calls = %d, want 1", n)
		}
	})

	t.Run("a resync writes nothing", func(t *testing.T) {
		before := writes("createInstances") + writes("updatePerInstanceConfigs") + writes("applyUpdatesToInstances")
		o := getObj(t, "db-0")
		o.Annotations = map[string]string{k8s.InternalForceReconcileAnnotation: "1"}
		if err := env.Kube.Update(env.Ctx, o); err != nil {
			t.Fatal(err)
		}
		time.Sleep(3 * time.Second)
		env.WaitReady(gvk, "example", "db-0")
		if after := writes("createInstances") + writes("updatePerInstanceConfigs") + writes("applyUpdatesToInstances"); after != before {
			t.Errorf("a resync wrote %d times", after-before)
		}
	})

	t.Run("changed metadata is one update and one apply", func(t *testing.T) {
		updates, applies := writes("updatePerInstanceConfigs"), writes("applyUpdatesToInstances")
		o := getObj(t, "db-0")
		o.Spec.PreservedState.Metadata = map[string]string{"role": "replica"}
		if err := env.Kube.Update(env.Ctx, o); err != nil {
			t.Fatal(err)
		}
		waitFor(t, func() bool {
			c, ok := configs()[db]
			return ok && c.GetPreservedState().GetMetadata()["role"] == "replica"
		})
		env.WaitReady(gvk, "example", "db-0")
		if n := writes("updatePerInstanceConfigs") - updates; n != 1 {
			t.Errorf("updatePerInstanceConfigs calls = %d, want 1", n)
		}
		if n := writes("applyUpdatesToInstances") - applies; n != 1 {
			t.Errorf("applyUpdatesToInstances calls = %d, want 1", n)
		}
	})

	t.Run("the CRD refuses a change of group", func(t *testing.T) {
		o := getObj(t, "db-0")
		o.Spec.RegionInstanceGroupManagerRef.External = "another-group"
		err := env.Kube.Update(env.Ctx, o)
		if err == nil || !strings.Contains(err.Error(), "regionInstanceGroupManagerRef is immutable") {
			t.Errorf("changing the group: got %v, want the API server to refuse it", err)
		}
	})

	t.Run("delete deletes the config", func(t *testing.T) {
		if err := env.Kube.Delete(env.Ctx, getObj(t, "db-0")); err != nil {
			t.Fatal(err)
		}
		env.WaitGone(&computev1beta1.ComputeRegionPerInstanceConfig{ObjectMeta: metav1.ObjectMeta{Namespace: "example", Name: "db-0"}})
		waitFor(t, func() bool { _, ok := configs()[db]; return !ok })
	})

	t.Run("abandon keeps the config", func(t *testing.T) {
		if err := env.Kube.Create(env.Ctx, newConfig("db-1", "primary")); err != nil {
			t.Fatal(err)
		}
		env.WaitReady(gvk, "example", "db-1")
		o := getObj(t, "db-1")
		o.Annotations = map[string]string{"cnrm.cloud.google.com/deletion-policy": "abandon"}
		if err := env.Kube.Update(env.Ctx, o); err != nil {
			t.Fatal(err)
		}
		if err := env.Kube.Delete(env.Ctx, o); err != nil {
			t.Fatal(err)
		}
		env.WaitGone(&computev1beta1.ComputeRegionPerInstanceConfig{ObjectMeta: metav1.ObjectMeta{Namespace: "example", Name: "db-1"}})
		if _, ok := configs()[mockenv.Name("db-1")]; !ok {
			t.Errorf("after abandon, the per-instance config %s is gone, want it kept", mockenv.Name("db-1"))
		}
	})
}
