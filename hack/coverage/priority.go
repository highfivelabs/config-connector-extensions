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

var priorityHead = []string{"service", "tier", "category", "rationale", "source"}

// The categories, in the order the report lists them.
var categories = []string{"platform-foundation", "security-adjacent", "application", "other"}

// Priority is the content of priority.csv: a tier and a category per
// service, and categories per resource (`host/Name`, or `host/Prefix*`,
// the longest match wins; they override the service's category).
type Priority struct {
	Tier   map[string]int
	Cat    map[string]string // service -> category
	ResCat map[string]string // resource id or glob -> category
}

// Category of a resource: its own row, the longest glob, its service's.
func (p *Priority) Category(host, id string) string {
	if c, ok := p.ResCat[id]; ok {
		return c
	}
	best, cat := -1, ""
	for k, c := range p.ResCat {
		if strings.HasSuffix(k, "*") && strings.HasPrefix(id, strings.TrimSuffix(k, "*")) && len(k) > best {
			best, cat = len(k), c
		}
	}
	if best >= 0 {
		return cat
	}
	if c, ok := p.Cat[host]; ok {
		return c
	}
	return "other"
}

func (p *Priority) TierOf(host string) int {
	if t, ok := p.Tier[host]; ok {
		return t
	}
	return 3
}

// loadPriority reads docs/coverage/priority.csv.
func loadPriority(repo string) (*Priority, error) {
	out := &Priority{Tier: map[string]int{}, Cat: map[string]string{}, ResCat: map[string]string{}}
	f, err := os.Open(filepath.Join(repo, "docs", "coverage", "priority.csv"))
	if err != nil {
		if os.IsNotExist(err) {
			return out, nil
		}
		return nil, err
	}
	defer f.Close()
	rows, err := csv.NewReader(f).ReadAll()
	if err != nil {
		return nil, err
	}
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
		ok := false
		for _, c := range categories {
			ok = ok || c == r[2]
		}
		if !ok {
			return nil, fmt.Errorf("priority.csv line %d: category must be one of %s, got %q", i+1, strings.Join(categories, ", "), r[2])
		}
		if strings.Contains(r[0], "/") {
			out.ResCat[r[0]] = r[2]
			continue
		}
		out.Tier[r[0]] = t
		out.Cat[r[0]] = r[2]
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

// serviceCategory is the category of a service (by host prefix): what a
// platform team needs to establish a target (organization, folders,
// projects, policy, IAM, networking, keys, logs, a managed Kubernetes
// cluster), what security teams need next to it, and application
// primitives. A service in none of these but used by a tracked module is
// `application`; one used by none is `other` (and not listed).
var serviceCategory = map[string]string{
	// platform foundation
	"cloudresourcemanager": "platform-foundation", "iam": "platform-foundation", "orgpolicy": "platform-foundation",
	"cloudbilling": "platform-foundation", "billingbudgets": "platform-foundation", "serviceusage": "platform-foundation",
	"essentialcontacts": "platform-foundation", "compute": "platform-foundation", "servicenetworking": "platform-foundation",
	"vpcaccess": "platform-foundation", "networkconnectivity": "platform-foundation", "dns": "platform-foundation",
	"cloudkms": "platform-foundation", "logging": "platform-foundation", "container": "platform-foundation",
	"gkehub": "platform-foundation", "cloudidentity": "platform-foundation",
	// security-adjacent
	"securitycenter": "security-adjacent", "securitycentermanagement": "security-adjacent",
	"accesscontextmanager": "security-adjacent", "cloudasset": "security-adjacent",
	"binaryauthorization": "security-adjacent", "secretmanager": "security-adjacent", "privateca": "security-adjacent",
	"certificatemanager": "security-adjacent", "networksecurity": "security-adjacent",
	"containeranalysis": "other", // notes and occurrences are scan output, not configuration "iap": "security-adjacent", "privilegedaccessmanager": "security-adjacent",
	"assuredworkloads":  "security-adjacent", "securityposture": "security-adjacent",
	"websecurityscanner": "security-adjacent", "recaptchaenterprise": "security-adjacent",
	"policysimulator": "other", // replays and previews, not configuration "auditmanager": "security-adjacent",
	// application
	"pubsub": "application", "storage": "application", "run": "application", "cloudfunctions": "application",
	"sqladmin": "application", "spanner": "application", "bigquery": "application", "artifactregistry": "application",
	"firestore": "application", "bigtableadmin": "application", "alloydb": "application", "redis": "application",
	"memcache": "application", "cloudscheduler": "application", "cloudtasks": "application", "workflows": "application",
	"eventarc": "application", "appengine": "application", "datastore": "application", "cloudbuild": "application",
	"clouddeploy": "application", "composer": "application", "dataflow": "application", "dataproc": "application",
	"aiplatform": "application", "notebooks": "application", "gkebackup": "application", "apigee": "application",
	"apigateway": "application", "monitoring": "application",
}

// computeCategory: Compute Engine is mostly VPC networking (foundation)
// but also machines, disks and load balancers (application). Listed are
// the resources that are not application; every other Compute resource
// gets an `application` row.
var computeCategory = map[string]string{
	"Address": "platform-foundation", "GlobalAddress": "platform-foundation", "Network": "platform-foundation",
	"Subnetwork": "platform-foundation", "Route": "platform-foundation", "Firewall": "platform-foundation",
	"FirewallPolicy": "platform-foundation", "NetworkFirewallPolicy": "platform-foundation",
	"RegionNetworkFirewallPolicy": "platform-foundation", "OrganizationSecurityPolicy": "platform-foundation",
	"Router": "platform-foundation", "VpnGateway": "platform-foundation", "VpnTunnel": "platform-foundation",
	"TargetVpnGateway": "platform-foundation", "ExternalVpnGateway": "platform-foundation",
	"Interconnect": "platform-foundation", "InterconnectAttachment": "platform-foundation",
	"InterconnectAttachmentGroup": "platform-foundation", "InterconnectGroup": "platform-foundation",
	"NetworkAttachment": "platform-foundation", "ServiceAttachment": "platform-foundation",
	"PublicAdvertisedPrefix": "platform-foundation", "PublicDelegatedPrefix": "platform-foundation",
	"GlobalPublicDelegatedPrefix": "platform-foundation", "CrossSiteNetwork": "platform-foundation",
	"WireGroup":      "platform-foundation",
	"SecurityPolicy": "security-adjacent", "RegionSecurityPolicy": "security-adjacent",
	"NetworkEdgeSecurityService": "security-adjacent", "SslPolicy": "security-adjacent",
	"RegionSslPolicy": "security-adjacent", "PacketMirroring": "security-adjacent",
}

// otherResource: resources of a category's services that are data, not
// configuration (findings, assessments), so they stay out of its numbers.
var otherResource = map[string]string{
	"securitycenter.googleapis.com/SourceFinding":   "findings are scan output",
	"recaptchaenterprise.googleapis.com/Assessment": "an assessment is a request, not configuration",
}

func tierOfCategory(c string) int {
	switch c {
	case "platform-foundation":
		return 1
	case "other":
		return 3
	}
	return 2
}

// derivePriority reads the catalog's gcp-inventory.md and writes
// docs/coverage/priority.csv. A service's category comes from
// serviceCategory (else `application` when a module uses it); tier 1 is
// platform-foundation, tier 2 the other categories. The module usage (with
// the foundational modules, ranks 1 to 19, first) is kept as the
// rationale.
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
	src := fmt.Sprintf("catalog docs/gcp-inventory.md @ %s; category by hand", sha)
	hosts := map[string]bool{}
	for h := range byService {
		hosts[h] = true
	}
	for s := range serviceCategory {
		hosts[s+".googleapis.com"] = true
	}
	var rows [][]string
	for _, h := range sortedKeys(hosts) {
		cat, ok := serviceCategory[strings.TrimSuffix(h, ".googleapis.com")]
		if !ok {
			cat = "application"
		}
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
		var why []string
		if len(foundational) > 0 {
			why = append(why, "foundational modules: "+strings.Join(foundational, ", "))
		}
		if len(other) > 0 {
			why = append(why, "other modules: "+joinMax(other, 6))
		}
		if len(why) == 0 {
			why = append(why, "no tracked module uses it")
		}
		rows = append(rows, []string{h, strconv.Itoa(tierOfCategory(cat)), cat, strings.Join(why, "; "), src})
	}
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i][1] != rows[j][1] {
			return rows[i][1] < rows[j][1]
		}
		return rows[i][0] < rows[j][0]
	})
	for _, id := range sortedKeys(otherResource) {
		if otherResource[id] == "" {
			continue
		}
		rows = append(rows, []string{id, "3", "other", otherResource[id], src})
	}
	// Compute: application unless listed.
	const ce = "compute.googleapis.com"
	rows = append(rows, []string{ce + "/*", "2", "application", "Compute Engine machines, disks, images, load balancers and the rest", src})
	for _, n := range sortedKeys(computeCategory) {
		c := computeCategory[n]
		rows = append(rows, []string{ce + "/" + n, strconv.Itoa(tierOfCategory(c)), c, "VPC networking and network security", src})
	}
	return os.WriteFile(filepath.Join(repo, "docs", "coverage", "priority.csv"), csvBytes(append([][]string{priorityHead}, rows...)), 0o644)
}

func joinMax(s []string, n int) string {
	if len(s) <= n {
		return strings.Join(s, ", ")
	}
	return fmt.Sprintf("%s and %d more", strings.Join(s[:n], ", "), len(s)-n)
}
