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

// Where Config Connector has only an alpha kind for a resource, ours
// (v1beta1, docs/design.md decision 26) comes first in the row's GVKs, so a
// resourceRef by name finds our object when both exist.
//
// The resource types Config Connector's IAMPolicyMember cannot reference
// (the seed catalog's rank 2, docs/design.md 6.5): every one is marked
// NotInIAMPolicyMember. Written here from each service's REST reference
// and from Terraform's google_*_iam_binding resources for the same types
// (read, not copied), for the paths, the verbs and the name patterns.
//
// Rows the tests exercise end to end, on the fake cloud and on a real
// project: SecretManagerSecret (regional), StorageManagedFolder and
// ComputeBackendService (docs/live-runs.md). The others are checked for
// their names and URLs (iampolicy_test.go) but have not been called
// against the real API: a wrong path or verb shows as the API's own error
// on the object's Ready condition, and is a one-row fix.

import (
	"fmt"
	"regexp"
	"strings"

	"k8s.io/apimachinery/pkg/runtime/schema"
)

// cnrm is a Config Connector kind.
func cnrm(service, version, kind string) schema.GroupVersionKind {
	return gvk(service+".cnrm.cloud.google.com", version, kind)
}

// inLocation names a resource that is directly under a project and a
// location: projects/{project}/locations/{location}/{collection}/{id}.
func inLocation(collection string) func(ObjectParts) (string, error) {
	return func(o ObjectParts) (string, error) {
		if o.Project == "" || o.Location == "" {
			return "", fmt.Errorf("the object has no project or no spec.location; use resourceRef.external")
		}
		return "projects/" + o.Project + "/locations/" + o.Location + "/" + collection + "/" + o.ID, nil
	}
}

// mixin is a row of the common shape: the google.iam.v1 mixin under a
// versioned root, read with GET unless post.
func mixin(kind, host, version string, post bool, name string, gvks []schema.GroupVersionKind, from func(ObjectParts) (string, error)) *ResourceType {
	return &ResourceType{
		Kind: kind, NotInIAMPolicyMember: true, GVKs: gvks,
		Host: host, Version: version, GetWithPost: post,
		Name: re(name), FromObject: from,
	}
}

var managedFolderExternal = regexp.MustCompile(`^projects/[^/]+/buckets/([^/]+)/managedfolders/(.+)$`)

func moreTypes() []*ResourceType {
	const loc = `projects/[^/]+/locations/[^/]+`
	return []*ResourceType{
		{
			// A managed folder's name ends with a slash in the API; the
			// bucket's name is in the path, the project is "_".
			Kind: "StorageManagedFolder", NotInIAMPolicyMember: true,
			GVKs:     []schema.GroupVersionKind{cnrm("storage", "v1alpha1", "StorageManagedFolder")},
			Protocol: Storage, Host: "storage.googleapis.com",
			Name: re(`projects/_/buckets/[^/]+/managedFolders/.+`),
			FromObject: func(o ObjectParts) (string, error) {
				// status.externalRef is projects/{project}/buckets/{bucket}/managedfolders/{folder}.
				m := managedFolderExternal.FindStringSubmatch(o.ExternalRef)
				if m == nil {
					return "", fmt.Errorf("the StorageManagedFolder has no status.externalRef yet")
				}
				return "projects/_/buckets/" + m[1] + "/managedFolders/" + strings.TrimSuffix(m[2], "/") + "/", nil
			},
		},
		mixin("LoggingLogView", "logging.googleapis.com", "v2", true,
			loc+`/buckets/[^/]+/views/[^/]+`,
			[]schema.GroupVersionKind{cnrm("logging", "v1beta1", "LoggingLogView")}, nil),
		mixin("RunWorkerPool", "run.googleapis.com", "v2", false,
			loc+`/workerPools/[^/]+`,
			[]schema.GroupVersionKind{cnrm("run", "v1alpha1", "RunWorkerPool")}, inLocation("workerPools")),
		mixin("PrivateCACAPool", "privateca.googleapis.com", "v1", false,
			loc+`/caPools/[^/]+`,
			[]schema.GroupVersionKind{cnrm("privateca", "v1beta1", "PrivateCACAPool")}, inLocation("caPools")),
		mixin("CloudDeployDeliveryPipeline", "clouddeploy.googleapis.com", "v1", false,
			loc+`/deliveryPipelines/[^/]+`,
			[]schema.GroupVersionKind{cnrm("clouddeploy", "v1beta1", "CloudDeployDeliveryPipeline")}, inLocation("deliveryPipelines")),
		mixin("CloudDeployTarget", "clouddeploy.googleapis.com", "v1", false,
			loc+`/targets/[^/]+`,
			[]schema.GroupVersionKind{gvk("clouddeploy.gcp.hfl.dev", "v1beta1", "CloudDeployTarget"), cnrm("clouddeploy", "v1alpha1", "CloudDeployTarget")}, inLocation("targets")),
		mixin("DataformRepository", "dataform.googleapis.com", "v1beta1", false,
			loc+`/repositories/[^/]+`,
			[]schema.GroupVersionKind{cnrm("dataform", "v1beta1", "DataformRepository")}, inLocation("repositories")),
		mixin("CloudFunctions2Function", "cloudfunctions.googleapis.com", "v2", false,
			loc+`/functions/[^/]+`,
			[]schema.GroupVersionKind{gvk("cloudfunctions2.gcp.hfl.dev", "v1beta1", "CloudFunctions2Function"), cnrm("cloudfunctions2", "v1alpha1", "CloudFunctions2Function")}, inLocation("functions")),
		mixin("DataplexLake", "dataplex.googleapis.com", "v1", false,
			loc+`/lakes/[^/]+`,
			[]schema.GroupVersionKind{gvk("dataplex.gcp.hfl.dev", "v1beta1", "DataplexLake"), cnrm("dataplex", "v1alpha1", "DataplexLake")}, inLocation("lakes")),
		mixin("DataplexZone", "dataplex.googleapis.com", "v1", false,
			loc+`/lakes/[^/]+/zones/[^/]+`,
			[]schema.GroupVersionKind{gvk("dataplex.gcp.hfl.dev", "v1beta1", "DataplexZone"), cnrm("dataplex", "v1alpha1", "DataplexZone")}, nil),
		mixin("DataplexDataScan", "dataplex.googleapis.com", "v1", false,
			loc+`/dataScans/[^/]+`,
			[]schema.GroupVersionKind{gvk("dataplex.gcp.hfl.dev", "v1beta1", "DataplexDataScan"), cnrm("dataplex", "v1alpha1", "DataplexDataScan")}, inLocation("dataScans")),
		mixin("DataplexAspectType", "dataplex.googleapis.com", "v1", false,
			loc+`/aspectTypes/[^/]+`,
			[]schema.GroupVersionKind{gvk("dataplex.gcp.hfl.dev", "v1beta1", "DataplexAspectType"), cnrm("dataplex", "v1alpha1", "DataplexAspectType")}, inLocation("aspectTypes")),
		mixin("WorkstationConfig", "workstations.googleapis.com", "v1", false,
			loc+`/workstationClusters/[^/]+/workstationConfigs/[^/]+`,
			[]schema.GroupVersionKind{cnrm("workstations", "v1beta1", "WorkstationConfig")}, nil),
		mixin("Workstation", "workstations.googleapis.com", "v1", false,
			loc+`/workstationClusters/[^/]+/workstationConfigs/[^/]+/workstations/[^/]+`,
			[]schema.GroupVersionKind{cnrm("workstations", "v1beta1", "Workstation")}, nil),
		mixin("BigQueryAnalyticsHubDataExchange", "analyticshub.googleapis.com", "v1", true,
			loc+`/dataExchanges/[^/]+`,
			[]schema.GroupVersionKind{cnrm("bigqueryanalyticshub", "v1beta1", "BigQueryAnalyticsHubDataExchange")}, inLocation("dataExchanges")),
		mixin("BigQueryAnalyticsHubListing", "analyticshub.googleapis.com", "v1", true,
			loc+`/dataExchanges/[^/]+/listings/[^/]+`,
			[]schema.GroupVersionKind{cnrm("bigqueryanalyticshub", "v1beta1", "BigQueryAnalyticsHubListing")}, nil),
		mixin("APIGatewayAPI", "apigateway.googleapis.com", "v1", false,
			`projects/[^/]+/locations/global/apis/[^/]+`,
			[]schema.GroupVersionKind{cnrm("apigateway", "v1beta1", "APIGatewayAPI")},
			func(o ObjectParts) (string, error) {
				return "projects/" + o.Project + "/locations/global/apis/" + o.ID, nil
			}),
		mixin("APIGatewayAPIConfig", "apigateway.googleapis.com", "v1", false,
			`projects/[^/]+/locations/global/apis/[^/]+/configs/[^/]+`,
			[]schema.GroupVersionKind{gvk("apigateway.gcp.hfl.dev", "v1beta1", "APIGatewayAPIConfig"), cnrm("apigateway", "v1alpha1", "APIGatewayAPIConfig")}, nil),
		mixin("APIGatewayGateway", "apigateway.googleapis.com", "v1", false,
			loc+`/gateways/[^/]+`,
			[]schema.GroupVersionKind{gvk("apigateway.gcp.hfl.dev", "v1beta1", "APIGatewayGateway"), cnrm("apigateway", "v1alpha1", "APIGatewayGateway")}, inLocation("gateways")),
		mixin("BinaryAuthorizationAttestor", "binaryauthorization.googleapis.com", "v1", false,
			`projects/[^/]+/attestors/[^/]+`,
			[]schema.GroupVersionKind{cnrm("binaryauthorization", "v1beta1", "BinaryAuthorizationAttestor")},
			func(o ObjectParts) (string, error) { return "projects/" + o.Project + "/attestors/" + o.ID, nil }),
		mixin("BigQueryConnectionConnection", "bigqueryconnection.googleapis.com", "v1", true,
			loc+`/connections/[^/]+`,
			[]schema.GroupVersionKind{cnrm("bigqueryconnection", "v1beta1", "BigQueryConnectionConnection")}, inLocation("connections")),
		mixin("SecureSourceManagerInstance", "securesourcemanager.googleapis.com", "v1", false,
			loc+`/instances/[^/]+`,
			[]schema.GroupVersionKind{cnrm("securesourcemanager", "v1beta1", "SecureSourceManagerInstance")}, inLocation("instances")),
		mixin("SecureSourceManagerRepository", "securesourcemanager.googleapis.com", "v1", false,
			loc+`/repositories/[^/]+`,
			[]schema.GroupVersionKind{cnrm("securesourcemanager", "v1beta1", "SecureSourceManagerRepository")}, inLocation("repositories")),
		mixin("DataCatalogTagTemplate", "datacatalog.googleapis.com", "v1", true,
			loc+`/tagTemplates/[^/]+`,
			[]schema.GroupVersionKind{cnrm("datacatalog", "v1alpha1", "DataCatalogTagTemplate")}, inLocation("tagTemplates")),
		mixin("CloudBuildConnection", "cloudbuild.googleapis.com", "v2", false,
			loc+`/connections/[^/]+`,
			[]schema.GroupVersionKind{gvk("cloudbuild.gcp.hfl.dev", "v1beta1", "CloudBuildConnection"), cnrm("cloudbuild", "v1alpha1", "CloudBuildConnection")}, inLocation("connections")),
		// No Config Connector kind: external only.
		mixin("WorkbenchInstance", "notebooks.googleapis.com", "v2", false,
			loc+`/instances/[^/]+`, nil, nil),
		// Ours (servicemanagement.gcp.hfl.dev); Config Connector has none.
		mixin("EndpointsService", "servicemanagement.googleapis.com", "v1", true,
			`services/[^/]+`,
			[]schema.GroupVersionKind{gvk("servicemanagement.gcp.hfl.dev", "v1alpha1", "EndpointsService")},
			func(o ObjectParts) (string, error) {
				if o.ExternalRef != "" {
					return o.ExternalRef, nil
				}
				return "services/" + o.ID, nil
			}),
		mixin("IAPTunnelInstance", "iap.googleapis.com", "v1", true,
			`projects/[^/]+/iap_tunnel/zones/[^/]+/instances/[^/]+`, nil, nil),
		mixin("IAPWebCloudRunService", "iap.googleapis.com", "v1", true,
			`projects/[^/]+/iap_web/cloud_run-[^/]+/services/[^/]+`, nil, nil),

		// A BigQuery dataset, whose policy is its access list (bigquery.go).
		{
			Kind: "BigQueryDataset", NotInIAMPolicyMember: true,
			GVKs:     []schema.GroupVersionKind{cnrm("bigquery", "v1beta1", "BigQueryDataset")},
			Protocol: BigQuery, Host: "bigquery.googleapis.com", Version: "v2",
			Name: re(`projects/[^/]+/datasets/[A-Za-z0-9_]+`),
			FromObject: func(o ObjectParts) (string, error) {
				return "projects/" + o.Project + "/datasets/" + o.ID, nil
			},
		},

		// Compute Engine, whose policy is read and set with its own verbs.
		{
			Kind: "ComputeBackendService", NotInIAMPolicyMember: true,
			GVKs:     []schema.GroupVersionKind{cnrm("compute", "v1beta1", "ComputeBackendService")},
			Protocol: Compute, Host: "compute.googleapis.com", Version: "v1",
			Name: re(`projects/[^/]+/(?:global|regions/[^/]+)/backendServices/[^/]+`),
			FromObject: func(o ObjectParts) (string, error) {
				if o.Location == "" || o.Location == "global" {
					return "projects/" + o.Project + "/global/backendServices/" + o.ID, nil
				}
				return "projects/" + o.Project + "/regions/" + o.Location + "/backendServices/" + o.ID, nil
			},
		},
		{
			Kind: "ComputeMachineImage", NotInIAMPolicyMember: true,
			GVKs:     []schema.GroupVersionKind{cnrm("compute", "v1alpha1", "ComputeMachineImage")},
			Protocol: Compute, Host: "compute.googleapis.com", Version: "v1",
			Name: re(`projects/[^/]+/global/machineImages/[^/]+`),
			FromObject: func(o ObjectParts) (string, error) {
				return "projects/" + o.Project + "/global/machineImages/" + o.ID, nil
			},
		},
	}
}
