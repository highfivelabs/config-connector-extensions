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
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"slices"
	"sort"
	"strings"
	"time"

	billingv1alpha1 "github.com/GoogleCloudPlatform/k8s-config-connector/apis/billing/v1alpha1"
	refsv1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/apis/refs/v1beta1"
	"github.com/GoogleCloudPlatform/k8s-config-connector/apis/refs/v1beta1/secret"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/config"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct/directbase"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/k8s"
	"google.golang.org/api/googleapi"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// NewModel is the direct reconciler's model for one kind.
func NewModel(k *Kind) func(context.Context, *config.ControllerConfig) (directbase.Model, error) {
	return func(ctx context.Context, cfg *config.ControllerConfig) (directbase.Model, error) {
		return &model{kind: k, config: *cfg}, nil
	}
}

type model struct {
	kind   *Kind
	config config.ControllerConfig
}

var _ directbase.Model = &model{}

func (m *model) AdapterForObject(ctx context.Context, op *directbase.AdapterForObjectOperation) (directbase.Adapter, error) {
	u := op.GetUnstructured()
	ext, _, _ := unstructured.NestedString(u.Object, "status", "externalRef")
	var name, inputVersion string
	var body map[string]any
	switch deleting := !u.GetDeletionTimestamp().IsZero(); {
	case deleting && k8s.HasAbandonAnnotation(u):
		// Abandoning: the reconciler makes no call, so read nothing either.
		// directbase reads (Find) before it looks at the policy, and a read
		// that fails (BigLake's 403 for a catalog deleted behind our back,
		// Kind.MissingIs403) would otherwise hold the finalizer forever.
		return &adapter{kind: m.kind}, nil
	case deleting && ext != "":
		// Deleting a resource that was made or acquired: its name is in
		// status, and the parts of a name cannot change, so the parent is not
		// resolved. A parent object already gone or going (everything deleted
		// at once) would otherwise hold the delete forever (IAMBinding's rule,
		// pkg/iampolicy.Resolve).
		name = ext
	default:
		var err error
		name, body, inputVersion, err = m.kind.resolve(ctx, op.Reader, u)
		if err != nil {
			if deleting {
				// Never made or acquired (no status.externalRef) and its parent
				// cannot be resolved: there is nothing of ours to delete.
				klog.FromContext(ctx).Info("deleting without a call: no status.externalRef and the parent cannot be resolved", "kind", m.kind.GVK.Kind, "error", err.Error())
				return &adapter{kind: m.kind}, nil
			}
			return nil, err
		}
		if ext != "" && ext != name {
			return nil, fmt.Errorf("cannot change %s identity (old=%q, new=%q)", m.kind.GVK.Kind, ext, name)
		}
	}
	httpClient, err := m.config.NewAuthenticatedHTTPClient(ctx)
	if err != nil {
		return nil, err
	}
	api := &API{HTTP: httpClient, BaseURL: m.kind.BaseURL, LocationHost: m.kind.LocationHost, GetQuery: m.kind.GetQuery}
	spec, _, _ := unstructured.NestedMap(u.Object, "spec")
	deleting := !u.GetDeletionTimestamp().IsZero()
	var resend []string
	if applied, _, _ := unstructured.NestedString(u.Object, "status", "inputVersion"); m.kind.Updatable && applied != inputVersion {
		// The input-only fields, or a Secret one comes from, changed since
		// they were last sent: send them again, since a read cannot
		// compare them.
		resend = m.kind.WriteOnlySet(body)
	}
	a := &adapter{kind: m.kind, api: api, name: name, desired: body, inputVersion: inputVersion, query: m.kind.QueryOf(spec), recorded: ext != "", deleting: deleting, resend: resend}
	if len(m.kind.SecretOutputs) > 0 && !deleting {
		if a.secrets, err = newSecretOutput(m.kind, op.Reader, u); err != nil {
			return nil, err
		}
	}
	return a, nil
}

// AdapterForURL is not supported: these kinds are not exported.
func (m *model) AdapterForURL(ctx context.Context, url string) (directbase.Adapter, error) {
	return nil, nil
}

// resolve is the object's resource name, the request body its spec asks
// for, and the version of its input-only fields (status.inputVersion).
func (k *Kind) resolve(ctx context.Context, reader client.Reader, u *unstructured.Unstructured) (string, map[string]any, string, error) {
	spec, _, err := unstructured.NestedMap(u.Object, "spec")
	if err != nil {
		return "", nil, "", fmt.Errorf("reading spec: %w", err)
	}
	if spec == nil {
		spec = map[string]any{}
	}
	parent := ""
	if len(k.Parents) > 0 {
		if parent, err = resolveParent(ctx, reader, u, k.Parents, spec); err != nil {
			return "", nil, "", err
		}
	}
	id := u.GetName()
	if v, ok := spec["resourceID"].(string); ok && v != "" {
		id = v
	}
	params := map[string]string{}
	for _, p := range k.PathParams() {
		v, _ := spec[p].(string)
		params[p] = v
	}
	name, err := k.Render(parent, id, params)
	if err != nil {
		return "", nil, "", err
	}
	body := k.Body(spec)
	version, err := k.resolveSecrets(ctx, reader, u.GetNamespace(), body)
	if err != nil {
		return "", nil, "", err
	}
	return name, body, version, nil
}

// resolveSecrets replaces each Secrets field body sets, a secret.Legacy
// (value, or valueFrom.secretKeyRef to a Secret in the object's
// namespace), with its string, as Config Connector's SQLUser does for its
// password. It returns the version of the body's input-only fields
// (status.inputVersion), "" when the kind has none.
func (k *Kind) resolveSecrets(ctx context.Context, reader client.Reader, namespace string, body map[string]any) (string, error) {
	if len(k.writeOnly()) == 0 {
		return "", nil
	}
	h := sha256.New()
	for _, p := range k.InputOnly {
		v, found, _ := unstructured.NestedFieldNoCopy(body, strings.Split(p, ".")...)
		b, _ := json.Marshal(v)
		fmt.Fprintf(h, "%s\x00%v\x00%s\x00", p, found, b)
	}
	for _, p := range k.Secrets {
		path := strings.Split(p, ".")
		raw, found, err := unstructured.NestedFieldNoCopy(body, path...)
		if err != nil {
			return "", fmt.Errorf("reading spec.%s: %w", p, err)
		}
		if !found || raw == nil {
			fmt.Fprintf(h, "%s\x00unset\x00", p)
			continue
		}
		m, ok := raw.(map[string]any)
		if !ok {
			return "", fmt.Errorf("spec.%s: want value or valueFrom.secretKeyRef, got %T", p, raw)
		}
		legacy := &secret.Legacy{}
		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(m, legacy); err != nil {
			return "", fmt.Errorf("reading spec.%s: %w", p, err)
		}
		var v string
		switch {
		case legacy.Value != nil && legacy.ValueFrom != nil:
			return "", fmt.Errorf("spec.%s: set one of value and valueFrom, not both", p)
		case legacy.Value != nil:
			// The value is in the spec already; its digest goes into the
			// version's, never into status on its own.
			v = *legacy.Value
			fmt.Fprintf(h, "%s\x00value\x00%x\x00", p, sha256.Sum256([]byte(v)))
		case legacy.ValueFrom != nil && legacy.ValueFrom.SecretKeyRef != nil:
			ref := legacy.ValueFrom.SecretKeyRef
			if ref.Name == "" || ref.Key == "" {
				return "", fmt.Errorf("spec.%s.valueFrom.secretKeyRef needs name and key", p)
			}
			sec := &corev1.Secret{}
			if err := reader.Get(ctx, types.NamespacedName{Namespace: namespace, Name: ref.Name}, sec); err != nil {
				if apierrors.IsNotFound(err) {
					return "", fmt.Errorf("spec.%s: Secret %s/%s not found", p, namespace, ref.Name)
				}
				return "", fmt.Errorf("spec.%s: reading Secret %s/%s: %w", p, namespace, ref.Name, err)
			}
			data, ok := sec.Data[ref.Key]
			if !ok {
				return "", fmt.Errorf("spec.%s: Secret %s/%s has no key %q", p, namespace, ref.Name, ref.Key)
			}
			v = string(data)
			// The Secret's name, key and resourceVersion, not its value
			// (ComputeRouter's md5AuthenticationKeysVersion).
			fmt.Fprintf(h, "%s\x00secret\x00%s\x00%s\x00%s\x00", p, ref.Name, ref.Key, sec.ResourceVersion)
		default:
			unstructured.RemoveNestedField(body, path...)
			fmt.Fprintf(h, "%s\x00unset\x00", p)
			continue
		}
		if err := unstructured.SetNestedField(body, v, path...); err != nil {
			return "", fmt.Errorf("setting spec.%s: %w", p, err)
		}
	}
	return hex.EncodeToString(h.Sum(nil))[:16], nil
}

// Body is the request body a spec asks for: the spec without the parent
// reference, the fields that make up the name, the query parameters
// (Query), resourceID and, for a kind with SecretOutputs, secretOutput.
func (k *Kind) Body(spec map[string]any) map[string]any {
	body := runtime.DeepCopyJSON(spec)
	delete(body, "resourceID")
	if len(k.SecretOutputs) > 0 {
		delete(body, SecretOutputSpec)
	}
	for _, f := range k.Query {
		delete(body, f)
	}
	for _, p := range k.Parents {
		delete(body, RefField(p))
	}
	for _, p := range k.PathParams() {
		delete(body, p)
	}
	return body
}

func resolveParent(ctx context.Context, reader client.Reader, u *unstructured.Unstructured, parents []string, spec map[string]any) (string, error) {
	var set []string
	for _, p := range parents {
		if _, ok := spec[RefField(p)]; ok {
			set = append(set, p)
		}
	}
	if len(set) != 1 {
		var fields []string
		for _, p := range parents {
			fields = append(fields, "spec."+RefField(p))
		}
		return "", fmt.Errorf("exactly one of %s is required", strings.Join(fields, ", "))
	}
	raw, _ := spec[RefField(set[0])].(map[string]any)
	switch set[0] {
	case "project":
		ref := &refsv1beta1.ProjectRef{}
		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(raw, ref); err != nil {
			return "", fmt.Errorf("reading spec.projectRef: %w", err)
		}
		p, err := refsv1beta1.ResolveProject(ctx, reader, u.GetNamespace(), ref)
		if err != nil {
			return "", err
		}
		return "projects/" + p.ProjectID, nil
	case "folder":
		ref := &refsv1beta1.FolderRef{}
		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(raw, ref); err != nil {
			return "", fmt.Errorf("reading spec.folderRef: %w", err)
		}
		f, err := refsv1beta1.ResolveFolder(ctx, reader, u, ref)
		if err != nil {
			return "", err
		}
		return "folders/" + f.FolderID, nil
	case "organization":
		ref := &refsv1beta1.OrganizationRef{}
		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(raw, ref); err != nil {
			return "", fmt.Errorf("reading spec.organizationRef: %w", err)
		}
		o, err := refsv1beta1.ResolveOrganization(ctx, reader, u, ref)
		if err != nil {
			return "", err
		}
		return "organizations/" + o.OrganizationID, nil
	case "billingAccount":
		ref := &billingv1alpha1.BillingAccountRef{}
		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(raw, ref); err != nil {
			return "", fmt.Errorf("reading spec.billingAccountRef: %w", err)
		}
		if ref.External == "" {
			if ref.Name == "" {
				return "", fmt.Errorf("spec.billingAccountRef needs external or name")
			}
			if err := ref.Normalize(ctx, reader, u.GetNamespace()); err != nil {
				return "", fmt.Errorf("resolving spec.billingAccountRef: %w", err)
			}
		}
		id := &billingv1alpha1.BillingAccountIdentity{}
		if err := id.FromExternal(ref.External); err != nil {
			return "", fmt.Errorf("resolving spec.billingAccountRef: %w", err)
		}
		return "billingAccounts/" + id.BillingAccount, nil
	}
	return "", fmt.Errorf("unknown parent kind %q", set[0])
}

type adapter struct {
	kind    *Kind
	api     *API
	name    string
	desired map[string]any
	// query is the spec's query parameters (Kind.Query), sent with every
	// write.
	query  map[string]string
	actual map[string]any
	// recorded: the object has made or acquired its resource
	// (status.externalRef is set).
	recorded bool
	// deleting: the object is being deleted (it has a deletion timestamp).
	deleting bool
	// inputVersion is the version of the spec's input-only fields
	// (Kind.InputOnly), written to status.inputVersion.
	inputVersion string
	// resend are top-level fields holding input-only fields
	// (Kind.InputOnly, Kind.Secrets) that Update sends whatever the read
	// shows, because status.inputVersion is not inputVersion.
	resend []string
	// secrets is where the API's secrets go (Kind.SecretOutputs); nil for
	// a kind without them, and while deleting.
	secrets *secretOutput
}

var _ directbase.Adapter = &adapter{}

func (a *adapter) Find(ctx context.Context) (bool, error) {
	if a.name == "" {
		// Deleting an object that never had a resource (AdapterForObject).
		return false, nil
	}
	got, err := a.api.Get(ctx, a.name)
	if err != nil {
		if direct.IsNotFound(err) {
			return false, nil
		}
		if a.kind.MissingIs403 && !a.recorded && direct.HasHTTPCode(err, http.StatusForbidden) {
			// The API answers 403 for a resource that does not exist
			// (Kind.MissingIs403), and this object has never made or acquired
			// one: create it. A real denial fails the create.
			klog.FromContext(ctx).V(2).Info("reading 403 before a create as not found", "kind", a.kind.GVK.Kind, "name", a.name)
			return false, nil
		}
		if a.deleting && a.goneOn403(err) {
			// Deleting, and the read answers 403: gone only when a listing
			// proves it (GoneAfter403); else a denial, and the finalizer stays.
			gone, why, perr := a.GoneAfter403(ctx)
			if perr != nil {
				return false, fmt.Errorf("getting %s %s: %w; and proving it gone: %v", a.kind.GVK.Kind, a.name, err, perr)
			}
			if gone {
				klog.FromContext(ctx).Info("deleting: the read answered 403 and the resource is gone", "kind", a.kind.GVK.Kind, "name", a.name, "proof", why)
				return false, nil
			}
			return false, fmt.Errorf("getting %s %s: %w (not taken for gone: %s)", a.kind.GVK.Kind, a.name, err, why)
		}
		return false, fmt.Errorf("getting %s %s: %w", a.kind.GVK.Kind, a.name, err)
	}
	if a.kind.softDeleted(got) {
		return false, nil
	}
	a.actual = got
	return true, nil
}

func (k *Kind) softDeleted(obj map[string]any) bool {
	if k.SoftDelete == nil {
		return false
	}
	v, _ := obj[k.SoftDelete.Field].(string)
	return v == k.SoftDelete.Value
}

func (a *adapter) Create(ctx context.Context, op *directbase.CreateOperation) error {
	log := klog.FromContext(ctx)
	log.V(2).Info("creating", "kind", a.kind.GVK.Kind, "name", a.name)
	if a.kind.PatchCreates {
		// No create method: patch makes the resource.
		if err := a.patch(ctx, a.desired); err != nil {
			return fmt.Errorf("creating %s %s: %w", a.kind.GVK.Kind, a.name, err)
		}
		got, err := a.api.Get(ctx, a.name)
		if err != nil {
			return fmt.Errorf("reading %s %s after create: %w", a.kind.GVK.Kind, a.name, err)
		}
		return op.UpdateStatus(ctx, a.status(got), nil)
	}
	if a.secrets != nil {
		// Before the resource exists: a Secret that cannot be ours stops the
		// create here, not after the API has made a secret it returns once.
		if err := a.secrets.reserve(ctx); err != nil {
			return err
		}
	}
	coll := CollectionOf(a.name)
	id := a.name[len(coll)+1:]
	query := map[string]string{}
	body := a.desired
	if a.kind.InsertStyle {
		body = runtime.DeepCopyJSON(a.desired)
		body["name"] = id
	} else if a.kind.IDParam != "" || a.kind.NameInBody {
		if a.kind.IDParam != "" {
			query[a.kind.IDParam] = id
		}
		if a.kind.NameInBody {
			body = runtime.DeepCopyJSON(a.desired)
			body["name"] = a.name
		}
	}
	resp, err := a.api.Do(ctx, http.MethodPost, coll, query, body)
	if err != nil {
		return fmt.Errorf("creating %s %s: %w", a.kind.GVK.Kind, a.name, err)
	}
	created := resp
	if a.kind.CreateLRO {
		if created, err = a.api.Wait(ctx, resp); err != nil {
			return fmt.Errorf("creating %s %s: %w", a.kind.GVK.Kind, a.name, err)
		}
	}
	// Read it back: an operation's response or a create's answer may leave
	// out what a get shows.
	got, err := a.api.Get(ctx, a.name)
	if err != nil {
		if a.secrets != nil {
			// The next reconcile finds the resource and reads the rest, but
			// only this create's answer holds Kind.CreateOnlyOutputs: keep
			// what it has now.
			if werr := a.secrets.write(ctx, secretValues(a.kind, nil, created), true); werr != nil {
				return fmt.Errorf("reading %s %s after create: %w; and %v", a.kind.GVK.Kind, a.name, err, werr)
			}
		}
		return fmt.Errorf("reading %s %s after create: %w", a.kind.GVK.Kind, a.name, err)
	}
	if a.secrets != nil {
		// The create's answer holds what only a create returns
		// (Kind.CreateOnlyOutputs); a read holds the rest.
		if err := a.secrets.write(ctx, secretValues(a.kind, got, created), false); err != nil {
			return fmt.Errorf("%s %s was made, but %w", a.kind.GVK.Kind, a.name, err)
		}
	}
	return op.UpdateStatus(ctx, a.status(got), nil)
}

func (a *adapter) Update(ctx context.Context, op *directbase.UpdateOperation) error {
	mask := Diff(a.kind.Comparable(a.desired), a.actual)
	if len(a.resend) > 0 {
		mask = union(mask, a.resend)
	}
	got := a.actual
	if len(mask) > 0 {
		if !a.kind.Updatable {
			return fmt.Errorf("%s %s: %s differ from the cloud, and the API cannot change them in place; delete and recreate the object", a.kind.GVK.Kind, a.name, strings.Join(mask, ", "))
		}
		klog.FromContext(ctx).V(2).Info("updating", "kind", a.kind.GVK.Kind, "name", a.name, "mask", mask)
		if a.kind.PatchCreates || a.kind.PatchNoMask {
			if err := a.patch(ctx, a.desired); err != nil {
				return fmt.Errorf("updating %s %s: %w", a.kind.GVK.Kind, a.name, err)
			}
			got, err := a.api.Get(ctx, a.name)
			if err != nil {
				return fmt.Errorf("reading %s %s after update: %w", a.kind.GVK.Kind, a.name, err)
			}
			if err := a.writeSecrets(ctx, got); err != nil {
				return err
			}
			return op.UpdateStatus(ctx, a.status(got), nil)
		}
		body := map[string]any{}
		for _, f := range mask {
			if v, ok := a.desired[f]; ok {
				body[f] = v
			}
		}
		method, query := http.MethodPatch, map[string]string{"updateMask": strings.Join(mask, ",")}
		if a.kind.UpdatePUT && !a.kind.InsertStyle {
			// A replace (Kind.UpdatePUT, a named PUT method such as
			// replacePlatformPolicy): the whole spec, no mask.
			body, query = runtime.DeepCopyJSON(a.desired), nil
		}
		if a.kind.InsertStyle {
			query = nil
			if a.kind.UpdatePUT {
				// update replaces the resource: the read one with ours over it.
				body = runtime.DeepCopyJSON(a.actual)
				for f, v := range a.desired {
					body[f] = v
				}
			}
			if fp, ok := a.actual["fingerprint"]; ok {
				body["fingerprint"] = fp
			}
			body["name"] = a.actual["name"]
		}
		if a.kind.UpdatePUT {
			method = http.MethodPut
		}
		if len(a.query) > 0 && query == nil {
			query = map[string]string{}
		}
		for k, v := range a.query {
			query[k] = v
		}
		resp, err := a.api.Do(ctx, method, a.name, query, body)
		if err != nil {
			return fmt.Errorf("updating %s %s: %w", a.kind.GVK.Kind, a.name, err)
		}
		if a.kind.UpdateLRO {
			if _, err := a.api.Wait(ctx, resp); err != nil {
				return fmt.Errorf("updating %s %s: %w", a.kind.GVK.Kind, a.name, err)
			}
		}
		if got, err = a.api.Get(ctx, a.name); err != nil {
			return fmt.Errorf("reading %s %s after update: %w", a.kind.GVK.Kind, a.name, err)
		}
	}
	if err := a.writeSecrets(ctx, got); err != nil {
		return err
	}
	return op.UpdateStatus(ctx, a.status(got), nil)
}

// writeSecrets keeps the object's Secret holding what a read of the
// resource shows of its SecretOutputs, and fails when it lacks one a read
// cannot give back (Kind.CreateOnlyOutputs).
func (a *adapter) writeSecrets(ctx context.Context, got map[string]any) error {
	if a.secrets == nil {
		return nil
	}
	if err := a.secrets.write(ctx, secretValues(a.kind, got, nil), false); err != nil {
		return fmt.Errorf("%s %s: %w", a.kind.GVK.Kind, a.name, err)
	}
	return nil
}

// patch sends body to the resource's name, with the spec's query
// parameters and an update mask of the fields it sets, or, when the API's
// patch takes none (PatchNoMask), as it is. It waits for the operation
// when patch answers one.
func (a *adapter) patch(ctx context.Context, body map[string]any) error {
	query := map[string]string{}
	for k, v := range a.query {
		query[k] = v
	}
	if !a.kind.PatchNoMask {
		fields := make([]string, 0, len(body))
		for f := range body {
			fields = append(fields, f)
		}
		sort.Strings(fields)
		if len(fields) > 0 {
			query["updateMask"] = strings.Join(fields, ",")
		}
	}
	resp, err := a.api.Do(ctx, http.MethodPatch, a.name, query, body)
	if err != nil {
		return err
	}
	if a.kind.UpdateLRO {
		if _, err := a.api.Wait(ctx, resp); err != nil {
			return err
		}
	}
	return nil
}

// Diff is the top-level fields whose desired value the actual object does
// not hold, sorted. A field the spec leaves out is the API's to default
// and is never a difference; within a field, what the spec sets must
// match (Subset).
func Diff(desired, actual map[string]any) []string {
	var mask []string
	for f, v := range desired {
		if !Subset(v, actual[f]) {
			mask = append(mask, f)
		}
	}
	sort.Strings(mask)
	return mask
}

// union is a and b without repeats, sorted.
func union(a, b []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range append(append([]string(nil), a...), b...) {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	sort.Strings(out)
	return out
}

// Subset reports whether every value want sets is in got: maps by key,
// lists element by element and of equal length, numbers by value whether
// the API wrote them as numbers or (int64) as strings, and a Compute
// Engine resource's path as its full URL (sameComputeURL).
func Subset(want, got any) bool {
	switch w := want.(type) {
	case nil:
		return true
	case map[string]any:
		g, ok := got.(map[string]any)
		if !ok {
			return len(w) == 0 && got == nil
		}
		for k, v := range w {
			if !Subset(v, g[k]) {
				return false
			}
		}
		return true
	case []any:
		g, ok := got.([]any)
		if !ok {
			return len(w) == 0 && got == nil
		}
		if len(w) != len(g) {
			return false
		}
		for i := range w {
			if !Subset(w[i], g[i]) {
				return false
			}
		}
		return true
	case string:
		if g, ok := got.(string); ok {
			return w == g || sameComputeURL(w, g) || sameBucket(w, g)
		}
		if got == nil {
			return w == ""
		}
		return w == scalarString(got)
	case bool:
		g, _ := got.(bool)
		return w == g
	default:
		if got == nil {
			return scalarString(w) == "0"
		}
		return scalarString(w) == scalarString(got)
	}
}

// bucketPathRE is a Cloud Storage bucket's resource path,
// projects/P/buckets/B.
var bucketPathRE = regexp.MustCompile(`^projects/[^/]+/buckets/([^/]+)$`)

// sameBucket reports whether want, a bucket's path
// (projects/P/buckets/B), names the bucket got names by its bare name:
// Dataplex answers an asset's resourceSpec.name with the bare bucket name
// whatever path it was sent (run cce-11140351-e560), which otherwise is
// a difference on every reconcile, and a patch, forever.
func sameBucket(want, got string) bool {
	m := bucketPathRE.FindStringSubmatch(want)
	return m != nil && m[1] == got
}

// computeURLPrefixes are the forms of a Compute Engine resource's URL
// that the API writes in place of the path a request sent
// (projects/P/regions/R/subnetworks/S): it answers with its own full URL.
var computeURLPrefixes = []string{
	"https://www.googleapis.com/compute/v1/",
	"https://www.googleapis.com/compute/beta/",
	"https://compute.googleapis.com/compute/v1/",
	"https://compute.googleapis.com/compute/beta/",
}

// sameComputeURL reports whether two strings name the same Compute Engine
// resource, one as its full URL, the other as that URL or its path from
// projects/.
func sameComputeURL(a, b string) bool {
	trim := func(s string) (string, bool) {
		for _, p := range computeURLPrefixes {
			if strings.HasPrefix(s, p) {
				return strings.TrimPrefix(s, p), true
			}
		}
		return s, false
	}
	a, aURL := trim(a)
	b, bURL := trim(b)
	return (aURL || bURL) && strings.HasPrefix(a, "projects/") && a == b
}

func scalarString(v any) string {
	switch n := v.(type) {
	case float64:
		if n == float64(int64(n)) {
			return fmt.Sprintf("%d", int64(n))
		}
		return fmt.Sprintf("%v", n)
	case int64:
		return fmt.Sprintf("%d", n)
	case int32:
		return fmt.Sprintf("%d", n)
	case int:
		return fmt.Sprintf("%d", n)
	default:
		return fmt.Sprintf("%v", v)
	}
}

// status is what UpdateStatus writes (it takes a pointer to a struct).
type status struct {
	ExternalRef   string         `json:"externalRef"`
	ObservedState map[string]any `json:"observedState,omitempty"`
	InputVersion  string         `json:"inputVersion,omitempty"`
}

func (a *adapter) status(got map[string]any) *status {
	observed := map[string]any{}
	for _, f := range a.kind.Output {
		if slices.Contains(a.kind.SecretOutputs, f) {
			// Never in status: the object's Secret holds it (secretOutput).
			continue
		}
		if v, ok := got[f]; ok && f != "name" {
			observed[f] = v
		}
	}
	s := &status{ExternalRef: a.name, InputVersion: a.inputVersion}
	if len(observed) > 0 {
		s.ObservedState = observed
	}
	return s
}

func (a *adapter) Export(ctx context.Context) (*unstructured.Unstructured, error) {
	return nil, fmt.Errorf("%s does not support export", a.kind.GVK.Kind)
}

// DeleteRetryFor bounds how long Delete tries again a delete the API
// refuses because the resource is still in use (InUse); after it, the
// error goes back to the reconciler, which tries again later.
var DeleteRetryFor = 10 * time.Minute

func (a *adapter) Delete(ctx context.Context, op *directbase.DeleteOperation) (bool, error) {
	// A delete the API refuses because something still uses the resource
	// (an API Gateway config whose gateway is being deleted beside it) is
	// tried again, less often each time, until it goes through or
	// DeleteRetryFor passes. The finalizer stays until the delete, and its
	// operation, ends.
	wait := a.api.PollInterval
	if wait <= 0 {
		wait = 2 * time.Second
	}
	deadline := time.Now().Add(DeleteRetryFor)
	if f := a.kind.DisableBeforeDelete; f != "" && a.actual != nil && a.actual[f] != true {
		// The API wants the resource disabled before it deletes it.
		klog.FromContext(ctx).V(2).Info("disabling before the delete", "kind", a.kind.GVK.Kind, "name", a.name, "field", f)
		resp, err := a.api.Do(ctx, http.MethodPatch, a.name, map[string]string{"updateMask": f}, map[string]any{f: true})
		if err == nil && a.kind.UpdateLRO {
			_, err = a.api.Wait(ctx, resp)
		}
		if err != nil && !direct.IsNotFound(err) {
			return false, fmt.Errorf("disabling %s %s before deleting it: %w", a.kind.GVK.Kind, a.name, err)
		}
	}
	for {
		err := a.deleteOnce(ctx)
		if err == nil {
			return true, nil
		}
		if !InUse(err) || time.Now().Add(wait).After(deadline) {
			return false, fmt.Errorf("deleting %s %s: %w", a.kind.GVK.Kind, a.name, err)
		}
		klog.FromContext(ctx).Info("delete refused while the resource is in use; trying again", "kind", a.kind.GVK.Kind, "name", a.name, "retryAfter", wait, "error", err.Error())
		select {
		case <-ctx.Done():
			return false, fmt.Errorf("deleting %s %s: %w (last: %v)", a.kind.GVK.Kind, a.name, ctx.Err(), err)
		case <-time.After(wait):
		}
		if wait += wait / 2; wait > 30*time.Second {
			wait = 30 * time.Second
		}
	}
}

// deleteOnce sends the delete and, when it answers an operation, waits for
// the operation to end: the resource is gone only then, and an operation
// that fails (an API Gateway config still in use) is the delete's error.
func (a *adapter) deleteOnce(ctx context.Context) error {
	resp, err := a.api.Do(ctx, http.MethodDelete, a.name, nil, nil)
	if err != nil {
		if direct.IsNotFound(err) {
			return nil
		}
		if a.goneOn403(err) {
			// The resource went between the read and the delete (its parent's
			// delete took it along): gone only when a listing proves it.
			gone, why, perr := a.GoneAfter403(ctx)
			if perr != nil {
				return fmt.Errorf("%w; and proving it gone: %v", err, perr)
			}
			if gone {
				klog.FromContext(ctx).Info("the delete answered 403 and the resource is gone", "kind", a.kind.GVK.Kind, "name", a.name, "proof", why)
				return nil
			}
			return fmt.Errorf("%w (not taken for gone: %s)", err, why)
		}
		return err
	}
	if a.kind.DeleteLRO {
		if _, err := a.api.Wait(ctx, resp); err != nil && !direct.IsNotFound(err) {
			return err
		}
	}
	return nil
}

// inUseRE matches what APIs say when a resource cannot go while another
// uses it or it holds others: API Gateway's "config is in use" and "has
// nested resources".
var inUseRE = regexp.MustCompile(`(?i)\bin use\b|is used by|nested resources`)

// InUse reports whether err is an API's refusal to delete a resource that
// is still in use or still holds others: FAILED_PRECONDITION, or a
// message that says so. A delete refused so goes through once the user or
// the child is gone.
func InUse(err error) bool {
	var ge *googleapi.Error
	if !errors.As(err, &ge) {
		return false
	}
	switch ge.Code {
	case http.StatusBadRequest, http.StatusConflict, http.StatusPreconditionFailed:
	default:
		return false
	}
	return strings.Contains(ge.Body, `"FAILED_PRECONDITION"`) || inUseRE.MatchString(ge.Message)
}

// CloudLabels are the found resource's labels, for the slot guard
// (pkg/slotguard), when its collection has them.
func (a *adapter) CloudLabels() map[string]string {
	raw, _ := a.actual["labels"].(map[string]any)
	out := make(map[string]string, len(raw))
	for k, v := range raw {
		if s, ok := v.(string); ok {
			out[k] = s
		}
	}
	return out
}
