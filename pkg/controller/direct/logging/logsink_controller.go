// Copyright 2026 Google LLC
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

// SPDX-License-Identifier: Apache-2.0

// Modified by High Five Labs for config-connector-extensions: copied from Config Connector
// pkg/controller/direct/logging/logginglogsink_controller.go (UPSTREAM.md)
// for our LoggingLogSink (docs/design.md decisions 19 and 26). Changed: the
// Logging REST client (google.golang.org/api/logging/v2) in place of the
// protobuf one, whose LogSink at the pin has no interceptChildren, so
// Update builds its mask from the fields that differ by hand (the same
// top-level fields upstream's DiffForTopLevelFields compares; BigQuery
// options only when the spec sets them, since the API fills them in for
// a BigQuery destination); destination.projectRef is resolved to the
// project's ID; a folder's or organization's sink is always sent
// uniqueWriterIdentity=true (the API refuses false there, as Terraform's
// google_logging_folder_sink notes), and so is a sink that already has a
// unique identity (the API refuses going back to the shared one); status
// carries externalRef; a delete uses status.externalRef and does not
// resolve the parent again (the rule of our generated kinds, so that a
// folder deleted with its sinks does not hold them). Find, create and
// delete are upstream's.

package logging

import (
	"context"
	"fmt"
	"reflect"
	"sort"
	"strings"

	loggingv1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/apis/logging/v1beta1"
	refsv1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/apis/refs/v1beta1"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/config"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct/common"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct/directbase"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct/registry"
	api "google.golang.org/api/logging/v2"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"

	krm "github.com/highfivelabs/config-connector-extensions/apis/logging/v1beta1"
)

// sharedWriterIdentity is the writer identity of a project sink made
// without uniqueWriterIdentity, in the days the API had one.
const sharedWriterIdentity = "serviceAccount:cloud-logs@system.gserviceaccount.com"

func init() {
	registry.RegisterModel(krm.LoggingLogSinkGVK, NewLogSinkModel)
}

func NewLogSinkModel(ctx context.Context, config *config.ControllerConfig) (directbase.Model, error) {
	return &logSinkModel{config: *config}, nil
}

var _ directbase.Model = &logSinkModel{}

type logSinkModel struct {
	config config.ControllerConfig
}

func (m *logSinkModel) client(ctx context.Context) (*api.Service, error) {
	opts, err := m.config.RESTClientOptions()
	if err != nil {
		return nil, err
	}
	svc, err := api.NewService(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("building Logging client: %w", err)
	}
	return svc, nil
}

func (m *logSinkModel) AdapterForObject(ctx context.Context, op *directbase.AdapterForObjectOperation) (directbase.Adapter, error) {
	u := op.GetUnstructured()
	reader := op.Reader
	obj := &krm.LoggingLogSink{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, &obj); err != nil {
		return nil, fmt.Errorf("error converting to %T: %w", obj, err)
	}

	gcpClient, err := m.client(ctx)
	if err != nil {
		return nil, err
	}

	ext := direct.ValueOf(obj.Status.ExternalRef)
	if deleting := !obj.GetDeletionTimestamp().IsZero(); deleting {
		if ext != "" {
			// Deleting a sink that was made or acquired: its name is in
			// status, and the parts of a name cannot change, so neither the
			// parent nor the destination is resolved. A folder object, or a
			// destination bucket, deleted at the same time would otherwise
			// hold the delete forever.
			id := &krm.LoggingLogSinkIdentity{}
			if err := id.FromExternal(ext); err != nil {
				return nil, err
			}
			return &logSinkAdapter{id: id, gcpClient: gcpClient}, nil
		}
		if _, err := obj.GetIdentity(ctx, reader); err != nil {
			// Never made or acquired (no status.externalRef) and its parent
			// cannot be resolved: there is nothing of ours to delete.
			klog.FromContext(ctx).Info("deleting LoggingLogSink without a call: no status.externalRef and the parent cannot be resolved", "error", err.Error())
			return &logSinkAdapter{}, nil
		}
	}

	normalizeLogSinkDestination(&obj.Spec.Destination)
	if err := common.NormalizeReferences(ctx, reader, obj, nil); err != nil {
		return nil, fmt.Errorf("normalizing references: %w", err)
	}

	id, err := obj.GetIdentity(ctx, reader)
	if err != nil {
		return nil, err
	}

	if r := obj.Spec.Destination.ProjectRef; r != nil {
		project, err := refsv1beta1.ResolveProject(ctx, reader, obj.GetNamespace(), &refsv1beta1.ProjectRef{External: r.External, Name: r.Name, Namespace: r.Namespace})
		if err != nil {
			return nil, fmt.Errorf("resolving spec.destination.projectRef: %w", err)
		}
		r.External, r.Name, r.Namespace = "projects/"+project.ProjectID, "", ""
	}

	mapCtx := &direct.MapContext{}
	desired := LoggingLogSinkSpec_ToAPI(mapCtx, &obj.Spec)
	if mapCtx.Err() != nil {
		return nil, mapCtx.Err()
	}

	return &logSinkAdapter{
		id:                   id.(*krm.LoggingLogSinkIdentity),
		gcpClient:            gcpClient,
		desired:              desired,
		setsBigqueryOptions:  obj.Spec.BigqueryOptions != nil,
		uniqueWriterIdentity: direct.ValueOf(obj.Spec.UniqueWriterIdentity),
	}, nil
}

// normalizeLogSinkDestination puts the destination's references in the
// forms their types validate, as normalizeSinkDestination does for
// LoggingBillingAccountSink (the same four references).
func normalizeLogSinkDestination(d *krm.LoggingLogSinkDestination) {
	normalizeSinkDestination(&loggingv1beta1.LogSinkDestination{
		BigQueryDatasetRef:  d.BigQueryDatasetRef,
		LoggingLogBucketRef: d.LoggingLogBucketRef,
		PubSubTopicRef:      d.PubSubTopicRef,
		StorageBucketRef:    d.StorageBucketRef,
	})
}

func (m *logSinkModel) AdapterForURL(ctx context.Context, url string) (directbase.Adapter, error) {
	// TODO: Support URLs
	return nil, nil
}

type logSinkAdapter struct {
	// id is nil for a sink being deleted that was never made (no
	// status.externalRef) and whose parent is gone.
	id                   *krm.LoggingLogSinkIdentity
	gcpClient            *api.Service
	desired              *api.LogSink
	actual               *api.LogSink
	setsBigqueryOptions  bool
	uniqueWriterIdentity bool
}

var _ directbase.Adapter = &logSinkAdapter{}

// unique is the uniqueWriterIdentity to send: the spec's, or true for a
// folder's or organization's sink, or for a sink that has a unique
// identity already.
func (a *logSinkAdapter) unique() bool {
	if a.uniqueWriterIdentity || a.id.HierarchyParent() {
		return true
	}
	return a.actual != nil && a.actual.WriterIdentity != "" && a.actual.WriterIdentity != sharedWriterIdentity
}

func (a *logSinkAdapter) Find(ctx context.Context) (bool, error) {
	if a.id == nil {
		return false, nil
	}
	log := klog.FromContext(ctx)
	log.V(2).Info("getting LoggingLogSink", "name", a.id)

	actual, err := a.gcpClient.Sinks.Get(a.id.String()).Context(ctx).Do()
	if err != nil {
		if direct.IsNotFound(err) {
			return false, nil
		}
		return false, fmt.Errorf("getting LoggingLogSink %q: %w", a.id, err)
	}

	a.actual = actual
	return true, nil
}

func (a *logSinkAdapter) Create(ctx context.Context, createOp *directbase.CreateOperation) error {
	log := klog.FromContext(ctx)
	log.V(2).Info("creating LoggingLogSink", "id", a.id.String())

	a.desired.Name = a.id.ID()

	created, err := a.gcpClient.Sinks.Create(a.id.ParentString(), a.desired).
		UniqueWriterIdentity(a.unique()).Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("creating LoggingLogSink %s: %w", a.id, err)
	}
	log.V(2).Info("successfully created LoggingLogSink", "name", a.id)

	return a.updateStatus(ctx, createOp, created)
}

func (a *logSinkAdapter) Update(ctx context.Context, updateOp *directbase.UpdateOperation) error {
	log := klog.FromContext(ctx)
	log.V(2).Info("updating LoggingLogSink", "name", a.id.String())

	paths := compareLogSink(a.actual, a.desired, a.setsBigqueryOptions)

	latest := a.actual
	if len(paths) > 0 {
		log.V(2).Info("updating LoggingLogSink", "name", a.id, "paths", paths)
		a.desired.Name = a.id.ID()
		// A field in the mask is sent even when it is false or empty.
		a.desired.ForceSendFields = []string{"Description", "Destination", "Disabled", "Filter", "IncludeChildren", "InterceptChildren", "Exclusions"}

		updated, err := a.gcpClient.Sinks.Update(a.id.String(), a.desired).
			UpdateMask(strings.Join(paths, ",")).
			UniqueWriterIdentity(a.unique()).Context(ctx).Do()
		if err != nil {
			return fmt.Errorf("updating LoggingLogSink %s: %w", a.id.String(), err)
		}
		latest = updated
	}

	return a.updateStatus(ctx, updateOp, latest)
}

func (a *logSinkAdapter) updateStatus(ctx context.Context, op directbase.Operation, latest *api.LogSink) error {
	mapCtx := &direct.MapContext{}
	status := LoggingLogSinkStatus_FromAPI(mapCtx, latest)
	if mapCtx.Err() != nil {
		return mapCtx.Err()
	}
	status.ExternalRef = direct.LazyPtr(a.id.String())
	return op.UpdateStatus(ctx, status, nil)
}

func (a *logSinkAdapter) Export(ctx context.Context) (*unstructured.Unstructured, error) {
	if a.actual == nil {
		return nil, fmt.Errorf("Find() not called")
	}
	u := &unstructured.Unstructured{}

	obj := &krm.LoggingLogSink{}
	mapCtx := &direct.MapContext{}
	obj.Spec = direct.ValueOf(LoggingLogSinkSpec_FromAPI(mapCtx, a.actual))
	if mapCtx.Err() != nil {
		return nil, mapCtx.Err()
	}

	uObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
	if err != nil {
		return nil, err
	}

	u.Object = uObj
	u.SetName(a.actual.Name)
	u.SetGroupVersionKind(krm.LoggingLogSinkGVK)
	return u, nil
}

func (a *logSinkAdapter) Delete(ctx context.Context, deleteOp *directbase.DeleteOperation) (bool, error) {
	if a.id == nil {
		return true, nil
	}
	log := klog.FromContext(ctx)
	log.V(2).Info("deleting LoggingLogSink", "name", a.id)

	_, err := a.gcpClient.Sinks.Delete(a.id.String()).Context(ctx).Do()
	if err != nil {
		if direct.IsNotFound(err) {
			log.V(2).Info("skipping delete for non-existent LoggingLogSink, assuming it was already deleted", "name", a.id.String())
			return true, nil
		}
		return false, fmt.Errorf("deleting LoggingLogSink %s: %w", a.id, err)
	}
	log.V(2).Info("successfully deleted LoggingLogSink", "name", a.id)
	return true, nil
}

// compareLogSink is the update mask: the spec's top-level fields whose
// value differs from the cloud's, read through the spec (so output-only
// fields such as an exclusion's createTime are not compared).
func compareLogSink(actual, desired *api.LogSink, setsBigqueryOptions bool) []string {
	mapCtx := &direct.MapContext{}
	got := LoggingLogSinkSpec_ToAPI(mapCtx, LoggingLogSinkSpec_FromAPI(mapCtx, actual))
	var paths []string
	diff := func(path string, want, have any) {
		if !reflect.DeepEqual(want, have) {
			paths = append(paths, path)
		}
	}
	diff("description", desired.Description, got.Description)
	diff("destination", desired.Destination, got.Destination)
	diff("disabled", desired.Disabled, got.Disabled)
	diff("filter", desired.Filter, got.Filter)
	diff("includeChildren", desired.IncludeChildren, got.IncludeChildren)
	diff("interceptChildren", desired.InterceptChildren, got.InterceptChildren)
	if len(desired.Exclusions) != 0 || len(got.Exclusions) != 0 {
		diff("exclusions", desired.Exclusions, got.Exclusions)
	}
	if setsBigqueryOptions {
		diff("bigqueryOptions", desired.BigqueryOptions, got.BigqueryOptions)
	}
	sort.Strings(paths)
	return paths
}
