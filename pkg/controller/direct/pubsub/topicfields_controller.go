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

package pubsub

// Written here: PubSubTopicFields, a field kind (docs/design.md section 6.6,
// pattern 2; decision 37). No Config Connector code or pull request sets a
// topic's ingestionDataSourceSettings or messageTransforms. The topic is
// read with topics.get and written with topics.patch, whose update mask
// names only the fields this object owns. Config Connector's PubSubTopic
// runs Terraform by default, whose google_pubsub_topic update patches with
// a mask of the fields it changed (kmsKeyName, labels,
// messageStoragePolicy, schemaSettings, messageRetentionDuration; read at
// the pin's fork), so it leaves these alone; its opt-in direct controller
// sends a mask of the fields that differ from its spec, which has neither.
// Both to be confirmed live, as for the other field kinds.

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	pubsubv1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/apis/pubsub/v1beta1"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/config"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct/directbase"
	api "google.golang.org/api/pubsub/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"

	krm "github.com/highfivelabs/config-connector-extensions/apis/pubsub/v1alpha1"
	"github.com/highfivelabs/config-connector-extensions/pkg/fieldkind"
)

// NewTopicFieldsModel is the PubSubTopicFields model.
func NewTopicFieldsModel(ctx context.Context, config *config.ControllerConfig) (directbase.Model, error) {
	return &topicFieldsModel{config: *config}, nil
}

type topicFieldsModel struct {
	config config.ControllerConfig
}

var _ directbase.Model = &topicFieldsModel{}

func (m *topicFieldsModel) AdapterForObject(ctx context.Context, op *directbase.AdapterForObjectOperation) (directbase.Adapter, error) {
	obj := &krm.PubSubTopicFields{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(op.GetUnstructured().Object, obj); err != nil {
		return nil, fmt.Errorf("error converting to %T: %w", obj, err)
	}
	ref := obj.Spec.ResourceRef
	if ref == nil || (ref.External == "" && ref.Name == "") {
		return nil, fmt.Errorf("spec.resourceRef needs external or name")
	}
	prev := direct.ValueOf(obj.Status.ExternalRef)
	if fieldkind.Deleting(obj, prev) {
		return fieldkind.Gone{}, nil
	}
	if obj.GetDeletionTimestamp() != nil && prev != "" {
		// Deleting: the topic is in status; Config Connector's PubSubTopic
		// may be gone already.
		ref = &pubsubv1beta1.PubSubTopicRef{External: prev}
	} else {
		r := *ref
		if err := r.Normalize(ctx, op.Reader, obj.GetNamespace()); err != nil {
			return nil, err
		}
		ref = &r
	}
	id := &pubsubv1beta1.PubSubTopicIdentity{}
	if err := id.FromExternal(ref.External); err != nil {
		return nil, err
	}
	if prev != "" && prev != id.String() {
		return nil, fmt.Errorf("cannot change the topic of PubSubTopicFields (old=%q, new=%q)", prev, id.String())
	}
	if err := fieldkind.CheckSoleOwner(ctx, op.Reader, krm.PubSubTopicFieldsGVK, obj, id.String()); err != nil {
		return nil, err
	}
	opts, err := m.config.RESTClientOptions()
	if err != nil {
		return nil, err
	}
	c, err := api.NewService(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("building pubsub client: %w", err)
	}
	return &topicFieldsAdapter{id: id, gcpClient: c, desired: &obj.Spec}, nil
}

func (m *topicFieldsModel) AdapterForURL(ctx context.Context, url string) (directbase.Adapter, error) {
	return nil, nil
}

type topicFieldsAdapter struct {
	id        *pubsubv1beta1.PubSubTopicIdentity
	gcpClient *api.Service
	desired   *krm.PubSubTopicFieldsSpec
	actual    *api.Topic
}

var _ directbase.Adapter = &topicFieldsAdapter{}

// Find reports whether the topic exists. A missing one is "not found" so
// that a delete finishes when it is gone; a create then waits (Create).
func (a *topicFieldsAdapter) Find(ctx context.Context) (bool, error) {
	t, err := a.gcpClient.Projects.Topics.Get(a.id.String()).Context(ctx).Do()
	if err != nil {
		if direct.IsNotFound(err) {
			return false, nil
		}
		return false, fmt.Errorf("getting Pub/Sub topic %s: %w", a.id, err)
	}
	a.actual = t
	return true, nil
}

// Create is called only while the topic does not exist: this kind never
// makes it, so it waits for it.
func (a *topicFieldsAdapter) Create(ctx context.Context, createOp *directbase.CreateOperation) error {
	return fieldkind.NotFoundError("PubSubTopic", a.id.String())
}

// owned are the fields this kind can own: their JSON names, which are the
// API's, and so the update mask's paths.
var owned = []string{"ingestionDataSourceSettings", "messageTransforms"}

// ownedFields is which of owned the spec sets.
func ownedFields(spec *krm.PubSubTopicFieldsSpec) []string {
	var out []string
	if spec.IngestionDataSourceSettings != nil {
		out = append(out, "ingestionDataSourceSettings")
	}
	if spec.MessageTransforms != nil {
		out = append(out, "messageTransforms")
	}
	return out
}

// topicJSON is a value's owned fields as JSON values, the empty ones
// pruned, so that unset, "", false and [] compare the same. The API
// answers its own output-only fields (each source's state) in the same
// objects; those are dropped.
func topicJSON(v any) (map[string]any, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, err
	}
	out := map[string]any{}
	for _, f := range owned {
		if p := pruneEmpty(m[f]); p != nil {
			out[f] = p
		}
	}
	if s, ok := out["ingestionDataSourceSettings"].(map[string]any); ok {
		for _, src := range s {
			if src, ok := src.(map[string]any); ok {
				delete(src, "state")
			}
		}
	}
	return out, nil
}

func (a *topicFieldsAdapter) Update(ctx context.Context, updateOp *directbase.UpdateOperation) error {
	want, err := topicJSON(a.desired)
	if err != nil {
		return err
	}
	have, err := topicJSON(a.actual)
	if err != nil {
		return err
	}
	var mask []string
	for _, f := range ownedFields(a.desired) {
		if !reflect.DeepEqual(want[f], have[f]) {
			mask = append(mask, f)
		}
	}
	if len(mask) > 0 {
		klog.FromContext(ctx).V(2).Info("setting fields of a Pub/Sub topic", "name", a.id, "updateMask", mask)
		body, err := specToTopic(a.desired)
		if err != nil {
			return err
		}
		if err := a.patch(ctx, body, mask); err != nil {
			return err
		}
	}
	return a.updateStatus(ctx, updateOp)
}

// specToTopic is the spec's fields as the API's topic: the JSON names are
// the API's.
func specToTopic(spec *krm.PubSubTopicFieldsSpec) (*api.Topic, error) {
	s := spec.DeepCopy()
	s.ResourceRef = nil
	b, err := json.Marshal(s)
	if err != nil {
		return nil, err
	}
	out := &api.Topic{}
	if err := json.Unmarshal(b, out); err != nil {
		return nil, fmt.Errorf("mapping the spec to the API: %w", err)
	}
	return out, nil
}

// patch sends the fields mask names, and nothing else.
func (a *topicFieldsAdapter) patch(ctx context.Context, body *api.Topic, mask []string) error {
	t, err := a.gcpClient.Projects.Topics.Patch(a.id.String(), &api.UpdateTopicRequest{
		Topic:      body,
		UpdateMask: strings.Join(mask, ","),
	}).Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("setting %s of Pub/Sub topic %s: %w", strings.Join(mask, ", "), a.id, err)
	}
	a.actual = t
	return nil
}

// Delete clears the fields this object owns (the topic's state before
// it: no import source, no transforms), and never deletes the topic. A
// topic that is gone is not reached: Find said not found.
func (a *topicFieldsAdapter) Delete(ctx context.Context, deleteOp *directbase.DeleteOperation) (bool, error) {
	have, err := topicJSON(a.actual)
	if err != nil {
		return false, err
	}
	var mask []string
	for _, f := range ownedFields(a.desired) {
		if have[f] != nil {
			mask = append(mask, f)
		}
	}
	if len(mask) == 0 {
		return true, nil
	}
	klog.FromContext(ctx).V(2).Info("clearing fields of a Pub/Sub topic", "name", a.id, "updateMask", mask)
	if err := a.patch(ctx, &api.Topic{}, mask); err != nil {
		return false, err
	}
	return true, nil
}

func (a *topicFieldsAdapter) Export(ctx context.Context) (*unstructured.Unstructured, error) {
	return nil, nil
}

func (a *topicFieldsAdapter) updateStatus(ctx context.Context, op directbase.Operation) error {
	status := &krm.PubSubTopicFieldsStatus{ExternalRef: direct.LazyPtr(a.id.String())}
	if a.actual != nil {
		status.State = direct.LazyPtr(a.actual.State)
	}
	return op.UpdateStatus(ctx, status, nil)
}

// pruneEmpty drops "", false, 0, null, and lists and objects that are (or
// become) empty; an empty object in a list stays, as a placeholder.
func pruneEmpty(v any) any {
	switch t := v.(type) {
	case map[string]any:
		out := map[string]any{}
		for k, e := range t {
			if p := pruneEmpty(e); p != nil {
				out[k] = p
			}
		}
		if len(out) == 0 {
			return nil
		}
		return out
	case []any:
		var out []any
		for _, e := range t {
			p := pruneEmpty(e)
			if p == nil {
				p = map[string]any{}
			}
			out = append(out, p)
		}
		if len(out) == 0 {
			return nil
		}
		return out
	case string:
		if t == "" {
			return nil
		}
	case bool:
		if !t {
			return nil
		}
	case float64:
		if t == 0 {
			return nil
		}
	case nil:
		return nil
	}
	return v
}
