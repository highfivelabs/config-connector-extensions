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

// Written here: PrivateCACAPoolFields, a field kind (docs/design.md section
// 6.6, pattern 2; decision 37). No Config Connector code or pull request
// sets a CA pool's issuancePolicy.backdateDuration or
// issuancePolicy.baselineValues.nameConstraints. The pool is read with
// caPools.get and written with caPools.patch and an update mask of
// issuance_policy, sending the pool's issuance policy as read with only the
// owned fields changed (the API's masks name top-level fields, as Config
// Connector's own updates do). Config Connector's PrivateCACAPool masks
// top-level fields too: the DCL's updateMask is the top-level names of its
// field diffs and the direct controller's comes from DiffForTopLevelFields,
// and neither knows these fields, so neither sees a diff on them. When
// Config Connector changes its own issuance policy fields, its update sends
// an issuance policy without ours, and the API clears them until this
// kind's next reconcile puts them back (the partial case of section 6.6,
// as ContainerNodePoolFields' kubeletConfig).

import (
	"context"
	"fmt"

	api "cloud.google.com/go/security/privateca/apiv1"
	pb "cloud.google.com/go/security/privateca/apiv1/privatecapb"
	"github.com/GoogleCloudPlatform/k8s-config-connector/apis/privateca/privatecarefs"
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

// NewCAPoolFieldsModel is the PrivateCACAPoolFields model.
func NewCAPoolFieldsModel(ctx context.Context, config *config.ControllerConfig) (directbase.Model, error) {
	return &caPoolFieldsModel{config: *config}, nil
}

type caPoolFieldsModel struct {
	config config.ControllerConfig
}

var _ directbase.Model = &caPoolFieldsModel{}

// The update mask path this kind ever writes.
const issuancePolicyPath = "issuance_policy"

func (m *caPoolFieldsModel) AdapterForObject(ctx context.Context, op *directbase.AdapterForObjectOperation) (directbase.Adapter, error) {
	obj := &krm.PrivateCACAPoolFields{}
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
		// Deleting: the pool this object set is in its status; Config
		// Connector's PrivateCACAPool may be gone already.
		ref = &privatecarefs.PrivateCACAPoolRef{External: prev}
	} else if err := ref.Normalize(ctx, op.Reader, obj.GetNamespace()); err != nil {
		return nil, err
	}
	id := &privatecarefs.PrivateCACAPoolIdentity{}
	if err := id.FromExternal(privatecarefs.StripCAPoolPrefix(ref.External)); err != nil {
		return nil, err
	}
	if prev != "" && prev != id.String() {
		return nil, fmt.Errorf("cannot change the CA pool of PrivateCACAPoolFields (old=%q, new=%q)", prev, id.String())
	}
	spec := obj.Spec.IssuancePolicy
	if spec == nil {
		return nil, fmt.Errorf("spec.issuancePolicy is required")
	}
	var backdate *durationpb.Duration
	if spec.BackdateDuration != nil {
		d, err := parseDuration("spec.issuancePolicy.backdateDuration", *spec.BackdateDuration)
		if err != nil {
			return nil, err
		}
		backdate = d
	}
	if err := fieldkind.CheckSoleOwner(ctx, op.Reader, krm.PrivateCACAPoolFieldsGVK, obj, id.String()); err != nil {
		return nil, err
	}
	c, err := newClient(ctx, &m.config)
	if err != nil {
		return nil, err
	}
	return &caPoolFieldsAdapter{id: id, gcpClient: c, spec: spec, backdate: backdate}, nil
}

func (m *caPoolFieldsModel) AdapterForURL(ctx context.Context, url string) (directbase.Adapter, error) {
	return nil, nil
}

type caPoolFieldsAdapter struct {
	id        *privatecarefs.PrivateCACAPoolIdentity
	gcpClient *api.CertificateAuthorityClient
	spec      *krm.CAPoolFields_IssuancePolicy
	backdate  *durationpb.Duration
	actual    *pb.CaPool
}

var _ directbase.Adapter = &caPoolFieldsAdapter{}

// ownsBackdate and ownsNameConstraints say which fields the spec owns.
func (a *caPoolFieldsAdapter) ownsBackdate() bool { return a.spec.BackdateDuration != nil }
func (a *caPoolFieldsAdapter) ownsNameConstraints() bool {
	return a.spec.BaselineValues != nil && a.spec.BaselineValues.NameConstraints != nil
}

// Find reports whether the pool exists. A missing pool is "not found" so
// that a delete finishes when it is gone; a create then waits (Create).
func (a *caPoolFieldsAdapter) Find(ctx context.Context) (bool, error) {
	p, err := a.gcpClient.GetCaPool(ctx, &pb.GetCaPoolRequest{Name: a.id.String()})
	if err != nil {
		if direct.IsNotFound(err) {
			return false, nil
		}
		return false, fmt.Errorf("getting CA pool %s: %w", a.id, err)
	}
	a.actual = p
	return true, nil
}

// Create is called only while the pool does not exist: this kind never
// makes it, so it waits for it.
func (a *caPoolFieldsAdapter) Create(ctx context.Context, createOp *directbase.CreateOperation) error {
	return fieldkind.NotFoundError("PrivateCACAPool", a.id.String())
}

// withOwned is the pool's issuance policy with the owned fields set to
// what the spec says (set), or cleared (!set). Nil when nothing is left.
func (a *caPoolFieldsAdapter) withOwned(set bool) *pb.CaPool_IssuancePolicy {
	ip := proto.CloneOf(a.actual.GetIssuancePolicy())
	if ip == nil {
		ip = &pb.CaPool_IssuancePolicy{}
	}
	if a.ownsBackdate() {
		ip.BackdateDuration = nil
		if set {
			ip.BackdateDuration = a.backdate
		}
	}
	if a.ownsNameConstraints() {
		if ip.BaselineValues == nil {
			ip.BaselineValues = &pb.X509Parameters{}
		}
		ip.BaselineValues.NameConstraints = nil
		if set {
			ip.BaselineValues.NameConstraints = nameConstraintsToProto(a.spec.BaselineValues.NameConstraints)
		}
		if proto.Size(ip.BaselineValues) == 0 {
			ip.BaselineValues = nil
		}
	}
	if proto.Size(ip) == 0 {
		return nil
	}
	return ip
}

// differs reports whether the owned fields of the pool differ from want.
func (a *caPoolFieldsAdapter) differs(want *pb.CaPool_IssuancePolicy) bool {
	have := a.actual.GetIssuancePolicy()
	if a.ownsBackdate() && !sameDuration(have.GetBackdateDuration(), want.GetBackdateDuration()) {
		return true
	}
	if a.ownsNameConstraints() {
		h, w := have.GetBaselineValues().GetNameConstraints(), want.GetBaselineValues().GetNameConstraints()
		if (h == nil) != (w == nil) || !sameNameConstraints(h, w) {
			return true
		}
	}
	return false
}

func (a *caPoolFieldsAdapter) Update(ctx context.Context, updateOp *directbase.UpdateOperation) error {
	if want := a.withOwned(true); a.differs(want) {
		klog.FromContext(ctx).V(2).Info("setting fields of a CA pool", "name", a.id)
		if err := a.patch(ctx, want); err != nil {
			return err
		}
	}
	return updateOp.UpdateStatus(ctx, &krm.PrivateCACAPoolFieldsStatus{ExternalRef: direct.LazyPtr(a.id.String())}, nil)
}

func (a *caPoolFieldsAdapter) patch(ctx context.Context, ip *pb.CaPool_IssuancePolicy) error {
	op, err := a.gcpClient.UpdateCaPool(ctx, &pb.UpdateCaPoolRequest{
		CaPool:     &pb.CaPool{Name: a.id.String(), IssuancePolicy: ip},
		UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{issuancePolicyPath}},
	})
	if err != nil {
		return fmt.Errorf("setting the issuance policy of CA pool %s: %w", a.id, err)
	}
	updated, err := op.Wait(ctx)
	if err != nil {
		return fmt.Errorf("waiting for the issuance policy of CA pool %s: %w", a.id, err)
	}
	a.actual = updated
	return nil
}

// Delete clears the fields this object owns (the pool then has none, as
// before it). The pool is never deleted. A pool that is gone is not
// reached: Find said not found.
func (a *caPoolFieldsAdapter) Delete(ctx context.Context, deleteOp *directbase.DeleteOperation) (bool, error) {
	want := a.withOwned(false)
	if !a.differs(want) {
		return true, nil
	}
	klog.FromContext(ctx).V(2).Info("clearing fields of a CA pool", "name", a.id)
	if err := a.patch(ctx, want); err != nil {
		return false, err
	}
	return true, nil
}

func (a *caPoolFieldsAdapter) Export(ctx context.Context) (*unstructured.Unstructured, error) {
	return nil, nil
}
