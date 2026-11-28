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

package discokind

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/k8s"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var (
	ccNetwork = schema.GroupVersionKind{Group: "compute.cnrm.cloud.google.com", Version: "v1beta1", Kind: "ComputeNetwork"}
	ccSA      = schema.GroupVersionKind{Group: "iam.cnrm.cloud.google.com", Version: "v1beta1", Kind: "IAMServiceAccount"}
	ccSecret  = schema.GroupVersionKind{Group: "secretmanager.cnrm.cloud.google.com", Version: "v1beta1", Kind: "SecretManagerSecret"}
	ourSecret = schema.GroupVersionKind{Group: "secretmanager.gcp.hfl.dev", Version: "v1beta1", Kind: "SecretManagerSecret"}
	ourObject = schema.GroupVersionKind{Group: "storage.gcp.hfl.dev", Version: "v1beta1", Kind: "StorageBucketObject"}
)

func obj(gvk schema.GroupVersionKind, ns, name string, status map[string]any) *unstructured.Unstructured {
	u := &unstructured.Unstructured{Object: map[string]any{}}
	u.SetGroupVersionKind(gvk)
	u.SetNamespace(ns)
	u.SetName(name)
	if status != nil {
		u.Object["status"] = status
	}
	return u
}

func refClient(t *testing.T, objs ...*unstructured.Unstructured) client.Client {
	t.Helper()
	scheme := runtime.NewScheme()
	for _, gvk := range []schema.GroupVersionKind{ccNetwork, ccSA, ccSecret, ourSecret, ourObject} {
		scheme.AddKnownTypeWithName(gvk, &unstructured.Unstructured{})
		scheme.AddKnownTypeWithName(gvk.GroupVersion().WithKind(gvk.Kind+"List"), &unstructured.UnstructuredList{})
	}
	b := fake.NewClientBuilder().WithScheme(scheme)
	for _, o := range objs {
		b = b.WithObjects(o)
	}
	return b.Build()
}

func withOurKinds(t *testing.T, gvks ...schema.GroupVersionKind) {
	t.Helper()
	old := OurKinds
	OurKinds = func() []schema.GroupVersionKind { return gvks }
	t.Cleanup(func() { OurKinds = old })
}

func TestRefFieldName(t *testing.T) {
	for _, tc := range []struct {
		field string
		list  bool
		want  string
	}{
		{"network", false, "networkRef"},
		{"networkUrl", false, "networkUrlRef"},
		{"natSubnets", true, "natSubnetRefs"},
		{"ipsecInternalAddresses", true, "ipsecInternalAddressRefs"},
		{"accessLevels", true, "accessLevelRefs"},
		{"excludedNetworks", true, "excludedNetworkRefs"},
		{"gatewaySecurityPolicies", true, "gatewaySecurityPolicyRefs"},
	} {
		if got := RefFieldName(tc.field, tc.list); got != tc.want {
			t.Errorf("RefFieldName(%q, %v) = %q, want %q", tc.field, tc.list, got, tc.want)
		}
	}
}

// TestResolveRefs: each reference becomes its field's value, by external
// or by an object's name, through lists and nested objects; the
// reference itself leaves the body.
func TestResolveRefs(t *testing.T) {
	withOurKinds(t, ourSecret, ourObject)
	k := &Kind{GVK: schema.GroupVersionKind{Kind: "TestKind"}, Refs: []Ref{
		{Path: "network", Target: "ComputeNetwork"},
		{Path: "consumers[].networkUrl", Target: "ComputeNetwork"},
		{Path: "networks", List: true, Target: "ComputeNetwork"},
		{Path: "build.serviceAccount", Target: "IAMServiceAccount", Format: FormatServiceAccountName},
		{Path: "trigger.serviceAccountEmail", Target: "IAMServiceAccount"},
		{Path: "secrets[].secret", Target: "SecretManagerSecret", Format: FormatID},
		{Path: "source.object", Target: "StorageBucketObject", Format: FormatID},
	}}
	c := refClient(t,
		// Config Connector's network, Terraform-based: no externalRef, a selfLink.
		obj(ccNetwork, "ns", "net", map[string]any{"selfLink": "https://www.googleapis.com/compute/v1/projects/p/global/networks/net"}),
		obj(ccNetwork, "other", "net2", map[string]any{"externalRef": "projects/p/global/networks/net2"}),
		obj(ccSA, "ns", "sa", map[string]any{"email": "sa@p.iam.gserviceaccount.com"}),
		// Ours and Config Connector's of the same name: ours is found first.
		obj(ourSecret, "ns", "s", map[string]any{"externalRef": "projects/p/secrets/ours"}),
		obj(ccSecret, "ns", "s", map[string]any{"externalRef": "projects/p/secrets/theirs"}),
		obj(ccSecret, "ns", "only-cc", map[string]any{"externalRef": "projects/p/secrets/cc"}),
		obj(ourObject, "ns", "zip", map[string]any{"externalRef": "projects/_/buckets/b/objects/src/function.zip"}),
	)
	body := map[string]any{
		"networkRef": map[string]any{"name": "net"},
		"consumers": []any{
			map[string]any{"networkUrlRef": map[string]any{"name": "net2", "namespace": "other"}, "limit": int64(1)},
			map[string]any{"networkUrl": "projects/p/global/networks/plain"},
		},
		"networkRefs": []any{
			map[string]any{"external": "projects/p/global/networks/a"},
			map[string]any{"name": "net"},
		},
		"build":   map[string]any{"serviceAccountRef": map[string]any{"name": "sa"}},
		"trigger": map[string]any{"serviceAccountEmailRef": map[string]any{"external": "run@p.iam.gserviceaccount.com"}},
		"secrets": []any{
			map[string]any{"secretRef": map[string]any{"name": "s"}},
			map[string]any{"secretRef": map[string]any{"name": "only-cc"}},
			map[string]any{"secretRef": map[string]any{"external": "bare-id"}},
			map[string]any{"secretRef": map[string]any{"external": "projects/p/secrets/full"}},
		},
		"source": map[string]any{"objectRef": map[string]any{"name": "zip"}},
	}
	if err := k.ResolveRefs(context.Background(), c, "ns", body); err != nil {
		t.Fatal(err)
	}
	want := map[string]any{
		"network": "projects/p/global/networks/net",
		"consumers": []any{
			map[string]any{"networkUrl": "projects/p/global/networks/net2", "limit": int64(1)},
			map[string]any{"networkUrl": "projects/p/global/networks/plain"},
		},
		"networks": []any{"projects/p/global/networks/a", "projects/p/global/networks/net"},
		"build":    map[string]any{"serviceAccount": "projects/p/serviceAccounts/sa@p.iam.gserviceaccount.com"},
		"trigger":  map[string]any{"serviceAccountEmail": "run@p.iam.gserviceaccount.com"},
		"secrets": []any{
			map[string]any{"secret": "ours"},
			map[string]any{"secret": "cc"},
			map[string]any{"secret": "bare-id"},
			map[string]any{"secret": "full"},
		},
		"source": map[string]any{"object": "src/function.zip"},
	}
	if !reflect.DeepEqual(body, want) {
		t.Errorf("resolved\n%v\nwant\n%v", body, want)
	}
}

func TestResolveRefsRefuses(t *testing.T) {
	withOurKinds(t)
	k := &Kind{GVK: schema.GroupVersionKind{Kind: "TestKind"}, Refs: []Ref{{Path: "network", Target: "ComputeNetwork"}}}
	c := refClient(t, obj(ccNetwork, "ns", "pending", nil))
	for _, tc := range []struct {
		name string
		body map[string]any
		want string
		is   func(error) bool
	}{
		{"both the field and its reference", map[string]any{"network": "x", "networkRef": map[string]any{"external": "projects/p/global/networks/n"}}, "not both", nil},
		{"external and name", map[string]any{"networkRef": map[string]any{"external": "projects/p/global/networks/n", "name": "n"}}, "external or name, not both", nil},
		{"neither", map[string]any{"networkRef": map[string]any{}}, "set external, or name", nil},
		{"a bad external", map[string]any{"networkRef": map[string]any{"external": "nets/n"}}, "spec.networkRef.external", nil},
		{"an object that is not there", map[string]any{"networkRef": map[string]any{"name": "missing"}}, "", k8s.IsReferenceNotFoundError},
		{"an object that is not ready", map[string]any{"networkRef": map[string]any{"name": "pending"}}, "", func(err error) bool {
			var notReady *k8s.ReferenceNotReadyError
			return errors.As(err, &notReady)
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := k.ResolveRefs(context.Background(), c, "ns", tc.body)
			if err == nil {
				t.Fatal("no error")
			}
			if tc.want != "" && !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error %q does not say %q", err, tc.want)
			}
			if tc.is != nil && !tc.is(err) {
				t.Errorf("error %T %v is not the expected kind", err, err)
			}
		})
	}
}

// TestRefTargetsResolve: every generated kind's references name a target
// this package can resolve, with a known format (the registry is
// pkg/controller's; this checks the table refs.go holds).
func TestRefTargetsResolve(t *testing.T) {
	withOurKinds(t, ourSecret, ourObject, schema.GroupVersionKind{Group: "artifactregistry.gcp.hfl.dev", Version: "v1beta1", Kind: "ArtifactRegistryRepository"})
	for target, rt := range refTargets {
		if rt.upstream == nil && rt.id == nil {
			t.Errorf("%s: neither a Config Connector reference type nor an ID", target)
		}
		if rt.upstream != nil && rt.upstream().GetGVK().Kind != target {
			t.Errorf("%s: Config Connector's reference type is for %s", target, rt.upstream().GetGVK().Kind)
		}
	}
	// A service account outside IAM's domain (upstream's reference refuses
	// one as external) names no project: projects/-/.
	if got, _ := formatRef(&Ref{Format: FormatServiceAccountName}, "1-compute@developer.gserviceaccount.com"); got != "projects/-/serviceAccounts/1-compute@developer.gserviceaccount.com" {
		t.Errorf("serviceAccountName of a compute default account = %q", got)
	}
	for _, f := range RefFormats() {
		if _, err := formatRef(&Ref{Format: f, Target: "X"}, "projects/p/x/y"); err != nil {
			t.Errorf("format %q: %v", f, err)
		}
	}
}
