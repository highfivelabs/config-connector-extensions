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
	"fmt"
	"net/http"
	"sort"
	"strings"

	billingv1alpha1 "github.com/GoogleCloudPlatform/k8s-config-connector/apis/billing/v1alpha1"
	refsv1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/apis/refs/v1beta1"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/config"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct/directbase"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
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
	var name string
	var body map[string]any
	switch deleting := !u.GetDeletionTimestamp().IsZero(); {
	case deleting && ext != "":
		// Deleting a resource that was made or acquired: its name is in
		// status, and the parts of a name cannot change, so the parent is not
		// resolved. A parent object already gone or going (everything deleted
		// at once) would otherwise hold the delete forever (IAMBinding's rule,
		// pkg/iampolicy.Resolve).
		name = ext
	default:
		var err error
		name, body, err = m.kind.resolve(ctx, op.Reader, u)
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
	return &adapter{kind: m.kind, api: &API{HTTP: httpClient, BaseURL: m.kind.BaseURL}, name: name, desired: body}, nil
}

// AdapterForURL is not supported: these kinds are not exported.
func (m *model) AdapterForURL(ctx context.Context, url string) (directbase.Adapter, error) {
	return nil, nil
}

// resolve is the object's resource name and the request body its spec
// asks for.
func (k *Kind) resolve(ctx context.Context, reader client.Reader, u *unstructured.Unstructured) (string, map[string]any, error) {
	spec, _, err := unstructured.NestedMap(u.Object, "spec")
	if err != nil {
		return "", nil, fmt.Errorf("reading spec: %w", err)
	}
	if spec == nil {
		spec = map[string]any{}
	}
	parent := ""
	if len(k.Parents) > 0 {
		if parent, err = resolveParent(ctx, reader, u, k.Parents, spec); err != nil {
			return "", nil, err
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
		return "", nil, err
	}
	return name, k.Body(spec), nil
}

// Body is the request body a spec asks for: the spec without the parent
// reference, the fields that make up the name and resourceID.
func (k *Kind) Body(spec map[string]any) map[string]any {
	body := runtime.DeepCopyJSON(spec)
	delete(body, "resourceID")
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
	actual  map[string]any
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
	coll := CollectionOf(a.name)
	id := a.name[len(coll)+1:]
	query := map[string]string{}
	body := a.desired
	if a.kind.InsertStyle {
		body = runtime.DeepCopyJSON(a.desired)
		body["name"] = id
	} else if a.kind.IDParam != "" {
		query[a.kind.IDParam] = id
	}
	resp, err := a.api.Do(ctx, http.MethodPost, coll, query, body)
	if err != nil {
		return fmt.Errorf("creating %s %s: %w", a.kind.GVK.Kind, a.name, err)
	}
	if a.kind.CreateLRO {
		if _, err := a.api.Wait(ctx, resp); err != nil {
			return fmt.Errorf("creating %s %s: %w", a.kind.GVK.Kind, a.name, err)
		}
	}
	// Read it back: an operation's response or a create's answer may leave
	// out what a get shows.
	got, err := a.api.Get(ctx, a.name)
	if err != nil {
		return fmt.Errorf("reading %s %s after create: %w", a.kind.GVK.Kind, a.name, err)
	}
	return op.UpdateStatus(ctx, a.status(got), nil)
}

func (a *adapter) Update(ctx context.Context, op *directbase.UpdateOperation) error {
	mask := Diff(a.desired, a.actual)
	got := a.actual
	if len(mask) > 0 {
		if !a.kind.Updatable {
			return fmt.Errorf("%s %s: %s differ from the cloud, and the API cannot change them in place; delete and recreate the object", a.kind.GVK.Kind, a.name, strings.Join(mask, ", "))
		}
		klog.FromContext(ctx).V(2).Info("updating", "kind", a.kind.GVK.Kind, "name", a.name, "mask", mask)
		body := map[string]any{}
		for _, f := range mask {
			if v, ok := a.desired[f]; ok {
				body[f] = v
			}
		}
		method, query := http.MethodPatch, map[string]string{"updateMask": strings.Join(mask, ",")}
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
	return op.UpdateStatus(ctx, a.status(got), nil)
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
			return w == g || sameComputeURL(w, g)
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
}

func (a *adapter) status(got map[string]any) *status {
	observed := map[string]any{}
	for _, f := range a.kind.Output {
		if v, ok := got[f]; ok && f != "name" {
			observed[f] = v
		}
	}
	s := &status{ExternalRef: a.name}
	if len(observed) > 0 {
		s.ObservedState = observed
	}
	return s
}

func (a *adapter) Export(ctx context.Context) (*unstructured.Unstructured, error) {
	return nil, fmt.Errorf("%s does not support export", a.kind.GVK.Kind)
}

func (a *adapter) Delete(ctx context.Context, op *directbase.DeleteOperation) (bool, error) {
	resp, err := a.api.Do(ctx, http.MethodDelete, a.name, nil, nil)
	if err != nil {
		if direct.IsNotFound(err) {
			return true, nil
		}
		return false, fmt.Errorf("deleting %s %s: %w", a.kind.GVK.Kind, a.name, err)
	}
	if a.kind.DeleteLRO {
		if _, err := a.api.Wait(ctx, resp); err != nil && !direct.IsNotFound(err) {
			return false, fmt.Errorf("deleting %s %s: %w", a.kind.GVK.Kind, a.name, err)
		}
	}
	return true, nil
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
