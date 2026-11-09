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

package iampolicy

import (
	"strings"
	"testing"
)

// samples is, for every kind IAMPolicyMember cannot reference, a resource
// name and the URL its policy is read from. A new row needs one here: the
// test fails for a row without it.
var samples = map[string]struct{ name, getURL string }{
	"SecretManagerSecret":              {"projects/p/locations/europe-west1/secrets/s", "https://secretmanager.europe-west1.rep.googleapis.com/v1/projects/p/locations/europe-west1/secrets/s:getIamPolicy?options.requestedPolicyVersion=3"},
	"StorageManagedFolder":             {"projects/_/buckets/b/managedFolders/a/b/", "https://storage.googleapis.com/storage/v1/b/b/managedFolders/a%2Fb%2F/iam?optionsRequestedPolicyVersion=3"},
	"LoggingLogView":                   {"projects/p/locations/global/buckets/b/views/v", "https://logging.googleapis.com/v2/projects/p/locations/global/buckets/b/views/v:getIamPolicy"},
	"RunWorkerPool":                    {"projects/p/locations/us-central1/workerPools/w", "https://run.googleapis.com/v2/projects/p/locations/us-central1/workerPools/w:getIamPolicy?options.requestedPolicyVersion=3"},
	"PrivateCACAPool":                  {"projects/p/locations/us-central1/caPools/c", "https://privateca.googleapis.com/v1/projects/p/locations/us-central1/caPools/c:getIamPolicy?options.requestedPolicyVersion=3"},
	"CloudDeployDeliveryPipeline":      {"projects/p/locations/us-central1/deliveryPipelines/d", "https://clouddeploy.googleapis.com/v1/projects/p/locations/us-central1/deliveryPipelines/d:getIamPolicy?options.requestedPolicyVersion=3"},
	"CloudDeployTarget":                {"projects/p/locations/us-central1/targets/t", "https://clouddeploy.googleapis.com/v1/projects/p/locations/us-central1/targets/t:getIamPolicy?options.requestedPolicyVersion=3"},
	"DataformRepository":               {"projects/p/locations/us-central1/repositories/r", "https://dataform.googleapis.com/v1beta1/projects/p/locations/us-central1/repositories/r:getIamPolicy?options.requestedPolicyVersion=3"},
	"DataplexLake":                     {"projects/p/locations/us-central1/lakes/l", "https://dataplex.googleapis.com/v1/projects/p/locations/us-central1/lakes/l:getIamPolicy?options.requestedPolicyVersion=3"},
	"DataplexZone":                     {"projects/p/locations/us-central1/lakes/l/zones/z", "https://dataplex.googleapis.com/v1/projects/p/locations/us-central1/lakes/l/zones/z:getIamPolicy?options.requestedPolicyVersion=3"},
	"DataplexDataScan":                 {"projects/p/locations/us-central1/dataScans/s", "https://dataplex.googleapis.com/v1/projects/p/locations/us-central1/dataScans/s:getIamPolicy?options.requestedPolicyVersion=3"},
	"DataplexAspectType":               {"projects/p/locations/us-central1/aspectTypes/a", "https://dataplex.googleapis.com/v1/projects/p/locations/us-central1/aspectTypes/a:getIamPolicy?options.requestedPolicyVersion=3"},
	"WorkstationConfig":                {"projects/p/locations/us-central1/workstationClusters/c/workstationConfigs/w", "https://workstations.googleapis.com/v1/projects/p/locations/us-central1/workstationClusters/c/workstationConfigs/w:getIamPolicy?options.requestedPolicyVersion=3"},
	"Workstation":                      {"projects/p/locations/us-central1/workstationClusters/c/workstationConfigs/w/workstations/s", "https://workstations.googleapis.com/v1/projects/p/locations/us-central1/workstationClusters/c/workstationConfigs/w/workstations/s:getIamPolicy?options.requestedPolicyVersion=3"},
	"BigQueryAnalyticsHubDataExchange": {"projects/p/locations/us/dataExchanges/e", "https://analyticshub.googleapis.com/v1/projects/p/locations/us/dataExchanges/e:getIamPolicy"},
	"BigQueryAnalyticsHubListing":      {"projects/p/locations/us/dataExchanges/e/listings/l", "https://analyticshub.googleapis.com/v1/projects/p/locations/us/dataExchanges/e/listings/l:getIamPolicy"},
	"APIGatewayAPI":                    {"projects/p/locations/global/apis/a", "https://apigateway.googleapis.com/v1/projects/p/locations/global/apis/a:getIamPolicy?options.requestedPolicyVersion=3"},
	"APIGatewayAPIConfig":              {"projects/p/locations/global/apis/a/configs/c", "https://apigateway.googleapis.com/v1/projects/p/locations/global/apis/a/configs/c:getIamPolicy?options.requestedPolicyVersion=3"},
	"APIGatewayGateway":                {"projects/p/locations/us-central1/gateways/g", "https://apigateway.googleapis.com/v1/projects/p/locations/us-central1/gateways/g:getIamPolicy?options.requestedPolicyVersion=3"},
	"BinaryAuthorizationAttestor":      {"projects/p/attestors/a", "https://binaryauthorization.googleapis.com/v1/projects/p/attestors/a:getIamPolicy?options.requestedPolicyVersion=3"},
	"BigQueryConnectionConnection":     {"projects/p/locations/us/connections/c", "https://bigqueryconnection.googleapis.com/v1/projects/p/locations/us/connections/c:getIamPolicy"},
	"SecureSourceManagerInstance":      {"projects/p/locations/us-central1/instances/i", "https://securesourcemanager.googleapis.com/v1/projects/p/locations/us-central1/instances/i:getIamPolicy?options.requestedPolicyVersion=3"},
	"SecureSourceManagerRepository":    {"projects/p/locations/us-central1/repositories/r", "https://securesourcemanager.googleapis.com/v1/projects/p/locations/us-central1/repositories/r:getIamPolicy?options.requestedPolicyVersion=3"},
	"DataCatalogTagTemplate":           {"projects/p/locations/us-central1/tagTemplates/t", "https://datacatalog.googleapis.com/v1/projects/p/locations/us-central1/tagTemplates/t:getIamPolicy"},
	"CloudBuildConnection":             {"projects/p/locations/us-central1/connections/c", "https://cloudbuild.googleapis.com/v2/projects/p/locations/us-central1/connections/c:getIamPolicy?options.requestedPolicyVersion=3"},
	"WorkbenchInstance":                {"projects/p/locations/us-central1-a/instances/i", "https://notebooks.googleapis.com/v2/projects/p/locations/us-central1-a/instances/i:getIamPolicy?options.requestedPolicyVersion=3"},
	"EndpointsService":                 {"services/echo.endpoints.p.cloud.goog", "https://servicemanagement.googleapis.com/v1/services/echo.endpoints.p.cloud.goog:getIamPolicy"},
	"IAPTunnelInstance":                {"projects/p/iap_tunnel/zones/us-central1-a/instances/i", "https://iap.googleapis.com/v1/projects/p/iap_tunnel/zones/us-central1-a/instances/i:getIamPolicy"},
	"IAPWebCloudRunService":            {"projects/p/iap_web/cloud_run-us-central1/services/s", "https://iap.googleapis.com/v1/projects/p/iap_web/cloud_run-us-central1/services/s:getIamPolicy"},
	"ComputeBackendService":            {"projects/p/global/backendServices/b", "https://compute.googleapis.com/compute/v1/projects/p/global/backendServices/b/getIamPolicy?optionsRequestedPolicyVersion=3"},
	"ComputeMachineImage":              {"projects/p/global/machineImages/m", "https://compute.googleapis.com/compute/v1/projects/p/global/machineImages/m/getIamPolicy?optionsRequestedPolicyVersion=3"},
}

func TestEveryRowHasASample(t *testing.T) {
	for _, kind := range Kinds() {
		rt, _ := Lookup(kind)
		if !rt.NotInIAMPolicyMember {
			continue
		}
		if _, ok := samples[kind]; !ok {
			t.Errorf("%s is not in iampolicy.samples", kind)
		}
	}
	for kind := range samples {
		if _, ok := Lookup(kind); !ok {
			t.Errorf("samples has %s, which is not a resource type", kind)
		}
	}
}

func TestNamesAndURLs(t *testing.T) {
	for kind, s := range samples {
		rt, ok := Lookup(kind)
		if !ok {
			continue
		}
		got, err := rt.ResolveExternal(s.name)
		if err != nil || got != s.name {
			t.Errorf("%s: ResolveExternal(%q) = %q, %v", kind, s.name, got, err)
			continue
		}
		// A name of another shape is refused.
		if _, err := rt.ResolveExternal("not/a/" + kind); err == nil {
			t.Errorf("%s: a name of no known shape was accepted", kind)
		}
		c := &Client{}
		base, err := c.url(rt, s.name, "getIamPolicy")
		if err != nil {
			t.Errorf("%s: %v", kind, err)
			continue
		}
		want := s.getURL
		if i := strings.Index(want, "?"); i >= 0 {
			want = want[:i]
		}
		if base != want {
			t.Errorf("%s: policy URL\n got  %s\n want %s", kind, base, want)
		}
	}
}

func TestComputeBackendServiceRegionalName(t *testing.T) {
	rt, _ := Lookup("ComputeBackendService")
	const n = "projects/p/regions/us-east1/backendServices/b"
	if got, err := rt.ResolveExternal(n); err != nil || got != n {
		t.Fatalf("ResolveExternal(%q) = %q, %v", n, got, err)
	}
	for _, c := range []struct {
		location, want string
	}{
		{"", "projects/p/global/backendServices/b"},
		{"global", "projects/p/global/backendServices/b"},
		{"us-east1", n},
	} {
		got, err := rt.ResolveObject(ObjectParts{Project: "p", Location: c.location, ID: "b"})
		if err != nil || got != c.want {
			t.Errorf("location %q = %q, %v; want %q", c.location, got, err, c.want)
		}
	}
}

func TestManagedFolderFromObject(t *testing.T) {
	rt, _ := Lookup("StorageManagedFolder")
	got, err := rt.ResolveObject(ObjectParts{ExternalRef: "projects/p/buckets/b/managedfolders/a/b"})
	if want := "projects/_/buckets/b/managedFolders/a/b/"; err != nil || got != want {
		t.Errorf("= %q, %v; want %q", got, err, want)
	}
	if _, err := rt.ResolveObject(ObjectParts{}); err == nil {
		t.Error("an object with no externalRef must not resolve")
	}
	// The URL is the same with or without the trailing slash.
	c := &Client{}
	a, _ := c.url(rt, "projects/_/buckets/b/managedFolders/f", "")
	b, _ := c.url(rt, "projects/_/buckets/b/managedFolders/f/", "")
	if a != b || !strings.HasSuffix(a, "/managedFolders/f%2F/iam") {
		t.Errorf("urls %s and %s", a, b)
	}
}

func TestLocatedNameFromObject(t *testing.T) {
	rt, _ := Lookup("RunWorkerPool")
	got, err := rt.ResolveObject(ObjectParts{Project: "p", Location: "us-central1", ID: "w"})
	if want := "projects/p/locations/us-central1/workerPools/w"; err != nil || got != want {
		t.Errorf("= %q, %v; want %q", got, err, want)
	}
	if _, err := rt.ResolveObject(ObjectParts{Project: "p", ID: "w"}); err == nil {
		t.Error("a located resource with no location must not resolve")
	}
}
