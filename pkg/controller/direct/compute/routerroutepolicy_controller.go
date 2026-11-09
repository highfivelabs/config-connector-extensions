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

// Written here: no Config Connector code or pull request has a Cloud Router
// route policy. This controller follows Terraform's
// google_compute_router_route_policy (read, not copied): the policy is read
// with routers.getRoutePolicy, created and replaced whole with
// routers.updateRoutePolicy (carrying the fingerprint it read), and deleted
// with routers.deleteRoutePolicy; type, description and terms change in
// place.

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	gcp "cloud.google.com/go/compute/apiv1"
	pb "cloud.google.com/go/compute/apiv1/computepb"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/config"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct/common"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct/directbase"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct/registry"
	"github.com/googleapis/gax-go/v2/apierror"
	"google.golang.org/protobuf/proto"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"

	krm "github.com/highfivelabs/config-connector-extensions/apis/compute/v1alpha1"
)

func init() {
	registry.RegisterModel(krm.ComputeRouterRoutePolicyGVK, NewRouterRoutePolicyModel)
}

func NewRouterRoutePolicyModel(ctx context.Context, config *config.ControllerConfig) (directbase.Model, error) {
	return &routePolicyModel{config: *config}, nil
}

type routePolicyModel struct {
	config config.ControllerConfig
}

var _ directbase.Model = &routePolicyModel{}

func (m *routePolicyModel) AdapterForObject(ctx context.Context, op *directbase.AdapterForObjectOperation) (directbase.Adapter, error) {
	obj := &krm.ComputeRouterRoutePolicy{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(op.GetUnstructured().Object, obj); err != nil {
		return nil, fmt.Errorf("error converting to %T: %w", obj, err)
	}
	if err := common.NormalizeReferences(ctx, op.Reader, obj, nil); err != nil {
		return nil, fmt.Errorf("normalizing references: %w", err)
	}
	id, err := obj.GetIdentity(ctx, op.Reader)
	if err != nil {
		return nil, err
	}
	opts, err := m.config.RESTClientOptions()
	if err != nil {
		return nil, err
	}
	c, err := gcp.NewRoutersRESTClient(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("building routers client: %w", err)
	}
	return &routePolicyAdapter{id: id.(*krm.ComputeRouterRoutePolicyIdentity), gcpClient: c, desired: obj}, nil
}

func (m *routePolicyModel) AdapterForURL(ctx context.Context, url string) (directbase.Adapter, error) {
	return nil, nil
}

type routePolicyAdapter struct {
	id        *krm.ComputeRouterRoutePolicyIdentity
	gcpClient *gcp.RoutersClient
	desired   *krm.ComputeRouterRoutePolicy
	actual    *pb.RoutePolicy
}

var _ directbase.Adapter = &routePolicyAdapter{}

func expr(in *krm.RoutePolicyExpr) *pb.Expr {
	if in == nil {
		return nil
	}
	return &pb.Expr{Expression: direct.PtrTo(in.Expression), Title: in.Title, Description: in.Description, Location: in.Location}
}

// policy is the desired policy as the API takes it.
func (a *routePolicyAdapter) policy() *pb.RoutePolicy {
	s := a.desired.Spec
	out := &pb.RoutePolicy{Name: direct.PtrTo(a.id.Policy), Type: s.Type, Description: s.Description}
	for _, t := range s.Terms {
		term := &pb.RoutePolicyPolicyTerm{Priority: direct.PtrTo(t.Priority), Match: expr(t.Match)}
		for i := range t.Actions {
			term.Actions = append(term.Actions, expr(&t.Actions[i]))
		}
		out.Terms = append(out.Terms, term)
	}
	return out
}

func (a *routePolicyAdapter) Find(ctx context.Context) (bool, error) {
	resp, err := a.gcpClient.GetRoutePolicy(ctx, &pb.GetRoutePolicyRouterRequest{
		Project: a.id.Project, Region: a.id.Region, Router: a.id.Router, Policy: direct.PtrTo(a.id.Policy),
	})
	if err != nil {
		if routePolicyMissing(err) {
			return false, nil
		}
		return false, fmt.Errorf("getting ComputeRouterRoutePolicy %s: %w", a.id, err)
	}
	a.actual = resp.GetResource()
	return true, nil
}

func (a *routePolicyAdapter) write(ctx context.Context, desired *pb.RoutePolicy) error {
	op, err := a.gcpClient.UpdateRoutePolicy(ctx, &pb.UpdateRoutePolicyRouterRequest{
		Project: a.id.Project, Region: a.id.Region, Router: a.id.Router, RoutePolicyResource: desired,
	})
	if err != nil {
		return fmt.Errorf("writing ComputeRouterRoutePolicy %s: %w", a.id, err)
	}
	if err := op.Wait(ctx); err != nil {
		return fmt.Errorf("writing ComputeRouterRoutePolicy %s: %w", a.id, err)
	}
	return nil
}

func (a *routePolicyAdapter) Create(ctx context.Context, createOp *directbase.CreateOperation) error {
	klog.FromContext(ctx).V(2).Info("creating ComputeRouterRoutePolicy", "name", a.id)
	if err := a.write(ctx, a.policy()); err != nil {
		return err
	}
	return a.readAndUpdateStatus(ctx, createOp)
}

func (a *routePolicyAdapter) Update(ctx context.Context, updateOp *directbase.UpdateOperation) error {
	desired := a.policy()
	actual := proto.Clone(a.actual).(*pb.RoutePolicy)
	actual.Fingerprint = nil
	if proto.Equal(desired, actual) {
		return a.updateStatusFrom(ctx, updateOp, a.actual)
	}
	klog.FromContext(ctx).V(2).Info("replacing ComputeRouterRoutePolicy", "name", a.id)
	desired.Fingerprint = a.actual.Fingerprint
	if err := a.write(ctx, desired); err != nil {
		return err
	}
	return a.readAndUpdateStatus(ctx, updateOp)
}

func (a *routePolicyAdapter) Delete(ctx context.Context, deleteOp *directbase.DeleteOperation) (bool, error) {
	klog.FromContext(ctx).V(2).Info("deleting ComputeRouterRoutePolicy", "name", a.id)
	op, err := a.gcpClient.DeleteRoutePolicy(ctx, &pb.DeleteRoutePolicyRouterRequest{
		Project: a.id.Project, Region: a.id.Region, Router: a.id.Router, Policy: direct.PtrTo(a.id.Policy),
	})
	if err != nil {
		if routePolicyMissing(err) {
			return true, nil
		}
		return false, fmt.Errorf("deleting ComputeRouterRoutePolicy %s: %w", a.id, err)
	}
	if err := op.Wait(ctx); err != nil {
		return false, fmt.Errorf("deleting ComputeRouterRoutePolicy %s: %w", a.id, err)
	}
	return true, nil
}

func (a *routePolicyAdapter) Export(ctx context.Context) (*unstructured.Unstructured, error) {
	return nil, nil
}

func (a *routePolicyAdapter) readAndUpdateStatus(ctx context.Context, op directbase.Operation) error {
	if _, err := a.Find(ctx); err != nil {
		return err
	}
	return a.updateStatusFrom(ctx, op, a.actual)
}

func (a *routePolicyAdapter) updateStatusFrom(ctx context.Context, op directbase.Operation, actual *pb.RoutePolicy) error {
	return op.UpdateStatus(ctx, &krm.ComputeRouterRoutePolicyStatus{
		ExternalRef:   direct.LazyPtr(a.id.String()),
		ObservedState: &krm.ComputeRouterRoutePolicyObservedState{Fingerprint: direct.LazyPtr(actual.GetFingerprint())},
	}, nil)
}

// routePolicyMissing reports whether err says the route policy does not
// exist. For a router's missing policy the API answers 400, "Invalid value
// for field 'policy': '<name>'. The policy does not exist", not 404 (live
// run cce-11091509-3d77, docs/live-runs.md); a missing router is a 404.
func routePolicyMissing(err error) bool {
	if direct.IsNotFound(err) {
		return true
	}
	var ae *apierror.APIError
	return errors.As(err, &ae) && ae.HTTPCode() == http.StatusBadRequest && strings.Contains(err.Error(), "The policy does not exist")
}
