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
// pkg/controller/direct/run/runworkerpool_controller.go (UPSTREAM.md), the direct
// RunService controller, for a Cloud Run service (google.cloud.run.v2.Services):
// the worker pool's calls become the service's, without instance splits; the KRM
// types are this project's. The controller owns the whole service and updates it
// with an update mask of the top-level fields that differ, so fields this kind
// does not model are left as they are. Written here: the server defaults a
// service has that a worker pool does not, which compareService takes from what
// was read when the spec leaves them unset.

package run

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	gcp "cloud.google.com/go/run/apiv2"
	pb "cloud.google.com/go/run/apiv2/runpb"
	"google.golang.org/api/option"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"

	refs "github.com/GoogleCloudPlatform/k8s-config-connector/apis/refs/v1beta1"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/config"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct/common"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct/directbase"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct/registry"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/mappers"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/structuredreporting"

	krm "github.com/highfivelabs/config-connector-extensions/apis/run/v1alpha1"
)

func init() {
	registry.RegisterModel(krm.RunServiceGVK, NewRunServiceModel)
}

func NewRunServiceModel(ctx context.Context, config *config.ControllerConfig) (directbase.Model, error) {
	return &modelService{config: *config}, nil
}

var _ directbase.Model = &modelService{}

type modelService struct {
	config config.ControllerConfig
}

func (m *modelService) client(ctx context.Context) (*gcp.ServicesClient, error) {
	var opts []option.ClientOption
	opts, err := m.config.RESTClientOptions()
	if err != nil {
		return nil, err
	}
	gcpClient, err := gcp.NewServicesRESTClient(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("building Service client: %w", err)
	}
	return gcpClient, err
}

func (m *modelService) AdapterForObject(ctx context.Context, op *directbase.AdapterForObjectOperation) (directbase.Adapter, error) {
	u := op.GetUnstructured()
	reader := op.Reader
	obj := &krm.RunService{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, &obj); err != nil {
		return nil, fmt.Errorf("error converting to %T: %w", obj, err)
	}

	identity, err := obj.GetIdentity(ctx, reader)
	if err != nil {
		return nil, err
	}
	id := identity.(*krm.RunServiceIdentity)

	if err := common.NormalizeReferences(ctx, reader, obj, nil); err != nil {
		return nil, fmt.Errorf("normalizing references: %w", err)
	}
	if err := ResolveRunServiceRefs(ctx, reader, obj); err != nil {
		return nil, err
	}

	mapCtx := &direct.MapContext{}
	copied := obj.DeepCopy()
	desired := RunServiceSpec_v1alpha1_ToProto(mapCtx, &copied.Spec)
	if mapCtx.Err() != nil {
		return nil, mapCtx.Err()
	}

	gcpClient, err := m.client(ctx)
	if err != nil {
		return nil, err
	}
	return &ServiceAdapter{
		id:        id,
		gcpClient: gcpClient,
		desired:   desired,
	}, nil
}

func (m *modelService) AdapterForURL(ctx context.Context, url string) (directbase.Adapter, error) {
	log := klog.FromContext(ctx)
	if s, ok := strings.CutPrefix(url, "//run.googleapis.com/"); ok {
		s = strings.TrimPrefix(s, "v2/")

		var id krm.RunServiceIdentity
		if err := id.FromExternal(s); err != nil {
			log.V(2).Error(err, "url did not match RunService format", "url", url)
			return nil, nil
		}

		gcpClient, err := m.client(ctx)
		if err != nil {
			return nil, err
		}
		return &ServiceAdapter{
			gcpClient: gcpClient,
			id:        &id,
		}, nil
	}
	return nil, nil
}

type ServiceAdapter struct {
	id        *krm.RunServiceIdentity
	gcpClient *gcp.ServicesClient
	desired   *pb.Service
	actual    *pb.Service
}

var _ directbase.Adapter = &ServiceAdapter{}

func (a *ServiceAdapter) Find(ctx context.Context) (bool, error) {
	log := klog.FromContext(ctx)
	log.V(2).Info("getting Service", "name", a.id.String())

	req := &pb.GetServiceRequest{Name: a.id.String()}
	found, err := a.gcpClient.GetService(ctx, req)
	if err != nil {
		if direct.IsNotFound(err) {
			return false, nil
		}
		return false, fmt.Errorf("getting Service %q: %w", a.id.String(), err)
	}

	a.actual = found
	return true, nil
}

func (a *ServiceAdapter) Create(ctx context.Context, createOp *directbase.CreateOperation) error {
	log := klog.FromContext(ctx)
	log.V(2).Info("creating Service", "name", a.id.String())

	desired := proto.Clone(a.desired).(*pb.Service)
	desired.Name = ""

	req := &pb.CreateServiceRequest{
		Parent:    a.id.ParentString(),
		Service:   desired,
		ServiceId: a.id.Service,
	}
	op, err := a.gcpClient.CreateService(ctx, req)
	if err != nil {
		return fmt.Errorf("creating Service %s: %w", a.id.String(), err)
	}
	created, err := op.Wait(ctx)
	if err != nil {
		return fmt.Errorf("waiting for creation of Service %q: %w", a.id.String(), err)
	}
	log.V(2).Info("successfully created Service", "name", a.id.String())

	if err := createOp.SetMBUR(ctx, a.computeMBUR); err != nil {
		return fmt.Errorf("setting MBUR: %w", err)
	}

	latest, err := a.gcpClient.GetService(ctx, &pb.GetServiceRequest{Name: created.GetName()})
	if err != nil {
		return fmt.Errorf("getting Service %q: %w", a.id.String(), err)
	}
	created = latest

	return a.updateStatus(ctx, createOp, created)
}

func (a *ServiceAdapter) Update(ctx context.Context, updateOp *directbase.UpdateOperation) error {
	log := klog.FromContext(ctx)
	log.V(2).Info("updating Service", "name", a.id.String())

	a.desired.Name = a.id.String()

	mburMatch, err := updateOp.CompareMBUR(a.computeMBUR)
	if err != nil {
		return fmt.Errorf("comparing MBUR: %w", err)
	}

	diffs, updateMask, err := compareService(ctx, a.actual, a.desired, mburMatch)
	if err != nil {
		return err
	}

	latest := a.actual
	if diffs.HasDiff() {
		structuredreporting.ReportDiff(ctx, diffs)

		req := &pb.UpdateServiceRequest{
			Service:    a.desired,
			UpdateMask: updateMask,
		}
		op, err := a.gcpClient.UpdateService(ctx, req)
		if err != nil {
			return fmt.Errorf("updating Service %s: %w", a.id.String(), err)
		}
		updated, err := op.Wait(ctx)
		if err != nil {
			return fmt.Errorf("waiting for update of Service %q: %w", a.id.String(), err)
		}
		log.V(2).Info("successfully updated Service", "name", a.id.String())
		latest = updated

		if err := updateOp.SetMBUR(ctx, a.computeMBUR); err != nil {
			return fmt.Errorf("setting MBUR: %w", err)
		}

		latestGet, err := a.gcpClient.GetService(ctx, &pb.GetServiceRequest{Name: latest.GetName()})
		if err != nil {
			return fmt.Errorf("getting Service %q: %w", a.id.String(), err)
		}
		latest = latestGet
	}

	return a.updateStatus(ctx, updateOp, latest)
}

func (a *ServiceAdapter) computeMBUR() (string, error) {
	if len(a.desired.CustomAudiences) == 0 {
		return "", nil
	}
	bytes, err := json.Marshal(a.desired.CustomAudiences)
	if err != nil {
		return "", fmt.Errorf("marshalling customAudiences: %w", err)
	}
	h := sha256.Sum256(bytes)
	return hex.EncodeToString(h[:]), nil
}

func compareService(ctx context.Context, actual, desired *pb.Service, mburMatch bool) (*structuredreporting.Diff, *fieldmaskpb.FieldMask, error) {
	maskedActual, err := mappers.OnlySpecFields(actual, RunServiceSpec_v1alpha1_FromProto, RunServiceSpec_v1alpha1_ToProto)
	if err != nil {
		return nil, nil, err
	}
	maskedActual.Name = desired.Name

	clonedDesired := proto.Clone(desired).(*pb.Service)

	populateDefaults := func(obj *pb.Service) {
		// Populate GCP/server defaults here if needed
	}
	populateDefaults(maskedActual)
	populateDefaults(clonedDesired)

	// If desired leaves optional fields unspecified, inherit existing actual values to avoid false diffs.
	if maskedActual != nil {
		if clonedDesired.LaunchStage == 0 && maskedActual.LaunchStage != 0 {
			clonedDesired.LaunchStage = maskedActual.LaunchStage
		}
		if clonedDesired.Scaling == nil && maskedActual.Scaling != nil {
			clonedDesired.Scaling = maskedActual.Scaling
		}
		// Written here: a service's server defaults.
		if clonedDesired.Ingress == 0 && maskedActual.Ingress != 0 {
			clonedDesired.Ingress = maskedActual.Ingress
		}
		if clonedDesired.Template != nil && maskedActual.Template != nil {
			if clonedDesired.Template.ServiceAccount == "" && maskedActual.Template.ServiceAccount != "" {
				clonedDesired.Template.ServiceAccount = maskedActual.Template.ServiceAccount
			}
			if clonedDesired.Template.Revision == "" && maskedActual.Template.Revision != "" {
				clonedDesired.Template.Revision = maskedActual.Template.Revision
			}
			// Written here: a service template's server defaults.
			if clonedDesired.Template.Timeout == nil {
				clonedDesired.Template.Timeout = maskedActual.Template.Timeout
			}
			if clonedDesired.Template.MaxInstanceRequestConcurrency == 0 {
				clonedDesired.Template.MaxInstanceRequestConcurrency = maskedActual.Template.MaxInstanceRequestConcurrency
			}
			if clonedDesired.Template.ExecutionEnvironment == 0 {
				clonedDesired.Template.ExecutionEnvironment = maskedActual.Template.ExecutionEnvironment
			}
			if clonedDesired.Template.Scaling == nil {
				clonedDesired.Template.Scaling = maskedActual.Template.Scaling
			}
			actualContainersByName := make(map[string]*pb.Container)
			for _, actCont := range maskedActual.Template.Containers {
				if actCont.GetName() != "" {
					actualContainersByName[actCont.GetName()] = actCont
				}
			}
			for i, desCont := range clonedDesired.Template.Containers {
				var actCont *pb.Container
				if desCont.GetName() != "" {
					actCont = actualContainersByName[desCont.GetName()]
				}
				if actCont == nil && i < len(maskedActual.Template.Containers) {
					actCont = maskedActual.Template.Containers[i]
				}
				if actCont != nil {
					if desCont.Resources == nil && actCont.Resources != nil {
						desCont.Resources = actCont.Resources
					} else if desCont.Resources != nil && actCont.Resources != nil {
						if desCont.Resources.Limits == nil && actCont.Resources.Limits != nil {
							desCont.Resources.Limits = actCont.Resources.Limits
						} else if desCont.Resources.Limits != nil && actCont.Resources.Limits != nil {
							for k, v := range actCont.Resources.Limits {
								if _, exists := desCont.Resources.Limits[k]; !exists {
									desCont.Resources.Limits[k] = v
								}
							}
						}
					}
				}
			}
		}
	}

	// CustomAudiences is mutable-but-unreadable (missing from the GET response in real GCP).
	// If the SHA-256 hash stored in the mutable-but-unreadable-fields-hash annotation matches desired state,
	// the user hasn't changed customAudiences, so we copy desired.CustomAudiences into maskedActual to suppress false diffs.
	if mburMatch {
		maskedActual.CustomAudiences = clonedDesired.CustomAudiences
	} else if len(clonedDesired.CustomAudiences) == 0 {
		// User cleared customAudiences (stored hash was non-empty, desired is now empty).
		// Populate a placeholder in maskedActual so DiffForTopLevelFields detects the removal.
		maskedActual.CustomAudiences = []string{"(unreadable)"}
	}

	diffs, updateMask, err := common.DiffForTopLevelFields(ctx, clonedDesired.ProtoReflect(), maskedActual.ProtoReflect())
	if err != nil {
		return nil, nil, err
	}
	return diffs, updateMask, nil
}

func (a *ServiceAdapter) updateStatus(ctx context.Context, op directbase.Operation, latest *pb.Service) error {
	mapCtx := &direct.MapContext{}
	status := &krm.RunServiceStatus{}
	status.ObservedState = RunServiceObservedState_v1alpha1_FromProto(mapCtx, latest)
	if mapCtx.Err() != nil {
		return mapCtx.Err()
	}
	status.ExternalRef = direct.LazyPtr(a.id.String())
	return op.UpdateStatus(ctx, status, nil)
}

func (a *ServiceAdapter) Export(ctx context.Context) (*unstructured.Unstructured, error) {
	if a.actual == nil {
		return nil, fmt.Errorf("Find() not called")
	}
	u := &unstructured.Unstructured{}

	obj := &krm.RunService{}
	mapCtx := &direct.MapContext{}
	obj.Spec = direct.ValueOf(RunServiceSpec_v1alpha1_FromProto(mapCtx, a.actual))
	if mapCtx.Err() != nil {
		return nil, mapCtx.Err()
	}

	obj.Spec.Location = &a.id.Location
	obj.Spec.ResourceID = direct.LazyPtr(a.id.Service)
	obj.Spec.ProjectRef = &refs.ProjectRef{External: a.id.Project}

	uObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
	if err != nil {
		return nil, err
	}

	u.Object = uObj
	u.SetName(a.id.Service)
	u.SetGroupVersionKind(krm.RunServiceGVK)

	return u, nil
}

func (a *ServiceAdapter) Delete(ctx context.Context, deleteOp *directbase.DeleteOperation) (bool, error) {
	log := klog.FromContext(ctx)
	log.V(2).Info("deleting Service", "name", a.id.String())

	req := &pb.DeleteServiceRequest{Name: a.id.String()}
	op, err := a.gcpClient.DeleteService(ctx, req)
	if err != nil {
		if direct.IsNotFound(err) {
			log.V(2).Info("skipping delete for non-existent Service, assuming it was already deleted", "name", a.id.String())
			return true, nil
		}
		return false, fmt.Errorf("deleting Service %s: %w", a.id.String(), err)
	}
	log.V(2).Info("successfully deleted Service", "name", a.id.String())

	if _, err = op.Wait(ctx); err != nil {
		return false, fmt.Errorf("waiting delete Service %s: %w", a.id.String(), err)
	}
	return true, nil
}
