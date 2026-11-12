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
// pkg/controller/direct/aiplatform/aiplatformreasoningengine_controller.go (UPSTREAM.md).
// Changed: our KRM types (stable, aiplatform.gcp.hfl.dev/v1beta1, docs/design.md
// decision 26); the model is returned to our manager, not registered with
// Config Connector's registry; the API is reached over its JSON REST surface
// (pkg/discokind's API, with the protos as JSON), not the gRPC client: the
// Vertex AI Go client has no REST transport for reasoning engines, and our
// manager reaches Google Cloud, and the fake cloud, through the controller
// config's HTTP client; Export is not supported (no kind here exports).

package aiplatform

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	pb "cloud.google.com/go/aiplatform/apiv1/aiplatformpb"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/config"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct/common"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct/directbase"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/structuredreporting"
	"google.golang.org/api/googleapi"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"

	krm "github.com/highfivelabs/config-connector-extensions/apis/aiplatform/v1beta1"
	"github.com/highfivelabs/config-connector-extensions/pkg/discokind"
)

func NewAIPlatformReasoningEngineModel(ctx context.Context, config *config.ControllerConfig) (directbase.Model, error) {
	return &aiplatformReasoningEngineModel{config: config}, nil
}

var _ directbase.Model = &aiplatformReasoningEngineModel{}

type aiplatformReasoningEngineModel struct {
	config *config.ControllerConfig
}

// api is Vertex AI's regional REST endpoint for location.
func (m *aiplatformReasoningEngineModel) api(ctx context.Context, location string) (*discokind.API, error) {
	httpClient, err := m.config.NewAuthenticatedHTTPClient(ctx)
	if err != nil {
		return nil, err
	}
	return &discokind.API{HTTP: httpClient, BaseURL: fmt.Sprintf("https://%s-aiplatform.googleapis.com/v1/", location)}, nil
}

func (m *aiplatformReasoningEngineModel) AdapterForObject(ctx context.Context, op *directbase.AdapterForObjectOperation) (directbase.Adapter, error) {
	u := op.GetUnstructured()
	reader := op.Reader
	obj := &krm.AIPlatformReasoningEngine{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, &obj); err != nil {
		return nil, fmt.Errorf("error converting to %T: %w", obj, err)
	}

	id, err := obj.GetIdentity(ctx, reader)
	if err != nil {
		return nil, err
	}

	// Always call common.NormalizeReferences to resolve any resource references
	if err := common.NormalizeReferences(ctx, reader, obj, nil); err != nil {
		return nil, fmt.Errorf("normalizing references: %w", err)
	}

	typedID, ok := id.(*krm.AIPlatformReasoningEngineIdentity)
	if !ok {
		return nil, fmt.Errorf("expected AIPlatformReasoningEngineIdentity, got %T", id)
	}

	api, err := m.api(ctx, typedID.Location)
	if err != nil {
		return nil, err
	}

	mapCtx := &direct.MapContext{}
	desiredpb := AIPlatformReasoningEngineSpec_ToProto(mapCtx, &obj.Spec)
	if mapCtx.Err() != nil {
		return nil, fmt.Errorf("mapping spec to proto: %w", mapCtx.Err())
	}

	return &AIPlatformReasoningEngineAdapter{
		id:        typedID,
		api:       api,
		desiredpb: desiredpb,
		desired:   obj,
	}, nil
}

// AdapterForURL is not supported: no kind here is exported.
func (m *aiplatformReasoningEngineModel) AdapterForURL(ctx context.Context, url string) (directbase.Adapter, error) {
	return nil, nil
}

type AIPlatformReasoningEngineAdapter struct {
	id        *krm.AIPlatformReasoningEngineIdentity
	api       *discokind.API
	desiredpb *pb.ReasoningEngine
	desired   *krm.AIPlatformReasoningEngine
	actual    *pb.ReasoningEngine
}

var _ directbase.Adapter = &AIPlatformReasoningEngineAdapter{}

// toJSON is a message as the REST API takes it.
func toJSON(m proto.Message) (map[string]any, error) {
	b, err := protojson.Marshal(m)
	if err != nil {
		return nil, err
	}
	out := map[string]any{}
	if err := json.Unmarshal(b, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// fromJSON reads a REST answer into m, ignoring fields newer than the pinned
// protos.
func fromJSON(in map[string]any, m proto.Message) error {
	b, err := json.Marshal(in)
	if err != nil {
		return err
	}
	return protojson.UnmarshalOptions{DiscardUnknown: true}.Unmarshal(b, m)
}

func isNotFound(err error) bool {
	if e, ok := err.(*googleapi.Error); ok {
		return e.Code == http.StatusNotFound
	}
	return direct.IsNotFound(err)
}

func (a *AIPlatformReasoningEngineAdapter) get(ctx context.Context) (*pb.ReasoningEngine, error) {
	got, err := a.api.Get(ctx, a.id.String())
	if err != nil {
		return nil, err
	}
	out := &pb.ReasoningEngine{}
	if err := fromJSON(got, out); err != nil {
		return nil, fmt.Errorf("decoding AIPlatformReasoningEngine %q: %w", a.id.String(), err)
	}
	return out, nil
}

func (a *AIPlatformReasoningEngineAdapter) Find(ctx context.Context) (bool, error) {
	// Rule 2: Find() Pre-Check Guard for service-generated ID
	if a.id.ReasoningEngine == "" {
		return false, nil
	}

	log := klog.FromContext(ctx)
	log.V(2).Info("getting AIPlatformReasoningEngine", "name", a.id.String())

	reasoningEnginepb, err := a.get(ctx)
	if err != nil {
		if isNotFound(err) {
			return false, nil
		}
		return false, fmt.Errorf("getting AIPlatformReasoningEngine %q: %w", a.id.String(), err)
	}

	a.actual = reasoningEnginepb

	return true, nil
}

func (a *AIPlatformReasoningEngineAdapter) Create(ctx context.Context, createOp *directbase.CreateOperation) error {
	log := klog.FromContext(ctx)
	log.V(2).Info("creating AIPlatformReasoningEngine", "parent", a.id.ParentString())

	// Rule 3: Do not set desiredpb.Name, let GCP generate the identifier.
	a.desiredpb.Name = ""

	body, err := toJSON(a.desiredpb)
	if err != nil {
		return err
	}
	op, err := a.api.Do(ctx, http.MethodPost, a.id.ParentString()+"/reasoningEngines", nil, body)
	if err != nil {
		return fmt.Errorf("creating AIPlatformReasoningEngine in %q: %w", a.id.ParentString(), err)
	}

	log.V(2).Info("successfully started creation of AIPlatformReasoningEngine", "parent", a.id.ParentString())

	createdJSON, err := a.api.Wait(ctx, op)
	if err != nil {
		return fmt.Errorf("waiting for AIPlatformReasoningEngine creation in %q: %w", a.id.ParentString(), err)
	}
	created := &pb.ReasoningEngine{}
	if err := fromJSON(createdJSON, created); err != nil {
		return fmt.Errorf("decoding created AIPlatformReasoningEngine: %w", err)
	}

	if created.Name == "" {
		return fmt.Errorf("creating AIPlatformReasoningEngine in %q: the operation returned no name", a.id.ParentString())
	}
	tempID := &krm.AIPlatformReasoningEngineIdentity{}
	if err := tempID.FromExternal(created.Name); err != nil {
		return fmt.Errorf("parsing created AIPlatformReasoningEngine name %q: %w", created.Name, err)
	}
	a.id.ReasoningEngine = tempID.ReasoningEngine

	log.V(2).Info("successfully completed creation of AIPlatformReasoningEngine", "name", a.id.String())

	// Fetch fully-populated resource immediately after LRO success
	latest, err := a.get(ctx)
	if err != nil {
		return fmt.Errorf("fetching newly created AIPlatformReasoningEngine %q: %w", a.id.String(), err)
	}

	return a.updateStatus(ctx, createOp, latest)
}

func (a *AIPlatformReasoningEngineAdapter) Update(ctx context.Context, updateOp *directbase.UpdateOperation) error {
	log := klog.FromContext(ctx)
	log.V(2).Info("updating AIPlatformReasoningEngine", "name", a.id.String())

	diffs, updateMask, err := common.CompareBrownfieldSpec(
		ctx,
		&a.desired.Spec,
		a.actual,
		AIPlatformReasoningEngineSpec_FromProto,
		AIPlatformReasoningEngineSpec_ToProto,
		nil,
	)
	if err != nil {
		return err
	}

	if !diffs.HasDiff() {
		log.V(2).Info("no field needs update", "name", a.id.String())
		return a.updateStatus(ctx, updateOp, a.actual)
	}

	diffs.Object = updateOp.GetUnstructured()
	structuredreporting.ReportDiff(ctx, diffs)

	a.desiredpb.Name = a.id.String()
	body, err := toJSON(a.desiredpb)
	if err != nil {
		return err
	}
	op, err := a.api.Do(ctx, http.MethodPatch, a.id.String(), map[string]string{"updateMask": strings.Join(updateMask.GetPaths(), ",")}, body)
	if err != nil {
		return fmt.Errorf("updating AIPlatformReasoningEngine %q: %w", a.id.String(), err)
	}

	log.V(2).Info("successfully started update of AIPlatformReasoningEngine", "name", a.id.String())

	if _, err := a.api.Wait(ctx, op); err != nil {
		return fmt.Errorf("waiting for AIPlatformReasoningEngine %q update: %w", a.id.String(), err)
	}

	log.V(2).Info("successfully completed update of AIPlatformReasoningEngine", "name", a.id.String())

	latest, err := a.get(ctx)
	if err != nil {
		return fmt.Errorf("fetching updated AIPlatformReasoningEngine %q: %w", a.id.String(), err)
	}

	return a.updateStatus(ctx, updateOp, latest)
}

// CloudLabels are the engine's labels as read by Find, for the slot guard
// (pkg/slotguard).
func (a *AIPlatformReasoningEngineAdapter) CloudLabels() map[string]string {
	return a.actual.GetLabels()
}

func (a *AIPlatformReasoningEngineAdapter) Export(ctx context.Context) (*unstructured.Unstructured, error) {
	return nil, fmt.Errorf("export of AIPlatformReasoningEngine is not supported")
}

func (a *AIPlatformReasoningEngineAdapter) updateStatus(ctx context.Context, op directbase.Operation, latest *pb.ReasoningEngine) error {
	mapCtx := &direct.MapContext{}
	status := &krm.AIPlatformReasoningEngineStatus{}
	status.ObservedState = AIPlatformReasoningEngineObservedState_FromProto(mapCtx, latest)
	if mapCtx.Err() != nil {
		return fmt.Errorf("mapping status: %w", mapCtx.Err())
	}
	status.ExternalRef = direct.LazyPtr(a.id.String())
	return op.UpdateStatus(ctx, status, nil)
}

func (a *AIPlatformReasoningEngineAdapter) Delete(ctx context.Context, deleteOp *directbase.DeleteOperation) (bool, error) {
	log := klog.FromContext(ctx)
	log.V(2).Info("deleting AIPlatformReasoningEngine", "name", a.id.String())

	op, err := a.api.Do(ctx, http.MethodDelete, a.id.String(), nil, nil)
	if err != nil {
		if isNotFound(err) {
			return true, nil
		}
		return false, fmt.Errorf("deleting AIPlatformReasoningEngine %q: %w", a.id.String(), err)
	}

	if _, err := a.api.Wait(ctx, op); err != nil {
		return false, fmt.Errorf("waiting for deletion of AIPlatformReasoningEngine %q: %w", a.id.String(), err)
	}

	return true, nil
}
