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

package apigateway_test

// What the API Gateway kinds need in the cloud: an API config its API, a
// gateway its API and config (the fixtures' ${NAME}-api and
// ${NAME}-config), for the test's object and its abandoned one. Made the
// same way in the fake (mockgcp serves APIs) and live, and deleted when the
// test ends, after the kinds' own objects: a config before its API, each
// delete waiting for its operation and trying again while the API answers
// that something still uses it.

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	apigatewayv1beta1 "github.com/highfivelabs/config-connector-extensions/apis/apigateway/v1beta1"
	"github.com/highfivelabs/config-connector-extensions/pkg/discokind"
	"github.com/highfivelabs/config-connector-extensions/pkg/discokind/kindtest"
	"github.com/highfivelabs/config-connector-extensions/pkg/test/mockenv"
)

const apiBase = "https://apigateway.googleapis.com/v1/"

// openAPI is the fixtures' document, base64: the least Swagger 2.0 an API
// config takes (API Gateway refuses OpenAPI 3).
const openAPI = "c3dhZ2dlcjogIjIuMCIKaW5mbzoKICB0aXRsZTogY2NlLXRlc3QKICB2ZXJzaW9uOiAxLjAuMApzY2hlbWVzOgotIGh0dHBzCnByb2R1Y2VzOgotIGFwcGxpY2F0aW9uL2pzb24KcGF0aHM6CiAgL2hlbGxvOgogICAgZ2V0OgogICAgICBvcGVyYXRpb25JZDogaGVsbG8KICAgICAgeC1nb29nbGUtYmFja2VuZDoKICAgICAgICBhZGRyZXNzOiBodHRwczovL2V4YW1wbGUuY29tCiAgICAgIHJlc3BvbnNlczoKICAgICAgICAiMjAwIjoKICAgICAgICAgIGRlc2NyaXB0aW9uOiBPSwo="

func init() {
	kindtest.RegisterSetup("APIGatewayAPIConfig", func(t *testing.T, env *mockenv.Env) {
		for _, b := range []string{"apigatewayapiconfig", "apigatewayapiconfig-kept"} {
			makeAPI(t, env, mockenv.Name(b)+"-api")
		}
	})
	kindtest.RegisterSetup("APIGatewayGateway", func(t *testing.T, env *mockenv.Env) {
		for _, b := range []string{"apigatewaygateway", "apigatewaygateway-kept"} {
			n := mockenv.Name(b)
			makeAPI(t, env, n+"-api")
			makeConfig(t, env, n+"-api", n+"-config")
		}
	})
}

func apiClient(env *mockenv.Env) *discokind.API {
	a := &discokind.API{HTTP: env.HTTP, BaseURL: apiBase}
	if !mockenv.Live() {
		a.PollInterval = 200 * time.Millisecond
	}
	return a
}

func labels() map[string]any {
	out := map[string]any{}
	for k, v := range mockenv.Labels() {
		out[k] = v
	}
	return out
}

// makeAPI makes the API id and deletes it when the test ends.
func makeAPI(t *testing.T, env *mockenv.Env, id string) string {
	t.Helper()
	coll := "projects/" + mockenv.Project + "/locations/global/apis"
	make1(t, env, coll, "apiId", id, map[string]any{"displayName": "cce test api", "labels": labels()})
	return coll + "/" + id
}

// makeConfig makes the API config id of the API api and deletes it when the
// test ends.
func makeConfig(t *testing.T, env *mockenv.Env, api, id string) string {
	t.Helper()
	coll := "projects/" + mockenv.Project + "/locations/global/apis/" + api + "/configs"
	make1(t, env, coll, "apiConfigId", id, map[string]any{
		"displayName":      "cce test config",
		"labels":           labels(),
		"openapiDocuments": []any{map[string]any{"document": map[string]any{"path": "openapi.yaml", "contents": openAPI}}},
	})
	return coll + "/" + id
}

// make1 creates coll/id, waits for its operation, and deletes it when the
// test ends (cleanups run last-made first, so a config goes before its API).
func make1(t *testing.T, env *mockenv.Env, coll, idParam, id string, body map[string]any) {
	t.Helper()
	api := apiClient(env)
	name := coll + "/" + id
	op, err := api.Do(env.Ctx, http.MethodPost, coll, map[string]string{idParam: id}, body)
	if err == nil {
		_, err = api.Wait(env.Ctx, op)
	}
	if err != nil {
		t.Fatalf("creating %s: %v", name, err)
	}
	t.Logf("created %s", name)
	t.Cleanup(func() {
		if err := deleteAndWait(context.Background(), api, name); err != nil {
			t.Errorf("cleanup: deleting %s: %v", name, err)
		}
	})
}

// deleteAndWait deletes name and waits for the operation, trying again
// while the API answers that something still uses it (a gateway still
// going, a config under an API), for at most mockenv.Timeout(); gone
// already is done.
func deleteAndWait(ctx context.Context, api *discokind.API, name string) error {
	ctx, cancel := context.WithTimeout(ctx, mockenv.Timeout())
	defer cancel()
	for {
		op, err := api.Do(ctx, http.MethodDelete, name, nil, nil)
		if err == nil {
			_, err = api.Wait(ctx, op)
		}
		if err == nil || direct.IsNotFound(err) {
			return nil
		}
		if !discokind.InUse(err) {
			return err
		}
		select {
		case <-ctx.Done():
			return err
		case <-time.After(pollEvery()):
		}
	}
}

func pollEvery() time.Duration {
	if mockenv.Live() {
		return 15 * time.Second
	}
	return 200 * time.Millisecond
}

// TestAPIGatewayDeleteGatewayAndConfigTogether deletes a gateway and the
// API config it serves at once, as deleting a whole release does. The
// catalog's live run in hfl-dev-514205 found both deletes returning before
// their operations ended: the finalizers went at once, the config's delete
// failed in the cloud ("in use") while the gateway was still going, the
// config stayed there, FAILED, and its API could not go ("has nested
// resources"). Each delete must wait for its operation and try again while
// the config is in use; then both are gone from the cloud and the API can
// be deleted. In the fake, deletes take two seconds and refuse a resource
// in use, as the real API's do.
func TestAPIGatewayDeleteGatewayAndConfigTogether(t *testing.T) {
	const ns = "apigateway-together"
	env := mockenv.New(t, ns)
	if env.Disco != nil {
		env.Disco.SetDeleteDelay(2 * time.Second)
		env.Disco.SetRefuseInUse(true)
	}
	api := apiClient(env)
	n := mockenv.Name("apigw-together")
	apiName := makeAPI(t, env, n+"-api")

	config := &unstructured.Unstructured{Object: map[string]any{
		"spec": map[string]any{
			"projectRef":       map[string]any{"external": mockenv.Project},
			"location":         "global",
			"api":              n + "-api",
			"displayName":      "cce test config",
			"openapiDocuments": []any{map[string]any{"document": map[string]any{"path": "openapi.yaml", "contents": openAPI}}},
		},
	}}
	config.SetGroupVersionKind(apigatewayv1beta1.APIGatewayAPIConfigGVK)
	config.SetNamespace(ns)
	config.SetName(n + "-config")
	config.SetLabels(mockenv.Labels())
	configName := apiName + "/configs/" + n + "-config"

	gateway := &unstructured.Unstructured{Object: map[string]any{
		"spec": map[string]any{
			"projectRef":  map[string]any{"external": mockenv.Project},
			"location":    "us-central1",
			"apiConfig":   configName,
			"displayName": "cce test gateway",
		},
	}}
	gateway.SetGroupVersionKind(apigatewayv1beta1.APIGatewayGatewayGVK)
	gateway.SetNamespace(ns)
	gateway.SetName(n + "-gw")
	gateway.SetLabels(mockenv.Labels())
	gatewayName := "projects/" + mockenv.Project + "/locations/us-central1/gateways/" + n + "-gw"

	// Whatever the test leaves in the cloud goes before the API's cleanup
	// (registered later, so run earlier): the gateway, then the config.
	t.Cleanup(func() {
		for _, name := range []string{gatewayName, configName} {
			if err := deleteAndWait(context.Background(), api, name); err != nil {
				t.Errorf("cleanup: deleting %s: %v", name, err)
			}
		}
	})

	if err := env.Kube.Create(env.Ctx, config); err != nil {
		t.Fatal(err)
	}
	env.WaitReady(apigatewayv1beta1.APIGatewayAPIConfigGVK, ns, config.GetName())
	if err := env.Kube.Create(env.Ctx, gateway); err != nil {
		t.Fatal(err)
	}
	env.WaitReady(apigatewayv1beta1.APIGatewayGatewayGVK, ns, gateway.GetName())

	// Both at once.
	for _, u := range []*unstructured.Unstructured{gateway, config} {
		if err := env.Kube.Delete(env.Ctx, u); err != nil {
			t.Fatal(err)
		}
	}
	for _, u := range []*unstructured.Unstructured{gateway, config} {
		env.WaitGone(u)
	}

	// The finalizers went only once the cloud's resources were gone.
	for _, name := range []string{gatewayName, configName} {
		got, err := api.Get(env.Ctx, name)
		if !direct.IsNotFound(err) {
			t.Errorf("%s is still in the cloud after its object was deleted: %v %v", name, got["state"], err)
		}
	}

	// And the API has nothing left under it: its delete goes through at
	// the first try.
	op, err := api.Do(env.Ctx, http.MethodDelete, apiName, nil, nil)
	if err == nil {
		_, err = api.Wait(env.Ctx, op)
	}
	if err != nil {
		if strings.Contains(err.Error(), "nested") {
			t.Fatalf("the API still holds resources after both objects were deleted: %v", err)
		}
		t.Fatalf("deleting the API %s: %v", apiName, err)
	}
}
