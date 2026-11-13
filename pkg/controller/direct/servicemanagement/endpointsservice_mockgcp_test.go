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

package servicemanagement_test

// EndpointsService end to end on the fake cloud (pkg/test/mockenv, with
// the Service Management fake, pkg/controller/direct/servicemanagement/fake)
// or a real project (docs/live-runs.md). The case: the catalog's endpoints
// module, an OpenAPI service on <name>.endpoints.<project>.cloud.goog, its
// configuration changed once, another configuration rolled out by hand and
// put back, one service abandoned and one deleted. Deleted services keep
// their names for 30 days; every name carries the run's ID.

import (
	"encoding/base64"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	refsv1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/apis/refs/v1beta1"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/k8s"
	gcp "google.golang.org/api/servicemanagement/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"

	krm "github.com/highfivelabs/config-connector-extensions/apis/servicemanagement/v1alpha1"
	"github.com/highfivelabs/config-connector-extensions/pkg/test/mockenv"
)

const namespace = "example"

var gvk = krm.EndpointsServiceGVK

func openAPI(host, title string) string {
	return fmt.Sprintf(`swagger: "2.0"
info:
  title: %q
  description: "config-connector-extensions EndpointsService test"
  version: "1.0.0"
host: %q
schemes:
  - "https"
produces:
  - "application/json"
paths:
  "/echo":
    get:
      operationId: "echo"
      responses:
        "200":
          description: "ok"
`, title, host)
}

func TestEndpointsService(t *testing.T) {
	env := mockenv.New(t, namespace)
	client, err := gcp.NewService(env.Ctx, env.GCPOptions()...)
	if err != nil {
		t.Fatal(err)
	}
	if mockenv.Live() && os.Getenv("E2E_SERVICE_MANAGEMENT") != "true" {
		t.Skip("a live EndpointsService needs servicemanagement.googleapis.com on in the project and roles/servicemanagement.admin for the live-test service account (run cce-11130544-c338: 'servicemanagement.services.list' denied); set E2E_SERVICE_MANAGEMENT=true where it has them (docs/live-runs.md)")
	}
	serviceName := func(base string) string {
		return mockenv.Name(base) + ".endpoints." + mockenv.Project + ".cloud.goog"
	}
	wait := func(op *gcp.Operation) error {
		for !op.Done {
			time.Sleep(2 * time.Second)
			var err error
			if op, err = client.Operations.Get(op.Name).Context(env.Ctx).Do(); err != nil {
				return err
			}
		}
		if op.Error != nil {
			return fmt.Errorf("operation %s: %s", op.Name, op.Error.Message)
		}
		return nil
	}
	gone := func(err error) bool {
		return direct.IsNotFound(err) || direct.HasHTTPCode(err, 403)
	}
	// Delete every service the test names, whatever the test did.
	cleanup := func(name string) {
		t.Cleanup(func() {
			op, err := client.Services.Delete(name).Context(env.Ctx).Do()
			if err == nil {
				err = wait(op)
			}
			if err != nil && !gone(err) && !direct.IsBadRequest(err) {
				t.Errorf("deleting service %s: %v", name, err)
			}
		})
	}
	writes := func(name, what string) int {
		n := 0
		for _, r := range env.Requests() {
			if strings.HasPrefix(r, "POST ") && strings.Contains(r, "/v1/services/"+name+"/"+what) {
				n++
			}
		}
		return n
	}
	getObj := func(t *testing.T, name string) *krm.EndpointsService {
		t.Helper()
		obj := &krm.EndpointsService{}
		if err := env.Kube.Get(env.Ctx, types.NamespacedName{Namespace: namespace, Name: name}, obj); err != nil {
			t.Fatal(err)
		}
		return obj
	}
	update := func(t *testing.T, name string, change func(*krm.EndpointsService)) {
		t.Helper()
		if err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
			o := getObj(t, name)
			change(o)
			return env.Kube.Update(env.Ctx, o)
		}); err != nil {
			t.Fatal(err)
		}
	}
	resync := func(t *testing.T, name string) {
		t.Helper()
		update(t, name, func(o *krm.EndpointsService) {
			o.Annotations = map[string]string{k8s.InternalForceReconcileAnnotation: time.Now().Format(time.RFC3339Nano)}
		})
	}
	configID := func(u *unstructured.Unstructured) string {
		id, _, _ := unstructured.NestedString(u.Object, "status", "observedState", "configID")
		return id
	}
	waitConfig := func(t *testing.T, name, notID string) *unstructured.Unstructured {
		t.Helper()
		deadline := time.Now().Add(mockenv.Timeout())
		for {
			u := env.WaitReady(gvk, namespace, name)
			if id := configID(u); id != "" && id != notID {
				return u
			}
			if time.Now().After(deadline) {
				t.Fatalf("status.observedState.configID stayed %q", notID)
			}
			time.Sleep(time.Second)
		}
	}
	newService := func(objName, svc string) *krm.EndpointsService {
		return &krm.EndpointsService{
			ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: objName, Labels: mockenv.Labels()},
			Spec: krm.EndpointsServiceSpec{
				ResourceID:    mockenv.Ptr(svc),
				ProjectRef:    &refsv1beta1.ProjectRef{External: mockenv.Project},
				OpenAPIConfig: mockenv.Ptr(openAPI(svc, "echo")),
			},
		}
	}

	echo := serviceName("echo")
	cleanup(echo)
	if err := env.Kube.Create(env.Ctx, newService("echo", echo)); err != nil {
		t.Fatal(err)
	}

	var firstConfig string
	t.Run("create makes the service, submits its configuration and rolls it out", func(t *testing.T) {
		u := env.WaitReady(gvk, namespace, "echo")
		if ref, _, _ := unstructured.NestedString(u.Object, "status", "externalRef"); ref != "services/"+echo {
			t.Errorf("status.externalRef = %q", ref)
		}
		if dns, _, _ := unstructured.NestedString(u.Object, "status", "observedState", "dnsAddress"); dns != echo {
			t.Errorf("status.observedState.dnsAddress = %q, want %q", dns, echo)
		}
		firstConfig = configID(u)
		if firstConfig == "" {
			t.Fatal("status.observedState.configID is not set")
		}
		svc, err := client.Services.Get(echo).Context(env.Ctx).Do()
		if err != nil {
			t.Fatalf("reading the service: %v", err)
		}
		if svc.ProducerProjectId != mockenv.Project {
			t.Errorf("producer project = %q", svc.ProducerProjectId)
		}
		active, err := client.Services.GetConfig(echo).Context(env.Ctx).Do()
		if err != nil {
			t.Fatalf("reading the active configuration: %v", err)
		}
		if active.Id != firstConfig || active.Title != "echo" {
			t.Errorf("active configuration = %s %q, want %s \"echo\"", active.Id, active.Title, firstConfig)
		}
		if s, r := writes(echo, "configs:submit"), writes(echo, "rollouts"); s != 1 || r != 1 {
			t.Errorf("submitted %d and rolled out %d times, want once each", s, r)
		}
	})

	t.Run("a resync writes nothing", func(t *testing.T) {
		s, r := writes(echo, "configs:submit"), writes(echo, "rollouts")
		resync(t, "echo")
		time.Sleep(3 * time.Second)
		env.WaitReady(gvk, namespace, "echo")
		if s2, r2 := writes(echo, "configs:submit"), writes(echo, "rollouts"); s2 != s || r2 != r {
			t.Errorf("a resync submitted %d and rolled out %d times", s2-s, r2-r)
		}
	})

	var secondConfig string
	t.Run("a configuration change is submitted and rolled out once", func(t *testing.T) {
		s, r := writes(echo, "configs:submit"), writes(echo, "rollouts")
		update(t, "echo", func(o *krm.EndpointsService) { o.Spec.OpenAPIConfig = mockenv.Ptr(openAPI(echo, "echo v2")) })
		u := waitConfig(t, "echo", firstConfig)
		secondConfig = configID(u)
		active, err := client.Services.GetConfig(echo).Context(env.Ctx).Do()
		if err != nil {
			t.Fatal(err)
		}
		if active.Id != secondConfig || active.Title != "echo v2" {
			t.Errorf("active configuration = %s %q, want %s \"echo v2\"", active.Id, active.Title, secondConfig)
		}
		if s2, r2 := writes(echo, "configs:submit"), writes(echo, "rollouts"); s2-s != 1 || r2-r != 1 {
			t.Errorf("submitted %d and rolled out %d times, want once each", s2-s, r2-r)
		}
	})

	t.Run("a configuration rolled out by hand is put back", func(t *testing.T) {
		op, err := client.Services.Configs.Submit(echo, &gcp.SubmitConfigSourceRequest{ConfigSource: &gcp.ConfigSource{Files: []*gcp.ConfigFile{{
			FilePath: "openapi.yaml", FileType: "OPEN_API_YAML",
			FileContents: base64.StdEncoding.EncodeToString([]byte(openAPI(echo, "by hand"))),
		}}}}).Context(env.Ctx).Do()
		if err == nil {
			err = wait(op)
		}
		if err != nil {
			t.Fatalf("submitting by hand: %v", err)
		}
		configs, err := client.Services.Configs.List(echo).Context(env.Ctx).Do()
		if err != nil || len(configs.ServiceConfigs) == 0 {
			t.Fatalf("listing configurations: %v", err)
		}
		byHand := ""
		for _, c := range configs.ServiceConfigs {
			if c.Title == "by hand" {
				byHand = c.Id
			}
		}
		if byHand == "" {
			t.Fatal("the configuration submitted by hand is not listed")
		}
		op, err = client.Services.Rollouts.Create(echo, &gcp.Rollout{ServiceName: echo,
			TrafficPercentStrategy: &gcp.TrafficPercentStrategy{Percentages: map[string]float64{byHand: 100}}}).Context(env.Ctx).Do()
		if err == nil {
			err = wait(op)
		}
		if err != nil {
			t.Fatalf("rolling out by hand: %v", err)
		}
		resync(t, "echo")
		waitConfig(t, "echo", secondConfig)
		active, err := client.Services.GetConfig(echo).Context(env.Ctx).Do()
		if err != nil {
			t.Fatal(err)
		}
		if active.Title != "echo v2" {
			t.Errorf("active configuration's title = %q, want the spec's \"echo v2\"", active.Title)
		}
	})

	t.Run("the service's name and project cannot change", func(t *testing.T) {
		o := getObj(t, "echo")
		o.Spec.ResourceID = mockenv.Ptr(serviceName("other"))
		if err := env.Kube.Update(env.Ctx, o); err == nil || !strings.Contains(err.Error(), "resourceID is immutable") {
			t.Errorf("changing the name: err = %v", err)
		}
		o = getObj(t, "echo")
		o.Spec.ProjectRef = &refsv1beta1.ProjectRef{External: "other-project"}
		if err := env.Kube.Update(env.Ctx, o); err == nil || !strings.Contains(err.Error(), "projectRef is immutable") {
			t.Errorf("changing the project: err = %v", err)
		}
	})

	t.Run("delete removes the service", func(t *testing.T) {
		obj := getObj(t, "echo")
		if err := env.Kube.Delete(env.Ctx, obj); err != nil {
			t.Fatal(err)
		}
		env.WaitGone(obj)
		if _, err := client.Services.Get(echo).Context(env.Ctx).Do(); err == nil {
			t.Error("the deleted service still answers a get")
		} else if !gone(err) {
			t.Errorf("reading the deleted service: %v", err)
		}
	})

	t.Run("abandon keeps the service", func(t *testing.T) {
		kept := serviceName("kept")
		cleanup(kept)
		obj := newService("kept", kept)
		obj.Annotations = map[string]string{"cnrm.cloud.google.com/deletion-policy": "abandon"}
		if err := env.Kube.Create(env.Ctx, obj); err != nil {
			t.Fatal(err)
		}
		env.WaitReady(gvk, namespace, "kept")
		if err := env.Kube.Delete(env.Ctx, obj); err != nil {
			t.Fatal(err)
		}
		env.WaitGone(obj)
		if _, err := client.Services.Get(kept).Context(env.Ctx).Do(); err != nil {
			t.Errorf("the abandoned service is gone: %v", err)
		}
	})
}
