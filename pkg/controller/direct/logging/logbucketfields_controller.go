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

package logging

// Written here: LoggingLogBucketFields, a field kind (docs/design.md
// section 6.6, pattern 2; decision 37). No Config Connector code or pull
// request sets a log bucket's index configs, restricted fields or CMEK
// key. The bucket is read with buckets.get and written with buckets.patch
// and an update mask naming only the owned fields that differ, so no other
// field of the bucket is sent. Config Connector's LoggingLogBucket masks
// only the fields it changed too: the DCL's updateMask comes from its own
// field diffs (description, retentionDays, locked, analyticsEnabled), and
// the direct controller's from a round trip through its spec, which has
// none of these.

import (
	"context"
	"fmt"
	"slices"

	gcp "cloud.google.com/go/logging/apiv2"
	pb "cloud.google.com/go/logging/apiv2/loggingpb"
	loggingv1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/apis/logging/v1beta1"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/config"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct/directbase"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"

	krm "github.com/highfivelabs/config-connector-extensions/apis/logging/v1alpha1"
	"github.com/highfivelabs/config-connector-extensions/pkg/fieldkind"
)

// NewLogBucketFieldsModel is the LoggingLogBucketFields model.
func NewLogBucketFieldsModel(ctx context.Context, config *config.ControllerConfig) (directbase.Model, error) {
	return &logBucketFieldsModel{config: *config}, nil
}

type logBucketFieldsModel struct {
	config config.ControllerConfig
}

var _ directbase.Model = &logBucketFieldsModel{}

// The update mask paths this kind ever writes.
const (
	indexConfigsPath     = "index_configs"
	restrictedFieldsPath = "restricted_fields"
	cmekSettingsPath     = "cmek_settings"
)

func (m *logBucketFieldsModel) AdapterForObject(ctx context.Context, op *directbase.AdapterForObjectOperation) (directbase.Adapter, error) {
	obj := &krm.LoggingLogBucketFields{}
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
		// Deleting: the bucket this object set is in its status; Config
		// Connector's LoggingLogBucket may be gone already.
		ref = &loggingv1beta1.LoggingLogBucketRef{External: prev}
	} else if err := ref.Normalize(ctx, op.Reader, obj.GetNamespace()); err != nil {
		return nil, err
	}
	id := &loggingv1beta1.LogBucketIdentity{}
	if err := id.FromExternal(ref.External); err != nil {
		return nil, err
	}
	if prev != "" && prev != id.String() {
		return nil, fmt.Errorf("cannot change the bucket of LoggingLogBucketFields (old=%q, new=%q)", prev, id.String())
	}
	desired, err := desiredBucketFields(&obj.Spec)
	if err != nil {
		return nil, err
	}
	if err := fieldkind.CheckSoleOwner(ctx, op.Reader, krm.LoggingLogBucketFieldsGVK, obj, id.String()); err != nil {
		return nil, err
	}
	opts, err := m.config.RESTClientOptions()
	if err != nil {
		return nil, err
	}
	c, err := gcp.NewConfigRESTClient(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("building logging client: %w", err)
	}
	return &logBucketFieldsAdapter{id: id, gcpClient: c, spec: &obj.Spec, desired: desired}, nil
}

// desiredBucketFields is the spec's fields as the API takes them.
func desiredBucketFields(spec *krm.LoggingLogBucketFieldsSpec) (*pb.LogBucket, error) {
	out := &pb.LogBucket{RestrictedFields: spec.RestrictedFields}
	for i, ic := range spec.IndexConfigs {
		v, ok := pb.IndexType_value[ic.Type]
		if !ok || v == 0 {
			return nil, fmt.Errorf("spec.indexConfigs[%d].type: unknown index type %q", i, ic.Type)
		}
		out.IndexConfigs = append(out.IndexConfigs, &pb.IndexConfig{FieldPath: ic.FieldPath, Type: pb.IndexType(v)})
	}
	if spec.CmekSettings != nil {
		out.CmekSettings = &pb.CmekSettings{KmsKeyName: spec.CmekSettings.KmsKeyName}
	}
	return out, nil
}

func (m *logBucketFieldsModel) AdapterForURL(ctx context.Context, url string) (directbase.Adapter, error) {
	return nil, nil
}

type logBucketFieldsAdapter struct {
	id        *loggingv1beta1.LogBucketIdentity
	gcpClient *gcp.ConfigClient
	spec      *krm.LoggingLogBucketFieldsSpec
	desired   *pb.LogBucket
	actual    *pb.LogBucket
}

var _ directbase.Adapter = &logBucketFieldsAdapter{}

// Find reports whether the bucket exists. A missing bucket (or one the API
// has deleted, kept for 7 days) is "not found" so that a delete finishes
// when it is gone; a create then waits (Create).
func (a *logBucketFieldsAdapter) Find(ctx context.Context) (bool, error) {
	b, err := a.gcpClient.GetBucket(ctx, &pb.GetBucketRequest{Name: a.id.String()})
	if err != nil {
		if direct.IsNotFound(err) {
			return false, nil
		}
		return false, fmt.Errorf("getting log bucket %s: %w", a.id, err)
	}
	if b.GetLifecycleState() == pb.LifecycleState_DELETE_REQUESTED {
		return false, nil
	}
	a.actual = b
	return true, nil
}

// Create is called only while the bucket does not exist: this kind never
// makes it, so it waits for it.
func (a *logBucketFieldsAdapter) Create(ctx context.Context, createOp *directbase.CreateOperation) error {
	return fieldkind.NotFoundError("LoggingLogBucket", a.id.String())
}

type indexKey struct {
	path string
	typ  pb.IndexType
}

// sameIndexes compares two lists of index configs as sets: the API may
// return them in its own order, each with its creation time.
func sameIndexes(a, b []*pb.IndexConfig) bool {
	keys := func(in []*pb.IndexConfig) []indexKey {
		out := make([]indexKey, 0, len(in))
		for _, ic := range in {
			out = append(out, indexKey{ic.GetFieldPath(), ic.GetType()})
		}
		slices.SortFunc(out, func(x, y indexKey) int {
			if x.path != y.path {
				if x.path < y.path {
					return -1
				}
				return 1
			}
			return int(x.typ) - int(y.typ)
		})
		return slices.Compact(out)
	}
	return slices.Equal(keys(a), keys(b))
}

func sameStrings(a, b []string) bool {
	a, b = slices.Clone(a), slices.Clone(b)
	slices.Sort(a)
	slices.Sort(b)
	return slices.Equal(slices.Compact(a), slices.Compact(b))
}

// changedPaths is the mask of the owned fields that differ from the
// bucket's.
func (a *logBucketFieldsAdapter) changedPaths() []string {
	var paths []string
	if a.spec.IndexConfigs != nil && !sameIndexes(a.actual.GetIndexConfigs(), a.desired.GetIndexConfigs()) {
		paths = append(paths, indexConfigsPath)
	}
	if a.spec.RestrictedFields != nil && !sameStrings(a.actual.GetRestrictedFields(), a.desired.GetRestrictedFields()) {
		paths = append(paths, restrictedFieldsPath)
	}
	if a.spec.CmekSettings != nil && a.actual.GetCmekSettings().GetKmsKeyName() != a.desired.GetCmekSettings().GetKmsKeyName() {
		paths = append(paths, cmekSettingsPath)
	}
	return paths
}

func (a *logBucketFieldsAdapter) Update(ctx context.Context, updateOp *directbase.UpdateOperation) error {
	if paths := a.changedPaths(); len(paths) > 0 {
		klog.FromContext(ctx).V(2).Info("setting fields of a log bucket", "name", a.id, "paths", paths)
		if err := a.patch(ctx, a.desired, paths); err != nil {
			return err
		}
	}
	return a.updateStatus(ctx, updateOp)
}

func (a *logBucketFieldsAdapter) patch(ctx context.Context, body *pb.LogBucket, paths []string) error {
	updated, err := a.gcpClient.UpdateBucket(ctx, &pb.UpdateBucketRequest{
		Name:       a.id.String(),
		Bucket:     body,
		UpdateMask: &fieldmaskpb.FieldMask{Paths: paths},
	})
	if err != nil {
		return fmt.Errorf("setting %v of log bucket %s: %w", paths, a.id, err)
	}
	a.actual = updated
	return nil
}

// Delete clears the index configs and restricted fields this object owns
// (the bucket then has none, as before it), and leaves the CMEK key: the
// API cannot remove a bucket's CMEK. The bucket is never deleted. A bucket
// that is gone is not reached: Find said not found.
func (a *logBucketFieldsAdapter) Delete(ctx context.Context, deleteOp *directbase.DeleteOperation) (bool, error) {
	var paths []string
	if a.spec.IndexConfigs != nil && len(a.actual.GetIndexConfigs()) > 0 {
		paths = append(paths, indexConfigsPath)
	}
	if a.spec.RestrictedFields != nil && len(a.actual.GetRestrictedFields()) > 0 {
		paths = append(paths, restrictedFieldsPath)
	}
	if len(paths) == 0 {
		return true, nil
	}
	klog.FromContext(ctx).V(2).Info("clearing fields of a log bucket", "name", a.id, "paths", paths)
	if err := a.patch(ctx, &pb.LogBucket{}, paths); err != nil {
		return false, err
	}
	return true, nil
}

func (a *logBucketFieldsAdapter) Export(ctx context.Context) (*unstructured.Unstructured, error) {
	return nil, nil
}

func (a *logBucketFieldsAdapter) updateStatus(ctx context.Context, op directbase.Operation) error {
	status := &krm.LoggingLogBucketFieldsStatus{ExternalRef: direct.LazyPtr(a.id.String())}
	if sa := a.actual.GetCmekSettings().GetServiceAccountId(); sa != "" {
		status.CmekServiceAccountID = direct.LazyPtr(sa)
	}
	return op.UpdateStatus(ctx, status, nil)
}
