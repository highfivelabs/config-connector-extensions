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

package privateca

// Written here: PrivateCACertificateTemplateFields, a field kind
// (docs/design.md section 6.6, pattern 2; decision 37). No Config Connector
// code or pull request sets a certificate template's maximumLifetime or
// predefinedValues.nameConstraints. The template is read with
// certificateTemplates.get and written with certificateTemplates.patch and
// an update mask of maximum_lifetime and/or predefined_values (only the
// ones that differ); predefined_values is sent as the template has it,
// with only the name constraints changed. Config Connector's
// PrivateCACertificateTemplate masks top-level fields (the DCL's updateMask
// is the top-level names of its field diffs, the direct controller's comes
// from DiffForTopLevelFields) and knows neither field: maximumLifetime is
// never in its mask; when it changes its own predefined values, its update
// sends them without our name constraints, and the API clears them until
// this kind's next reconcile puts them back (the partial case of section
// 6.6, as ContainerNodePoolFields' kubeletConfig).

import (
	"context"
	"fmt"

	api "cloud.google.com/go/security/privateca/apiv1"
	pb "cloud.google.com/go/security/privateca/apiv1/privatecapb"
	privatecav1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/apis/privateca/v1beta1"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/config"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct/directbase"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"

	krm "github.com/highfivelabs/config-connector-extensions/apis/privateca/v1alpha1"
	"github.com/highfivelabs/config-connector-extensions/pkg/fieldkind"
)

// NewCertificateTemplateFieldsModel is the
// PrivateCACertificateTemplateFields model.
func NewCertificateTemplateFieldsModel(ctx context.Context, config *config.ControllerConfig) (directbase.Model, error) {
	return &certificateTemplateFieldsModel{config: *config}, nil
}

type certificateTemplateFieldsModel struct {
	config config.ControllerConfig
}

var _ directbase.Model = &certificateTemplateFieldsModel{}

// The update mask paths this kind ever writes.
const (
	maximumLifetimePath  = "maximum_lifetime"
	predefinedValuesPath = "predefined_values"
)

func (m *certificateTemplateFieldsModel) AdapterForObject(ctx context.Context, op *directbase.AdapterForObjectOperation) (directbase.Adapter, error) {
	obj := &krm.PrivateCACertificateTemplateFields{}
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
		// Deleting: the template this object set is in its status; Config
		// Connector's PrivateCACertificateTemplate may be gone already.
		ref = &privatecav1beta1.PrivateCACertificateTemplateRef{External: prev}
	} else if err := ref.Normalize(ctx, op.Reader, obj.GetNamespace()); err != nil {
		return nil, err
	}
	id := &privatecav1beta1.PrivateCACertificateTemplateIdentity{}
	if err := id.FromExternal(ref.External); err != nil {
		return nil, err
	}
	if prev != "" && prev != id.String() {
		return nil, fmt.Errorf("cannot change the certificate template of PrivateCACertificateTemplateFields (old=%q, new=%q)", prev, id.String())
	}
	var lifetime *durationpb.Duration
	if obj.Spec.MaximumLifetime != nil {
		d, err := parseDuration("spec.maximumLifetime", *obj.Spec.MaximumLifetime)
		if err != nil {
			return nil, err
		}
		lifetime = d
	}
	if err := fieldkind.CheckSoleOwner(ctx, op.Reader, krm.PrivateCACertificateTemplateFieldsGVK, obj, id.String()); err != nil {
		return nil, err
	}
	c, err := newClient(ctx, &m.config)
	if err != nil {
		return nil, err
	}
	return &certificateTemplateFieldsAdapter{id: id, gcpClient: c, spec: &obj.Spec, lifetime: lifetime}, nil
}

func (m *certificateTemplateFieldsModel) AdapterForURL(ctx context.Context, url string) (directbase.Adapter, error) {
	return nil, nil
}

type certificateTemplateFieldsAdapter struct {
	id        *privatecav1beta1.PrivateCACertificateTemplateIdentity
	gcpClient *api.CertificateAuthorityClient
	spec      *krm.PrivateCACertificateTemplateFieldsSpec
	lifetime  *durationpb.Duration
	actual    *pb.CertificateTemplate
}

var _ directbase.Adapter = &certificateTemplateFieldsAdapter{}

func (a *certificateTemplateFieldsAdapter) ownsLifetime() bool { return a.spec.MaximumLifetime != nil }
func (a *certificateTemplateFieldsAdapter) ownsNameConstraints() bool {
	return a.spec.PredefinedValues != nil && a.spec.PredefinedValues.NameConstraints != nil
}

// Find reports whether the template exists. A missing template is "not
// found" so that a delete finishes when it is gone; a create then waits
// (Create).
func (a *certificateTemplateFieldsAdapter) Find(ctx context.Context) (bool, error) {
	t, err := a.gcpClient.GetCertificateTemplate(ctx, &pb.GetCertificateTemplateRequest{Name: a.id.String()})
	if err != nil {
		if direct.IsNotFound(err) {
			return false, nil
		}
		return false, fmt.Errorf("getting certificate template %s: %w", a.id, err)
	}
	a.actual = t
	return true, nil
}

// Create is called only while the template does not exist: this kind
// never makes it, so it waits for it.
func (a *certificateTemplateFieldsAdapter) Create(ctx context.Context, createOp *directbase.CreateOperation) error {
	return fieldkind.NotFoundError("PrivateCACertificateTemplate", a.id.String())
}

// desired is the body and mask that set the owned fields to what the spec
// says (set) or clear them (!set), naming only the fields that differ.
func (a *certificateTemplateFieldsAdapter) desired(set bool) (*pb.CertificateTemplate, []string) {
	body := &pb.CertificateTemplate{Name: a.id.String()}
	var paths []string
	if a.ownsLifetime() {
		var want *durationpb.Duration
		if set {
			want = a.lifetime
		}
		if (want == nil) != (a.actual.GetMaximumLifetime() == nil) || !sameDuration(want, a.actual.GetMaximumLifetime()) {
			body.MaximumLifetime = want
			paths = append(paths, maximumLifetimePath)
		}
	}
	if a.ownsNameConstraints() {
		var want *pb.X509Parameters_NameConstraints
		if set {
			want = nameConstraintsToProto(a.spec.PredefinedValues.NameConstraints)
		}
		have := a.actual.GetPredefinedValues().GetNameConstraints()
		if (want == nil) != (have == nil) || !sameNameConstraints(want, have) {
			pv := proto.CloneOf(a.actual.GetPredefinedValues())
			if pv == nil {
				pv = &pb.X509Parameters{}
			}
			pv.NameConstraints = want
			if proto.Size(pv) == 0 {
				pv = nil
			}
			body.PredefinedValues = pv
			paths = append(paths, predefinedValuesPath)
		}
	}
	return body, paths
}

func (a *certificateTemplateFieldsAdapter) Update(ctx context.Context, updateOp *directbase.UpdateOperation) error {
	if body, paths := a.desired(true); len(paths) > 0 {
		klog.FromContext(ctx).V(2).Info("setting fields of a certificate template", "name", a.id, "paths", paths)
		if err := a.patch(ctx, body, paths); err != nil {
			return err
		}
	}
	return updateOp.UpdateStatus(ctx, &krm.PrivateCACertificateTemplateFieldsStatus{ExternalRef: direct.LazyPtr(a.id.String())}, nil)
}

func (a *certificateTemplateFieldsAdapter) patch(ctx context.Context, body *pb.CertificateTemplate, paths []string) error {
	op, err := a.gcpClient.UpdateCertificateTemplate(ctx, &pb.UpdateCertificateTemplateRequest{
		CertificateTemplate: body,
		UpdateMask:          &fieldmaskpb.FieldMask{Paths: paths},
	})
	if err != nil {
		return fmt.Errorf("setting %v of certificate template %s: %w", paths, a.id, err)
	}
	updated, err := op.Wait(ctx)
	if err != nil {
		return fmt.Errorf("waiting for %v of certificate template %s: %w", paths, a.id, err)
	}
	a.actual = updated
	return nil
}

// Delete clears the fields this object owns. The template is never
// deleted. A template that is gone is not reached: Find said not found.
func (a *certificateTemplateFieldsAdapter) Delete(ctx context.Context, deleteOp *directbase.DeleteOperation) (bool, error) {
	body, paths := a.desired(false)
	if len(paths) == 0 {
		return true, nil
	}
	klog.FromContext(ctx).V(2).Info("clearing fields of a certificate template", "name", a.id, "paths", paths)
	if err := a.patch(ctx, body, paths); err != nil {
		return false, err
	}
	return true, nil
}

func (a *certificateTemplateFieldsAdapter) Export(ctx context.Context) (*unstructured.Unstructured, error) {
	return nil, nil
}
