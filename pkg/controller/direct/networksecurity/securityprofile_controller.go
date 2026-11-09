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

// Modified by High Five Labs for config-connector-extensions. Copied from Config Connector
// pkg/controller/direct/networksecurity/networksecuritysecurityprofile_controller.go
// (UPSTREAM.md). Changed: the REST clients (SecurityProfileGroupClient, and
// OrganizationSecurityProfileGroupClient for an organization's profile; their
// methods and operations are the same Go types) in place of upstream's gRPC
// connection, which our manager does not configure; an unchanged object still
// has its status written; the comparison ignores what Magic Modules'
// SecurityProfile.yaml ignores (list order, the URL filter the API adds);
// no export; the model is returned to our manager instead of registered with
// Config Connector's registry; our KRM types.

package networksecurity

import (
	"context"
	"fmt"
	"slices"
	"strings"

	gcp "cloud.google.com/go/networksecurity/apiv1"
	pb "cloud.google.com/go/networksecurity/apiv1/networksecuritypb"
	"github.com/googleapis/gax-go/v2"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/fieldmaskpb"

	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/config"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct/common"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct/directbase"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/structuredreporting"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"

	krm "github.com/highfivelabs/config-connector-extensions/apis/networksecurity/v1alpha1"
)

func NewSecurityProfileModel(ctx context.Context, config *config.ControllerConfig) (directbase.Model, error) {
	return &securityProfileModel{config: *config}, nil
}

var _ directbase.Model = &securityProfileModel{}

type securityProfileModel struct {
	config config.ControllerConfig
}

// securityProfileClient is what the adapter calls: the methods the project
// service's client (SecurityProfileGroupClient) and the organization
// service's (OrganizationSecurityProfileGroupClient) share. Written here.
type securityProfileClient interface {
	GetSecurityProfile(context.Context, *pb.GetSecurityProfileRequest, ...gax.CallOption) (*pb.SecurityProfile, error)
	CreateSecurityProfile(context.Context, *pb.CreateSecurityProfileRequest, ...gax.CallOption) (*gcp.CreateSecurityProfileOperation, error)
	UpdateSecurityProfile(context.Context, *pb.UpdateSecurityProfileRequest, ...gax.CallOption) (*gcp.UpdateSecurityProfileOperation, error)
	DeleteSecurityProfile(context.Context, *pb.DeleteSecurityProfileRequest, ...gax.CallOption) (*gcp.DeleteSecurityProfileOperation, error)
}

func (m *securityProfileModel) client(ctx context.Context, id *krm.NetworkSecuritySecurityProfileIdentity) (securityProfileClient, error) {
	opts, err := m.config.RESTClientOptions()
	if err != nil {
		return nil, err
	}
	if id.Organization != "" {
		gcpClient, err := gcp.NewOrganizationSecurityProfileGroupRESTClient(ctx, opts...)
		if err != nil {
			return nil, fmt.Errorf("building networksecurity organization security profile REST client: %w", err)
		}
		return gcpClient, nil
	}
	gcpClient, err := gcp.NewSecurityProfileGroupRESTClient(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("building networksecurity security profile REST client: %w", err)
	}
	return gcpClient, nil
}

func (m *securityProfileModel) AdapterForObject(ctx context.Context, op *directbase.AdapterForObjectOperation) (directbase.Adapter, error) {
	u := op.GetUnstructured()
	reader := op.Reader
	obj := &krm.NetworkSecuritySecurityProfile{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, &obj); err != nil {
		return nil, fmt.Errorf("error converting to %T: %w", obj, err)
	}

	if err := common.NormalizeReferences(ctx, reader, obj, nil); err != nil {
		return nil, fmt.Errorf("normalizing references: %w", err)
	}

	id, err := obj.GetIdentity(ctx, reader)
	if err != nil {
		return nil, err
	}
	spID := id.(*krm.NetworkSecuritySecurityProfileIdentity)

	gcpClient, err := m.client(ctx, spID)
	if err != nil {
		return nil, err
	}

	mapCtx := &direct.MapContext{}
	desired := NetworkSecuritySecurityProfileSpec_v1alpha1_ToProto(mapCtx, &obj.Spec)
	if mapCtx.Err() != nil {
		return nil, mapCtx.Err()
	}

	desired.Name = spID.String()

	return &securityProfileAdapter{
		gcpClient: gcpClient,
		id:        spID,
		desired:   desired,
	}, nil
}

func (m *securityProfileModel) AdapterForURL(ctx context.Context, url string) (directbase.Adapter, error) {
	return nil, nil
}

type securityProfileAdapter struct {
	gcpClient securityProfileClient
	id        *krm.NetworkSecuritySecurityProfileIdentity
	desired   *pb.SecurityProfile
	actual    *pb.SecurityProfile
}

var _ directbase.Adapter = &securityProfileAdapter{}

func (a *securityProfileAdapter) Find(ctx context.Context) (bool, error) {
	log := klog.FromContext(ctx)
	log.V(2).Info("getting SecurityProfile", "name", a.id)

	req := &pb.GetSecurityProfileRequest{Name: a.id.String()}
	actual, err := a.gcpClient.GetSecurityProfile(ctx, req)
	if err != nil {
		if direct.IsNotFound(err) {
			return false, nil
		}
		return false, fmt.Errorf("getting SecurityProfile %q from gcp: %w", a.id.String(), err)
	}

	a.actual = actual
	return true, nil
}

func (a *securityProfileAdapter) Create(ctx context.Context, createOp *directbase.CreateOperation) error {
	log := klog.FromContext(ctx)
	log.V(2).Info("creating SecurityProfile", "name", a.id)

	req := &pb.CreateSecurityProfileRequest{
		Parent:            a.id.ParentString(),
		SecurityProfileId: a.id.Securityprofile,
		SecurityProfile:   a.desired,
	}
	op, err := a.gcpClient.CreateSecurityProfile(ctx, req)
	if err != nil {
		return fmt.Errorf("creating SecurityProfile %s: %w", a.id, err)
	}

	if _, err := op.Wait(ctx); err != nil {
		return fmt.Errorf("SecurityProfile %s waiting creation: %w", a.id, err)
	}

	latest, err := a.gcpClient.GetSecurityProfile(ctx, &pb.GetSecurityProfileRequest{Name: a.id.String()})
	if err != nil {
		return fmt.Errorf("getting SecurityProfile after creation: %w", err)
	}

	log.V(2).Info("successfully created SecurityProfile", "name", a.id)

	return a.updateStatus(ctx, createOp, latest)
}

func (a *securityProfileAdapter) Update(ctx context.Context, updateOp *directbase.UpdateOperation) error {
	log := klog.FromContext(ctx)
	log.V(2).Info("updating SecurityProfile", "name", a.id)

	diffs, updateMask, err := compareSecurityProfile(ctx, a.actual, a.desired)
	if err != nil {
		return err
	}

	diffs.Object = updateOp.GetUnstructured()
	structuredreporting.ReportDiff(ctx, diffs)

	if !diffs.HasDiff() {
		log.V(2).Info("no field needs update", "name", a.id)
		return a.updateStatus(ctx, updateOp, a.actual)
	}

	log.V(2).Info("fields need update", "name", a.id, "updateMask", updateMask)

	req := &pb.UpdateSecurityProfileRequest{
		UpdateMask:      updateMask,
		SecurityProfile: a.desired,
	}
	req.SecurityProfile.Name = a.id.String()

	op, err := a.gcpClient.UpdateSecurityProfile(ctx, req)
	if err != nil {
		return fmt.Errorf("updating SecurityProfile %s: %w", a.id, err)
	}
	if _, err := op.Wait(ctx); err != nil {
		return fmt.Errorf("SecurityProfile %s waiting update: %w", a.id, err)
	}

	latest, err := a.gcpClient.GetSecurityProfile(ctx, &pb.GetSecurityProfileRequest{Name: a.id.String()})
	if err != nil {
		return fmt.Errorf("getting SecurityProfile after update: %w", err)
	}

	log.V(2).Info("successfully updated SecurityProfile", "name", a.id)

	return a.updateStatus(ctx, updateOp, latest)
}

func compareSecurityProfile(ctx context.Context, actual, desired *pb.SecurityProfile) (*structuredreporting.Diff, *fieldmaskpb.FieldMask, error) {
	mapCtx := &direct.MapContext{}
	spec := NetworkSecuritySecurityProfileSpec_v1alpha1_FromProto(mapCtx, actual)
	if mapCtx.Err() != nil {
		return nil, nil, mapCtx.Err()
	}
	maskedActual := NetworkSecuritySecurityProfileSpec_v1alpha1_ToProto(mapCtx, spec)
	if mapCtx.Err() != nil {
		return nil, nil, mapCtx.Err()
	}
	maskedActual.Name = desired.Name

	// Written here, after Magic Modules' SecurityProfile.yaml: the override
	// and filter lists are sets (is_set), and the API adds a default URL
	// filter (priority 2147483647) that is not a difference unless the spec
	// names it (custom_flatten url_filters). Both sides are compared in one
	// order; desired is not changed.
	cmpDesired := proto.CloneOf(desired)
	normalizeSecurityProfile(cmpDesired, nil)
	normalizeSecurityProfile(maskedActual, desired)

	diffs, updateMask, err := common.DiffForTopLevelFields(ctx, cmpDesired.ProtoReflect(), maskedActual.ProtoReflect())
	if err != nil {
		return nil, nil, err
	}
	return diffs, updateMask, nil
}

// defaultURLFilterPriority is the priority of the URL filter the API adds to
// every URL filtering profile.
const defaultURLFilterPriority = 2147483647

// normalizeSecurityProfile sorts p's lists. With desired given, p is what the
// API returned, and the API's default URL filter is dropped unless desired
// has it. Written here.
func normalizeSecurityProfile(p *pb.SecurityProfile, desired *pb.SecurityProfile) {
	if tp := p.GetThreatPreventionProfile(); tp != nil {
		slices.SortFunc(tp.SeverityOverrides, func(a, b *pb.SeverityOverride) int {
			return strings.Compare(a.GetSeverity().String()+"/"+a.GetAction().String(), b.GetSeverity().String()+"/"+b.GetAction().String())
		})
		slices.SortFunc(tp.ThreatOverrides, func(a, b *pb.ThreatOverride) int {
			return strings.Compare(a.GetThreatId()+"/"+a.GetAction().String(), b.GetThreatId()+"/"+b.GetAction().String())
		})
		slices.SortFunc(tp.AntivirusOverrides, func(a, b *pb.AntivirusOverride) int {
			return strings.Compare(a.GetProtocol().String()+"/"+a.GetAction().String(), b.GetProtocol().String()+"/"+b.GetAction().String())
		})
	}
	if uf := p.GetUrlFilteringProfile(); uf != nil {
		if desired != nil {
			wantDefault := slices.ContainsFunc(desired.GetUrlFilteringProfile().GetUrlFilters(), func(f *pb.UrlFilter) bool {
				return f.GetPriority() == defaultURLFilterPriority
			})
			if !wantDefault {
				uf.UrlFilters = slices.DeleteFunc(uf.UrlFilters, func(f *pb.UrlFilter) bool {
					return f.GetPriority() == defaultURLFilterPriority
				})
			}
		}
		slices.SortFunc(uf.UrlFilters, func(a, b *pb.UrlFilter) int {
			if a.GetPriority() < b.GetPriority() {
				return -1
			}
			if a.GetPriority() > b.GetPriority() {
				return 1
			}
			return 0
		})
	}
	if len(p.Labels) == 0 {
		p.Labels = nil
	}
}

func (a *securityProfileAdapter) Export(ctx context.Context) (*unstructured.Unstructured, error) {
	return nil, nil
}

func (a *securityProfileAdapter) Delete(ctx context.Context, deleteOp *directbase.DeleteOperation) (bool, error) {
	log := klog.FromContext(ctx)
	log.V(2).Info("deleting SecurityProfile", "name", a.id)

	req := &pb.DeleteSecurityProfileRequest{Name: a.id.String()}
	op, err := a.gcpClient.DeleteSecurityProfile(ctx, req)
	if err != nil {
		if direct.IsNotFound(err) {
			log.V(2).Info("skipping delete for non-existent SecurityProfile, assuming it was already deleted", "name", a.id)
			return true, nil
		}
		return false, fmt.Errorf("deleting SecurityProfile %s: %w", a.id, err)
	}

	if err := op.Wait(ctx); err != nil {
		return false, fmt.Errorf("waiting delete SecurityProfile %s: %w", a.id, err)
	}

	log.V(2).Info("successfully deleted SecurityProfile", "name", a.id)
	return true, nil
}

func (a *securityProfileAdapter) updateStatus(ctx context.Context, op directbase.Operation, latest *pb.SecurityProfile) error {
	mapCtx := &direct.MapContext{}
	status := &krm.NetworkSecuritySecurityProfileStatus{}
	status.ObservedState = NetworkSecuritySecurityProfileObservedState_v1alpha1_FromProto(mapCtx, latest)
	if mapCtx.Err() != nil {
		return mapCtx.Err()
	}

	externalRef := a.id.String()
	status.ExternalRef = &externalRef

	return op.UpdateStatus(ctx, status, nil)
}
