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

// Written here: PubSubSubscriptionFields, a field kind (docs/design.md
// section 6.6, pattern 2; decision 37), PubSubTopicFields' twin. No Config
// Connector code or pull request sets a subscription's messageTransforms
// or bigtableConfig, or its exports' service accounts, filename datetime
// format, maximum messages per file or Avro useTopicSchema. (useTableSchema
// is left to Config Connector pull request #13712.) The subscription is
// read with subscriptions.get and written with subscriptions.patch.
//
// Config Connector's PubSubSubscription runs Terraform by default, whose
// google_pubsub_subscription update patches with a mask of the top-level
// fields it changed (read at the pin's fork); its opt-in direct
// controller sends a mask of the top-level fields that differ from its
// spec. Neither has messageTransforms or bigtableConfig, so neither sends
// them: those two are ours alone, with masks naming only them. The export
// fields are the partial case (as a node pool's kubeletConfig is,
// ContainerNodePoolFields): the API takes bigqueryConfig and
// cloudStorageConfig whole, so ours sends the export as read with its
// fields set, and a change Config Connector makes to the same export
// sends it without them, until our next reconcile puts them back. Both to
// be confirmed live.

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

// NewSubscriptionFieldsModel is the PubSubSubscriptionFields model.
func NewSubscriptionFieldsModel(ctx context.Context, config *config.ControllerConfig) (directbase.Model, error) {
	return &subscriptionFieldsModel{config: *config}, nil
}

type subscriptionFieldsModel struct {
	config config.ControllerConfig
}

var _ directbase.Model = &subscriptionFieldsModel{}

func (m *subscriptionFieldsModel) AdapterForObject(ctx context.Context, op *directbase.AdapterForObjectOperation) (directbase.Adapter, error) {
	obj := &krm.PubSubSubscriptionFields{}
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
		// Deleting: the subscription is in status; Config Connector's
		// PubSubSubscription may be gone already.
		ref = &pubsubv1beta1.PubSubSubscriptionRef{External: prev}
	} else {
		r := *ref
		if err := r.Normalize(ctx, op.Reader, obj.GetNamespace()); err != nil {
			return nil, err
		}
		ref = &r
	}
	id := &pubsubv1beta1.PubSubSubscriptionIdentity{}
	if err := id.FromExternal(ref.External); err != nil {
		return nil, err
	}
	if prev != "" && prev != id.String() {
		return nil, fmt.Errorf("cannot change the subscription of PubSubSubscriptionFields (old=%q, new=%q)", prev, id.String())
	}
	if err := fieldkind.CheckSoleOwner(ctx, op.Reader, krm.PubSubSubscriptionFieldsGVK, obj, id.String()); err != nil {
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
	return &subscriptionFieldsAdapter{id: id, gcpClient: c, desired: &obj.Spec}, nil
}

func (m *subscriptionFieldsModel) AdapterForURL(ctx context.Context, url string) (directbase.Adapter, error) {
	return nil, nil
}

type subscriptionFieldsAdapter struct {
	id        *pubsubv1beta1.PubSubSubscriptionIdentity
	gcpClient *api.Service
	desired   *krm.PubSubSubscriptionFieldsSpec
	actual    *api.Subscription
}

var _ directbase.Adapter = &subscriptionFieldsAdapter{}

// Find reports whether the subscription exists. A missing one is "not
// found" so that a delete finishes when it is gone; a create then waits
// (Create).
func (a *subscriptionFieldsAdapter) Find(ctx context.Context) (bool, error) {
	s, err := a.gcpClient.Projects.Subscriptions.Get(a.id.String()).Context(ctx).Do()
	if err != nil {
		if direct.IsNotFound(err) {
			return false, nil
		}
		return false, fmt.Errorf("getting Pub/Sub subscription %s: %w", a.id, err)
	}
	a.actual = s
	return true, nil
}

// Create is called only while the subscription does not exist: this kind
// never makes it, so it waits for it.
func (a *subscriptionFieldsAdapter) Create(ctx context.Context, createOp *directbase.CreateOperation) error {
	return fieldkind.NotFoundError("PubSubSubscription", a.id.String())
}

// asJSON is v as a JSON value, empty values pruned (pruneEmpty), so that
// unset, "", false and [] compare the same.
func asJSON(v any) (any, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	var out any
	if err := json.Unmarshal(b, &out); err != nil {
		return nil, err
	}
	return pruneEmpty(out), nil
}

// fromSpec maps a spec value to the API's type: the JSON names are the
// API's.
func fromSpec(in, out any) error {
	b, err := json.Marshal(in)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(b, out); err != nil {
		return fmt.Errorf("mapping the spec to the API: %w", err)
	}
	return nil
}

// exportsElsewhere is where the subscription delivers other than Bigtable
// ("" for a pull subscription or a Bigtable export).
func exportsElsewhere(s *api.Subscription) string {
	switch {
	case s.PushConfig != nil && s.PushConfig.PushEndpoint != "":
		return "pushes to " + s.PushConfig.PushEndpoint
	case s.BigqueryConfig != nil && s.BigqueryConfig.Table != "":
		return "exports to BigQuery"
	case s.CloudStorageConfig != nil && s.CloudStorageConfig.Bucket != "":
		return "exports to Cloud Storage"
	}
	return ""
}

// change is one top-level field of a patch: its mask path and the
// subscription's value for it.
type change struct {
	path string
	set  func(*api.Subscription)
}

// changes are the patches of the owned fields that differ from the
// subscription: want sets them as the spec says; clear (Delete) sets them
// to their state before this object.
func (a *subscriptionFieldsAdapter) changes(clear bool) ([]change, error) {
	d, s := a.desired, a.actual
	var out []change

	if d.MessageTransforms != nil {
		var want []*api.MessageTransform
		if !clear {
			if err := fromSpec(d.MessageTransforms, &want); err != nil {
				return nil, err
			}
		}
		w, err := asJSON(want)
		if err != nil {
			return nil, err
		}
		h, err := asJSON(s.MessageTransforms)
		if err != nil {
			return nil, err
		}
		if !reflect.DeepEqual(w, h) {
			out = append(out, change{"messageTransforms", func(b *api.Subscription) { b.MessageTransforms = want }})
		}
	}

	if d.BigtableConfig != nil {
		var want *api.BigtableConfig
		if !clear {
			want = &api.BigtableConfig{}
			if err := fromSpec(d.BigtableConfig, want); err != nil {
				return nil, err
			}
		}
		var have *api.BigtableConfig
		if s.BigtableConfig != nil {
			c := *s.BigtableConfig
			c.State = "" // output only
			have = &c
		}
		w, err := asJSON(want)
		if err != nil {
			return nil, err
		}
		h, err := asJSON(have)
		if err != nil {
			return nil, err
		}
		if !reflect.DeepEqual(w, h) {
			if !clear {
				if other := exportsElsewhere(s); other != "" {
					return nil, fmt.Errorf("Pub/Sub subscription %s %s, and a subscription delivers to one place: take that out of Config Connector's PubSubSubscription first (or Config Connector would put it back), then this object sets bigtableConfig", a.id, other)
				}
			}
			out = append(out, change{"bigtableConfig", func(b *api.Subscription) { b.BigtableConfig = want }})
		}
	}

	if f := d.BigqueryConfig; f != nil && f.ServiceAccountEmail != nil {
		if s.BigqueryConfig == nil || s.BigqueryConfig.Table == "" {
			if !clear {
				return nil, fmt.Errorf("Pub/Sub subscription %s has no BigQuery export: spec.bigqueryConfig sets fields of the one Config Connector's PubSubSubscription makes (its spec.bigqueryConfig)", a.id)
			}
		} else {
			want := ""
			if !clear {
				want = *f.ServiceAccountEmail
			}
			if s.BigqueryConfig.ServiceAccountEmail != want {
				// The API takes the export whole: as read, with ours set.
				c := *s.BigqueryConfig
				c.State = ""
				c.ServiceAccountEmail = want
				out = append(out, change{"bigqueryConfig", func(b *api.Subscription) { b.BigqueryConfig = &c }})
			}
		}
	}

	if f := d.CloudStorageConfig; f != nil {
		if s.CloudStorageConfig == nil || s.CloudStorageConfig.Bucket == "" {
			if !clear {
				return nil, fmt.Errorf("Pub/Sub subscription %s has no Cloud Storage export: spec.cloudStorageConfig sets fields of the one Config Connector's PubSubSubscription makes (its spec.cloudStorageConfig)", a.id)
			}
		} else {
			have := s.CloudStorageConfig
			c := *have
			c.State = ""
			differs := false
			if f.ServiceAccountEmail != nil {
				want := ""
				if !clear {
					want = *f.ServiceAccountEmail
				}
				differs = differs || have.ServiceAccountEmail != want
				c.ServiceAccountEmail = want
			}
			if f.FilenameDatetimeFormat != nil {
				want := ""
				if !clear {
					want = *f.FilenameDatetimeFormat
				}
				differs = differs || have.FilenameDatetimeFormat != want
				c.FilenameDatetimeFormat = want
			}
			if f.MaxMessages != nil {
				var want int64
				if !clear {
					want = *f.MaxMessages
				}
				differs = differs || have.MaxMessages != want
				c.MaxMessages = want
			}
			if f.AvroConfig != nil && f.AvroConfig.UseTopicSchema != nil {
				switch {
				case have.AvroConfig != nil:
					want := !clear && *f.AvroConfig.UseTopicSchema
					avro := *have.AvroConfig
					differs = differs || avro.UseTopicSchema != want
					avro.UseTopicSchema = want
					c.AvroConfig = &avro
				case !clear:
					return nil, fmt.Errorf("Pub/Sub subscription %s's Cloud Storage export writes text: spec.cloudStorageConfig.avroConfig sets a field of an Avro export (Config Connector's spec.cloudStorageConfig.avroConfig)", a.id)
				}
			}
			if differs {
				out = append(out, change{"cloudStorageConfig", func(b *api.Subscription) { b.CloudStorageConfig = &c }})
			}
		}
	}
	return out, nil
}

func (a *subscriptionFieldsAdapter) Update(ctx context.Context, updateOp *directbase.UpdateOperation) error {
	ch, err := a.changes(false)
	if err != nil {
		return err
	}
	if err := a.patch(ctx, "setting", ch); err != nil {
		return err
	}
	return a.updateStatus(ctx, updateOp)
}

// patch sends the changes, with an update mask naming their fields and
// nothing else.
func (a *subscriptionFieldsAdapter) patch(ctx context.Context, verb string, ch []change) error {
	if len(ch) == 0 {
		return nil
	}
	body := &api.Subscription{}
	var mask []string
	for _, c := range ch {
		c.set(body)
		mask = append(mask, c.path)
	}
	klog.FromContext(ctx).V(2).Info(verb+" fields of a Pub/Sub subscription", "name", a.id, "updateMask", mask)
	s, err := a.gcpClient.Projects.Subscriptions.Patch(a.id.String(), &api.UpdateSubscriptionRequest{
		Subscription: body,
		UpdateMask:   strings.Join(mask, ","),
	}).Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("%s %s of Pub/Sub subscription %s: %w", verb, strings.Join(mask, ", "), a.id, err)
	}
	a.actual = s
	return nil
}

// Delete clears the fields this object owns (the subscription's state
// before it: no transforms, no Bigtable export, the exports' fields
// unset), and never deletes the subscription. A subscription that is gone
// is not reached: Find said not found.
func (a *subscriptionFieldsAdapter) Delete(ctx context.Context, deleteOp *directbase.DeleteOperation) (bool, error) {
	ch, err := a.changes(true)
	if err != nil {
		return false, err
	}
	if err := a.patch(ctx, "clearing", ch); err != nil {
		return false, err
	}
	return true, nil
}

func (a *subscriptionFieldsAdapter) Export(ctx context.Context) (*unstructured.Unstructured, error) {
	return nil, nil
}

func (a *subscriptionFieldsAdapter) updateStatus(ctx context.Context, op directbase.Operation) error {
	status := &krm.PubSubSubscriptionFieldsStatus{ExternalRef: direct.LazyPtr(a.id.String())}
	if a.actual != nil {
		status.State = direct.LazyPtr(a.actual.State)
		if a.desired.BigtableConfig != nil && a.actual.BigtableConfig != nil {
			status.BigtableConfigState = direct.LazyPtr(a.actual.BigtableConfig.State)
		}
	}
	return op.UpdateStatus(ctx, status, nil)
}
