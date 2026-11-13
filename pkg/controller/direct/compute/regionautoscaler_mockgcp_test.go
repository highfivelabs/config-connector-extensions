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

// ComputeRegionAutoscaler end to end on the fake cloud (pkg/test/mockenv,
// mockgcp with patch 0041) or a real project (docs/live-runs.md). The
// case: a regional managed instance group of e2-micro VMs scaled on CPU.
// The test makes the groups through the API (a network, a subnet, an
// e2-micro instance template and two regional groups of size 0); the
// autoscalers keep them at one VM each, and deleting the groups at the
// end deletes the VMs.

import (
	"strings"
	"testing"
	"time"

	gcp "cloud.google.com/go/compute/apiv1"
	pb "cloud.google.com/go/compute/apiv1/computepb"
	"github.com/GoogleCloudPlatform/k8s-config-connector/apis/refs"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/k8s"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"

	computev1beta1 "github.com/highfivelabs/config-connector-extensions/apis/compute/v1beta1"
	"github.com/highfivelabs/config-connector-extensions/pkg/test/mockenv"
)

const regionAutoscalerRegion = "us-central1"

// newRegionalAutoscaledGroups makes an e2-micro instance template in a
// network of its own and a regional managed instance group of size 0 for
// each base, and deletes them when the test ends (the groups first, which
// deletes their VMs). It returns the groups' names.
func newRegionalAutoscaledGroups(t *testing.T, env *mockenv.Env, base string, groupBases ...string) []string {
	t.Helper()
	network := newNetwork(t, env, base+"-net")
	subnets, err := gcp.NewSubnetworksRESTClient(env.Ctx, env.GCPOptions()...)
	if err != nil {
		t.Fatal(err)
	}
	subnet := mockenv.Name(base) + "-subnet"
	op, err := subnets.Insert(env.Ctx, &pb.InsertSubnetworkRequest{
		Project: mockenv.Project, Region: regionAutoscalerRegion,
		SubnetworkResource: &pb.Subnetwork{Name: mockenv.Ptr(subnet), Network: mockenv.Ptr(network), IpCidrRange: mockenv.Ptr("10.40.0.0/24")},
	})
	if err == nil {
		err = op.Wait(env.Ctx)
	}
	if err != nil {
		t.Fatalf("making subnet %s: %v", subnet, err)
	}
	t.Cleanup(func() {
		defer subnets.Close()
		op, err := subnets.Delete(env.Ctx, &pb.DeleteSubnetworkRequest{Project: mockenv.Project, Region: regionAutoscalerRegion, Subnetwork: subnet})
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
					Subnetwork: mockenv.Ptr("projects/" + mockenv.Project + "/regions/" + regionAutoscalerRegion + "/subnetworks/" + subnet),
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
	t.Cleanup(func() { groups.Close() })
	var names []string
	for _, b := range groupBases {
		group := mockenv.Name(b)
		op, err := groups.Insert(env.Ctx, &pb.InsertRegionInstanceGroupManagerRequest{
			Project: mockenv.Project, Region: regionAutoscalerRegion,
			InstanceGroupManagerResource: &pb.InstanceGroupManager{
				Name:             mockenv.Ptr(group),
				BaseInstanceName: mockenv.Ptr(group),
				InstanceTemplate: mockenv.Ptr("projects/" + mockenv.Project + "/global/instanceTemplates/" + template),
				TargetSize:       mockenv.Ptr(int32(0)),
			},
		})
		if err == nil {
			err = op.Wait(env.Ctx)
		}
		if err != nil {
			t.Fatalf("making regional instance group manager %s: %v", group, err)
		}
		t.Cleanup(func() {
			op, err := groups.Delete(env.Ctx, &pb.DeleteRegionInstanceGroupManagerRequest{Project: mockenv.Project, Region: regionAutoscalerRegion, InstanceGroupManager: group})
			if err == nil {
				err = op.Wait(env.Ctx)
			}
			if err != nil && !direct.IsNotFound(err) {
				t.Errorf("deleting regional instance group manager %s: %v", group, err)
			}
		})
		names = append(names, group)
	}
	return names
}

func TestComputeRegionAutoscaler(t *testing.T) {
	gvk := computev1beta1.ComputeRegionAutoscalerGVK
	env := mockenv.New(t, "example")
	autoscalers, err := gcp.NewRegionAutoscalersRESTClient(env.Ctx, env.GCPOptions()...)
	if err != nil {
		t.Fatal(err)
	}
	groups := newRegionalAutoscaledGroups(t, env, "ras", "ras-web", "ras-kept")
	groupURL := func(group string) string {
		return "projects/" + mockenv.Project + "/regions/" + regionAutoscalerRegion + "/instanceGroupManagers/" + group
	}
	// The autoscalers go before their groups (cleanups run last first); an
	// abandoned one, or one a failed step left, is deleted here.
	t.Cleanup(func() {
		defer autoscalers.Close()
		for _, base := range []string{"web-rscaler", "kept-rscaler"} {
			op, err := autoscalers.Delete(env.Ctx, &pb.DeleteRegionAutoscalerRequest{Project: mockenv.Project, Region: regionAutoscalerRegion, Autoscaler: mockenv.Name(base)})
			if err == nil {
				err = op.Wait(env.Ctx)
			}
			if err != nil && !direct.IsNotFound(err) {
				t.Errorf("deleting regional autoscaler %s: %v", mockenv.Name(base), err)
			}
		}
	})
	get := func(base string) (*pb.Autoscaler, error) {
		return autoscalers.Get(env.Ctx, &pb.GetRegionAutoscalerRequest{Project: mockenv.Project, Region: regionAutoscalerRegion, Autoscaler: mockenv.Name(base)})
	}
	writes := func() int {
		n := 0
		for _, r := range env.Requests() {
			if (strings.HasPrefix(r, "PATCH ") || strings.HasPrefix(r, "PUT ") || strings.HasPrefix(r, "POST ")) && strings.Contains(r, "/autoscalers") {
				n++
			}
		}
		return n
	}
	getObj := func(t *testing.T, name string) *computev1beta1.ComputeRegionAutoscaler {
		t.Helper()
		obj := &computev1beta1.ComputeRegionAutoscaler{}
		if err := env.Kube.Get(env.Ctx, types.NamespacedName{Namespace: "example", Name: name}, obj); err != nil {
			t.Fatal(err)
		}
		return obj
	}
	newScaler := func(name, group string) *computev1beta1.ComputeRegionAutoscaler {
		return &computev1beta1.ComputeRegionAutoscaler{
			ObjectMeta: metav1.ObjectMeta{Namespace: "example", Name: name, Labels: mockenv.Labels()},
			Spec: computev1beta1.ComputeRegionAutoscalerSpec{
				ProjectRef:  refs.ProjectRef{External: mockenv.Project},
				Region:      regionAutoscalerRegion,
				ResourceID:  mockenv.Ptr(mockenv.Name(name)),
				Description: mockenv.Ptr("the web tier"),
				Target:      groupURL(group),
				AutoscalingPolicy: computev1beta1.RegionautoscalerAutoscalingPolicy{
					MinReplicas:    1,
					MaxReplicas:    1,
					CooldownPeriod: mockenv.Ptr(int32(60)),
					Mode:           mockenv.Ptr("ON"),
					CpuUtilization: &computev1beta1.RegionautoscalerCpuUtilization{Target: 0.6, PredictiveMethod: mockenv.Ptr("NONE")},
				},
			},
		}
	}

	if err := env.Kube.Create(env.Ctx, newScaler("web-rscaler", groups[0])); err != nil {
		t.Fatal(err)
	}

	t.Run("create makes the autoscaler on the regional group", func(t *testing.T) {
		u := env.WaitReady(gvk, "example", "web-rscaler")
		got, err := get("web-rscaler")
		if err != nil {
			t.Fatalf("reading the autoscaler: %v", err)
		}
		if !strings.HasSuffix(got.GetTarget(), "/regions/"+regionAutoscalerRegion+"/instanceGroupManagers/"+groups[0]) || got.GetAutoscalingPolicy().GetMaxNumReplicas() != 1 ||
			got.GetAutoscalingPolicy().GetCpuUtilization().GetUtilizationTarget() != 0.6 {
			t.Errorf("autoscaler = %v", got)
		}
		if !strings.HasSuffix(got.GetRegion(), "/regions/"+regionAutoscalerRegion) {
			t.Errorf("region = %q", got.GetRegion())
		}
		if sl, _, _ := unstructured.NestedString(u.Object, "status", "selfLink"); !strings.HasSuffix(sl, "/regions/"+regionAutoscalerRegion+"/autoscalers/"+mockenv.Name("web-rscaler")) {
			t.Errorf("status.selfLink = %q", sl)
		}
	})

	t.Run("a resync writes nothing", func(t *testing.T) {
		before := writes()
		o := getObj(t, "web-rscaler")
		o.Annotations = map[string]string{k8s.InternalForceReconcileAnnotation: "1"}
		if err := env.Kube.Update(env.Ctx, o); err != nil {
			t.Fatal(err)
		}
		time.Sleep(3 * time.Second)
		env.WaitReady(gvk, "example", "web-rscaler")
		if n := writes() - before; n != 0 {
			t.Errorf("a resync wrote %d times", n)
		}
	})

	t.Run("a changed target utilization is one patch", func(t *testing.T) {
		before := writes()
		o := getObj(t, "web-rscaler")
		o.Spec.AutoscalingPolicy.CpuUtilization.Target = 0.8
		if err := env.Kube.Update(env.Ctx, o); err != nil {
			t.Fatal(err)
		}
		waitFor(t, func() bool {
			got, err := get("web-rscaler")
			return err == nil && got.GetAutoscalingPolicy().GetCpuUtilization().GetUtilizationTarget() == 0.8
		})
		env.WaitReady(gvk, "example", "web-rscaler")
		if n := writes() - before; n != 1 {
			t.Errorf("writes = %d, want one patch", n)
		}
	})

	t.Run("the CRD refuses a change of target", func(t *testing.T) {
		o := getObj(t, "web-rscaler")
		o.Spec.Target = groupURL(groups[1])
		err := env.Kube.Update(env.Ctx, o)
		if err == nil || !strings.Contains(err.Error(), "target is immutable") {
			t.Errorf("changing the target: got %v, want the API server to refuse it", err)
		}
	})

	t.Run("delete deletes the autoscaler", func(t *testing.T) {
		if err := env.Kube.Delete(env.Ctx, getObj(t, "web-rscaler")); err != nil {
			t.Fatal(err)
		}
		env.WaitGone(&computev1beta1.ComputeRegionAutoscaler{ObjectMeta: metav1.ObjectMeta{Namespace: "example", Name: "web-rscaler"}})
		if _, err := get("web-rscaler"); !direct.IsNotFound(err) {
			t.Errorf("after delete, reading the autoscaler: %v, want not found", err)
		}
	})

	t.Run("abandon keeps the autoscaler", func(t *testing.T) {
		if err := env.Kube.Create(env.Ctx, newScaler("kept-rscaler", groups[1])); err != nil {
			t.Fatal(err)
		}
		env.WaitReady(gvk, "example", "kept-rscaler")
		o := getObj(t, "kept-rscaler")
		o.Annotations = map[string]string{"cnrm.cloud.google.com/deletion-policy": "abandon"}
		if err := env.Kube.Update(env.Ctx, o); err != nil {
			t.Fatal(err)
		}
		if err := env.Kube.Delete(env.Ctx, o); err != nil {
			t.Fatal(err)
		}
		env.WaitGone(&computev1beta1.ComputeRegionAutoscaler{ObjectMeta: metav1.ObjectMeta{Namespace: "example", Name: "kept-rscaler"}})
		if _, err := get("kept-rscaler"); err != nil {
			t.Errorf("after abandon, reading the autoscaler: %v, want it kept", err)
		}
	})
}
