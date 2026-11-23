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

// RunJob (ours) end to end on the fake cloud (pkg/test/mockenv, patch 0069)
// or a real project (docs/live-runs.md). The case: a batch job on a GPU with
// labels on the job and its executions, a task template that never retries,
// and a Binary Authorization policy by name, none of which Config
// Connector's RunJob can express; then a gRPC startup probe. Job names are
// mockenv.Name(...), and the test deletes every job it made at the end, the
// abandoned one included. Against a real project the image is Cloud Run's
// public sample job, the GPU only with E2E_RUN_GPU=true (as for RunService),
// and the Binary Authorization policy only with E2E_RUN_BINAUTHZ_POLICY (a
// cloudRun platform policy's name, projects/P/platforms/cloudRun/policies/N);
// without it the job uses the project's default policy.

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
	"google.golang.org/protobuf/encoding/protojson"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"

	runv1alpha1 "github.com/highfivelabs/config-connector-extensions/apis/run/v1alpha1"
	runv1beta1 "github.com/highfivelabs/config-connector-extensions/apis/run/v1beta1"
	"github.com/highfivelabs/config-connector-extensions/pkg/test/mockenv"
)

var jobGVK = runv1alpha1.RunJobGVK

func jobCloudName(name string) string {
	return "projects/" + mockenv.Project + "/locations/" + region + "/jobs/" + mockenv.Name(name)
}

// jobImages are the job's two images; against a real project they must
// exist.
func jobImages() (string, string) {
	if mockenv.Live() {
		return "us-docker.pkg.dev/cloudrun/container/job:latest", "us-docker.pkg.dev/cloudrun/container/job"
	}
	return "us-docker.pkg.dev/example/batch/worker:1", "us-docker.pkg.dev/example/batch/worker:2"
}

// binauthzPolicy is the Binary Authorization policy the job names: made up
// in the fake cloud, E2E_RUN_BINAUTHZ_POLICY against a real project (empty
// without it: the job then uses the project's default policy).
func binauthzPolicy() string {
	if mockenv.Live() {
		return os.Getenv("E2E_RUN_BINAUTHZ_POLICY")
	}
	return "projects/" + mockenv.Project + "/platforms/cloudRun/policies/batch-strict"
}

func newJob(name string) *runv1alpha1.RunJob {
	image, _ := jobImages()
	labels := map[string]string{"team": "batch"}
	for k, v := range mockenv.Labels() {
		labels[k] = v
	}
	j := &runv1alpha1.RunJob{
		ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: name, Labels: labels},
		Spec: runv1alpha1.RunJobSpec{
			ResourceID: mockenv.Ptr(mockenv.Name(name)),
			ProjectRef: &refsv1beta1.ProjectRef{External: mockenv.Project},
			Location:   mockenv.Ptr(region),
			Template: &runv1alpha1.ExecutionTemplate{
				Labels:      map[string]string{"stage": "nightly"},
				TaskCount:   mockenv.Ptr(int32(2)),
				Parallelism: mockenv.Ptr(int32(1)),
				Template: &runv1alpha1.TaskTemplate{
					Containers: []runv1beta1.Container{{
						Image: mockenv.Ptr(image),
						Resources: &runv1beta1.ResourceRequirements{
							Limits: map[string]string{"cpu": "4", "memory": "16Gi", "nvidia.com/gpu": "1"},
						},
					}},
					MaxRetries:                 mockenv.Ptr(int32(0)),
					Timeout:                    mockenv.Ptr("1800s"),
					NodeSelector:               &runv1beta1.NodeSelector{Accelerator: mockenv.Ptr("nvidia-l4")},
					GpuZonalRedundancyDisabled: mockenv.Ptr(true),
				},
			},
		},
	}
	if p := binauthzPolicy(); p != "" {
		j.Spec.BinaryAuthorization = &runv1alpha1.BinaryAuthorization{Policy: mockenv.Ptr(p)}
	} else {
		j.Spec.BinaryAuthorization = &runv1alpha1.BinaryAuthorization{UseDefault: mockenv.Ptr(true)}
	}
	if !gpu() {
		t := j.Spec.Template.Template
		t.NodeSelector, t.GpuZonalRedundancyDisabled = nil, nil
		delete(t.Containers[0].Resources.Limits, "nvidia.com/gpu")
		t.Containers[0].Resources.Limits["cpu"], t.Containers[0].Resources.Limits["memory"] = "1", "512Mi"
	}
	return j
}

// jobUpdates counts the job's updates (UpdateJob is a PATCH of the job,
// with no update mask).
func jobUpdates(env *mockenv.Env, name string) int {
	n := 0
	for _, r := range env.Requests() {
		if !strings.HasPrefix(r, "PATCH ") {
			continue
		}
		if u, err := url.Parse(strings.TrimPrefix(r, "PATCH ")); err == nil && u.Path == "/v2/"+jobCloudName(name) {
			n++
		}
	}
	return n
}

func TestRunJob(t *testing.T) {
	env := mockenv.New(t, namespace)
	cloud, err := gcp.NewJobsRESTClient(env.Ctx, env.GCPOptions()...)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cloud.Close() })
	// Delete every job the test made, the abandoned one included.
	t.Cleanup(func() {
		for _, name := range []string{"nightly", "kept-job"} {
			op, err := cloud.DeleteJob(env.Ctx, &pb.DeleteJobRequest{Name: jobCloudName(name)})
			if err == nil {
				_, err = op.Wait(env.Ctx)
			}
			if err != nil && status.Code(err) != codes.NotFound {
				t.Errorf("deleting job %s: %v", jobCloudName(name), err)
			}
		}
	})
	read := func(t *testing.T, name string) *pb.Job {
		t.Helper()
		job, err := cloud.GetJob(env.Ctx, &pb.GetJobRequest{Name: jobCloudName(name)})
		if err != nil {
			t.Fatalf("reading the job from the cloud: %v", err)
		}
		return job
	}
	getObj := func(t *testing.T, name string) *runv1alpha1.RunJob {
		t.Helper()
		obj := &runv1alpha1.RunJob{}
		if err := env.Kube.Get(env.Ctx, types.NamespacedName{Namespace: namespace, Name: name}, obj); err != nil {
			t.Fatal(err)
		}
		return obj
	}
	image1, image2 := jobImages()

	t.Run("the schema refuses what a job cannot take", func(t *testing.T) {
		for _, c := range []struct {
			what   string
			change func(*runv1alpha1.RunJob)
			want   string
		}{
			{"a service's base image", func(j *runv1alpha1.RunJob) {
				j.Spec.Template.Template.Containers[0].BaseImageURI = mockenv.Ptr("us-central1-docker.pkg.dev/serverless-runtimes/google-22/runtimes/go122")
			}, "a job takes none of them"},
			{"a service's CPU idle", func(j *runv1alpha1.RunJob) {
				j.Spec.Template.Template.Containers[0].Resources.CPUIdle = mockenv.Ptr(true)
			}, "a job takes none of them"},
			{"a default and a named policy", func(j *runv1alpha1.RunJob) {
				j.Spec.BinaryAuthorization = &runv1alpha1.BinaryAuthorization{UseDefault: mockenv.Ptr(true), Policy: mockenv.Ptr("projects/p/platforms/cloudRun/policies/x")}
			}, "one of useDefault and policy"},
		} {
			j := newJob("refused")
			c.change(j)
			err := env.Kube.Create(env.Ctx, j)
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Errorf("creating a job with %s: err = %v, want one containing %q", c.what, err, c.want)
				if err == nil {
					_ = env.Kube.Delete(env.Ctx, j)
					env.WaitGone(j)
				}
			}
		}
	})

	const name = "nightly"
	job := newJob(name)
	if err := env.Kube.Create(env.Ctx, job); err != nil {
		t.Fatal(err)
	}

	t.Run("create makes the job with the fields Config Connector's lacks", func(t *testing.T) {
		u := env.WaitReady(jobGVK, namespace, name)
		if ref, _, _ := unstructured.NestedString(u.Object, "status", "externalRef"); ref != jobCloudName(name) {
			t.Errorf("status.externalRef = %q, want %q", ref, jobCloudName(name))
		}
		got := read(t, name)
		if got.GetLabels()["team"] != "batch" {
			t.Errorf("cloud labels = %v, want team=batch (from metadata.labels)", got.GetLabels())
		}
		et := got.GetTemplate()
		if et.GetLabels()["stage"] != "nightly" || et.GetTaskCount() != 2 || et.GetParallelism() != 1 {
			t.Errorf("cloud execution template labels / task count / parallelism = %v / %d / %d", et.GetLabels(), et.GetTaskCount(), et.GetParallelism())
		}
		tt := et.GetTemplate()
		if _, ok := tt.GetRetries().(*pb.TaskTemplate_MaxRetries); !ok || tt.GetMaxRetries() != 0 {
			t.Errorf("cloud task template retries = %v, want maxRetries 0", tt.GetRetries())
		}
		if tt.GetTimeout().AsDuration() != 30*time.Minute {
			t.Errorf("cloud task timeout = %v, want 30m", tt.GetTimeout().AsDuration())
		}
		if !gpu() {
			t.Log("GPU checks skipped: no Cloud Run GPU quota in this project (set E2E_RUN_GPU=true where there is)")
		} else if tt.GetNodeSelector().GetAccelerator() != "nvidia-l4" || !tt.GetGpuZonalRedundancyDisabled() {
			t.Errorf("cloud task template node selector / GPU zonal redundancy = %v / %v", tt.GetNodeSelector(), tt.GetGpuZonalRedundancyDisabled())
		}
		if c := tt.GetContainers(); len(c) != 1 || c[0].GetImage() != image1 || (gpu() && c[0].GetResources().GetLimits()["nvidia.com/gpu"] != "1") {
			t.Errorf("cloud containers = %v", c)
		}
		if p := binauthzPolicy(); p != "" && got.GetBinaryAuthorization().GetPolicy() != p {
			t.Errorf("cloud Binary Authorization = %v, want policy %q", got.GetBinaryAuthorization(), p)
		}
	})

	t.Run("a resync sends no update", func(t *testing.T) {
		// The fake cloud fills in server defaults (launch stage, timeout of
		// a template without one, service account, execution environment,
		// resources); none may show as a difference.
		before := jobUpdates(env, name)
		gets := func() int {
			n := 0
			for _, r := range env.Requests() {
				if strings.HasPrefix(r, "GET ") && strings.Contains(r, "/v2/"+jobCloudName(name)+"?") {
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
				t.Fatal("the controller did not read the job again")
			}
			time.Sleep(100 * time.Millisecond)
		}
		time.Sleep(time.Second)
		env.WaitReady(jobGVK, namespace, name)
		if n := jobUpdates(env, name) - before; n != 0 {
			tmpl, _ := protojson.Marshal(read(t, name).GetTemplate())
			t.Errorf("%d updates after a resync with an unchanged spec, want none; the cloud's template: %s", n, tmpl)
		}
	})

	t.Run("labels, the image and the policy change with one update", func(t *testing.T) {
		before := jobUpdates(env, name)
		o := getObj(t, name)
		o.Labels["team"] = "data"
		o.Spec.Template.Labels = map[string]string{"stage": "hourly"}
		o.Spec.Template.Template.Containers[0].Image = mockenv.Ptr(image2)
		if !mockenv.Live() {
			o.Spec.BinaryAuthorization.Policy = mockenv.Ptr("projects/" + mockenv.Project + "/platforms/cloudRun/policies/batch-relaxed")
		}
		if err := env.Kube.Update(env.Ctx, o); err != nil {
			t.Fatal(err)
		}
		env.WaitReady(jobGVK, namespace, name)
		got := read(t, name)
		if got.GetLabels()["team"] != "data" || got.GetTemplate().GetLabels()["stage"] != "hourly" {
			t.Errorf("cloud labels / execution template labels = %v / %v", got.GetLabels(), got.GetTemplate().GetLabels())
		}
		tt := got.GetTemplate().GetTemplate()
		if tt.GetContainers()[0].GetImage() != image2 {
			t.Errorf("cloud image = %q, want %q", tt.GetContainers()[0].GetImage(), image2)
		}
		if !mockenv.Live() && !strings.HasSuffix(got.GetBinaryAuthorization().GetPolicy(), "/batch-relaxed") {
			t.Errorf("cloud Binary Authorization = %v, want the batch-relaxed policy", got.GetBinaryAuthorization())
		}
		if gpu() && tt.GetNodeSelector().GetAccelerator() != "nvidia-l4" {
			t.Errorf("an update lost the node selector: %v", tt.GetNodeSelector())
		}
		if _, ok := tt.GetRetries().(*pb.TaskTemplate_MaxRetries); !ok || tt.GetMaxRetries() != 0 || got.GetTemplate().GetTaskCount() != 2 {
			t.Errorf("an update changed maxRetries or the task count: %v, %d", tt.GetRetries(), got.GetTemplate().GetTaskCount())
		}
		if n := jobUpdates(env, name) - before; n != 1 {
			t.Errorf("updates = %d, want 1", n)
		}
	})

	t.Run("a gRPC startup probe is added", func(t *testing.T) {
		before := jobUpdates(env, name)
		o := getObj(t, name)
		o.Spec.Template.Template.Containers[0].StartupProbe = &runv1beta1.Probe{
			Grpc: &runv1beta1.GrpcAction{Port: mockenv.Ptr(int32(8081)), Service: mockenv.Ptr("batch.Health")},
		}
		if err := env.Kube.Update(env.Ctx, o); err != nil {
			t.Fatal(err)
		}
		env.WaitReady(jobGVK, namespace, name)
		p := read(t, name).GetTemplate().GetTemplate().GetContainers()[0].GetStartupProbe()
		if p.GetGrpc().GetPort() != 8081 || p.GetGrpc().GetService() != "batch.Health" {
			t.Errorf("cloud startup probe = %v, want gRPC on 8081 for batch.Health", p)
		}
		if n := jobUpdates(env, name) - before; n != 1 {
			t.Errorf("updates = %d, want 1", n)
		}
		// The server's probe timings are taken as read: no further update.
		before = jobUpdates(env, name)
		o = getObj(t, name)
		o.Annotations = map[string]string{k8s.InternalForceReconcileAnnotation: "2"}
		if err := env.Kube.Update(env.Ctx, o); err != nil {
			t.Fatal(err)
		}
		time.Sleep(2 * time.Second)
		env.WaitReady(jobGVK, namespace, name)
		if n := jobUpdates(env, name) - before; n != 0 {
			t.Errorf("%d updates after a resync with the probe's timings left to the server, want none", n)
		}
	})

	t.Run("location is immutable", func(t *testing.T) {
		o := getObj(t, name)
		o.Spec.Location = mockenv.Ptr("europe-west1")
		err := env.Kube.Update(env.Ctx, o)
		if err == nil || !strings.Contains(err.Error(), "location is immutable") {
			t.Errorf("changing the location: err = %v, want location is immutable", err)
		}
	})

	t.Run("delete removes the job", func(t *testing.T) {
		if err := env.Kube.Delete(env.Ctx, job); err != nil {
			t.Fatal(err)
		}
		env.WaitGone(&runv1alpha1.RunJob{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: name}})
		_, err := cloud.GetJob(env.Ctx, &pb.GetJobRequest{Name: jobCloudName(name)})
		if status.Code(err) != codes.NotFound {
			t.Errorf("reading the deleted job: got %v, want NotFound", err)
		}
	})

	t.Run("abandon keeps the job", func(t *testing.T) {
		const kept = "kept-job"
		k := newJob(kept)
		k.Annotations = map[string]string{"cnrm.cloud.google.com/deletion-policy": "abandon"}
		if err := env.Kube.Create(env.Ctx, k); err != nil {
			t.Fatal(err)
		}
		env.WaitReady(jobGVK, namespace, kept)
		if err := env.Kube.Delete(env.Ctx, k); err != nil {
			t.Fatal(err)
		}
		env.WaitGone(k)
		read(t, kept)
	})
}
