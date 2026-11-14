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

// Package servicemanagement has the direct controller for EndpointsService,
// a Cloud Endpoints service. Config Connector has no Service Management
// code to copy (UPSTREAM.md), so it is written here on Config Connector's
// direct reconciler (directbase), shaped after serviceusage's
// ServiceIdentity controller, with the API's Go client
// (google.golang.org/api/servicemanagement/v1). It does what Terraform's
// google_endpoints_service does: create the managed service (or undelete
// it, when it was deleted within the API's 30 days), submit the
// configuration files, and roll the new configuration out to all traffic.
// A configuration is submitted again only when the files change (a digest
// in status) or another configuration was rolled out out of band.
package servicemanagement

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	refsv1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/apis/refs/v1beta1"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/config"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct/common"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct/directbase"
	gcp "google.golang.org/api/servicemanagement/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"

	krm "github.com/highfivelabs/config-connector-extensions/apis/servicemanagement/v1alpha1"
)

// NewEndpointsServiceModel is the direct reconciler's model.
func NewEndpointsServiceModel(ctx context.Context, cfg *config.ControllerConfig) (directbase.Model, error) {
	return &endpointsServiceModel{config: *cfg}, nil
}

type endpointsServiceModel struct {
	config config.ControllerConfig
}

var _ directbase.Model = &endpointsServiceModel{}

func (m *endpointsServiceModel) AdapterForObject(ctx context.Context, op *directbase.AdapterForObjectOperation) (directbase.Adapter, error) {
	u := op.GetUnstructured()
	obj := &krm.EndpointsService{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, obj); err != nil {
		return nil, fmt.Errorf("error converting to %T: %w", obj, err)
	}
	project, err := refsv1beta1.ResolveProject(ctx, op.Reader, obj.GetNamespace(), obj.Spec.ProjectRef)
	if err != nil {
		return nil, err
	}
	name := obj.GetName()
	if obj.Spec.ResourceID != nil && *obj.Spec.ResourceID != "" {
		name = *obj.Spec.ResourceID
	}
	ref := "services/" + name
	if ext := obj.Status.ExternalRef; ext != nil && *ext != "" && *ext != ref {
		return nil, fmt.Errorf("cannot change EndpointsService identity (old=%q, new=%q)", *ext, ref)
	}
	files, err := configFiles(&obj.Spec)
	if err != nil {
		return nil, err
	}
	opts, err := m.config.RESTClientOptions()
	if err != nil {
		return nil, err
	}
	client, err := gcp.NewService(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("building servicemanagement v1 client: %w", err)
	}
	return &endpointsServiceAdapter{
		client:  client,
		name:    name,
		project: project.ProjectID,
		files:   files,
		digest:  digest(files),
		desired: obj,
	}, nil
}

func (m *endpointsServiceModel) AdapterForURL(ctx context.Context, url string) (directbase.Adapter, error) {
	return nil, nil
}

// configFiles are the files the spec submits, as Terraform's
// google_endpoints_service sends them.
func configFiles(spec *krm.EndpointsServiceSpec) ([]*gcp.ConfigFile, error) {
	b64 := func(s string) string { return base64.StdEncoding.EncodeToString([]byte(s)) }
	switch {
	case spec.OpenAPIConfig != nil && spec.GRPCConfig == nil:
		fileType, path := "OPEN_API_YAML", "openapi.yaml"
		if strings.HasPrefix(strings.TrimSpace(*spec.OpenAPIConfig), "{") {
			fileType, path = "OPEN_API_JSON", "openapi.json"
		}
		return []*gcp.ConfigFile{{FilePath: path, FileType: fileType, FileContents: b64(*spec.OpenAPIConfig)}}, nil
	case spec.GRPCConfig != nil && spec.OpenAPIConfig == nil && spec.ProtocOutputBase64 != nil:
		return []*gcp.ConfigFile{
			{FilePath: "service.yaml", FileType: "SERVICE_CONFIG_YAML", FileContents: b64(*spec.GRPCConfig)},
			{FilePath: "api_descriptor.pb", FileType: "FILE_DESCRIPTOR_SET_PROTO", FileContents: *spec.ProtocOutputBase64},
		}, nil
	}
	return nil, fmt.Errorf("exactly one of spec.openapiConfig and spec.grpcConfig (with spec.protocOutputBase64) is required")
}

// digest is the SHA-256 of the files' types, paths and contents.
func digest(files []*gcp.ConfigFile) string {
	h := sha256.New()
	for _, f := range files {
		fmt.Fprintf(h, "%s\x00%s\x00%s\x00", f.FileType, f.FilePath, f.FileContents)
	}
	return hex.EncodeToString(h.Sum(nil))
}

type endpointsServiceAdapter struct {
	client  *gcp.APIService
	name    string
	project string
	files   []*gcp.ConfigFile
	digest  string
	desired *krm.EndpointsService
	actual  *gcp.ManagedService
}

var _ directbase.Adapter = &endpointsServiceAdapter{}

// notFound: the API answers a get of a service that does not exist, or
// that the caller cannot see, with 404 or 403 ("does not exist or you do
// not have permission"); Terraform takes any error as "create it". A 403
// that really is a missing permission fails at the create instead.
func notFound(err error) bool {
	return direct.IsNotFound(err) || direct.HasHTTPCode(err, http.StatusForbidden)
}

func (a *endpointsServiceAdapter) Find(ctx context.Context) (bool, error) {
	got, err := a.client.Services.Get(a.name).Context(ctx).Do()
	if err != nil {
		if notFound(err) {
			return false, nil
		}
		return false, fmt.Errorf("getting service %s: %w", a.name, err)
	}
	if got.ProducerProjectId != "" && got.ProducerProjectId != a.project {
		return false, fmt.Errorf("service %s belongs to project %s, not spec.projectRef's %s", a.name, got.ProducerProjectId, a.project)
	}
	a.actual = got
	return true, nil
}

func (a *endpointsServiceAdapter) Create(ctx context.Context, op *directbase.CreateOperation) error {
	log := klog.FromContext(ctx)
	log.V(2).Info("creating", "service", a.name)
	created, err := a.client.Services.Create(&gcp.ManagedService{ServiceName: a.name, ProducerProjectId: a.project}).Context(ctx).Do()
	if err != nil && direct.IsAlreadyExists(err) {
		// Deleted less than 30 days ago: the name is held until then, and
		// undelete brings the service back.
		log.V(2).Info("undeleting", "service", a.name)
		created, err = a.client.Services.Undelete(a.name).Context(ctx).Do()
	}
	if err != nil {
		return fmt.Errorf("creating service %s: %w", a.name, err)
	}
	if _, err := a.wait(ctx, created); err != nil {
		return fmt.Errorf("creating service %s: %w", a.name, err)
	}
	if err := a.submitAndRollOut(ctx); err != nil {
		return err
	}
	return a.updateStatus(ctx, op)
}

func (a *endpointsServiceAdapter) Update(ctx context.Context, op *directbase.UpdateOperation) error {
	active, err := a.activeConfig(ctx)
	if err != nil {
		return err
	}
	st := a.desired.Status
	var statusConfigID string
	if st.ObservedState != nil && st.ObservedState.ConfigID != nil {
		statusConfigID = *st.ObservedState.ConfigID
	}
	switch {
	case active == nil:
		klog.FromContext(ctx).V(2).Info("no configuration rolled out", "service", a.name)
	case st.ConfigDigest == nil || *st.ConfigDigest != a.digest:
		klog.FromContext(ctx).V(2).Info("configuration files changed", "service", a.name)
	case active.Id != statusConfigID:
		klog.FromContext(ctx).V(2).Info("another configuration was rolled out", "service", a.name, "active", active.Id, "ours", statusConfigID)
	default:
		return a.updateStatus(ctx, op)
	}
	if err := a.submitAndRollOut(ctx); err != nil {
		return err
	}
	return a.updateStatus(ctx, op)
}

// submitAndRollOut submits the spec's files as a new configuration and
// rolls it out to all traffic.
func (a *endpointsServiceAdapter) submitAndRollOut(ctx context.Context) error {
	submitted, err := a.client.Services.Configs.Submit(a.name, &gcp.SubmitConfigSourceRequest{
		ConfigSource: &gcp.ConfigSource{Files: a.files},
	}).Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("submitting the configuration of service %s: %w", a.name, err)
	}
	done, err := a.wait(ctx, submitted)
	if err != nil {
		return fmt.Errorf("submitting the configuration of service %s: %w", a.name, err)
	}
	resp := &gcp.SubmitConfigSourceResponse{}
	if err := json.Unmarshal(done.Response, resp); err != nil {
		return fmt.Errorf("reading the submitted configuration of service %s: %w", a.name, err)
	}
	if resp.ServiceConfig == nil || resp.ServiceConfig.Id == "" {
		return fmt.Errorf("submitting the configuration of service %s: the answer has no configuration ID", a.name)
	}
	id := resp.ServiceConfig.Id
	klog.FromContext(ctx).V(2).Info("rolling out", "service", a.name, "config", id)
	rollout, err := a.client.Services.Rollouts.Create(a.name, &gcp.Rollout{
		ServiceName:            a.name,
		TrafficPercentStrategy: &gcp.TrafficPercentStrategy{Percentages: map[string]float64{id: 100}},
	}).Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("rolling out configuration %s of service %s: %w", id, a.name, err)
	}
	if _, err := a.wait(ctx, rollout); err != nil {
		return fmt.Errorf("rolling out configuration %s of service %s: %w", id, a.name, err)
	}
	return nil
}

// activeConfig is the configuration the service serves, nil when none was
// rolled out yet.
func (a *endpointsServiceAdapter) activeConfig(ctx context.Context) (*gcp.Service, error) {
	got, err := a.client.Services.GetConfig(a.name).Context(ctx).Do()
	if err != nil {
		if direct.IsNotFound(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("getting the configuration of service %s: %w", a.name, err)
	}
	return got, nil
}

func (a *endpointsServiceAdapter) updateStatus(ctx context.Context, op directbase.Operation) error {
	active, err := a.activeConfig(ctx)
	if err != nil {
		return err
	}
	if active == nil {
		return fmt.Errorf("service %s has no configuration rolled out", a.name)
	}
	ref := "services/" + a.name
	digest := a.digest
	status := &krm.EndpointsServiceStatus{
		ExternalRef:   &ref,
		ConfigDigest:  &digest,
		ObservedState: observedState(active),
	}
	return op.UpdateStatus(ctx, status, nil)
}

func strPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func observedState(c *gcp.Service) *krm.EndpointsServiceObservedState {
	out := &krm.EndpointsServiceObservedState{ConfigID: strPtr(c.Id), DNSAddress: strPtr(c.Name)}
	for _, api := range c.Apis {
		o := krm.EndpointsServiceAPI{Name: strPtr(api.Name), Syntax: strPtr(api.Syntax), Version: strPtr(api.Version)}
		for _, m := range api.Methods {
			o.Methods = append(o.Methods, krm.EndpointsServiceMethod{
				Name: strPtr(m.Name), Syntax: strPtr(m.Syntax),
				RequestType: strPtr(m.RequestTypeUrl), ResponseType: strPtr(m.ResponseTypeUrl),
			})
		}
		out.APIs = append(out.APIs, o)
	}
	for _, e := range c.Endpoints {
		out.Endpoints = append(out.Endpoints, krm.EndpointsServiceEndpoint{Name: strPtr(e.Name), Address: strPtr(e.Target)})
	}
	return out
}

func (a *endpointsServiceAdapter) Export(ctx context.Context) (*unstructured.Unstructured, error) {
	return nil, fmt.Errorf("EndpointsService does not support export")
}

// Delete deletes the service. The API keeps a deleted service for 30 days,
// in which its name cannot be used again but undelete brings it back
// (Create does).
func (a *endpointsServiceAdapter) Delete(ctx context.Context, op *directbase.DeleteOperation) (bool, error) {
	deleted, err := a.client.Services.Delete(a.name).Context(ctx).Do()
	if err != nil {
		if direct.IsNotFound(err) {
			return true, nil
		}
		return false, fmt.Errorf("deleting service %s: %w", a.name, err)
	}
	if _, err := a.wait(ctx, deleted); err != nil && !direct.IsNotFound(err) {
		return false, fmt.Errorf("deleting service %s: %w", a.name, err)
	}
	return true, nil
}

func (a *endpointsServiceAdapter) wait(ctx context.Context, op *gcp.Operation) (*gcp.Operation, error) {
	if op.Done {
		if op.Error != nil {
			return nil, fmt.Errorf("operation %q failed: %s", op.Name, op.Error.Message)
		}
		return op, nil
	}
	return common.WaitForOperation(ctx, 2*time.Second, func(current *gcp.Operation) (bool, error) {
		if !current.Done {
			return false, nil
		}
		if current.Error != nil {
			return true, fmt.Errorf("operation %q failed: %s", op.Name, current.Error.Message)
		}
		return true, nil
	}, func() (*gcp.Operation, error) {
		current, err := a.client.Operations.Get(op.Name).Context(ctx).Do()
		if err != nil {
			return nil, fmt.Errorf("getting operation %q: %w", op.Name, err)
		}
		return current, nil
	})
}
