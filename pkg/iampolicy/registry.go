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
	"fmt"
	"regexp"
	"sort"
	"strings"

	"k8s.io/apimachinery/pkg/runtime/schema"
)

// Protocol is how a service reads and sets a resource's IAM policy.
type Protocol int

const (
	// IAMv1 is the google.iam.v1.IAMPolicy mixin most services serve:
	// {resource}:getIamPolicy and {resource}:setIamPolicy under the
	// service's versioned root.
	IAMv1 Protocol = iota
	// Storage is Cloud Storage's JSON API: GET and PUT on {resource}/iam.
	Storage
	// Compute is Compute Engine's: GET {resource}/getIamPolicy and POST
	// {resource}/setIamPolicy under /compute/{version}, with the body
	// {"policy": ...}.
	Compute
	// BigQuery is a BigQuery dataset's access list, read and patched as
	// the dataset itself (bigquery.go): datasets have no IAM policy calls.
	BigQuery
)

// ResourceType is a kind of Google Cloud resource whose IAM policy can be
// bound. The table below is the only place that names a kind: both IAM
// kinds take their resourceRef.kind from it.
type ResourceType struct {
	// Kind is the Config Connector kind a resourceRef names, as
	// IAMPolicyMember's resourceRef.kind does. The kinds IAMPolicyMember
	// takes keep its spelling.
	Kind string
	// GVKs are the Kubernetes kinds a resourceRef.name may name, tried in
	// order when the reference gives no apiVersion. Empty means only
	// resourceRef.external works.
	GVKs []schema.GroupVersionKind
	// NotInIAMPolicyMember is true for the resource types Config
	// Connector's IAMPolicyMember cannot reference (the seed catalog's
	// "32 resource types", docs/design.md 6.5 rank 2).
	NotInIAMPolicyMember bool

	Protocol Protocol
	// Host is the API's host; {location} is the resource's location for a
	// regional endpoint. Version is the API's version path element, such
	// as v1 (not used by Storage).
	Host, Version string
	// GetWithPost reads the policy with POST rather than GET.
	GetWithPost bool

	// Name is the pattern of the resource's full name, anchored. A group
	// named location fills {location} in Host.
	Name *regexp.Regexp
	// Short turns an external reference that is not a full name (a
	// project ID, a bucket name, a service account email) into one. It
	// returns false when it does not apply.
	Short func(external string) (string, bool)
	// FromObject builds the full name from a referenced object's parts:
	// the project, spec.location, spec.resourceID or metadata.name, and
	// the object's status. Nil means external only.
	FromObject func(o ObjectParts) (string, error)
}

// ObjectParts are the parts of a referenced Kubernetes object that name
// its cloud resource.
type ObjectParts struct {
	Project, Location, ID string
	// ExternalRef is status.externalRef, when the kind sets it.
	ExternalRef string
	// Status is the object's status.
	Status map[string]any
}

func (o ObjectParts) statusString(key string) string {
	s, _ := o.Status[key].(string)
	return s
}

// Lookup returns the resource type for a kind.
func Lookup(kind string) (*ResourceType, bool) {
	t, ok := resourceTypes[kind]
	return t, ok
}

// Kinds are the resource kinds a resourceRef may name, sorted.
func Kinds() []string {
	out := make([]string, 0, len(resourceTypes))
	for k := range resourceTypes {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// ResolveExternal turns an external reference into the resource's full name.
func (t *ResourceType) ResolveExternal(external string) (string, error) {
	external = strings.TrimPrefix(external, "/")
	if t.Name.MatchString(external) {
		return external, nil
	}
	if t.Short != nil {
		if n, ok := t.Short(external); ok && t.Name.MatchString(n) {
			return n, nil
		}
	}
	return "", fmt.Errorf("external %q is not a %s (use %s)", external, t.Kind, t.Name)
}

// ResolveObject builds the full name from a referenced object.
func (t *ResourceType) ResolveObject(o ObjectParts) (string, error) {
	if o.ExternalRef != "" && t.Name.MatchString(o.ExternalRef) {
		return o.ExternalRef, nil
	}
	if t.FromObject == nil {
		return "", fmt.Errorf("a %s cannot be named by name; use resourceRef.external (%s)", t.Kind, t.Name)
	}
	name, err := t.FromObject(o)
	if err != nil {
		return "", err
	}
	if !t.Name.MatchString(name) {
		return "", fmt.Errorf("the %s resolved to %q, which is not %s", t.Kind, name, t.Name)
	}
	return name, nil
}

// Endpoint is the API host for the resource.
func (t *ResourceType) Endpoint(name string) string {
	host := t.Host
	if strings.Contains(host, "{location}") {
		loc := ""
		if m := t.Name.FindStringSubmatch(name); m != nil {
			if i := t.Name.SubexpIndex("location"); i >= 0 {
				loc = m[i]
			}
		}
		if loc == "" {
			// A global resource of a service that also has regional hosts.
			host = strings.Replace(host, "{location}.rep.", "", 1)
			host = strings.Replace(host, "..", ".", 1)
		} else {
			host = strings.ReplaceAll(host, "{location}", loc)
		}
	}
	return host
}

func gvk(group, version, kind string) schema.GroupVersionKind {
	return schema.GroupVersionKind{Group: group, Version: version, Kind: kind}
}

func re(s string) *regexp.Regexp { return regexp.MustCompile("^" + s + "$") }

// The resource types. Add a row to accept another resource; nothing else
// names a kind. Rows marked NotInIAMPolicyMember are rank 2 of the seed
// catalog (docs/design.md 6.5).
var resourceTypes = func() map[string]*ResourceType {
	list := []*ResourceType{
		{
			Kind: "Project", GVKs: []schema.GroupVersionKind{gvk("resourcemanager.cnrm.cloud.google.com", "v1beta1", "Project")},
			Host: "cloudresourcemanager.googleapis.com", Version: "v3", GetWithPost: true,
			Name:  re(`projects/[a-z][a-z0-9-]*[a-z0-9]`),
			Short: func(e string) (string, bool) { return "projects/" + e, !strings.Contains(e, "/") },
			FromObject: func(o ObjectParts) (string, error) {
				return "projects/" + o.ID, nil
			},
		},
		{
			Kind: "Folder", GVKs: []schema.GroupVersionKind{gvk("resourcemanager.cnrm.cloud.google.com", "v1beta1", "Folder")},
			Host: "cloudresourcemanager.googleapis.com", Version: "v3", GetWithPost: true,
			Name:  re(`folders/[0-9]+`),
			Short: func(e string) (string, bool) { return "folders/" + e, !strings.Contains(e, "/") },
			FromObject: func(o ObjectParts) (string, error) {
				if id := o.statusString("folderId"); id != "" {
					return "folders/" + id, nil
				}
				return "", fmt.Errorf("the Folder has no status.folderId yet")
			},
		},
		{
			Kind: "Organization",
			Host: "cloudresourcemanager.googleapis.com", Version: "v3", GetWithPost: true,
			Name:  re(`organizations/[0-9]+`),
			Short: func(e string) (string, bool) { return "organizations/" + e, !strings.Contains(e, "/") },
		},
		{
			Kind: "IAMServiceAccount", GVKs: []schema.GroupVersionKind{gvk("iam.cnrm.cloud.google.com", "v1beta1", "IAMServiceAccount")},
			Host: "iam.googleapis.com", Version: "v1", GetWithPost: true,
			Name: re(`projects/[^/]+/serviceAccounts/[^/]+`),
			Short: func(e string) (string, bool) {
				// An email, {id}@{project}.iam.gserviceaccount.com.
				id, rest, ok := strings.Cut(e, "@")
				project, found := strings.CutSuffix(rest, ".iam.gserviceaccount.com")
				if !ok || !found || id == "" || project == "" {
					return "", false
				}
				return "projects/" + project + "/serviceAccounts/" + e, true
			},
			FromObject: func(o ObjectParts) (string, error) {
				if email := o.statusString("email"); email != "" {
					return "projects/" + o.Project + "/serviceAccounts/" + email, nil
				}
				return "", fmt.Errorf("the IAMServiceAccount has no status.email yet")
			},
		},
		{
			Kind: "StorageBucket", GVKs: []schema.GroupVersionKind{gvk("storage.cnrm.cloud.google.com", "v1beta1", "StorageBucket")},
			Protocol: Storage, Host: "storage.googleapis.com",
			Name:       re(`projects/_/buckets/[^/]+`),
			Short:      func(e string) (string, bool) { return "projects/_/buckets/" + e, !strings.Contains(e, "/") },
			FromObject: func(o ObjectParts) (string, error) { return "projects/_/buckets/" + o.ID, nil },
		},
		{
			// Config Connector's IAMPolicyMember cannot name a regional secret.
			Kind: "SecretManagerSecret", NotInIAMPolicyMember: true,
			GVKs: []schema.GroupVersionKind{
				gvk("secretmanager.gcp.hfl.dev", "v1beta1", "SecretManagerSecret"),
				gvk("secretmanager.cnrm.cloud.google.com", "v1beta1", "SecretManagerSecret"),
			},
			Host: "secretmanager.{location}.rep.googleapis.com", Version: "v1",
			Name: re(`projects/[^/]+(?:/locations/(?P<location>[^/]+))?/secrets/[^/]+`),
			FromObject: func(o ObjectParts) (string, error) {
				if o.Location != "" {
					return "projects/" + o.Project + "/locations/" + o.Location + "/secrets/" + o.ID, nil
				}
				return "projects/" + o.Project + "/secrets/" + o.ID, nil
			},
		},
		{
			Kind: "ArtifactRegistryRepository",
			GVKs: []schema.GroupVersionKind{
				gvk("artifactregistry.gcp.hfl.dev", "v1beta1", "ArtifactRegistryRepository"),
				gvk("artifactregistry.cnrm.cloud.google.com", "v1beta1", "ArtifactRegistryRepository"),
			},
			Host: "artifactregistry.googleapis.com", Version: "v1",
			Name: re(`projects/[^/]+/locations/[^/]+/repositories/[^/]+`),
			FromObject: func(o ObjectParts) (string, error) {
				return "projects/" + o.Project + "/locations/" + o.Location + "/repositories/" + o.ID, nil
			},
		},
		{
			Kind: "RunService",
			GVKs: []schema.GroupVersionKind{
				gvk("run.gcp.hfl.dev", "v1alpha1", "RunService"),
				gvk("run.cnrm.cloud.google.com", "v1beta1", "RunService"),
			},
			Host: "run.googleapis.com", Version: "v2",
			Name: re(`projects/[^/]+/locations/[^/]+/services/[^/]+`),
			FromObject: func(o ObjectParts) (string, error) {
				return "projects/" + o.Project + "/locations/" + o.Location + "/services/" + o.ID, nil
			},
		},
		{
			Kind: "PubSubTopic", GVKs: []schema.GroupVersionKind{gvk("pubsub.cnrm.cloud.google.com", "v1beta1", "PubSubTopic")},
			Host: "pubsub.googleapis.com", Version: "v1",
			Name:       re(`projects/[^/]+/topics/[^/]+`),
			FromObject: func(o ObjectParts) (string, error) { return "projects/" + o.Project + "/topics/" + o.ID, nil },
		},
		{
			Kind: "PubSubSubscription", GVKs: []schema.GroupVersionKind{gvk("pubsub.cnrm.cloud.google.com", "v1beta1", "PubSubSubscription")},
			Host: "pubsub.googleapis.com", Version: "v1",
			Name: re(`projects/[^/]+/subscriptions/[^/]+`),
			FromObject: func(o ObjectParts) (string, error) {
				return "projects/" + o.Project + "/subscriptions/" + o.ID, nil
			},
		},
		{
			Kind: "KMSKeyRing", GVKs: []schema.GroupVersionKind{gvk("kms.cnrm.cloud.google.com", "v1beta1", "KMSKeyRing")},
			Host: "cloudkms.googleapis.com", Version: "v1",
			Name: re(`projects/[^/]+/locations/[^/]+/keyRings/[^/]+`),
			FromObject: func(o ObjectParts) (string, error) {
				return "projects/" + o.Project + "/locations/" + o.Location + "/keyRings/" + o.ID, nil
			},
		},
		{
			Kind: "KMSCryptoKey",
			Host: "cloudkms.googleapis.com", Version: "v1",
			Name: re(`projects/[^/]+/locations/[^/]+/keyRings/[^/]+/cryptoKeys/[^/]+`),
		},
	}
	list = append(list, moreTypes()...)
	m := map[string]*ResourceType{}
	for _, t := range list {
		m[t.Kind] = t
	}
	return m
}()
