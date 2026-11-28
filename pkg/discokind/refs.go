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

// References (kinds.yaml refs, docs/design.md section 2.4): a string field
// the API fills from another resource takes a <field>Ref beside it
// (<singular field>Refs for a list of strings), Config Connector's shape:
// external, or the name and namespace of an object. The controller
// resolves each before it compares or writes, through upstream's own
// reference types where Config Connector has the target kind.

import (
	"context"
	"fmt"
	"sort"
	"strings"

	accesscontextmanagerv1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/apis/accesscontextmanager/v1beta1"
	artifactregistryv1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/apis/artifactregistry/v1beta1"
	certificatemanagerv1alpha1 "github.com/GoogleCloudPlatform/k8s-config-connector/apis/certificatemanager/v1alpha1"
	certificatemanagerv1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/apis/certificatemanager/v1beta1"
	cloudbuildv1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/apis/cloudbuild/v1beta1"
	computerefs "github.com/GoogleCloudPlatform/k8s-config-connector/apis/compute/refs"
	computev1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/apis/compute/v1beta1"
	containerv1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/apis/container/v1beta1"
	"github.com/GoogleCloudPlatform/k8s-config-connector/apis/iam/iamrefs"
	kmsv1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/apis/kms/v1beta1"
	networksecurityv1alpha1 "github.com/GoogleCloudPlatform/k8s-config-connector/apis/networksecurity/v1alpha1"
	"github.com/GoogleCloudPlatform/k8s-config-connector/apis/privateca/privatecarefs"
	pubsubv1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/apis/pubsub/v1beta1"
	refsv1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/apis/refs/v1beta1"
	secretmanagerv1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/apis/secretmanager/v1beta1"
	storagev1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/apis/storage/v1beta1"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/k8s"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Ref is a spec string field (or list of strings) the controller can fill
// from a reference beside it.
type Ref struct {
	// Path is the field's dotted path in the spec, [] after a list of
	// objects: consumerAcceptLists[].networkUrl.
	Path string
	// List: the field is a list of strings, and its reference a list
	// (<singular field>Refs).
	List bool
	// Target is the kind the reference names (ComputeNetwork): this
	// repository's kind of that name first, then Config Connector's.
	Target string
	// Format is how the field takes the target's value: "" as the
	// target's reference resolves it (a name, projects/P/global/networks/N;
	// a service account's email), "id" its last part (a bucket's name, a
	// Secret Manager secret's ID), "serviceAccountName" a service account
	// as projects/P/serviceAccounts/EMAIL.
	Format string
	// Upjet is the Terraform resource Crossplane's Upjet provider
	// resolves the same field from, when it does (provenance; empty: the
	// reference is ours, from the API's description).
	Upjet string
}

// The formats a Ref takes.
const (
	FormatID                 = "id"
	FormatServiceAccountName = "serviceAccountName"
)

// RefFieldName is the spec field of a reference that fills field: field +
// "Ref", or for a list of strings its singular + "Refs" (natSubnets:
// natSubnetRefs).
func RefFieldName(field string, list bool) string {
	if !list {
		return field + "Ref"
	}
	return SingularField(field) + "Refs"
}

// SingularField is a plural field name's singular: addresses, address;
// policies, policy; subnets, subnet.
func SingularField(s string) string {
	switch {
	case strings.HasSuffix(s, "ies") && len(s) > 3:
		return strings.TrimSuffix(s, "ies") + "y"
	case strings.HasSuffix(s, "sses"), strings.HasSuffix(s, "xes"), strings.HasSuffix(s, "ches"), strings.HasSuffix(s, "shes"):
		return strings.TrimSuffix(s, "es")
	case strings.HasSuffix(s, "s") && !strings.HasSuffix(s, "ss"):
		return strings.TrimSuffix(s, "s")
	}
	return s
}

// refSource is one kind a reference by name may find, in order.
type refSource struct {
	gvk schema.GroupVersionKind
	// upstream is a new value of Config Connector's reference type for the
	// kind, whose Normalize reads the object (with its fallbacks for
	// Terraform-based kinds: status.selfLink, status.email); nil for a
	// kind of ours, whose value is its status.externalRef.
	upstream func() refsv1beta1.Ref
}

// refTarget is what a Ref's Target resolves through.
type refTarget struct {
	// upstream: Config Connector's kind, through its reference type.
	upstream func() refsv1beta1.Ref
	// id is the target's ID in its value, for FormatID (default the last
	// part: projects/P/buckets/B is B).
	id func(string) string
}

func upstreamRef[T any, P interface {
	*T
	refsv1beta1.Ref
}]() func() refsv1beta1.Ref {
	return func() refsv1beta1.Ref { return P(new(T)) }
}

// refTargets are the targets with a Config Connector kind. A target
// served only here (a generated kind, or a hand-written one in OurKinds)
// needs no entry.
var refTargets = map[string]refTarget{
	"AccessContextManagerAccessLevel": {upstream: upstreamRef[accesscontextmanagerv1beta1.AccessLevelRef]()},
	"ArtifactRegistryRepository":      {upstream: upstreamRef[artifactregistryv1beta1.ArtifactRegistryRepositoryRef]()},
	"CertificateManagerCertificate":   {upstream: upstreamRef[certificatemanagerv1beta1.CertificateManagerCertificateRef]()},
	"CertificateManagerTrustConfig":   {upstream: upstreamRef[certificatemanagerv1alpha1.CertificateManagerTrustConfigRef]()},
	"CloudBuildWorkerPool":            {upstream: upstreamRef[cloudbuildv1beta1.CloudBuildWorkerPoolRef]()},
	"ComputeAddress":                  {upstream: upstreamRef[computev1beta1.ComputeAddressRef]()},
	"ComputeForwardingRule":           {upstream: upstreamRef[computev1beta1.ForwardingRuleRef]()},
	"ComputeNetwork":                  {upstream: upstreamRef[computerefs.ComputeNetworkRef]()},
	"ComputeRouter":                   {upstream: upstreamRef[computev1beta1.ComputeRouterRef]()},
	"ComputeSubnetwork":               {upstream: upstreamRef[computev1beta1.ComputeSubnetworkRef]()},
	"ContainerCluster":                {upstream: upstreamRef[containerv1beta1.ContainerClusterRef]()},
	"IAMServiceAccount":               {upstream: upstreamRef[iamrefs.IAMServiceAccountRef]()},
	"KMSCryptoKey":                    {upstream: upstreamRef[kmsv1beta1.KMSCryptoKeyRef]()},
	"NetworkSecurityAddressGroup":     {upstream: upstreamRef[networksecurityv1alpha1.NetworkSecurityAddressGroupRef]()},
	"NetworkSecurityGatewaySecurityPolicy": {
		upstream: upstreamRef[networksecurityv1alpha1.NetworkSecurityGatewaySecurityPolicyRef](),
	},
	"NetworkSecurityTLSInspectionPolicy": {
		upstream: upstreamRef[networksecurityv1alpha1.NetworkSecurityTLSInspectionPolicyRef](),
	},
	"PrivateCACAPool":     {upstream: upstreamRef[privatecarefs.PrivateCACAPoolRef]()},
	"PubSubTopic":         {upstream: upstreamRef[pubsubv1beta1.PubSubTopicRef]()},
	"SecretManagerSecret": {upstream: upstreamRef[secretmanagerv1beta1.SecretManagerSecretRef]()},
	"StorageBucket":       {upstream: upstreamRef[storagev1beta1.StorageBucketRef]()},
	// Ours only (hand-written): its name is projects/_/buckets/B/objects/O,
	// and an object's name may hold slashes.
	"StorageBucketObject": {id: func(v string) string {
		if _, o, ok := strings.Cut(v, "/objects/"); ok {
			return o
		}
		return v
	}},
}

// OurKinds are this repository's kinds; pkg/controller sets it. A
// reference by name finds our kind of the target's name before Config
// Connector's.
var OurKinds func() []schema.GroupVersionKind

// RefTargetKnown reports whether a Ref's Target can be resolved: a kind of
// ours or one in refTargets.
func RefTargetKnown(target string) bool {
	return len(sourcesOf(target)) > 0
}

// RefTargetGVKs are the kinds a reference to target may name, in the order
// a reference by name tries them.
func RefTargetGVKs(target string) []schema.GroupVersionKind {
	var out []schema.GroupVersionKind
	for _, s := range sourcesOf(target) {
		out = append(out, s.gvk)
	}
	return out
}

func sourcesOf(target string) []refSource {
	var out []refSource
	if k := ByKind(target); k != nil {
		out = append(out, refSource{gvk: k.GVK})
	} else if OurKinds != nil {
		for _, gvk := range OurKinds() {
			if gvk.Kind == target {
				out = append(out, refSource{gvk: gvk})
				break
			}
		}
	}
	if t, ok := refTargets[target]; ok && t.upstream != nil {
		out = append(out, refSource{gvk: t.upstream().GetGVK(), upstream: t.upstream})
	}
	return out
}

// ResolveRefs replaces each reference body sets (Kind.Refs) with the value
// of the field it fills, reading referenced objects in namespace (the
// object's) unless the reference names another.
func (k *Kind) ResolveRefs(ctx context.Context, reader client.Reader, namespace string, body map[string]any) error {
	for _, r := range k.Refs {
		err := eachRefHolder(body, strings.Split(r.Path, "."), "spec", func(m map[string]any, leaf, at string) error {
			field := RefFieldName(leaf, r.List)
			raw, ok := m[field]
			if !ok {
				return nil
			}
			delete(m, field)
			if raw == nil {
				return nil
			}
			if _, both := m[leaf]; both {
				return fmt.Errorf("%s.%s: set %s or %s, not both", at, field, leaf, field)
			}
			if !r.List {
				v, err := resolveRef(ctx, reader, namespace, &r, raw, at+"."+field)
				if err != nil {
					return err
				}
				m[leaf] = v
				return nil
			}
			items, ok := raw.([]any)
			if !ok {
				return fmt.Errorf("%s.%s: want a list of references, got %T", at, field, raw)
			}
			vals := make([]any, 0, len(items))
			for i, it := range items {
				v, err := resolveRef(ctx, reader, namespace, &r, it, fmt.Sprintf("%s.%s[%d]", at, field, i))
				if err != nil {
					return err
				}
				vals = append(vals, v)
			}
			m[leaf] = vals
			return nil
		})
		if err != nil {
			return err
		}
	}
	return nil
}

// eachRefHolder calls fn with every object in body that holds the last
// part of path, through objects and lists of objects (a part ending in []).
func eachRefHolder(obj map[string]any, path []string, at string, fn func(m map[string]any, leaf, at string) error) error {
	if len(path) == 1 {
		return fn(obj, path[0], at)
	}
	part, list := strings.CutSuffix(path[0], "[]")
	v, ok := obj[part]
	if !ok || v == nil {
		return nil
	}
	if !list {
		m, ok := v.(map[string]any)
		if !ok {
			return fmt.Errorf("%s.%s: want an object, got %T", at, part, v)
		}
		return eachRefHolder(m, path[1:], at+"."+part, fn)
	}
	items, ok := v.([]any)
	if !ok {
		return fmt.Errorf("%s.%s: want a list, got %T", at, part, v)
	}
	for i, it := range items {
		m, ok := it.(map[string]any)
		if !ok {
			return fmt.Errorf("%s.%s[%d]: want an object, got %T", at, part, i, it)
		}
		if err := eachRefHolder(m, path[1:], fmt.Sprintf("%s.%s[%d]", at, part, i), fn); err != nil {
			return err
		}
	}
	return nil
}

// resolveRef is the value one reference gives its field.
func resolveRef(ctx context.Context, reader client.Reader, namespace string, r *Ref, raw any, at string) (string, error) {
	m, ok := raw.(map[string]any)
	if !ok {
		return "", fmt.Errorf("%s: want external, or name and namespace, got %T", at, raw)
	}
	var ref struct {
		External  string `json:"external"`
		Name      string `json:"name"`
		Namespace string `json:"namespace"`
	}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(m, &ref); err != nil {
		return "", fmt.Errorf("%s: %w", at, err)
	}
	sources := sourcesOf(r.Target)
	if len(sources) == 0 {
		return "", fmt.Errorf("%s: no kind %s to resolve the reference through", at, r.Target)
	}
	var ext string
	switch {
	case ref.External != "" && ref.Name != "":
		return "", fmt.Errorf("%s: set external or name, not both", at)
	case ref.External != "":
		if r.Format == FormatID && !strings.Contains(ref.External, "/") {
			// The ID itself (a bucket's name): the field takes it as it is.
			return ref.External, nil
		}
		if err := validateExternal(sources, ref.External); err != nil {
			return "", fmt.Errorf("%s.external: %w", at, err)
		}
		ext = ref.External
	case ref.Name != "":
		ns := ref.Namespace
		if ns == "" {
			ns = namespace
		}
		var err error
		if ext, err = byName(ctx, reader, sources, types.NamespacedName{Namespace: ns, Name: ref.Name}); err != nil {
			return "", err
		}
	default:
		return "", fmt.Errorf("%s: set external, or name (and namespace)", at)
	}
	return formatRef(r, ext)
}

// validateExternal checks an external value with the first source that can
// (Config Connector's reference type, or our generated kind's name).
func validateExternal(sources []refSource, ext string) error {
	for _, s := range sources {
		if s.upstream != nil {
			return s.upstream().ValidateExternal(ext)
		}
		if k := ByKind(s.gvk.Kind); k != nil && k.GVK == s.gvk {
			if !k.NameRegexp().MatchString(ext) {
				return fmt.Errorf("%q is not a %s name (%s)", ext, s.gvk.Kind, k.Name)
			}
			return nil
		}
	}
	return nil
}

// byName is the value of the first source's object of that name.
func byName(ctx context.Context, reader client.Reader, sources []refSource, key types.NamespacedName) (string, error) {
	for _, s := range sources {
		u := &unstructured.Unstructured{}
		u.SetGroupVersionKind(s.gvk)
		if err := reader.Get(ctx, key, u); err != nil {
			if apierrors.IsNotFound(err) || meta.IsNoMatchError(err) {
				continue
			}
			return "", fmt.Errorf("reading referenced %s %s: %w", s.gvk.Kind, key, err)
		}
		if s.upstream == nil {
			ext, _, _ := unstructured.NestedString(u.Object, "status", "externalRef")
			if ext == "" {
				// A kind of ours that records its name as a Compute Engine
				// self link instead (ComputeRouter, as Config Connector's).
				link, _, _ := unstructured.NestedString(u.Object, "status", "selfLink")
				ext = refsv1beta1.TrimComputeURIPrefix(link)
			}
			if ext == "" {
				return "", k8s.NewReferenceNotReadyError(s.gvk, key)
			}
			return ext, nil
		}
		ref := s.upstream()
		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(map[string]any{"name": key.Name, "namespace": key.Namespace}, ref); err != nil {
			return "", err
		}
		if err := ref.Normalize(ctx, reader, key.Namespace); err != nil {
			return "", err
		}
		return ref.GetExternal(), nil
	}
	return "", k8s.NewReferenceNotFoundError(sources[0].gvk, key)
}

// formatRef is the target's value as the field takes it (Ref.Format).
func formatRef(r *Ref, ext string) (string, error) {
	switch r.Format {
	case "":
		return ext, nil
	case FormatID:
		if t, ok := refTargets[r.Target]; ok && t.id != nil {
			return t.id(ext), nil
		}
		return ext[strings.LastIndex(ext, "/")+1:], nil
	case FormatServiceAccountName:
		email := ext[strings.LastIndex(ext, "/")+1:]
		project := "-"
		if acct, domain, ok := strings.Cut(email, "@"); ok && acct != "" {
			if p, ok := strings.CutSuffix(domain, ".iam.gserviceaccount.com"); ok && p != "" {
				project = p
			}
		}
		return "projects/" + project + "/serviceAccounts/" + email, nil
	}
	return "", fmt.Errorf("unknown reference format %q", r.Format)
}

// RefFormats are the formats a Ref takes, for the generator's check.
func RefFormats() []string {
	out := []string{"", FormatID, FormatServiceAccountName}
	sort.Strings(out)
	return out
}
