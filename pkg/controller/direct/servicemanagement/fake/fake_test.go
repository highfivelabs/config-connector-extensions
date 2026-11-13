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

package fake

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	"google.golang.org/api/googleapi"
	"google.golang.org/api/option"
	gcp "google.golang.org/api/servicemanagement/v1"
)

type refuse struct{ hits int }

func (r *refuse) RoundTrip(req *http.Request) (*http.Response, error) {
	r.hits++
	return nil, errors.New("not for the fake: " + req.URL.String())
}

func code(err error) int {
	var e *googleapi.Error
	if errors.As(err, &e) {
		return e.Code
	}
	return 0
}

func TestServiceLifecycle(t *testing.T) {
	ctx := context.Background()
	next := &refuse{}
	c, err := gcp.NewService(ctx, option.WithHTTPClient(&http.Client{Transport: New(next)}), option.WithoutAuthentication())
	if err != nil {
		t.Fatal(err)
	}
	const name = "api.endpoints.p.cloud.goog"
	if _, err := c.Services.Get(name).Do(); code(err) != 404 {
		t.Fatalf("get before create: %v", err)
	}
	op, err := c.Services.Create(&gcp.ManagedService{ServiceName: name, ProducerProjectId: "p"}).Do()
	if err != nil || !op.Done {
		t.Fatalf("create: %v %v", op, err)
	}
	if _, err := c.Services.Create(&gcp.ManagedService{ServiceName: name, ProducerProjectId: "p"}).Do(); code(err) != 409 {
		t.Errorf("second create: %v", err)
	}
	if _, err := c.Services.GetConfig(name).Do(); code(err) != 404 {
		t.Errorf("active configuration before a rollout: %v", err)
	}
	doc := "swagger: \"2.0\"\ninfo:\n  title: t1\n  version: \"1\"\nhost: " + name + "\nx-google-endpoints:\n  - name: " + name + "\n    target: 192.0.2.1\n"
	op, err = c.Services.Configs.Submit(name, &gcp.SubmitConfigSourceRequest{ConfigSource: &gcp.ConfigSource{Files: []*gcp.ConfigFile{{
		FilePath: "openapi.yaml", FileType: "OPEN_API_YAML", FileContents: base64.StdEncoding.EncodeToString([]byte(doc)),
	}}}}).Do()
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	resp := &gcp.SubmitConfigSourceResponse{}
	if err := json.Unmarshal(op.Response, resp); err != nil || resp.ServiceConfig == nil || resp.ServiceConfig.Id == "" {
		t.Fatalf("submit's answer: %s %v", op.Response, err)
	}
	id := resp.ServiceConfig.Id
	if _, err := c.Services.Rollouts.Create(name, &gcp.Rollout{TrafficPercentStrategy: &gcp.TrafficPercentStrategy{Percentages: map[string]float64{"nope": 100}}}).Do(); code(err) != 400 {
		t.Errorf("rollout of an unknown configuration: %v", err)
	}
	if _, err := c.Services.Rollouts.Create(name, &gcp.Rollout{TrafficPercentStrategy: &gcp.TrafficPercentStrategy{Percentages: map[string]float64{id: 100}}}).Do(); err != nil {
		t.Fatalf("rollout: %v", err)
	}
	active, err := c.Services.GetConfig(name).Do()
	if err != nil || active.Id != id || active.Name != name || active.Title != "t1" || len(active.Endpoints) != 1 || active.Endpoints[0].Target != "192.0.2.1" {
		t.Fatalf("active configuration: %+v %v", active, err)
	}
	list, err := c.Services.Configs.List(name).Do()
	if err != nil || len(list.ServiceConfigs) != 1 {
		t.Errorf("configurations: %v %v", list, err)
	}
	if _, err := c.Services.Delete(name).Do(); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := c.Services.Get(name).Do(); code(err) != 404 {
		t.Errorf("get after delete: %v", err)
	}
	if _, err := c.Services.Create(&gcp.ManagedService{ServiceName: name, ProducerProjectId: "p"}).Do(); code(err) != 409 {
		t.Errorf("create of a deleted name: %v", err)
	}
	if _, err := c.Services.Undelete(name).Do(); err != nil {
		t.Fatalf("undelete: %v", err)
	}
	if _, err := c.Services.Get(name).Do(); err != nil {
		t.Errorf("get after undelete: %v", err)
	}
	if next.hits != 0 {
		t.Errorf("%d requests went past the fake", next.hits)
	}
	// A service's IAM policy is mockgcp's.
	if _, err := c.Services.GetIamPolicy("services/"+name, &gcp.GetIamPolicyRequest{}).Do(); err == nil {
		t.Error("getIamPolicy did not reach the next round tripper")
	}
	if next.hits != 1 {
		t.Errorf("getIamPolicy: %d requests went past the fake, want 1", next.hits)
	}
}
