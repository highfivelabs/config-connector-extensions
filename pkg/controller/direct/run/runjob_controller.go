// Copyright 2025 Google LLC
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

// Modified by High Five Labs for config-connector-extensions. Copied from Config Connector
// pkg/controller/direct/run/runjob_controller.go (UPSTREAM.md), the direct
// RunJob controller, for our RunJob (run.gcp.hfl.dev/v1alpha1), which owns the
// whole job. The job's labels are metadata.labels, as upstream's. Changed:
// upstream decides whether to update by a cookie of the spec and the whole
// job as read (status.lastModifiedCookie), so any output field that moves (an
// execution, reconciling turning false) causes an update; ours compares the
// spec's fields with the job as read, as our RunService does, taking the
// server's defaults from what was read where the spec leaves them unset, and
// sends no update when they agree. The API's UpdateJob takes no update mask
// and replaces the job, so the update sends the whole desired job (with those
// defaults), as upstream's does. Written here: a delete from
// status.externalRef without resolving references, a read after create and
// update for the status, and CloudLabels for the slot guard (pkg/slotguard).

package run

import (
	"context"
	"fmt"
	"strings"

	gcp "cloud.google.com/go/run/apiv2"
	pb "cloud.google.com/go/run/apiv2/runpb"
	"google.golang.org/api/option"
	"google.golang.org/protobuf/proto"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"

	refs "github.com/GoogleCloudPlatform/k8s-config-connector/apis/refs/v1beta1"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/config"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct/common"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct/directbase"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct/registry"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/label"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/mappers"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/structuredreporting"

	krm "github.com/highfivelabs/config-connector-extensions/apis/run/v1alpha1"
)

func init() {
	registry.RegisterModel(krm.RunJobGVK, NewRunJobModel)
}

func NewRunJobModel(ctx context.Context, config *config.ControllerConfig) (directbase.Model, error) {
	return &modelJob{config: *config}, nil
}

var _ directbase.Model = &modelJob{}

type modelJob struct {
	config config.ControllerConfig
}

func (m *modelJob) client(ctx context.Context) (*gcp.JobsClient, error) {
	var opts []option.ClientOption
	opts, err := m.config.RESTClientOptions()
	if err != nil {
		return nil, err
	}
	gcpClient, err := gcp.NewJobsRESTClient(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("building Job client: %w", err)
	}
	return gcpClient, err
}

func (m *modelJob) AdapterForObject(ctx context.Context, op *directbase.AdapterForObjectOperation) (directbase.Adapter, error) {
	u := op.GetUnstructured()
	reader := op.Reader
	obj := &krm.RunJob{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, &obj); err != nil {
		return nil, fmt.Errorf("error converting to %T: %w", obj, err)
	}

	if ext := direct.ValueOf(obj.Status.ExternalRef); op.IsDeleting() && ext != "" {
		// Written here: deleting a job that was made needs only its name,
		// which is in status and cannot change; a project, service account
		// or secret object already gone would otherwise hold the delete.
		id := &krm.JobIdentity{}
		if err := id.FromExternal(ext); err != nil {
			return nil, err
		}
		gcpClient, err := m.client(ctx)
		if err != nil {
			return nil, err
		}
		return &JobAdapter{id: id, gcpClient: gcpClient}, nil
	}

	if err := common.NormalizeReferences(ctx, reader, obj, nil); err != nil {
		return nil, fmt.Errorf("normalizing references: %w", err)
	}

	id, err := krm.NewJobIdentity(ctx, reader, obj)
	if err != nil {
		return nil, err
	}
	if err := ResolveRunJobRefs(ctx, reader, obj); err != nil {
		return nil, err
	}

	mapCtx := &direct.MapContext{}
	copied := obj.DeepCopy()
	desired := RunJobSpec_ToProto(mapCtx, &copied.Spec)
	if mapCtx.Err() != nil {
		return nil, mapCtx.Err()
	}

	desired.Labels = label.NewGCPLabelsFromK8sLabels(u.GetLabels())

	gcpClient, err := m.client(ctx)
	if err != nil {
		return nil, err
	}
	return &JobAdapter{
		id:        id,
		gcpClient: gcpClient,
		desired:   desired,
	}, nil
}

func (m *modelJob) AdapterForURL(ctx context.Context, url string) (directbase.Adapter, error) {
	log := klog.FromContext(ctx)
	if s, ok := strings.CutPrefix(url, "//run.googleapis.com/"); ok {
		// Direct controller for RunJob only handles v2.
		s = strings.TrimPrefix(s, "v2/")

		var id krm.JobIdentity
		if err := id.FromExternal(s); err != nil {
			log.V(2).Error(err, "url did not match RunJob format", "url", url)
			return nil, nil
		}

		gcpClient, err := m.client(ctx)
		if err != nil {
			return nil, err
		}
		return &JobAdapter{
			gcpClient: gcpClient,
			id:        &id,
		}, nil
	}
	return nil, nil
}

type JobAdapter struct {
	id        *krm.JobIdentity
	gcpClient *gcp.JobsClient
	desired   *pb.Job
	actual    *pb.Job
}

var _ directbase.Adapter = &JobAdapter{}

// Find retrieves the GCP resource.
// Return true means the object is found. This triggers Adapter `Update` call.
// Return false means the object is not found. This triggers Adapter `Create` call.
// Return a non-nil error requeues the requests.
func (a *JobAdapter) Find(ctx context.Context) (bool, error) {
	log := klog.FromContext(ctx)
	log.V(2).Info("getting Job", "name", a.id)

	req := &pb.GetJobRequest{Name: a.id.String()}
	found, err := a.gcpClient.GetJob(ctx, req)
	if err != nil {
		if direct.IsNotFound(err) {
			return false, nil
		}
		return false, fmt.Errorf("getting Job %q: %w", a.id, err)
	}

	a.actual = found
	return true, nil
}

// Create creates the resource in GCP based on `spec` and update the Config Connector object `status` based on the GCP response.
func (a *JobAdapter) Create(ctx context.Context, createOp *directbase.CreateOperation) error {
	log := klog.FromContext(ctx)
	log.V(2).Info("creating Job", "name", a.id)
	req := &pb.CreateJobRequest{
		Parent: a.id.ParentString(),
		Job:    a.desired,
		JobId:  a.id.ID(),
	}
	op, err := a.gcpClient.CreateJob(ctx, req)
	if err != nil {
		return fmt.Errorf("creating Job %s: %w", a.id, err)
	}
	if _, err := op.Wait(ctx); err != nil {
		return fmt.Errorf("waiting for creation of job %q: %w", a.id, err)
	}
	log.V(2).Info("successfully created Job", "name", a.id)

	latest, err := a.gcpClient.GetJob(ctx, &pb.GetJobRequest{Name: a.id.String()})
	if err != nil {
		return fmt.Errorf("getting Job %q: %w", a.id, err)
	}
	return a.updateStatus(ctx, createOp, latest)
}

// Update updates the resource in GCP based on `spec` and update the Config Connector object `status` based on the GCP response.
func (a *JobAdapter) Update(ctx context.Context, updateOp *directbase.UpdateOperation) error {
	log := klog.FromContext(ctx)
	log.V(2).Info("updating Job", "name", a.id)

	desired, diffs, err := compareJob(ctx, a.actual, a.desired)
	if err != nil {
		return err
	}
	if !diffs.HasDiff() {
		log.V(2).Info("resource is up to date", "name", a.id)
		return a.updateStatus(ctx, updateOp, a.actual)
	}
	structuredreporting.ReportDiff(ctx, diffs)

	// The API's UpdateJob takes no update mask: it replaces the job with
	// the one sent, so the whole desired job is sent.
	desired.Name = a.id.String()
	op, err := a.gcpClient.UpdateJob(ctx, &pb.UpdateJobRequest{Job: desired})
	if err != nil {
		return fmt.Errorf("updating Job %s: %w", a.id, err)
	}
	if _, err := op.Wait(ctx); err != nil {
		return fmt.Errorf("Job %s waiting update: %w", a.id, err)
	}
	log.Info("successfully updated Job", "name", a.id)

	latest, err := a.gcpClient.GetJob(ctx, &pb.GetJobRequest{Name: a.id.String()})
	if err != nil {
		return fmt.Errorf("getting Job %q: %w", a.id, err)
	}
	return a.updateStatus(ctx, updateOp, latest)
}

// compareJob is written here, after our RunService's compareService. It
// returns the desired job with the server's defaults taken from what was
// read where the spec leaves them unset, and its differences from the job
// as read, top-level field by top-level field.
func compareJob(ctx context.Context, actual, desired *pb.Job) (*pb.Job, *structuredreporting.Diff, error) {
	maskedActual, err := mappers.OnlySpecFields(actual, RunJobSpec_FromProto, RunJobSpec_ToProto)
	if err != nil {
		return nil, nil, err
	}
	// The labels are metadata.labels, not in the spec's mapping.
	maskedActual.Labels = actual.GetLabels()

	d := proto.Clone(desired).(*pb.Job)
	if d.LaunchStage == 0 {
		d.LaunchStage = maskedActual.LaunchStage
	}
	if d.Template == nil {
		d.Template = maskedActual.Template
	}
	if at := maskedActual.GetTemplate(); d.Template != nil && at != nil {
		if d.Template.TaskCount == 0 {
			d.Template.TaskCount = at.TaskCount
		}
		if d.Template.Parallelism == 0 {
			d.Template.Parallelism = at.Parallelism
		}
		if d.Template.Template == nil {
			d.Template.Template = at.Template
		}
		if dt, att := d.Template.Template, at.GetTemplate(); dt != nil && att != nil {
			if dt.Timeout == nil {
				dt.Timeout = att.Timeout
			}
			if dt.Retries == nil {
				dt.Retries = att.Retries
			}
			if dt.ServiceAccount == "" {
				dt.ServiceAccount = att.ServiceAccount
			}
			if dt.ExecutionEnvironment == 0 {
				dt.ExecutionEnvironment = att.ExecutionEnvironment
			}
			defaultContainers(dt.Containers, att.Containers)
		}
	}

	diffs, _, err := common.DiffForTopLevelFields(ctx, d.ProtoReflect(), maskedActual.ProtoReflect())
	if err != nil {
		return nil, nil, err
	}
	return d, diffs, nil
}

// defaultContainers fills in, in the desired containers, what the server
// gives a container the spec leaves unset: its resources (or the limits it
// leaves out), ports and probe timings. Written here, after compareService.
func defaultContainers(desired, actual []*pb.Container) {
	byName := make(map[string]*pb.Container)
	for _, c := range actual {
		if c.GetName() != "" {
			byName[c.GetName()] = c
		}
	}
	for i, d := range desired {
		var a *pb.Container
		if d.GetName() != "" {
			a = byName[d.GetName()]
		}
		if a == nil && i < len(actual) {
			a = actual[i]
		}
		if a == nil {
			continue
		}
		if len(d.Ports) == 0 {
			d.Ports = a.Ports
		}
		if d.StartupProbe == nil {
			d.StartupProbe = a.StartupProbe
		} else {
			defaultProbe(d.StartupProbe, a.StartupProbe)
		}
		defaultProbe(d.LivenessProbe, a.LivenessProbe)
		switch {
		case d.Resources == nil:
			d.Resources = a.Resources
		case a.Resources == nil:
		case d.Resources.Limits == nil:
			d.Resources.Limits = a.Resources.Limits
		default:
			for k, v := range a.Resources.Limits {
				if _, ok := d.Resources.Limits[k]; !ok {
					d.Resources.Limits[k] = v
				}
			}
		}
	}
}

// defaultProbe takes the timings the server fills in for a probe the spec
// gives without them.
func defaultProbe(desired, actual *pb.Probe) {
	if desired == nil || actual == nil {
		return
	}
	if desired.FailureThreshold == 0 {
		desired.FailureThreshold = actual.FailureThreshold
	}
	if desired.PeriodSeconds == 0 {
		desired.PeriodSeconds = actual.PeriodSeconds
	}
	if desired.TimeoutSeconds == 0 {
		desired.TimeoutSeconds = actual.TimeoutSeconds
	}
	if desired.InitialDelaySeconds == 0 {
		desired.InitialDelaySeconds = actual.InitialDelaySeconds
	}
}

func (a *JobAdapter) updateStatus(ctx context.Context, op directbase.Operation, latest *pb.Job) error {
	mapCtx := &direct.MapContext{}
	status := &krm.RunJobStatus{}
	status.ObservedState = RunJobObservedState_FromProto(mapCtx, latest)
	if mapCtx.Err() != nil {
		return mapCtx.Err()
	}
	status.ExternalRef = direct.LazyPtr(a.id.String())
	return op.UpdateStatus(ctx, status, nil)
}

// Export maps the GCP object to a Config Connector resource `spec`.
func (a *JobAdapter) Export(ctx context.Context) (*unstructured.Unstructured, error) {
	if a.actual == nil {
		return nil, fmt.Errorf("Find() not called")
	}

	obj := &krm.RunJob{}
	mapCtx := &direct.MapContext{}
	obj.Spec = direct.ValueOf(RunJobSpec_FromProto(mapCtx, a.actual))
	if mapCtx.Err() != nil {
		return nil, mapCtx.Err()
	}
	obj.Spec.ProjectRef = &refs.ProjectRef{External: a.id.ProjectID}
	obj.Spec.Location = &a.id.Location
	obj.Spec.ResourceID = direct.LazyPtr(a.id.ID())
	uObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
	if err != nil {
		return nil, err
	}

	u := &unstructured.Unstructured{Object: uObj}
	u.SetName(a.id.ID())
	u.SetGroupVersionKind(krm.RunJobGVK)
	u.SetLabels(a.actual.GetLabels())

	return u, nil
}

// Delete the resource from GCP service when the corresponding Config Connector resource is deleted.
func (a *JobAdapter) Delete(ctx context.Context, deleteOp *directbase.DeleteOperation) (bool, error) {
	log := klog.FromContext(ctx)
	log.V(2).Info("deleting Job", "name", a.id)

	name := a.id.String()
	req := &pb.DeleteJobRequest{Name: name}
	op, err := a.gcpClient.DeleteJob(ctx, req)
	if err != nil {
		if direct.IsNotFound(err) {
			// Return success if not found (assume it was already deleted).
			log.V(2).Info("skipping delete for non-existent Job, assuming it was already deleted", "name", a.id)
			return true, nil
		}
		return false, fmt.Errorf("deleting Job %s: %w", a.id, err)
	}
	log.V(2).Info("successfully deleted Job", "name", a.id)

	if _, err = op.Wait(ctx); err != nil {
		return false, fmt.Errorf("waiting delete Job %s: %w", a.id, err)
	}
	return true, nil
}

// CloudLabels are the found resource's labels, for the slot guard
// (pkg/slotguard). Written here.
func (a *JobAdapter) CloudLabels() map[string]string {
	return a.actual.GetLabels()
}
