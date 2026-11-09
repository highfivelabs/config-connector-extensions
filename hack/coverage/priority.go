// Copyright 2026 High Five Labs
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"encoding/csv"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// priority.csv says how much each service matters: tier 1 for the services
// the foundational modules use, tier 2 for any other service a tracked
// module uses, tier 3 (the default, never listed) for the rest. The file is
// an input the builder edits; the generator reads it. `-derive-priority`
// writes a first version from the catalog's inventory.

var priorityHead = []string{"service", "tier", "rationale", "source"}

// loadPriority reads docs/coverage/priority.csv: service -> tier.
func loadPriority(repo string) (map[string]int, error) {
	f, err := os.Open(filepath.Join(repo, "docs", "coverage", "priority.csv"))
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]int{}, nil
		}
		return nil, err
	}
	defer f.Close()
	rows, err := csv.NewReader(f).ReadAll()
	if err != nil {
		return nil, err
	}
	out := map[string]int{}
	for i, r := range rows {
		if i == 0 {
			if strings.Join(r, ",") != strings.Join(priorityHead, ",") {
				return nil, fmt.Errorf("priority.csv: header must be %s", strings.Join(priorityHead, ","))
			}
			continue
		}
		t, err := strconv.Atoi(r[1])
		if err != nil || t < 1 || t > 3 {
			return nil, fmt.Errorf("priority.csv line %d: tier must be 1, 2 or 3, got %q", i+1, r[1])
		}
		out[r[0]] = t
	}
	return out, nil
}

// tfService maps the prefix of a Terraform resource type (after "google_")
// to a service host. Longest prefix wins.
var tfService = map[string]string{
	"access_context_manager": "accesscontextmanager", "alloydb": "alloydb", "api_gateway": "apigateway",
	"apigee": "apigee", "artifact_registry": "artifactregistry", "assured_workloads": "assuredworkloads",
	"backup_dr": "backupdr", "biglake": "biglake", "bigquery_analytics_hub": "analyticshub",
	"bigquery_connection": "bigqueryconnection", "bigquery_reservation": "bigqueryreservation",
	"bigquery": "bigquery", "bigtable": "bigtableadmin", "billing_account": "cloudbilling",
	"billing_budget": "billingbudgets", "billing_project_info": "cloudbilling",
	"binary_authorization": "binaryauthorization", "certificate_manager": "certificatemanager",
	"chronicle": "chronicle", "cloud_asset": "cloudasset", "cloud_identity": "cloudidentity",
	"cloud_quotas": "cloudquotas", "cloud_run": "run", "cloud_scheduler": "cloudscheduler",
	"cloud_tasks": "cloudtasks", "cloudbuild": "cloudbuild", "cloudbuildv2": "cloudbuild",
	"clouddeploy": "clouddeploy", "cloudfunctions": "cloudfunctions", "cloudfunctions2": "cloudfunctions",
	"compute": "compute", "container_analysis": "containeranalysis", "container_registry": "containerregistry",
	"container": "container", "data_catalog": "datacatalog", "data_fusion": "datafusion",
	"data_loss_prevention": "dlp", "dataform": "dataform", "dataplex": "dataplex", "dataproc": "dataproc",
	"dialogflow_cx": "dialogflow", "discovery_engine": "discoveryengine", "dns": "dns",
	"endpoints": "servicemanagement", "essential_contacts": "essentialcontacts", "eventarc": "eventarc",
	"firestore": "firestore", "folder": "cloudresourcemanager", "gke_backup": "gkebackup",
	"gke_hub": "gkehub", "iam": "iam", "iap": "iap", "kms": "cloudkms", "logging": "logging",
	"looker": "looker", "managed_kafka": "managedkafka", "model_armor": "modelarmor", "monitoring": "monitoring",
	"network_connectivity": "networkconnectivity", "network_security": "networksecurity",
	"network_services": "networkservices", "org_policy": "orgpolicy", "organization": "cloudresourcemanager",
	"privateca": "privateca", "privileged_access_manager": "privilegedaccessmanager",
	"project": "cloudresourcemanager", "project_service": "serviceusage", "pubsub": "pubsub",
	"resource_manager": "cloudresourcemanager", "scc_management": "securitycentermanagement",
	"scc_v2": "securitycenter", "secret_manager": "secretmanager", "secure_source_manager": "securesourcemanager",
	"service_account": "iam", "service_directory": "servicedirectory", "service_networking": "servicenetworking",
	"sourcerepo": "sourcerepo", "spanner": "spanner", "sql": "sqladmin", "storage": "storage",
	"tags": "cloudresourcemanager", "tpu": "tpu", "vertex_ai": "aiplatform", "vmwareengine": "vmwareengine",
	"vpc_access": "vpcaccess", "workbench": "notebooks", "workflows": "workflows", "workstations": "workstations",
}

// iamOwners are the prefixes whose `_iam_*` resources are the module's own
// (a project's IAM is the project module's job); `_iam_*` on any other
// service's resource does not count as using that service.
var iamOwners = map[string]bool{"project": true, "folder": true, "organization": true, "service_account": true, "billing_account": true}

var iamSuffix = regexp.MustCompile(`_iam(_member|_binding|_policy|_audit_config|_custom_role)?$`)

func serviceOfTF(t string) (string, bool) {
	t = strings.TrimPrefix(t, "google_")
	t = strings.TrimPrefix(t, "beta_")
	best := ""
	for p := range tfService {
		if (t == p || strings.HasPrefix(t, p+"_")) && len(p) > len(best) {
			best = p
		}
	}
	if best == "" {
		return "", false
	}
	if iamSuffix.MatchString(t) && !iamOwners[best] && best != "iam" {
		return "", false
	}
	return tfService[best] + ".googleapis.com", true
}

// derivePriority reads the catalog's gcp-inventory.md and writes
// docs/coverage/priority.csv. Tier 1: a service a module of rank 1-19 (the
// builder's foundational list) uses; tier 2: any other module's.
func derivePriority(repo, catalog string) error {
	path := filepath.Join(catalog, "docs", "gcp-inventory.md")
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	sha := "unknown"
	if out, err := exec.Command("git", "-C", catalog, "rev-parse", "--short", "HEAD").Output(); err == nil {
		sha = strings.TrimSpace(string(out))
	}
	rank := map[string]int{}
	for _, m := range regexp.MustCompile(`(?m)^\| (\d+) \| \[([\w-]+)\]`).FindAllStringSubmatch(string(b), -1) {
		n, _ := strconv.Atoi(m[1])
		rank[m[2]] = n
	}
	byService := map[string]map[string]bool{}
	cur := ""
	head := regexp.MustCompile(`^### ([\w-]+)$`)
	row := regexp.MustCompile("^\\| `(google\\w*)` \\|")
	for _, line := range strings.Split(string(b), "\n") {
		if m := head.FindStringSubmatch(line); m != nil {
			cur = m[1]
		}
		if m := row.FindStringSubmatch(line); m != nil && cur != "" {
			if h, ok := serviceOfTF(m[1]); ok {
				if byService[h] == nil {
					byService[h] = map[string]bool{}
				}
				byService[h][cur] = true
			}
		}
	}
	rows := [][]string{priorityHead}
	hosts := sortedKeys(byService)
	sort.SliceStable(hosts, func(i, j int) bool { return hosts[i] < hosts[j] })
	var t1, t2 [][]string
	for _, h := range hosts {
		var foundational, other []string
		for m := range byService[h] {
			if rank[m] >= 1 && rank[m] <= 19 {
				foundational = append(foundational, m)
			} else {
				other = append(other, m)
			}
		}
		sort.Slice(foundational, func(i, j int) bool { return rank[foundational[i]] < rank[foundational[j]] })
		sort.Strings(other)
		src := fmt.Sprintf("catalog docs/gcp-inventory.md @ %s", sha)
		if len(foundational) > 0 {
			t1 = append(t1, []string{h, "1", "used by foundational modules: " + strings.Join(foundational, ", "), src})
		} else {
			t2 = append(t2, []string{h, "2", "used by modules: " + joinMax(other, 6), src})
		}
	}
	rows = append(rows, t1...)
	rows = append(rows, t2...)
	return os.WriteFile(filepath.Join(repo, "docs", "coverage", "priority.csv"), csvBytes(rows), 0o644)
}

func joinMax(s []string, n int) string {
	if len(s) <= n {
		return strings.Join(s, ", ")
	}
	return fmt.Sprintf("%s and %d more", strings.Join(s[:n], ", "), len(s)-n)
}
