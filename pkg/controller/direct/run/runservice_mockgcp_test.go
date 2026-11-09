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

package run_test

// RunService end to end on the fake cloud (pkg/test/mockenv, patch 0008)
// or a real project (docs/live-runs.md). The case: a public inference
// service on a GPU, which Config Connector's RunService cannot express (no
// invokerIamDisabled, no nodeSelector or gpuZonalRedundancyDisabled).
// Service names are mockenv.Name(...), and the test deletes every service
// it made at the end, the abandoned one included. Against a real project
// the images are Cloud Run's public sample, since the made-up ones do not
// exist.

import (
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	gcp "cloud.google.com/go/run/apiv2"
	pb "cloud.google.com/go/run/apiv2/runpb"
	refsv1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/apis/refs/v1beta1"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/k8s"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"

	runv1alpha1 "github.com/highfivelabs/config-connector-extensions/apis/run/v1alpha1"
	"github.com/highfivelabs/config-connector-extensions/pkg/test/mockenv"
)

const (
	namespace = "example"
	region    = "us-central1"
)

// image1 and image2 are two images of the service; against a real project
// they must exist.
var image1, image2 = images()

func images() (string, string) {
	if mockenv.Live() {
		return "us-docker.pkg.dev/cloudrun/container/hello:latest", "us-docker.pkg.dev/cloudrun/container/hello"
	}
	return "us-docker.pkg.dev/example/inference/server:1", "us-docker.pkg.dev/example/inference/server:2"
}

var gvk = runv1alpha1.RunServiceGVK

func cloudName(name string) string {
	return "projects/" + mockenv.Project + "/locations/" + region + "/services/" + mockenv.Name(name)
}

// gpu reports whether the service asks for a GPU: always in the fake
// cloud; against a real project only with E2E_RUN_GPU=true, since a GPU
// without zonal redundancy needs Cloud Run GPU quota the test project does
// not have (live run cce-11091509-3d77: "You do not have quota for using
// GPUs without zonal redundancy"). Without it the same lifecycle runs on a
// service with no GPU, and the GPU checks are skipped.
func gpu() bool {
	return !mockenv.Live() || os.Getenv("E2E_RUN_GPU") == "true"
}

func newService(name string) *runv1alpha1.RunService {
	s := &runv1alpha1.RunService{
		ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: name},
		Spec: runv1alpha1.RunServiceSpec{
			ResourceID:         mockenv.Ptr(mockenv.Name(name)),
			Labels:             mockenv.Labels(),
			ProjectRef:         &refsv1beta1.ProjectRef{External: mockenv.Project},
			Location:           mockenv.Ptr(region),
			Description:        mockenv.Ptr("inference on a GPU"),
			InvokerIamDisabled: mockenv.Ptr(true),
			Template: &runv1alpha1.RevisionTemplate{
				Containers: []runv1alpha1.Container{{
					Image: mockenv.Ptr(image1),
					Resources: &runv1alpha1.ResourceRequirements{
						Limits: map[string]string{"cpu": "4", "memory": "16Gi", "nvidia.com/gpu": "1"},
					},
				}},
				NodeSelector:               &runv1alpha1.NodeSelector{Accelerator: mockenv.Ptr("nvidia-l4")},
				GpuZonalRedundancyDisabled: mockenv.Ptr(true),
				Scaling:                    &runv1alpha1.RevisionScaling{MaxInstanceCount: mockenv.Ptr(int32(3))},
			},
		},
	}
	if !gpu() {
		t := s.Spec.Template
		t.NodeSelector, t.GpuZonalRedundancyDisabled = nil, nil
		delete(t.Containers[0].Resources.Limits, "nvidia.com/gpu")
		t.Containers[0].Resources.Limits["cpu"], t.Containers[0].Resources.Limits["memory"] = "1", "512Mi"
	}
	return s
}

// masks returns the update masks of the service's updates, in order.
func masks(t *testing.T, env *mockenv.Env, name string) []string {
	t.Helper()
	var out []string
	for _, r := range env.Requests() {
		if strings.HasPrefix(r, "PATCH ") && strings.Contains(r, "/v2/"+cloudName(name)+"?") {
			u, err := url.Parse(strings.TrimPrefix(r, "PATCH "))
			if err != nil {
				t.Fatal(err)
			}
			out = append(out, u.Query().Get("updateMask"))
		}
	}
	return out
}

func TestRunServiceGPUAndInvokerIAM(t *testing.T) {
	env := mockenv.New(t, namespace)
	cloud, err := gcp.NewServicesRESTClient(env.Ctx, env.GCPOptions()...)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cloud.Close() })
	// Delete every service the test made, the abandoned one included.
	t.Cleanup(func() {
		for _, name := range []string{"inference", "kept"} {
			op, err := cloud.DeleteService(env.Ctx, &pb.DeleteServiceRequest{Name: cloudName(name)})
			if err == nil {
				_, err = op.Wait(env.Ctx)
			}
			if err != nil && status.Code(err) != codes.NotFound {
				t.Errorf("deleting service %s: %v", cloudName(name), err)
			}
		}
	})
	read := func(t *testing.T, name string) *pb.Service {
		t.Helper()
		svc, err := cloud.GetService(env.Ctx, &pb.GetServiceRequest{Name: cloudName(name)})
		if err != nil {
			t.Fatalf("reading the service from the cloud: %v", err)
		}
		return svc
	}
	getObj := func(t *testing.T, name string) *runv1alpha1.RunService {
		t.Helper()
		obj := &runv1alpha1.RunService{}
		if err := env.Kube.Get(env.Ctx, types.NamespacedName{Namespace: namespace, Name: name}, obj); err != nil {
			t.Fatal(err)
		}
		return obj
	}

	const name = "inference"
	svc := newService(name)
	if err := env.Kube.Create(env.Ctx, svc); err != nil {
		t.Fatal(err)
	}

	t.Run("create makes a public GPU service", func(t *testing.T) {
		u := env.WaitReady(gvk, namespace, name)
		if ref, _, _ := unstructured.NestedString(u.Object, "status", "externalRef"); ref != cloudName(name) {
			t.Errorf("status.externalRef = %q, want %q", ref, cloudName(name))
		}
		got := read(t, name)
		if !got.GetInvokerIamDisabled() {
			t.Errorf("cloud invokerIamDisabled = false, want true")
		}
		tmpl := got.GetTemplate()
		if !gpu() {
			t.Log("GPU checks skipped: no Cloud Run GPU quota in this project (set E2E_RUN_GPU=true where there is)")
		} else if tmpl.GetNodeSelector().GetAccelerator() != "nvidia-l4" || !tmpl.GetGpuZonalRedundancyDisabled() {
			t.Errorf("cloud template node selector / GPU zonal redundancy = %v / %v", tmpl.GetNodeSelector(), tmpl.GetGpuZonalRedundancyDisabled())
		}
		if c := tmpl.GetContainers(); len(c) != 1 || c[0].GetImage() != image1 || (gpu() && c[0].GetResources().GetLimits()["nvidia.com/gpu"] != "1") {
			t.Errorf("cloud containers = %v", c)
		}
		if tmpl.GetScaling().GetMaxInstanceCount() != 3 {
			t.Errorf("cloud template scaling = %v, want max 3", tmpl.GetScaling())
		}
	})

	t.Run("a resync sends no update", func(t *testing.T) {
		// The fake cloud fills in server defaults (ingress, launch stage,
		// timeout, concurrency, execution environment, service account,
		// resources); none may show as a difference.
		before := len(masks(t, env, name))
		gets := func() int {
			n := 0
			for _, r := range env.Requests() {
				if strings.HasPrefix(r, "GET ") && strings.Contains(r, "/v2/"+cloudName(name)+"?") {
					n++
				}
			}
			return n
		}
		g := gets()
		o := getObj(t, name)
		o.Annotations = map[string]string{k8s.InternalForceReconcileAnnotation: "1"}
		if err := env.Kube.Update(env.Ctx, o); err != nil {
			t.Fatal(err)
		}
		deadline := time.Now().Add(30 * time.Second)
		for gets() == g {
			if time.Now().After(deadline) {
				t.Fatal("the controller did not read the service again")
			}
			time.Sleep(100 * time.Millisecond)
		}
		time.Sleep(time.Second)
		env.WaitReady(gvk, namespace, name)
		if m := masks(t, env, name)[before:]; len(m) != 0 {
			t.Errorf("updates after a resync with an unchanged spec: %q, want none", m)
		}
	})

	t.Run("a new image updates the template alone", func(t *testing.T) {
		before := len(masks(t, env, name))
		o := getObj(t, name)
		o.Spec.Template.Containers[0].Image = mockenv.Ptr(image2)
		if err := env.Kube.Update(env.Ctx, o); err != nil {
			t.Fatal(err)
		}
		env.WaitReady(gvk, namespace, name)
		got := read(t, name)
		if got.GetTemplate().GetContainers()[0].GetImage() != image2 {
			t.Errorf("cloud image = %q, want %q", got.GetTemplate().GetContainers()[0].GetImage(), image2)
		}
		if !got.GetInvokerIamDisabled() || (gpu() && got.GetTemplate().GetNodeSelector().GetAccelerator() != "nvidia-l4") {
			t.Errorf("a template update changed invokerIamDisabled or the node selector: %v, %v", got.GetInvokerIamDisabled(), got.GetTemplate().GetNodeSelector())
		}
		if m := masks(t, env, name)[before:]; len(m) != 1 || m[0] != "template" {
			t.Errorf("update masks = %q, want one: template", m)
		}
	})

	t.Run("invoker IAM is turned back on alone", func(t *testing.T) {
		before := len(masks(t, env, name))
		o := getObj(t, name)
		o.Spec.InvokerIamDisabled = mockenv.Ptr(false)
		if err := env.Kube.Update(env.Ctx, o); err != nil {
			t.Fatal(err)
		}
		env.WaitReady(gvk, namespace, name)
		if read(t, name).GetInvokerIamDisabled() {
			t.Errorf("cloud invokerIamDisabled = true, want false")
		}
		m := masks(t, env, name)[before:]
		if len(m) != 1 || (m[0] != "invokerIamDisabled" && m[0] != "invoker_iam_disabled") {
			t.Errorf("update masks = %q, want one: invokerIamDisabled", m)
		}
	})

	t.Run("delete removes the service", func(t *testing.T) {
		if err := env.Kube.Delete(env.Ctx, svc); err != nil {
			t.Fatal(err)
		}
		env.WaitGone(&runv1alpha1.RunService{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: name}})
		_, err := cloud.GetService(env.Ctx, &pb.GetServiceRequest{Name: cloudName(name)})
		if status.Code(err) != codes.NotFound {
			t.Errorf("reading the deleted service: got %v, want NotFound", err)
		}
	})

	t.Run("abandon keeps the service", func(t *testing.T) {
		const kept = "kept"
		k := newService(kept)
		k.Annotations = map[string]string{"cnrm.cloud.google.com/deletion-policy": "abandon"}
		if err := env.Kube.Create(env.Ctx, k); err != nil {
			t.Fatal(err)
		}
		env.WaitReady(gvk, namespace, kept)
		if err := env.Kube.Delete(env.Ctx, k); err != nil {
			t.Fatal(err)
		}
		env.WaitGone(k)
		read(t, kept)
	})
}
