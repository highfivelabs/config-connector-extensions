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

package compute

// Written here: ComputeServiceAttachmentFields, a field kind
// (docs/design.md section 6.6, pattern 2; decision 37). No Config
// Connector code or pull request sets a service attachment's
// reconcileConnections or propagatedConnectionLimit. The service attachment
// is read with serviceAttachments.get and written with
// serviceAttachments.patch, whose body names only the fields the spec sets
// (and the fingerprint read with them): the API's patch is a JSON merge
// patch, so no other field is touched. Config Connector's
// ComputeServiceAttachment runs the DCL, whose patch body
// (newUpdateServiceAttachmentPatchRequest) has neither field, so it leaves
// them alone (section 6.6).

import (
	"context"
	"fmt"
	"strings"

	gcp "cloud.google.com/go/compute/apiv1"
	pb "cloud.google.com/go/compute/apiv1/computepb"
	refs "github.com/GoogleCloudPlatform/k8s-config-connector/apis/refs/v1beta1"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/config"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct/directbase"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"

	krm "github.com/highfivelabs/config-connector-extensions/apis/compute/v1alpha1"
	"github.com/highfivelabs/config-connector-extensions/pkg/fieldkind"
)

// NewServiceAttachmentFieldsModel is the ComputeServiceAttachmentFields
// model.
func NewServiceAttachmentFieldsModel(ctx context.Context, config *config.ControllerConfig) (directbase.Model, error) {
	return &serviceAttachmentFieldsModel{config: *config}, nil
}

type serviceAttachmentFieldsModel struct {
	config config.ControllerConfig
}

var _ directbase.Model = &serviceAttachmentFieldsModel{}

// serviceAttachmentID is projects/{project}/regions/{region}/serviceAttachments/{name}.
type serviceAttachmentID struct {
	Project, Region, Name string
}

func (id serviceAttachmentID) String() string {
	return "projects/" + id.Project + "/regions/" + id.Region + "/serviceAttachments/" + id.Name
}

func parseServiceAttachmentID(external string) (serviceAttachmentID, error) {
	t := strings.Split(refs.TrimComputeURIPrefix(external), "/")
	if len(t) != 6 || t[0] != "projects" || t[2] != "regions" || t[4] != "serviceAttachments" || t[1] == "" || t[3] == "" || t[5] == "" {
		return serviceAttachmentID{}, fmt.Errorf("service attachment %q is not projects/{project}/regions/{region}/serviceAttachments/{name}", external)
	}
	return serviceAttachmentID{Project: t[1], Region: t[3], Name: t[5]}, nil
}

func (m *serviceAttachmentFieldsModel) AdapterForObject(ctx context.Context, op *directbase.AdapterForObjectOperation) (directbase.Adapter, error) {
	obj := &krm.ComputeServiceAttachmentFields{}
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
	external := prev
	if obj.GetDeletionTimestamp() == nil || prev == "" {
		// Not deleting: resolve the reference. (Deleting, the service
		// attachment is in status; Config Connector's object may be gone.)
		r := *ref
		if err := refs.ResolveComputeServiceAttachment(ctx, op.Reader, obj.GetNamespace(), &r); err != nil {
			return nil, err
		}
		external = r.External
	}
	id, err := parseServiceAttachmentID(external)
	if err != nil {
		return nil, err
	}
	if prev != "" && prev != id.String() {
		return nil, fmt.Errorf("cannot change the service attachment of ComputeServiceAttachmentFields (old=%q, new=%q)", prev, id.String())
	}
	if err := fieldkind.CheckSoleOwner(ctx, op.Reader, krm.ComputeServiceAttachmentFieldsGVK, obj, id.String()); err != nil {
		return nil, err
	}
	opts, err := m.config.RESTClientOptions()
	if err != nil {
		return nil, err
	}
	c, err := gcp.NewServiceAttachmentsRESTClient(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("building ServiceAttachments client: %w", err)
	}
	return &serviceAttachmentFieldsAdapter{id: id, gcpClient: c, desired: &obj.Spec}, nil
}

func (m *serviceAttachmentFieldsModel) AdapterForURL(ctx context.Context, url string) (directbase.Adapter, error) {
	return nil, nil
}

type serviceAttachmentFieldsAdapter struct {
	id        serviceAttachmentID
	gcpClient *gcp.ServiceAttachmentsClient
	desired   *krm.ComputeServiceAttachmentFieldsSpec
	actual    *pb.ServiceAttachment
}

var _ directbase.Adapter = &serviceAttachmentFieldsAdapter{}

// Find reports whether the service attachment exists. A missing one is
// "not found" so that a delete finishes when it is gone; a create then
// waits (Create).
func (a *serviceAttachmentFieldsAdapter) Find(ctx context.Context) (bool, error) {
	sa, err := a.gcpClient.Get(ctx, &pb.GetServiceAttachmentRequest{Project: a.id.Project, Region: a.id.Region, ServiceAttachment: a.id.Name})
	if err != nil {
		if direct.IsNotFound(err) {
			return false, nil
		}
		return false, fmt.Errorf("getting service attachment %s: %w", a.id, err)
	}
	a.actual = sa
	return true, nil
}

// Create is called only while the service attachment does not exist: this
// kind never makes it, so it waits for it.
func (a *serviceAttachmentFieldsAdapter) Create(ctx context.Context, createOp *directbase.CreateOperation) error {
	return fieldkind.NotFoundError("ComputeServiceAttachment", a.id.String())
}

// patchBody is the fields the spec sets that differ from the service
// attachment's, with its fingerprint; nil when nothing differs.
func (a *serviceAttachmentFieldsAdapter) patchBody() *pb.ServiceAttachment {
	body := &pb.ServiceAttachment{}
	changed := false
	if want := a.desired.ReconcileConnections; want != nil && a.actual.GetReconcileConnections() != *want {
		body.ReconcileConnections = direct.PtrTo(*want)
		changed = true
	}
	if want := a.desired.PropagatedConnectionLimit; want != nil && (a.actual.PropagatedConnectionLimit == nil || int64(a.actual.GetPropagatedConnectionLimit()) != int64(*want)) {
		body.PropagatedConnectionLimit = direct.PtrTo(uint32(*want))
		changed = true
	}
	if !changed {
		return nil
	}
	body.Fingerprint = a.actual.Fingerprint
	return body
}

func (a *serviceAttachmentFieldsAdapter) Update(ctx context.Context, updateOp *directbase.UpdateOperation) error {
	body := a.patchBody()
	if body == nil {
		return a.updateStatus(ctx, updateOp)
	}
	klog.FromContext(ctx).V(2).Info("setting fields of a service attachment", "name", a.id)
	op, err := a.gcpClient.Patch(ctx, &pb.PatchServiceAttachmentRequest{
		Project: a.id.Project, Region: a.id.Region, ServiceAttachment: a.id.Name,
		ServiceAttachmentResource: body,
	})
	if err == nil && !op.Done() {
		err = op.Wait(ctx)
	}
	if err != nil {
		return fmt.Errorf("setting fields of service attachment %s: %w", a.id, err)
	}
	return a.updateStatus(ctx, updateOp)
}

// Delete leaves the fields as they are: a patch can only set a value, not
// give a field back to its default, and the service attachment itself is
// never deleted. A service attachment that is gone is not reached: Find
// said not found.
func (a *serviceAttachmentFieldsAdapter) Delete(ctx context.Context, deleteOp *directbase.DeleteOperation) (bool, error) {
	return true, nil
}

func (a *serviceAttachmentFieldsAdapter) Export(ctx context.Context) (*unstructured.Unstructured, error) {
	return nil, nil
}

func (a *serviceAttachmentFieldsAdapter) updateStatus(ctx context.Context, op directbase.Operation) error {
	return op.UpdateStatus(ctx, &krm.ComputeServiceAttachmentFieldsStatus{ExternalRef: direct.LazyPtr(a.id.String())}, nil)
}
