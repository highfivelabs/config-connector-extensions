# config-connector-extensions

Kubernetes kinds for Google Cloud that
[Config Connector](https://github.com/GoogleCloudPlatform/k8s-config-connector)
(KCC) does not have yet, or has without a field a workload needs. Install
it beside Config Connector; it runs Config Connector's own reconciler for
a few extra kinds, in its own API groups, and retires each kind when
Config Connector ships it.

config-connector-extensions is not affiliated with or endorsed by Google.
"Config Connector" names the project it extends.

<!-- coverage:start -->
## Coverage

How much of Google Cloud's API can be managed as Kubernetes resources, counted in attributes (writable fields of a resource's create request) across 207 services and 1244 resources. **Official** is what Config Connector's CRDs at the pin (`UPSTREAM.md`) cover; **ours beyond official** is what only this repository's kinds cover; **uncovered** is in the API and in neither. The counts come from matching CRD fields to API fields by name, so they are estimates: see [docs/coverage](docs/coverage/README.md) for the method, its error and how to regenerate.

![All services](docs/coverage/bars/_total.svg)

Services do not matter equally, so the total misleads. They are in three tiers ([`priority.csv`](docs/coverage/priority.csv), editable): **tier 1** the services the foundational modules use, **tier 2** the services other tracked modules use, **tier 3** the rest. The services counted are Google Cloud infrastructure APIs; 116 non-cloud APIs in Discovery (Workspace, Ads, Play, Maps, YouTube, ...) are left out ([how](docs/coverage/README.md#tiers)).

![Coverage by tier](docs/coverage/bars/_tiers.svg)

<sub>Blue: official. Green: ours beyond official. Grey: uncovered. Each bar reads official / ours beyond / uncovered. Rollup: [`tiers.csv`](docs/coverage/tiers.csv), every service: [`services.csv`](docs/coverage/services.csv).</sub>

Tier 1, 32 services, most covered attributes first:

| Tier | Service | Coverage | Official | Ours beyond | Uncovered |
| ---: | --- | --- | ---: | ---: | ---: |
| 1 | `compute.googleapis.com` | ![compute.googleapis.com](docs/coverage/bars/compute.googleapis.com.svg) | 47.6% | 0.1% | 52.3% |
| 1 | `monitoring.googleapis.com` | ![monitoring.googleapis.com](docs/coverage/bars/monitoring.googleapis.com.svg) | 45.7% | 0.0% | 54.3% |
| 1 | `container.googleapis.com` | ![container.googleapis.com](docs/coverage/bars/container.googleapis.com.svg) | 29.2% | 0.0% | 70.8% |
| 1 | `run.googleapis.com` | ![run.googleapis.com](docs/coverage/bars/run.googleapis.com.svg) | 38.1% | 3.3% | 58.6% |
| 1 | `bigquery.googleapis.com` | ![bigquery.googleapis.com](docs/coverage/bars/bigquery.googleapis.com.svg) | 26.6% | 0.0% | 73.4% |
| 1 | `sqladmin.googleapis.com` | ![sqladmin.googleapis.com](docs/coverage/bars/sqladmin.googleapis.com.svg) | 41.6% | 0.4% | 58.0% |
| 1 | `networkconnectivity.googleapis.com` | ![networkconnectivity.googleapis.com](docs/coverage/bars/networkconnectivity.googleapis.com.svg) | 60.1% | 0.0% | 39.9% |
| 1 | `gkebackup.googleapis.com` | ![gkebackup.googleapis.com](docs/coverage/bars/gkebackup.googleapis.com.svg) | 90.8% | 0.0% | 9.2% |
| 1 | `dns.googleapis.com` | ![dns.googleapis.com](docs/coverage/bars/dns.googleapis.com.svg) | 29.5% | 0.0% | 70.5% |
| 1 | `eventarc.googleapis.com` | ![eventarc.googleapis.com](docs/coverage/bars/eventarc.googleapis.com.svg) | 88.5% | 0.0% | 11.5% |
| 1 | `accesscontextmanager.googleapis.com` | ![accesscontextmanager.googleapis.com](docs/coverage/bars/accesscontextmanager.googleapis.com.svg) | 51.9% | 0.0% | 48.1% |
| 1 | `storage.googleapis.com` | ![storage.googleapis.com](docs/coverage/bars/storage.googleapis.com.svg) | 27.7% | 4.0% | 68.3% |
| 1 | `artifactregistry.googleapis.com` | ![artifactregistry.googleapis.com](docs/coverage/bars/artifactregistry.googleapis.com.svg) | 68.3% | 14.3% | 17.5% |
| 1 | `iam.googleapis.com` | ![iam.googleapis.com](docs/coverage/bars/iam.googleapis.com.svg) | 35.4% | 8.0% | 56.6% |
| 1 | `logging.googleapis.com` | ![logging.googleapis.com](docs/coverage/bars/logging.googleapis.com.svg) | 9.3% | 0.0% | 90.7% |
| 1 | `pubsub.googleapis.com` | ![pubsub.googleapis.com](docs/coverage/bars/pubsub.googleapis.com.svg) | 36.9% | 0.0% | 63.1% |
| 1 | `securitycenter.googleapis.com` | ![securitycenter.googleapis.com](docs/coverage/bars/securitycenter.googleapis.com.svg) | 7.6% | 0.0% | 92.4% |
| 1 | `orgpolicy.googleapis.com` | ![orgpolicy.googleapis.com](docs/coverage/bars/orgpolicy.googleapis.com.svg) | 26.4% | 0.0% | 73.6% |
| 1 | `cloudasset.googleapis.com` | ![cloudasset.googleapis.com](docs/coverage/bars/cloudasset.googleapis.com.svg) | 100.0% | 0.0% | 0.0% |
| 1 | `securitycentermanagement.googleapis.com` | ![securitycentermanagement.googleapis.com](docs/coverage/bars/securitycentermanagement.googleapis.com.svg) | 100.0% | 0.0% | 0.0% |
| 1 | `cloudresourcemanager.googleapis.com` | ![cloudresourcemanager.googleapis.com](docs/coverage/bars/cloudresourcemanager.googleapis.com.svg) | 36.4% | 0.0% | 63.6% |
| 1 | `secretmanager.googleapis.com` | ![secretmanager.googleapis.com](docs/coverage/bars/secretmanager.googleapis.com.svg) | 33.3% | 33.3% | 33.3% |
| 1 | `modelarmor.googleapis.com` | ![modelarmor.googleapis.com](docs/coverage/bars/modelarmor.googleapis.com.svg) | 65.5% | 0.0% | 34.5% |
| 1 | `cloudkms.googleapis.com` | ![cloudkms.googleapis.com](docs/coverage/bars/cloudkms.googleapis.com.svg) | 42.9% | 0.0% | 57.1% |
| 1 | `bigqueryreservation.googleapis.com` | ![bigqueryreservation.googleapis.com](docs/coverage/bars/bigqueryreservation.googleapis.com.svg) | 53.1% | 0.0% | 46.9% |
| 1 | `privilegedaccessmanager.googleapis.com` | ![privilegedaccessmanager.googleapis.com](docs/coverage/bars/privilegedaccessmanager.googleapis.com.svg) | 61.1% | 0.0% | 38.9% |
| 1 | `vpcaccess.googleapis.com` | ![vpcaccess.googleapis.com](docs/coverage/bars/vpcaccess.googleapis.com.svg) | 100.0% | 0.0% | 0.0% |
| 1 | `assuredworkloads.googleapis.com` | ![assuredworkloads.googleapis.com](docs/coverage/bars/assuredworkloads.googleapis.com.svg) | 45.0% | 0.0% | 55.0% |
| 1 | `servicenetworking.googleapis.com` | ![servicenetworking.googleapis.com](docs/coverage/bars/servicenetworking.googleapis.com.svg) | 100.0% | 0.0% | 0.0% |
| 1 | `essentialcontacts.googleapis.com` | ![essentialcontacts.googleapis.com](docs/coverage/bars/essentialcontacts.googleapis.com.svg) | 33.3% | 0.0% | 66.7% |
| 1 | `cloudbilling.googleapis.com` | ![cloudbilling.googleapis.com](docs/coverage/bars/cloudbilling.googleapis.com.svg) | 13.3% | 0.0% | 86.7% |
| 1 | `cloudquotas.googleapis.com` | ![cloudquotas.googleapis.com](docs/coverage/bars/cloudquotas.googleapis.com.svg) | 0.0% | 0.0% | 100.0% |

<details><summary>Tier 2: 44 services other modules use</summary>

| Tier | Service | Coverage | Official | Ours beyond | Uncovered |
| ---: | --- | --- | ---: | ---: | ---: |
| 2 | `dlp.googleapis.com` | ![dlp.googleapis.com](docs/coverage/bars/dlp.googleapis.com.svg) | 32.9% | 0.0% | 67.1% |
| 2 | `aiplatform.googleapis.com` | ![aiplatform.googleapis.com](docs/coverage/bars/aiplatform.googleapis.com.svg) | 16.6% | 0.0% | 83.4% |
| 2 | `dialogflow.googleapis.com` | ![dialogflow.googleapis.com](docs/coverage/bars/dialogflow.googleapis.com.svg) | 37.9% | 0.0% | 62.1% |
| 2 | `dataproc.googleapis.com` | ![dataproc.googleapis.com](docs/coverage/bars/dataproc.googleapis.com.svg) | 47.5% | 0.0% | 52.5% |
| 2 | `networksecurity.googleapis.com` | ![networksecurity.googleapis.com](docs/coverage/bars/networksecurity.googleapis.com.svg) | 72.2% | 0.0% | 27.8% |
| 2 | `dataplex.googleapis.com` | ![dataplex.googleapis.com](docs/coverage/bars/dataplex.googleapis.com.svg) | 84.1% | 0.0% | 15.9% |
| 2 | `cloudbuild.googleapis.com` | ![cloudbuild.googleapis.com](docs/coverage/bars/cloudbuild.googleapis.com.svg) | 38.7% | 0.0% | 61.3% |
| 2 | `networkservices.googleapis.com` | ![networkservices.googleapis.com](docs/coverage/bars/networkservices.googleapis.com.svg) | 54.2% | 0.0% | 45.8% |
| 2 | `privateca.googleapis.com` | ![privateca.googleapis.com](docs/coverage/bars/privateca.googleapis.com.svg) | 71.3% | 0.0% | 28.7% |
| 2 | `containeranalysis.googleapis.com` | ![containeranalysis.googleapis.com](docs/coverage/bars/containeranalysis.googleapis.com.svg) | 13.1% | 0.0% | 86.9% |
| 2 | `discoveryengine.googleapis.com` | ![discoveryengine.googleapis.com](docs/coverage/bars/discoveryengine.googleapis.com.svg) | 24.3% | 0.0% | 75.7% |
| 2 | `clouddeploy.googleapis.com` | ![clouddeploy.googleapis.com](docs/coverage/bars/clouddeploy.googleapis.com.svg) | 56.7% | 0.0% | 43.3% |
| 2 | `notebooks.googleapis.com` | ![notebooks.googleapis.com](docs/coverage/bars/notebooks.googleapis.com.svg) | 91.7% | 0.0% | 8.3% |
| 2 | `apigee.googleapis.com` | ![apigee.googleapis.com](docs/coverage/bars/apigee.googleapis.com.svg) | 16.9% | 0.0% | 83.1% |
| 2 | `chronicle.googleapis.com` | ![chronicle.googleapis.com](docs/coverage/bars/chronicle.googleapis.com.svg) | 15.6% | 0.0% | 84.4% |
| 2 | `vmwareengine.googleapis.com` | ![vmwareengine.googleapis.com](docs/coverage/bars/vmwareengine.googleapis.com.svg) | 79.8% | 0.0% | 20.2% |
| 2 | `tpu.googleapis.com` | ![tpu.googleapis.com](docs/coverage/bars/tpu.googleapis.com.svg) | 91.5% | 0.0% | 8.5% |
| 2 | `alloydb.googleapis.com` | ![alloydb.googleapis.com](docs/coverage/bars/alloydb.googleapis.com.svg) | 56.2% | 0.0% | 43.8% |
| 2 | `dataform.googleapis.com` | ![dataform.googleapis.com](docs/coverage/bars/dataform.googleapis.com.svg) | 53.8% | 0.0% | 46.2% |
| 2 | `cloudfunctions.googleapis.com` | ![cloudfunctions.googleapis.com](docs/coverage/bars/cloudfunctions.googleapis.com.svg) | 80.3% | 0.0% | 19.7% |
| 2 | `workstations.googleapis.com` | ![workstations.googleapis.com](docs/coverage/bars/workstations.googleapis.com.svg) | 52.3% | 0.0% | 47.7% |
| 2 | `bigtableadmin.googleapis.com` | ![bigtableadmin.googleapis.com](docs/coverage/bars/bigtableadmin.googleapis.com.svg) | 30.4% | 0.0% | 69.6% |
| 2 | `spanner.googleapis.com` | ![spanner.googleapis.com](docs/coverage/bars/spanner.googleapis.com.svg) | 42.5% | 0.0% | 57.5% |
| 2 | `certificatemanager.googleapis.com` | ![certificatemanager.googleapis.com](docs/coverage/bars/certificatemanager.googleapis.com.svg) | 75.0% | 0.0% | 25.0% |
| 2 | `managedkafka.googleapis.com` | ![managedkafka.googleapis.com](docs/coverage/bars/managedkafka.googleapis.com.svg) | 54.5% | 0.0% | 45.5% |
| 2 | `gkehub.googleapis.com` | ![gkehub.googleapis.com](docs/coverage/bars/gkehub.googleapis.com.svg) | 16.5% | 0.0% | 83.5% |
| 2 | `analyticshub.googleapis.com` | ![analyticshub.googleapis.com](docs/coverage/bars/analyticshub.googleapis.com.svg) | 66.7% | 0.0% | 33.3% |
| 2 | `backupdr.googleapis.com` | ![backupdr.googleapis.com](docs/coverage/bars/backupdr.googleapis.com.svg) | 40.0% | 0.0% | 60.0% |
| 2 | `binaryauthorization.googleapis.com` | ![binaryauthorization.googleapis.com](docs/coverage/bars/binaryauthorization.googleapis.com.svg) | 75.7% | 0.0% | 24.3% |
| 2 | `cloudscheduler.googleapis.com` | ![cloudscheduler.googleapis.com](docs/coverage/bars/cloudscheduler.googleapis.com.svg) | 100.0% | 0.0% | 0.0% |
| 2 | `securesourcemanager.googleapis.com` | ![securesourcemanager.googleapis.com](docs/coverage/bars/securesourcemanager.googleapis.com.svg) | 58.3% | 0.0% | 41.7% |
| 2 | `billingbudgets.googleapis.com` | ![billingbudgets.googleapis.com](docs/coverage/bars/billingbudgets.googleapis.com.svg) | 85.7% | 0.0% | 14.3% |
| 2 | `apigateway.googleapis.com` | ![apigateway.googleapis.com](docs/coverage/bars/apigateway.googleapis.com.svg) | 81.0% | 0.0% | 19.0% |
| 2 | `bigqueryconnection.googleapis.com` | ![bigqueryconnection.googleapis.com](docs/coverage/bars/bigqueryconnection.googleapis.com.svg) | 34.8% | 0.0% | 65.2% |
| 2 | `firestore.googleapis.com` | ![firestore.googleapis.com](docs/coverage/bars/firestore.googleapis.com.svg) | 34.2% | 0.0% | 65.8% |
| 2 | `cloudtasks.googleapis.com` | ![cloudtasks.googleapis.com](docs/coverage/bars/cloudtasks.googleapis.com.svg) | 24.0% | 0.0% | 76.0% |
| 2 | `datafusion.googleapis.com` | ![datafusion.googleapis.com](docs/coverage/bars/datafusion.googleapis.com.svg) | 33.3% | 0.0% | 66.7% |
| 2 | `biglake.googleapis.com` | ![biglake.googleapis.com](docs/coverage/bars/biglake.googleapis.com.svg) | 100.0% | 0.0% | 0.0% |
| 2 | `cloudidentity.googleapis.com` | ![cloudidentity.googleapis.com](docs/coverage/bars/cloudidentity.googleapis.com.svg) | 21.3% | 0.0% | 78.7% |
| 2 | `datacatalog.googleapis.com` | ![datacatalog.googleapis.com](docs/coverage/bars/datacatalog.googleapis.com.svg) | 100.0% | 0.0% | 0.0% |
| 2 | `servicedirectory.googleapis.com` | ![servicedirectory.googleapis.com](docs/coverage/bars/servicedirectory.googleapis.com.svg) | 77.8% | 0.0% | 22.2% |
| 2 | `workflows.googleapis.com` | ![workflows.googleapis.com](docs/coverage/bars/workflows.googleapis.com.svg) | 70.0% | 0.0% | 30.0% |
| 2 | `looker.googleapis.com` | ![looker.googleapis.com](docs/coverage/bars/looker.googleapis.com.svg) | 0.0% | 0.0% | 100.0% |
| 2 | `servicemanagement.googleapis.com` | ![servicemanagement.googleapis.com](docs/coverage/bars/servicemanagement.googleapis.com.svg) | 0.0% | 0.0% | 100.0% |

</details>

<details><summary>Tier 3: 90 other services where something is covered</summary>

| Tier | Service | Coverage | Official | Ours beyond | Uncovered |
| ---: | --- | --- | ---: | ---: | ---: |
| 3 | `transcoder.googleapis.com` | ![transcoder.googleapis.com](docs/coverage/bars/transcoder.googleapis.com.svg) | 99.7% | 0.0% | 0.3% |
| 3 | `apihub.googleapis.com` | ![apihub.googleapis.com](docs/coverage/bars/apihub.googleapis.com.svg) | 54.7% | 0.0% | 45.3% |
| 3 | `datamigration.googleapis.com` | ![datamigration.googleapis.com](docs/coverage/bars/datamigration.googleapis.com.svg) | 62.5% | 0.0% | 37.5% |
| 3 | `datastream.googleapis.com` | ![datastream.googleapis.com](docs/coverage/bars/datastream.googleapis.com.svg) | 38.6% | 0.0% | 61.4% |
| 3 | `osconfig.googleapis.com` | ![osconfig.googleapis.com](docs/coverage/bars/osconfig.googleapis.com.svg) | 33.6% | 0.0% | 66.4% |
| 3 | `ces.googleapis.com` | ![ces.googleapis.com](docs/coverage/bars/ces.googleapis.com.svg) | 22.4% | 0.0% | 77.6% |
| 3 | `oracledatabase.googleapis.com` | ![oracledatabase.googleapis.com](docs/coverage/bars/oracledatabase.googleapis.com.svg) | 24.1% | 0.0% | 75.9% |
| 3 | `visionai.googleapis.com` | ![visionai.googleapis.com](docs/coverage/bars/visionai.googleapis.com.svg) | 52.0% | 0.0% | 48.0% |
| 3 | `appengine.googleapis.com` | ![appengine.googleapis.com](docs/coverage/bars/appengine.googleapis.com.svg) | 60.4% | 0.0% | 39.6% |
| 3 | `warehouse-visionai.googleapis.com` | ![warehouse-visionai.googleapis.com](docs/coverage/bars/warehouse-visionai.googleapis.com.svg) | 100.0% | 0.0% | 0.0% |
| 3 | `livestream.googleapis.com` | ![livestream.googleapis.com](docs/coverage/bars/livestream.googleapis.com.svg) | 66.0% | 0.0% | 34.0% |
| 3 | `batch.googleapis.com` | ![batch.googleapis.com](docs/coverage/bars/batch.googleapis.com.svg) | 88.7% | 0.0% | 11.3% |
| 3 | `composer.googleapis.com` | ![composer.googleapis.com](docs/coverage/bars/composer.googleapis.com.svg) | 95.3% | 0.0% | 4.7% |
| 3 | `commerceproducer.googleapis.com` | ![commerceproducer.googleapis.com](docs/coverage/bars/commerceproducer.googleapis.com.svg) | 95.3% | 0.0% | 4.7% |
| 3 | `contactcenterinsights.googleapis.com` | ![contactcenterinsights.googleapis.com](docs/coverage/bars/contactcenterinsights.googleapis.com.svg) | 29.8% | 0.0% | 70.2% |
| 3 | `gkemulticloud.googleapis.com` | ![gkemulticloud.googleapis.com](docs/coverage/bars/gkemulticloud.googleapis.com.svg) | 49.4% | 0.0% | 50.6% |
| 3 | `cloudsecuritycompliance.googleapis.com` | ![cloudsecuritycompliance.googleapis.com](docs/coverage/bars/cloudsecuritycompliance.googleapis.com.svg) | 94.3% | 0.0% | 5.7% |
| 3 | `hypercomputecluster.googleapis.com` | ![hypercomputecluster.googleapis.com](docs/coverage/bars/hypercomputecluster.googleapis.com.svg) | 98.4% | 0.0% | 1.6% |
| 3 | `redis.googleapis.com` | ![redis.googleapis.com](docs/coverage/bars/redis.googleapis.com.svg) | 72.0% | 0.0% | 28.0% |
| 3 | `securityposture.googleapis.com` | ![securityposture.googleapis.com](docs/coverage/bars/securityposture.googleapis.com.svg) | 93.7% | 0.0% | 6.3% |
| 3 | `retail.googleapis.com` | ![retail.googleapis.com](docs/coverage/bars/retail.googleapis.com.svg) | 54.4% | 0.0% | 45.6% |
| 3 | `netapp.googleapis.com` | ![netapp.googleapis.com](docs/coverage/bars/netapp.googleapis.com.svg) | 30.2% | 0.0% | 69.8% |
| 3 | `identitytoolkit.googleapis.com` | ![identitytoolkit.googleapis.com](docs/coverage/bars/identitytoolkit.googleapis.com.svg) | 57.8% | 0.0% | 42.2% |
| 3 | `vmmigration.googleapis.com` | ![vmmigration.googleapis.com](docs/coverage/bars/vmmigration.googleapis.com.svg) | 27.2% | 0.0% | 72.8% |
| 3 | `developerconnect.googleapis.com` | ![developerconnect.googleapis.com](docs/coverage/bars/developerconnect.googleapis.com.svg) | 68.1% | 0.0% | 31.9% |
| 3 | `gsuiteaddons.googleapis.com` | ![gsuiteaddons.googleapis.com](docs/coverage/bars/gsuiteaddons.googleapis.com.svg) | 98.0% | 0.0% | 2.0% |
| 3 | `saasservicemgmt.googleapis.com` | ![saasservicemgmt.googleapis.com](docs/coverage/bars/saasservicemgmt.googleapis.com.svg) | 52.2% | 0.0% | 47.8% |
| 3 | `storagetransfer.googleapis.com` | ![storagetransfer.googleapis.com](docs/coverage/bars/storagetransfer.googleapis.com.svg) | 40.0% | 0.0% | 60.0% |
| 3 | `contentwarehouse.googleapis.com` | ![contentwarehouse.googleapis.com](docs/coverage/bars/contentwarehouse.googleapis.com.svg) | 8.4% | 0.0% | 91.6% |
| 3 | `datalabeling.googleapis.com` | ![datalabeling.googleapis.com](docs/coverage/bars/datalabeling.googleapis.com.svg) | 67.1% | 0.0% | 32.9% |
| 3 | `cloudchannel.googleapis.com` | ![cloudchannel.googleapis.com](docs/coverage/bars/cloudchannel.googleapis.com.svg) | 52.9% | 0.0% | 47.1% |
| 3 | `networkmanagement.googleapis.com` | ![networkmanagement.googleapis.com](docs/coverage/bars/networkmanagement.googleapis.com.svg) | 55.3% | 0.0% | 44.7% |
| 3 | `apphub.googleapis.com` | ![apphub.googleapis.com](docs/coverage/bars/apphub.googleapis.com.svg) | 100.0% | 0.0% | 0.0% |
| 3 | `connectors.googleapis.com` | ![connectors.googleapis.com](docs/coverage/bars/connectors.googleapis.com.svg) | 8.2% | 0.0% | 91.8% |
| 3 | `edgenetwork.googleapis.com` | ![edgenetwork.googleapis.com](docs/coverage/bars/edgenetwork.googleapis.com.svg) | 95.0% | 0.0% | 5.0% |
| 3 | `telcoautomation.googleapis.com` | ![telcoautomation.googleapis.com](docs/coverage/bars/telcoautomation.googleapis.com.svg) | 90.2% | 0.0% | 9.8% |
| 3 | `edgecontainer.googleapis.com` | ![edgecontainer.googleapis.com](docs/coverage/bars/edgecontainer.googleapis.com.svg) | 77.8% | 0.0% | 22.2% |
| 3 | `bigquerydatatransfer.googleapis.com` | ![bigquerydatatransfer.googleapis.com](docs/coverage/bars/bigquerydatatransfer.googleapis.com.svg) | 73.9% | 0.0% | 26.1% |
| 3 | `memorystore.googleapis.com` | ![memorystore.googleapis.com](docs/coverage/bars/memorystore.googleapis.com.svg) | 60.0% | 0.0% | 40.0% |
| 3 | `healthcare.googleapis.com` | ![healthcare.googleapis.com](docs/coverage/bars/healthcare.googleapis.com.svg) | 19.1% | 0.0% | 80.9% |
| 3 | `migrationcenter.googleapis.com` | ![migrationcenter.googleapis.com](docs/coverage/bars/migrationcenter.googleapis.com.svg) | 59.6% | 0.0% | 40.4% |
| 3 | `storageinsights.googleapis.com` | ![storageinsights.googleapis.com](docs/coverage/bars/storageinsights.googleapis.com.svg) | 96.8% | 0.0% | 3.2% |
| 3 | `financialservices.googleapis.com` | ![financialservices.googleapis.com](docs/coverage/bars/financialservices.googleapis.com.svg) | 84.8% | 0.0% | 15.2% |
| 3 | `cloudiot.googleapis.com` | ![cloudiot.googleapis.com](docs/coverage/bars/cloudiot.googleapis.com.svg) | 59.1% | 0.0% | 40.9% |
| 3 | `metastore.googleapis.com` | ![metastore.googleapis.com](docs/coverage/bars/metastore.googleapis.com.svg) | 56.5% | 0.0% | 43.5% |
| 3 | `recaptchaenterprise.googleapis.com` | ![recaptchaenterprise.googleapis.com](docs/coverage/bars/recaptchaenterprise.googleapis.com.svg) | 25.7% | 0.0% | 74.3% |
| 3 | `translate.googleapis.com` | ![translate.googleapis.com](docs/coverage/bars/translate.googleapis.com.svg) | 100.0% | 0.0% | 0.0% |
| 3 | `translation.googleapis.com` | ![translation.googleapis.com](docs/coverage/bars/translation.googleapis.com.svg) | 100.0% | 0.0% | 0.0% |
| 3 | `videostitcher.googleapis.com` | ![videostitcher.googleapis.com](docs/coverage/bars/videostitcher.googleapis.com.svg) | 56.1% | 0.0% | 43.9% |
| 3 | `config.googleapis.com` | ![config.googleapis.com](docs/coverage/bars/config.googleapis.com.svg) | 52.5% | 0.0% | 47.5% |
| 3 | `configdelivery.googleapis.com` | ![configdelivery.googleapis.com](docs/coverage/bars/configdelivery.googleapis.com.svg) | 80.8% | 0.0% | 19.2% |
| 3 | `vectorsearch.googleapis.com` | ![vectorsearch.googleapis.com](docs/coverage/bars/vectorsearch.googleapis.com.svg) | 71.4% | 0.0% | 28.6% |
| 3 | `websecurityscanner.googleapis.com` | ![websecurityscanner.googleapis.com](docs/coverage/bars/websecurityscanner.googleapis.com.svg) | 90.0% | 0.0% | 10.0% |
| 3 | `file.googleapis.com` | ![file.googleapis.com](docs/coverage/bars/file.googleapis.com.svg) | 41.5% | 0.0% | 58.5% |
| 3 | `memcache.googleapis.com` | ![memcache.googleapis.com](docs/coverage/bars/memcache.googleapis.com.svg) | 76.2% | 0.0% | 23.8% |
| 3 | `vision.googleapis.com` | ![vision.googleapis.com](docs/coverage/bars/vision.googleapis.com.svg) | 100.0% | 0.0% | 0.0% |
| 3 | `jobs.googleapis.com` | ![jobs.googleapis.com](docs/coverage/bars/jobs.googleapis.com.svg) | 20.6% | 0.0% | 79.4% |
| 3 | `workloadmanager.googleapis.com` | ![workloadmanager.googleapis.com](docs/coverage/bars/workloadmanager.googleapis.com.svg) | 11.6% | 0.0% | 88.4% |
| 3 | `blockchainnodeengine.googleapis.com` | ![blockchainnodeengine.googleapis.com](docs/coverage/bars/blockchainnodeengine.googleapis.com.svg) | 92.3% | 0.0% | 7.7% |
| 3 | `pubsublite.googleapis.com` | ![pubsublite.googleapis.com](docs/coverage/bars/pubsublite.googleapis.com.svg) | 75.0% | 0.0% | 25.0% |
| 3 | `apikeys.googleapis.com` | ![apikeys.googleapis.com](docs/coverage/bars/apikeys.googleapis.com.svg) | 81.8% | 0.0% | 18.2% |
| 3 | `automl.googleapis.com` | ![automl.googleapis.com](docs/coverage/bars/automl.googleapis.com.svg) | 31.0% | 0.0% | 69.0% |
| 3 | `firebase.googleapis.com` | ![firebase.googleapis.com](docs/coverage/bars/firebase.googleapis.com.svg) | 47.4% | 0.0% | 52.6% |
| 3 | `parallelstore.googleapis.com` | ![parallelstore.googleapis.com](docs/coverage/bars/parallelstore.googleapis.com.svg) | 100.0% | 0.0% | 0.0% |
| 3 | `agentregistry.googleapis.com` | ![agentregistry.googleapis.com](docs/coverage/bars/agentregistry.googleapis.com.svg) | 24.2% | 0.0% | 75.8% |
| 3 | `rapidmigrationassessment.googleapis.com` | ![rapidmigrationassessment.googleapis.com](docs/coverage/bars/rapidmigrationassessment.googleapis.com.svg) | 72.7% | 0.0% | 27.3% |
| 3 | `cloudnumberregistry.googleapis.com` | ![cloudnumberregistry.googleapis.com](docs/coverage/bars/cloudnumberregistry.googleapis.com.svg) | 26.9% | 0.0% | 73.1% |
| 3 | `devicestreaming.googleapis.com` | ![devicestreaming.googleapis.com](docs/coverage/bars/devicestreaming.googleapis.com.svg) | 77.8% | 0.0% | 22.2% |
| 3 | `licensemanager.googleapis.com` | ![licensemanager.googleapis.com](docs/coverage/bars/licensemanager.googleapis.com.svg) | 100.0% | 0.0% | 0.0% |
| 3 | `testing.googleapis.com` | ![testing.googleapis.com](docs/coverage/bars/testing.googleapis.com.svg) | 7.1% | 0.0% | 92.9% |
| 3 | `appoptimize.googleapis.com` | ![appoptimize.googleapis.com](docs/coverage/bars/appoptimize.googleapis.com.svg) | 100.0% | 0.0% | 0.0% |
| 3 | `bigquerymigration.googleapis.com` | ![bigquerymigration.googleapis.com](docs/coverage/bars/bigquerymigration.googleapis.com.svg) | 15.0% | 0.0% | 85.0% |
| 3 | `dataflow.googleapis.com` | ![dataflow.googleapis.com](docs/coverage/bars/dataflow.googleapis.com.svg) | 1.5% | 0.0% | 98.5% |
| 3 | `iap.googleapis.com` | ![iap.googleapis.com](docs/coverage/bars/iap.googleapis.com.svg) | 100.0% | 0.0% | 0.0% |
| 3 | `parametermanager.googleapis.com` | ![parametermanager.googleapis.com](docs/coverage/bars/parametermanager.googleapis.com.svg) | 40.0% | 0.0% | 60.0% |
| 3 | `datalineage.googleapis.com` | ![datalineage.googleapis.com](docs/coverage/bars/datalineage.googleapis.com.svg) | 26.3% | 0.0% | 73.7% |
| 3 | `deploymentmanager.googleapis.com` | ![deploymentmanager.googleapis.com](docs/coverage/bars/deploymentmanager.googleapis.com.svg) | 62.5% | 0.0% | 37.5% |
| 3 | `ml.googleapis.com` | ![ml.googleapis.com](docs/coverage/bars/ml.googleapis.com.svg) | 2.9% | 0.0% | 97.1% |
| 3 | `bigquerydatapolicy.googleapis.com` | ![bigquerydatapolicy.googleapis.com](docs/coverage/bars/bigquerydatapolicy.googleapis.com.svg) | 44.4% | 0.0% | 55.6% |
| 3 | `ids.googleapis.com` | ![ids.googleapis.com](docs/coverage/bars/ids.googleapis.com.svg) | 66.7% | 0.0% | 33.3% |
| 3 | `speech.googleapis.com` | ![speech.googleapis.com](docs/coverage/bars/speech.googleapis.com.svg) | 44.4% | 0.0% | 55.6% |
| 3 | `datastore.googleapis.com` | ![datastore.googleapis.com](docs/coverage/bars/datastore.googleapis.com.svg) | 100.0% | 0.0% | 0.0% |
| 3 | `documentai.googleapis.com` | ![documentai.googleapis.com](docs/coverage/bars/documentai.googleapis.com.svg) | 11.1% | 0.0% | 88.9% |
| 3 | `geminidataanalytics.googleapis.com` | ![geminidataanalytics.googleapis.com](docs/coverage/bars/geminidataanalytics.googleapis.com.svg) | 0.7% | 0.0% | 99.3% |
| 3 | `workflowexecutions.googleapis.com` | ![workflowexecutions.googleapis.com](docs/coverage/bars/workflowexecutions.googleapis.com.svg) | 60.0% | 0.0% | 40.0% |
| 3 | `cloudcontrolspartner.googleapis.com` | ![cloudcontrolspartner.googleapis.com](docs/coverage/bars/cloudcontrolspartner.googleapis.com.svg) | 100.0% | 0.0% | 0.0% |
| 3 | `cloudsupport.googleapis.com` | ![cloudsupport.googleapis.com](docs/coverage/bars/cloudsupport.googleapis.com.svg) | 11.8% | 0.0% | 88.2% |
| 3 | `firebasedatabase.googleapis.com` | ![firebasedatabase.googleapis.com](docs/coverage/bars/firebasedatabase.googleapis.com.svg) | 100.0% | 0.0% | 0.0% |
| 3 | `oslogin.googleapis.com` | ![oslogin.googleapis.com](docs/coverage/bars/oslogin.googleapis.com.svg) | 100.0% | 0.0% | 0.0% |
| 3 | `firebasestorage.googleapis.com` | ![firebasestorage.googleapis.com](docs/coverage/bars/firebasestorage.googleapis.com.svg) | 33.3% | 0.0% | 66.7% |

</details>
<!-- coverage:end -->

The kinds:

| Kind | Gap it fills | Source |
| --- | --- | --- |
| `sql.gcp.hfl.dev/v1beta1` `SQLUser` | `spec.databaseRoles`: Config Connector's `SQLUser` cannot grant a Cloud SQL user its database roles (`User.databaseRoles`, Terraform's `google_sql_user.database_roles`) | ours, on upstream's `SQLSSLCert` controller; upstream PR #7148 is watched |
| `sqladmin.gcp.hfl.dev/v1alpha1` `SQLAdminBackup` | an on-demand Cloud SQL backup (`backupRuns`); upstream master has the types but no controller | upstream PR #12553 |
| `secretmanager.gcp.hfl.dev/v1beta1` `SecretManagerSecret`, `SecretManagerSecretVersion` | `spec.location`: a regional secret (Terraform's `google_secret_manager_regional_secret`), in no Config Connector release or on master | master's direct controllers plus upstream PR #6651 |
| `artifactregistry.gcp.hfl.dev/v1beta1` `ArtifactRegistryRepository` | `remoteRepositoryConfig.<format>Repository.customRepository.uri`: a remote repository that pulls through an upstream of your choosing (Docker, Maven, npm, Python), not only the public ones Artifact Registry names, with `upstreamCredentials` (username and Secret Manager password version) for one that needs a login; in no Config Connector release or on master | master's direct controller plus upstream PR #13517; `upstreamCredentials` written here |
| `iam.gcp.hfl.dev/v1alpha1` `IAMWorkloadIdentityPoolNamespace` | a namespace in a workload identity pool (managed workload identities, Terraform's `google_iam_workload_identity_pool_namespace`); Config Connector has types for it only in a pull request, and no controller | upstream PR #14209's types; controller and the pool reference written here |
| `iam.gcp.hfl.dev/v1alpha1` `IAMWorkloadIdentityPoolManagedIdentity` | a managed identity in a pool namespace, with the attestation rules that say which workloads may receive it (Terraform's `google_iam_workload_identity_pool_managed_identity`); types only in a Config Connector pull request, no controller | upstream PR #14418's types; controller, the namespace reference and `attestationRules` written here |
| `run.gcp.hfl.dev/v1alpha1` `RunService` | a Cloud Run service with `invokerIamDisabled` (public without IAM, where an organization policy refuses `allUsers`) and GPUs (`template.nodeSelector`, `gpuZonalRedundancyDisabled`), which Config Connector's Terraform-based `RunService` cannot express; a narrow direct kind that owns the whole service | Config Connector's direct `RunWorkerPool`, for a service; the service fields written here |
| `iam.gcp.hfl.dev/v1alpha1` `IAMWorkloadIdentityPoolProviderKey` | a key for a workload identity pool provider, whose public half (in status) encrypts the provider's SAML assertions; in no Config Connector code, pull request or Terraform resource | written here from the IAM v1 API |
| `storage.gcp.hfl.dev/v1alpha1` `StorageBucketObject` | a small object in a bucket (a configuration file, a script), its content inline or from a Secret or ConfigMap; in no Config Connector code or pull request | written here from the Cloud Storage JSON API |
| `compute.gcp.hfl.dev/v1alpha1` `ComputeRouterRoutePolicy` | a Cloud Router route policy (import or export terms, as CEL), as HA VPN and interconnect setups use; in no Config Connector code or pull request | written here from the Compute Engine API |
| `compute.gcp.hfl.dev/v1alpha1` `ComputeRegionNetworkEndpoint` | an endpoint (internet address, domain name or VM port) in a regional network endpoint group, such as a regional load balancer's backend; Config Connector has zonal and global endpoints only | written here after Config Connector's zonal `ComputeNetworkEndpoint` |
| `compute.gcp.hfl.dev/v1alpha1` `ComputeRegionNetworkFirewallPolicy` | a regional network firewall policy, for firewall rules that apply to a VPC in one region; Config Connector has only the global policy | Config Connector's opt-in direct `ComputeNetworkFirewallPolicy`, for a region |
| `logging.gcp.hfl.dev/v1alpha1` `LoggingSettings` | an organization's or folder's Log Router settings: the CMEK key, the default log bucket location, the `_Default` sink off for new projects; in no Config Connector code (one closed scaffold PR) | written here from the Cloud Logging API |
| `compute.gcp.hfl.dev/v1alpha1` `ComputeProjectMetadataItem` | one key of a project's common instance metadata (for example `enable-oslogin`), leaving the other keys alone; Config Connector's `ComputeProjectMetadata` owns the whole metadata | written here after Terraform's `google_compute_project_metadata_item` |
| `artifactregistry.gcp.hfl.dev/v1alpha1` `ArtifactRegistryTag` | a tag on a package's version (for example `prod` on an image digest), moved by changing `spec.version`; in no Config Connector code, pull request or Terraform resource | written here from the Artifact Registry API |
| `iam.gcp.hfl.dev/v1alpha1` `IAMRoleBinding` | one role on one resource with its members, authoritative for that role (Terraform's `google_*_iam_binding`): members added out of band are removed; Config Connector's `IAMPolicyMember` is additive and `IAMPartialPolicy` keeps what it lists | written here (`pkg/iampolicy`), reading and writing the policy over HTTP |
| `iam.gcp.hfl.dev/v1alpha1` `IAMRoleMember` | one member of one role on one resource, additive (Terraform's `google_*_iam_member`), for the resource types Config Connector's `IAMPolicyMember` cannot reference: regional secrets, managed folders, log views, Cloud Run worker pools, backend services and about two dozen more; the name is a working one (`docs/design.md` section 10) | written here, over the same `pkg/iampolicy` as `IAMRoleBinding` |

## Where it sits

| | Config Connector | config-connector-extensions |
| --- | --- | --- |
| API groups | `<service>.cnrm.cloud.google.com` | `<service>.gcp.hfl.dev` |
| Example | `sql.cnrm.cloud.google.com/v1beta1` `SQLUser` | `sql.gcp.hfl.dev/v1beta1` `SQLUser` |
| Process | KCC's controller manager | our own manager (`cmd/manager`), beside KCC |
| Reconciler | Terraform, DCL or direct, per kind | KCC's direct reconciler (`directbase`), imported unchanged |

The groups never clash with Config Connector's, so both can be installed
on one cluster, and a kind can move from our group to theirs when upstream
fills the gap. Config Connector will not reconcile a kind in its own group
that it does not know (its registration controller skips CRDs missing from
its static table), which is why a gap kind lives in our group, in our
process.

A gap kind references Config Connector kinds the same way KCC kinds
reference each other: `SQLUser.spec.instanceRef.name` names a
`sql.cnrm.cloud.google.com` `SQLInstance` in the same namespace, or
`instanceRef.external` gives `projects/<p>/instances/<i>`.

Annotations mean what they mean in Config Connector, because the reconciler
is Config Connector's: `cnrm.cloud.google.com/deletion-policy: abandon`,
`cnrm.cloud.google.com/project-id`, `.../reconcile-interval-in-seconds`,
`.../state-into-spec` and the actuation mode from `ConfigConnector` /
`ConfigConnectorContext`. An uninstall abandons cloud resources rather
than deleting them (KCC's deletion defender, also imported unchanged).

## The copy-upstream rule

> Where the official one already has a resource on master, copy their code
> as the implementation, changing as little as possible.

So:

- Config Connector is pinned to one commit of `master` (`UPSTREAM.md`) and
  used as a Go module from a local checkout. Its framework (the direct
  reconciler, references, identity, config, mockgcp) is imported, never
  edited in this tree.
- Per-kind code is copied from the nearest upstream source with its licence
  header kept and a "Modified by High Five Labs" note saying what changed.
  For `SQLUser`, upstream master reconciles it with Terraform, so there is
  no direct controller to copy: the Go types come from KCC's generated
  `SQLUser` types (same schema), and the controller is KCC's direct
  `SQLSSLCert` controller (the sibling Cloud SQL sub-resource that already
  moved from Terraform to direct) changed for users. See `docs/design.md`.
- Any change to upstream code we need (so far only the mockgcp fake) is a
  numbered patch in `patches/k8s-config-connector/`, applied to the
  checkout by `hack/upstream.sh`, and is meant to go upstream.

## Versions and retiring kinds

A config-connector-extensions release names the Config Connector release it is installed
beside: the two must agree on the API surface (references, annotations,
`ConfigConnector` and `ConfigConnectorContext`). When a Config Connector
release ships a kind or field that config-connector-extensions fills, our
kind is deprecated for a support window, with a documented move to Config
Connector's group (`docs/design.md` section 3.4).

The first case: an application's Google service account signs in to
Cloud SQL as an IAM database user, which starts with no privileges.
Without `databaseRoles`, someone runs a `GRANT` once by hand.

## Installing it

Beside Config Connector, with its operator or its install bundle:

```sh
ko apply -k config/default    # CRDs, the operator (config-connector-extensions-system)
```

The operator (`cmd/operator`, `pkg/operator`) starts the managers where
Config Connector runs its own, so a gap kind acts as the same Google
identity Config Connector would for the same namespace: in cluster mode
one manager in `cnrm-system`; in namespaced mode one per
`ConfigConnectorContext`, watching only that namespace. It holds no
Google credential.

Each manager runs as **its own Kubernetes service account**, next to
Config Connector's, carrying the same Workload Identity annotation and
only this project's rights. With Workload Identity that needs one more
binding per Google service account, the same command you ran for Config
Connector with our service account's name:

```sh
gcloud iam service-accounts add-iam-policy-binding <gsa> \
  --role=roles/iam.workloadIdentityUser \
  --member="serviceAccount:<cluster-project>.svc.id.goog[cnrm-system/config-connector-extensions-manager]"
# namespaced mode: ...[cnrm-system/config-connector-extensions-manager-<namespace>]
```

Without it, gap objects stay not Ready with a 403 from the metadata server
while Config Connector's own objects work. With a key Secret instead of
Workload Identity, the same Secret is mounted the same way and nothing
more is needed.

For a quick trial where you cannot change IAM, the managers can share
Config Connector's own service account instead: `--identity=config-connector`
on the operator, or the annotation `gcp.hfl.dev/identity: config-connector`
on the `ConfigConnector` object or one `ConfigConnectorContext`. The
manager then holds all of Config Connector's Kubernetes rights, including
writing every Secret in cluster mode. `docs/design.md` sections 9.2 and
9.4.

## Developing it

```sh
task upstream   # fetch Config Connector at the pin (~650 MB) and apply patches/
task check      # pin check, generated files current, gofmt, vet, build, tests
task build      # bin/manager, bin/operator, bin/fakegcp, bin/fakecloud
task operator-e2e  # the operator on kind with the fake cloud (tests/operator/run.sh)
task test-live PROJECT=<project>  # the same tests against a real project (docs/live-runs.md)
task coverage   # regenerate the Coverage section and docs/coverage/ (task coverage:check to verify)
```

`task test` (also `task test-mock`) runs each kind end to end: a real
kube-apiserver (envtest), Config Connector's direct reconciler, our
adapters, and Config Connector's mockgcp as a fake Google Cloud
(`pkg/test/mockenv`); and the operator against envtest. No real Google
Cloud and no recordings. With `E2E_GCP_TARGET=real`, which only
`task test-live` sets, the same tests run against a real project; every
kind has been run that way at least once (the ledger in
`docs/live-runs.md`).

```yaml
apiVersion: sql.gcp.hfl.dev/v1beta1
kind: SQLUser
metadata:
  name: app-runtime
  annotations:
    cnrm.cloud.google.com/deletion-policy: abandon
spec:
  resourceID: app-runtime@my-project.iam
  type: CLOUD_IAM_SERVICE_ACCOUNT
  instanceRef:
    name: example          # a sql.cnrm.cloud.google.com SQLInstance
  databaseRoles:
  - cloudsqlsuperuser
```

`databaseRoles` behaves as Terraform's `database_roles`: the roles are sent
when the user is created, and whenever the list changes the controller
calls `users.update` with `revokeExistingRoles`, so the user ends with
exactly the listed roles, whoever granted the others. Roles granted out of
band while the list is unchanged are not detected (the controller compares
against what it last applied, `status.appliedDatabaseRoles`).

## Layout

| Path | What |
| --- | --- |
| `apis/<service>/<version>/` | each kind's types and identity (copied, see headers) |
| `config/crds/` | generated CRDs (`hack/generate.sh`) |
| `config/operator/`, `config/default/` | the operator, its RBAC and the managers' roles; the install |
| `config/fake-cloud/` | tests only: the install with the in-cluster fake cloud |
| `pkg/controller/direct/<service>/` | each kind's adapter and its mockgcp test |
| `pkg/controller/` | registers our kinds on KCC's direct reconciler and deletion defender |
| `pkg/operator/`, `cmd/operator/` | runs the managers beside Config Connector's, as their own service accounts with its identity |
| `pkg/test/mockenv/` | the envtest + mockgcp harness the kinds' tests share |
| `pkg/fakecloud/`, `cmd/fakegcp/` | tests only: mockgcp as a network service, and the manager's `--fake-cloud-url` (compiled in only with `-tags fakecloud`) |
| `cmd/manager/` | the controller manager |
| `cmd/fakecloud/` | Config Connector's manager and ours against mockgcp, on the host (`docs/fake-cloud.md`) |
| `tests/operator/` | the operator on kind |
| `patches/k8s-config-connector/` | our changes to the upstream checkout, in order |
| `hack/` | fetch and check upstream, generate, regenerate mock protos |
| `hack/live/` | live runs against a real project and the sweeper (`docs/live-runs.md`) |
| `UPSTREAM.md` | the pinned commits |
| `docs/design.md` | architecture, sync, testing, choosing gaps, what is left |
| `docs/fake-cloud.md` | Config Connector's manager and ours against mockgcp on kind |
| `docs/live-runs.md` | the mock/live switch, the live-test service account, the ledger of live runs |

## Contributing

See `CONTRIBUTING.md`. Commits are signed off under the Developer
Certificate of Origin.

## Licence

Apache-2.0 (`LICENSE`). Code copied from Config Connector stays under its
Apache-2.0 licence with Google's copyright: each copied file keeps its
header and says how it was changed, and `NOTICE` names the sources. Binaries
also link code that Config Connector depends on, including MPL-2.0
packages from HashiCorp's Terraform plugin SDK (`NOTICE`). Nothing is
copied from the Terraform Google provider; its behaviour is followed, not
its code.
