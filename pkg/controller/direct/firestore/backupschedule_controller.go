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
// pkg/controller/direct/firestore/firestorebackupschedule_controller.go, with the admin
// client (client.go) it uses (UPSTREAM.md). Changed: our KRM types (stable,
// firestore.gcp.hfl.dev/v1beta1, docs/design.md decision 26) and mapper names;
// the model is returned to our manager, not registered with Config
// Connector's registry; Export is not supported (no kind here exports).

package firestore

import (
	"context"
	"fmt"
	"strings"

	apiv1 "cloud.google.com/go/firestore/apiv1/admin"
	pb "cloud.google.com/go/firestore/apiv1/admin/adminpb"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/config"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct/common"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct/directbase"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/structuredreporting"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"

	krm "github.com/highfivelabs/config-connector-extensions/apis/firestore/v1beta1"
)

func NewFirestoreBackupScheduleModel(ctx context.Context, config *config.ControllerConfig) (directbase.Model, error) {
	return &firestoreBackupScheduleModel{config: config}, nil
}

type firestoreBackupScheduleModel struct {
	config *config.ControllerConfig
}

var _ directbase.Model = &firestoreBackupScheduleModel{}

func newFirestoreAdminClient(ctx context.Context, config *config.ControllerConfig) (*apiv1.FirestoreAdminClient, error) {
	opts, err := config.RESTClientOptions()
	if err != nil {
		return nil, err
	}
	client, err := apiv1.NewFirestoreAdminRESTClient(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("building firestore admin client: %w", err)
	}
	return client, err
}

func (m *firestoreBackupScheduleModel) AdapterForObject(ctx context.Context, op *directbase.AdapterForObjectOperation) (directbase.Adapter, error) {
	u := op.GetUnstructured()
	reader := op.Reader
	firestoreAdminClient, err := newFirestoreAdminClient(ctx, m.config)
	if err != nil {
		return nil, err
	}

	obj := &krm.FirestoreBackupSchedule{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, &obj); err != nil {
		return nil, fmt.Errorf("error converting to %T: %w", obj, err)
	}

	id, err := obj.GetIdentity(ctx, reader)
	if err != nil {
		return nil, err
	}

	mapCtx := &direct.MapContext{}
	desired := FirestoreBackupScheduleSpec_ToProto(mapCtx, &obj.Spec)
	if mapCtx.Err() != nil {
		return nil, mapCtx.Err()
	}

	return &firestoreBackupScheduleAdapter{
		id:                   id.(*krm.FirestoreBackupScheduleIdentity),
		firestoreAdminClient: firestoreAdminClient,
		desired:              desired,
	}, nil
}

// AdapterForURL is not supported: no kind here is exported.
func (m *firestoreBackupScheduleModel) AdapterForURL(ctx context.Context, url string) (directbase.Adapter, error) {
	return nil, nil
}

type firestoreBackupScheduleAdapter struct {
	id                   *krm.FirestoreBackupScheduleIdentity
	firestoreAdminClient *apiv1.FirestoreAdminClient
	desired              *pb.BackupSchedule
	actual               *pb.BackupSchedule
}

var _ directbase.Adapter = &firestoreBackupScheduleAdapter{}

func (a *firestoreBackupScheduleAdapter) Find(ctx context.Context) (bool, error) {
	log := klog.FromContext(ctx)

	if a.id == nil || a.id.BackupSchedule == "" {
		return false, nil
	}

	fqn := a.id.String()
	log.V(2).Info("getting FirestoreBackupSchedule", "name", fqn)

	req := &pb.GetBackupScheduleRequest{Name: fqn}
	obj, err := a.firestoreAdminClient.GetBackupSchedule(ctx, req)
	if err != nil {
		if direct.IsNotFound(err) {
			return false, nil
		}
		return false, fmt.Errorf("getting FirestoreBackupSchedule %q: %w", fqn, err)
	}

	a.actual = obj
	return true, nil
}

func (a *firestoreBackupScheduleAdapter) Create(ctx context.Context, createOp *directbase.CreateOperation) error {
	log := klog.FromContext(ctx)
	parent := fmt.Sprintf("projects/%s/databases/%s", a.id.Project, a.id.Database)
	log.V(2).Info("creating FirestoreBackupSchedule", "parent", parent)

	resource := proto.CloneOf(a.desired)

	req := &pb.CreateBackupScheduleRequest{
		Parent:         parent,
		BackupSchedule: resource,
	}
	created, err := a.firestoreAdminClient.CreateBackupSchedule(ctx, req)
	if err != nil {
		return fmt.Errorf("creating FirestoreBackupSchedule in %s: %w", parent, err)
	}

	// Update the ID since it was generated
	if err := a.id.FromExternal("//firestore.googleapis.com/" + created.Name); err != nil {
		return fmt.Errorf("failed to parse generated backup schedule name %q: %w", created.Name, err)
	}

	log.V(2).Info("successfully created FirestoreBackupSchedule", "name", created.Name)

	return a.updateStatus(ctx, createOp, created)
}

func (a *firestoreBackupScheduleAdapter) updateStatus(ctx context.Context, op directbase.Operation, latest *pb.BackupSchedule) error {
	mapCtx := &direct.MapContext{}
	status := &krm.FirestoreBackupScheduleStatus{}
	status.ObservedState = FirestoreBackupScheduleObservedState_FromProto(mapCtx, latest)
	if mapCtx.Err() != nil {
		return mapCtx.Err()
	}
	status.ExternalRef = direct.PtrTo(a.id.String())
	return op.UpdateStatus(ctx, status, nil)
}

func (a *firestoreBackupScheduleAdapter) Update(ctx context.Context, updateOp *directbase.UpdateOperation) error {
	log := klog.FromContext(ctx)
	fqn := a.id.String()
	log.V(2).Info("updating FirestoreBackupSchedule", "name", fqn)

	resource := proto.CloneOf(a.desired)
	resource.Name = fqn

	paths, report, err := common.CompareProtoMessageStructuredDiff(resource, a.actual, common.BasicDiff)
	if err != nil {
		return err
	}

	var updatePaths []string
	hasRetention := false
	hasRecurrence := false
	for path := range paths {
		if strings.HasPrefix(path, "retention") {
			hasRetention = true
		}
		if strings.HasPrefix(path, "daily_recurrence") || strings.HasPrefix(path, "weekly_recurrence") {
			hasRecurrence = true
		}
	}
	if hasRetention {
		updatePaths = append(updatePaths, "retention")
	}
	if hasRecurrence {
		updatePaths = append(updatePaths, "recurrence")
	}

	if len(updatePaths) == 0 {
		return a.updateStatus(ctx, updateOp, a.actual)
	}

	report.Object = updateOp.GetUnstructured()
	structuredreporting.ReportDiff(ctx, report)

	req := &pb.UpdateBackupScheduleRequest{
		BackupSchedule: resource,
		UpdateMask: &fieldmaskpb.FieldMask{
			Paths: updatePaths,
		},
	}

	updated, err := a.firestoreAdminClient.UpdateBackupSchedule(ctx, req)
	if err != nil {
		return fmt.Errorf("updating FirestoreBackupSchedule %q: %w", fqn, err)
	}

	log.V(2).Info("successfully updated FirestoreBackupSchedule", "name", updated.Name)

	return a.updateStatus(ctx, updateOp, updated)
}

func (a *firestoreBackupScheduleAdapter) Export(ctx context.Context) (*unstructured.Unstructured, error) {
	return nil, fmt.Errorf("export of FirestoreBackupSchedule is not supported")
}

func (a *firestoreBackupScheduleAdapter) Delete(ctx context.Context, deleteOp *directbase.DeleteOperation) (bool, error) {
	if a.id == nil || a.id.BackupSchedule == "" {
		return true, nil
	}

	fqn := a.id.String()
	log := klog.FromContext(ctx)
	log.V(2).Info("deleting FirestoreBackupSchedule", "name", fqn)

	req := &pb.DeleteBackupScheduleRequest{
		Name: fqn,
	}
	if err := a.firestoreAdminClient.DeleteBackupSchedule(ctx, req); err != nil {
		if direct.IsNotFound(err) {
			log.V(2).Info("skipping delete for non-existent FirestoreBackupSchedule, assuming it was already deleted", "name", fqn)
			return true, nil
		}
		return false, fmt.Errorf("deleting FirestoreBackupSchedule %s: %w", fqn, err)
	}
	log.V(2).Info("successfully deleted FirestoreBackupSchedule", "name", fqn)

	return true, nil
}
