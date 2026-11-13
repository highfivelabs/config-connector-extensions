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
// for LoggingBillingAccountSink (docs/design.md decision 15). Changed: the
// kind, identity and mapper names; uniqueWriterIdentity is always true (a
// billing account's sink always gets its own writer identity, and
// Terraform's google_logging_billing_account_sink sends true); status
// carries externalRef. The client, find, create, update with a mask of
// the top-level fields that differ, and delete are upstream's.

package logging

import (
	"context"
	"fmt"
	"strings"

	loggingv1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/apis/logging/v1beta1"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/config"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct/common"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct/directbase"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct/registry"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/mappers"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/structuredreporting"

	gcp "cloud.google.com/go/logging/apiv2"
	loggingpb "cloud.google.com/go/logging/apiv2/loggingpb"
	"google.golang.org/protobuf/types/known/fieldmaskpb"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"

	krm "github.com/highfivelabs/config-connector-extensions/apis/logging/v1alpha1"
)

func init() {
	registry.RegisterModel(krm.LoggingBillingAccountSinkGVK, NewBillingAccountSinkModel)
}

// normalizeSinkDestination puts a destination's external in the form its
// reference type validates, from the forms a log sink's destination is
// written in (Terraform's destination, fabric's, gcloud's): a service
// prefix (storage.googleapis.com/, logging.googleapis.com/,
// bigquery.googleapis.com/, pubsub.googleapis.com/) is dropped, and a
// Cloud Storage bucket's bare name, or gs://name, becomes
// projects/_/buckets/name (a bucket's name is global, so its project is
// not needed). A bucket named by a StorageBucket object (name) is resolved
// by the reference itself. The mapper adds the service prefix back.
func normalizeSinkDestination(d *loggingv1beta1.LogSinkDestination) {
	trim := func(ext, prefix string) string { return strings.TrimPrefix(ext, prefix) }
	if r := d.StorageBucketRef; r != nil && r.External != "" {
		ext := trim(trim(r.External, "storage.googleapis.com/"), "gs://")
		if !strings.Contains(ext, "/") {
			ext = "projects/_/buckets/" + ext
		}
		r.External = ext
	}
	if r := d.LoggingLogBucketRef; r != nil && r.External != "" {
		r.External = trim(r.External, "logging.googleapis.com/")
	}
	if r := d.BigQueryDatasetRef; r != nil && r.External != "" {
		r.External = trim(r.External, "bigquery.googleapis.com/")
	}
	if r := d.PubSubTopicRef; r != nil && r.External != "" {
		r.External = trim(r.External, "pubsub.googleapis.com/")
	}
}

func NewBillingAccountSinkModel(ctx context.Context, config *config.ControllerConfig) (directbase.Model, error) {
	return &billingAccountSinkModel{config: *config}, nil
}

var _ directbase.Model = &billingAccountSinkModel{}

type billingAccountSinkModel struct {
	config config.ControllerConfig
}

func (m *billingAccountSinkModel) client(ctx context.Context) (*gcp.ConfigClient, error) {
	opts, err := m.config.RESTClientOptions()
	if err != nil {
		return nil, err
	}
	gcpClient, err := gcp.NewConfigRESTClient(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("building Logging Config client: %w", err)
	}
	return gcpClient, err
}

func (m *billingAccountSinkModel) AdapterForObject(ctx context.Context, op *directbase.AdapterForObjectOperation) (directbase.Adapter, error) {
	u := op.GetUnstructured()
	reader := op.Reader
	obj := &krm.LoggingBillingAccountSink{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, &obj); err != nil {
		return nil, fmt.Errorf("error converting to %T: %w", obj, err)
	}

	normalizeSinkDestination(&obj.Spec.Destination)
	if err := common.NormalizeReferences(ctx, reader, obj, nil); err != nil {
		return nil, fmt.Errorf("normalizing references: %w", err)
	}

	id, err := obj.GetIdentity(ctx, reader)
	if err != nil {
		return nil, err
	}

	gcpClient, err := m.client(ctx)
	if err != nil {
		return nil, err
	}

	mapCtx := &direct.MapContext{}
	desiredPb := LoggingBillingAccountSinkSpec_ToProto(mapCtx, &obj.Spec)
	if mapCtx.Err() != nil {
		return nil, mapCtx.Err()
	}

	return &billingAccountSinkAdapter{
		id:        id.(*krm.LoggingBillingAccountSinkIdentity),
		gcpClient: gcpClient,
		desired:   desiredPb,
	}, nil
}

func (m *billingAccountSinkModel) AdapterForURL(ctx context.Context, url string) (directbase.Adapter, error) {
	// TODO: Support URLs
	return nil, nil
}

type billingAccountSinkAdapter struct {
	id        *krm.LoggingBillingAccountSinkIdentity
	gcpClient *gcp.ConfigClient
	desired   *loggingpb.LogSink
	actual    *loggingpb.LogSink
}

var _ directbase.Adapter = &billingAccountSinkAdapter{}

func (a *billingAccountSinkAdapter) Find(ctx context.Context) (bool, error) {
	log := klog.FromContext(ctx)
	log.V(2).Info("getting LoggingBillingAccountSink", "name", a.id)

	req := &loggingpb.GetSinkRequest{SinkName: a.id.String()}
	actual, err := a.gcpClient.GetSink(ctx, req)
	if err != nil {
		if direct.IsNotFound(err) {
			return false, nil
		}
		return false, fmt.Errorf("getting LoggingBillingAccountSink %q: %w", a.id, err)
	}

	a.actual = actual
	return true, nil
}

func (a *billingAccountSinkAdapter) Create(ctx context.Context, createOp *directbase.CreateOperation) error {
	log := klog.FromContext(ctx)
	log.V(2).Info("creating LoggingBillingAccountSink", "id", a.id.String())

	a.desired.Name = a.id.ID()

	req := &loggingpb.CreateSinkRequest{
		Parent:               a.id.ParentString(),
		Sink:                 a.desired,
		UniqueWriterIdentity: true,
	}
	created, err := a.gcpClient.CreateSink(ctx, req)
	if err != nil {
		return fmt.Errorf("creating LoggingBillingAccountSink %s: %w", a.id, err)
	}
	log.V(2).Info("successfully created LoggingBillingAccountSink", "name", a.id)

	return a.updateStatus(ctx, createOp, created)
}

func (a *billingAccountSinkAdapter) Update(ctx context.Context, updateOp *directbase.UpdateOperation) error {
	log := klog.FromContext(ctx)
	log.V(2).Info("updating LoggingBillingAccountSink", "name", a.id.String())

	diffs, updateMask, err := compareBillingAccountSink(ctx, a.actual, a.desired)
	if err != nil {
		return err
	}

	latest := a.actual
	if diffs.HasDiff() {
		diffs.Object = updateOp.GetUnstructured()
		structuredreporting.ReportDiff(ctx, diffs)

		a.desired.Name = a.id.ID()

		req := &loggingpb.UpdateSinkRequest{
			SinkName:             a.id.String(),
			Sink:                 a.desired,
			UpdateMask:           updateMask,
			UniqueWriterIdentity: true,
		}

		updated, err := a.gcpClient.UpdateSink(ctx, req)
		if err != nil {
			return fmt.Errorf("updating LoggingBillingAccountSink %s: %w", a.id.String(), err)
		}
		latest = updated
	}

	return a.updateStatus(ctx, updateOp, latest)
}

func (a *billingAccountSinkAdapter) updateStatus(ctx context.Context, op directbase.Operation, latest *loggingpb.LogSink) error {
	mapCtx := &direct.MapContext{}
	status := LoggingBillingAccountSinkStatus_FromProto(mapCtx, latest)
	if mapCtx.Err() != nil {
		return mapCtx.Err()
	}
	status.ExternalRef = direct.LazyPtr(a.id.String())
	return op.UpdateStatus(ctx, status, nil)
}

func (a *billingAccountSinkAdapter) Export(ctx context.Context) (*unstructured.Unstructured, error) {
	if a.actual == nil {
		return nil, fmt.Errorf("Find() not called")
	}
	u := &unstructured.Unstructured{}

	obj := &krm.LoggingBillingAccountSink{}
	mapCtx := &direct.MapContext{}
	obj.Spec = direct.ValueOf(LoggingBillingAccountSinkSpec_FromProto(mapCtx, a.actual))
	if mapCtx.Err() != nil {
		return nil, mapCtx.Err()
	}
	obj.Spec.BillingAccountRef.External = a.id.ParentString()

	uObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
	if err != nil {
		return nil, err
	}

	u.Object = uObj
	u.SetName(a.actual.Name)
	u.SetGroupVersionKind(krm.LoggingBillingAccountSinkGVK)
	return u, nil
}

func (a *billingAccountSinkAdapter) Delete(ctx context.Context, deleteOp *directbase.DeleteOperation) (bool, error) {
	log := klog.FromContext(ctx)
	log.V(2).Info("deleting LoggingBillingAccountSink", "name", a.id)

	req := &loggingpb.DeleteSinkRequest{SinkName: a.id.String()}
	err := a.gcpClient.DeleteSink(ctx, req)
	if err != nil {
		if direct.IsNotFound(err) {
			log.V(2).Info("skipping delete for non-existent LoggingBillingAccountSink, assuming it was already deleted", "name", a.id.String())
			return true, nil
		}
		return false, fmt.Errorf("deleting LoggingBillingAccountSink %s: %w", a.id, err)
	}
	log.V(2).Info("successfully deleted LoggingBillingAccountSink", "name", a.id)
	return true, nil
}

func compareBillingAccountSink(ctx context.Context, actual, desired *loggingpb.LogSink) (*structuredreporting.Diff, *fieldmaskpb.FieldMask, error) {
	maskedActual, err := mappers.OnlySpecFields(actual, LoggingBillingAccountSinkSpec_FromProto, LoggingBillingAccountSinkSpec_ToProto)
	if err != nil {
		return nil, nil, err
	}
	maskedActual.Name = desired.Name
	diffs, updateMask, err := common.DiffForTopLevelFields(ctx, desired.ProtoReflect(), maskedActual.ProtoReflect())
	if err != nil {
		return nil, nil, err
	}
	return diffs, updateMask, nil
}
