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

// Written here: ComputeSecurityPolicyFields, a field kind (docs/design.md
// section 6.6, pattern 2; decision 37). No Config Connector code or pull
// request sets a Cloud Armor policy's Adaptive Protection threshold
// configs or its WAF request body inspection size. The policy is read
// with securityPolicies.get and written with securityPolicies.patch, a
// JSON merge patch whose body holds the policy's fingerprint and only the
// sub-objects with an owned field that differs, as read with that field
// changed; a field is cleared with an update mask naming it, since a
// merge patch cannot remove one. Config Connector's ComputeSecurityPolicy
// runs Terraform by default, whose update sends only the sub-objects it
// changed (adaptiveProtectionConfig with its enable, ruleVisibility and
// autoDeployConfig; advancedOptionsConfig with its own fields), with no
// mask: a merge keeps ours, but if the API replaces a sub-object it is
// sent, ours come back at the next reconcile (the partial case of section
// 6.6; to be seen live). Its opt-in direct controller sends its whole
// object and reads a threshold config with auto-deploy fields as its own
// autoDeployConfig, so this kind is not for a policy on the direct
// controller.

import (
	"context"
	"fmt"
	"slices"
	"strings"

	gcp "cloud.google.com/go/compute/apiv1"
	pb "cloud.google.com/go/compute/apiv1/computepb"
	computev1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/apis/compute/v1beta1"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/config"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct/directbase"
	"google.golang.org/protobuf/proto"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"

	krm "github.com/highfivelabs/config-connector-extensions/apis/compute/v1alpha1"
	"github.com/highfivelabs/config-connector-extensions/pkg/fieldkind"
)

// NewSecurityPolicyFieldsModel is the ComputeSecurityPolicyFields model.
func NewSecurityPolicyFieldsModel(ctx context.Context, config *config.ControllerConfig) (directbase.Model, error) {
	return &securityPolicyFieldsModel{config: *config}, nil
}

type securityPolicyFieldsModel struct {
	config config.ControllerConfig
}

var _ directbase.Model = &securityPolicyFieldsModel{}

// The update mask paths this kind sends, only to clear a field.
const (
	thresholdConfigsPath          = "adaptiveProtectionConfig.layer7DdosDefenseConfig.thresholdConfigs"
	requestBodyInspectionSizePath = "advancedOptionsConfig.requestBodyInspectionSize"
)

func (m *securityPolicyFieldsModel) AdapterForObject(ctx context.Context, op *directbase.AdapterForObjectOperation) (directbase.Adapter, error) {
	obj := &krm.ComputeSecurityPolicyFields{}
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
		// Deleting: the policy this object set is in its status; Config
		// Connector's ComputeSecurityPolicy may be gone already.
		ref = &computev1beta1.ComputeSecurityPolicyRef{External: prev}
	} else {
		r := *ref
		if err := r.Normalize(ctx, op.Reader, obj.GetNamespace()); err != nil {
			return nil, err
		}
		ref = &r
	}
	id := &computev1beta1.ComputeSecurityPolicyIdentity{}
	if err := id.FromExternal(ref.External); err != nil {
		return nil, err
	}
	if !id.IsGlobal() {
		return nil, fmt.Errorf("security policy %s is regional: ComputeSecurityPolicyFields sets a global policy's fields only", id)
	}
	if prev != "" && prev != id.String() {
		return nil, fmt.Errorf("cannot change the security policy of ComputeSecurityPolicyFields (old=%q, new=%q)", prev, id.String())
	}
	if err := fieldkind.CheckSoleOwner(ctx, op.Reader, krm.ComputeSecurityPolicyFieldsGVK, obj, id.String()); err != nil {
		return nil, err
	}
	opts, err := m.config.RESTClientOptions()
	if err != nil {
		return nil, err
	}
	c, err := gcp.NewSecurityPoliciesRESTClient(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("building SecurityPolicies client: %w", err)
	}
	return &securityPolicyFieldsAdapter{id: id, gcpClient: c, spec: &obj.Spec}, nil
}

func (m *securityPolicyFieldsModel) AdapterForURL(ctx context.Context, url string) (directbase.Adapter, error) {
	return nil, nil
}

type securityPolicyFieldsAdapter struct {
	id        *computev1beta1.ComputeSecurityPolicyIdentity
	gcpClient *gcp.SecurityPoliciesClient
	spec      *krm.ComputeSecurityPolicyFieldsSpec
	actual    *pb.SecurityPolicy
}

var _ directbase.Adapter = &securityPolicyFieldsAdapter{}

func (a *securityPolicyFieldsAdapter) ownsThresholds() bool {
	return a.spec.AdaptiveProtectionConfig != nil && a.spec.AdaptiveProtectionConfig.Layer7DdosDefenseConfig != nil
}

func (a *securityPolicyFieldsAdapter) ownsInspectionSize() bool {
	return a.spec.AdvancedOptionsConfig != nil
}

// Find reports whether the policy exists. A missing policy is "not
// found" so that a delete finishes when it is gone; a create then waits
// (Create).
func (a *securityPolicyFieldsAdapter) Find(ctx context.Context) (bool, error) {
	p, err := a.gcpClient.Get(ctx, &pb.GetSecurityPolicyRequest{Project: a.id.Project, SecurityPolicy: a.id.Name})
	if err != nil {
		if direct.IsNotFound(err) {
			return false, nil
		}
		return false, fmt.Errorf("getting security policy %s: %w", a.id, err)
	}
	a.actual = p
	return true, nil
}

// Create is called only while the policy does not exist: this kind never
// makes it, so it waits for it.
func (a *securityPolicyFieldsAdapter) Create(ctx context.Context, createOp *directbase.CreateOperation) error {
	return fieldkind.NotFoundError("ComputeSecurityPolicy", a.id.String())
}

// thresholdsToProto is the spec's threshold configs as the API takes them.
func thresholdsToProto(in []krm.SecurityPolicyAdaptiveProtectionConfigLayer7DdosDefenseConfigThresholdConfig) []*pb.SecurityPolicyAdaptiveProtectionConfigLayer7DdosDefenseConfigThresholdConfig {
	var out []*pb.SecurityPolicyAdaptiveProtectionConfigLayer7DdosDefenseConfigThresholdConfig
	for _, tc := range in {
		o := &pb.SecurityPolicyAdaptiveProtectionConfigLayer7DdosDefenseConfigThresholdConfig{
			Name:                                direct.LazyPtr(tc.Name),
			AutoDeployConfidenceThreshold:       tc.AutoDeployConfidenceThreshold,
			AutoDeployExpirationSec:             tc.AutoDeployExpirationSec,
			AutoDeployImpactedBaselineThreshold: tc.AutoDeployImpactedBaselineThreshold,
			AutoDeployLoadThreshold:             tc.AutoDeployLoadThreshold,
			DetectionAbsoluteQps:                tc.DetectionAbsoluteQps,
			DetectionLoadThreshold:              tc.DetectionLoadThreshold,
			DetectionRelativeToBaselineQps:      tc.DetectionRelativeToBaselineQps,
		}
		for _, g := range tc.TrafficGranularityConfigs {
			o.TrafficGranularityConfigs = append(o.TrafficGranularityConfigs, &pb.SecurityPolicyAdaptiveProtectionConfigLayer7DdosDefenseConfigThresholdConfigTrafficGranularityConfig{
				EnableEachUniqueValue: g.EnableEachUniqueValue,
				Type:                  g.Type,
				Value:                 g.Value,
			})
		}
		out = append(out, o)
	}
	return out
}

// sameThresholds compares the spec's threshold configs with the policy's,
// by name: the same names, and for each the fields the spec sets (the API
// may fill others in) and its granularity configs.
func sameThresholds(want []krm.SecurityPolicyAdaptiveProtectionConfigLayer7DdosDefenseConfigThresholdConfig, have []*pb.SecurityPolicyAdaptiveProtectionConfigLayer7DdosDefenseConfigThresholdConfig) bool {
	if len(want) != len(have) {
		return false
	}
	byName := map[string]*pb.SecurityPolicyAdaptiveProtectionConfigLayer7DdosDefenseConfigThresholdConfig{}
	for _, h := range have {
		byName[h.GetName()] = h
	}
	sameF := func(w, h *float32) bool { return w == nil || (h != nil && *w == *h) }
	for _, w := range want {
		h := byName[w.Name]
		if h == nil {
			return false
		}
		if !sameF(w.AutoDeployConfidenceThreshold, h.AutoDeployConfidenceThreshold) ||
			!sameF(w.AutoDeployImpactedBaselineThreshold, h.AutoDeployImpactedBaselineThreshold) ||
			!sameF(w.AutoDeployLoadThreshold, h.AutoDeployLoadThreshold) ||
			!sameF(w.DetectionAbsoluteQps, h.DetectionAbsoluteQps) ||
			!sameF(w.DetectionLoadThreshold, h.DetectionLoadThreshold) ||
			!sameF(w.DetectionRelativeToBaselineQps, h.DetectionRelativeToBaselineQps) {
			return false
		}
		if w.AutoDeployExpirationSec != nil && (h.AutoDeployExpirationSec == nil || *w.AutoDeployExpirationSec != *h.AutoDeployExpirationSec) {
			return false
		}
		key := func(enable bool, typ, value string) string { return fmt.Sprintf("%v/%s/%s", enable, typ, value) }
		var wg, hg []string
		for _, g := range w.TrafficGranularityConfigs {
			wg = append(wg, key(direct.ValueOf(g.EnableEachUniqueValue), direct.ValueOf(g.Type), direct.ValueOf(g.Value)))
		}
		for _, g := range h.GetTrafficGranularityConfigs() {
			hg = append(hg, key(g.GetEnableEachUniqueValue(), g.GetType(), g.GetValue()))
		}
		slices.Sort(wg)
		slices.Sort(hg)
		if !slices.Equal(wg, hg) {
			return false
		}
	}
	return true
}

// body is the patch that sets the owned fields to what the spec says
// (set) or clears them (!set), with the mask paths of the fields it
// clears; nil when nothing differs.
func (a *securityPolicyFieldsAdapter) body(set bool) (*pb.SecurityPolicy, []string) {
	out := &pb.SecurityPolicy{Fingerprint: a.actual.Fingerprint}
	var mask []string
	changed := false
	if a.ownsThresholds() {
		var want []krm.SecurityPolicyAdaptiveProtectionConfigLayer7DdosDefenseConfigThresholdConfig
		if set {
			want = a.spec.AdaptiveProtectionConfig.Layer7DdosDefenseConfig.ThresholdConfigs
		}
		have := a.actual.GetAdaptiveProtectionConfig().GetLayer7DdosDefenseConfig().GetThresholdConfigs()
		if !sameThresholds(want, have) {
			apc := proto.CloneOf(a.actual.GetAdaptiveProtectionConfig())
			if apc == nil {
				apc = &pb.SecurityPolicyAdaptiveProtectionConfig{}
			}
			if apc.Layer7DdosDefenseConfig == nil {
				apc.Layer7DdosDefenseConfig = &pb.SecurityPolicyAdaptiveProtectionConfigLayer7DdosDefenseConfig{}
			}
			apc.Layer7DdosDefenseConfig.ThresholdConfigs = thresholdsToProto(want)
			out.AdaptiveProtectionConfig = apc
			if len(want) == 0 {
				mask = append(mask, thresholdConfigsPath)
			}
			changed = true
		}
	}
	if a.ownsInspectionSize() {
		want := ""
		if set {
			want = a.spec.AdvancedOptionsConfig.RequestBodyInspectionSize
		}
		have := a.actual.GetAdvancedOptionsConfig().GetRequestBodyInspectionSize()
		if !strings.EqualFold(want, have) {
			aoc := proto.CloneOf(a.actual.GetAdvancedOptionsConfig())
			if aoc == nil {
				aoc = &pb.SecurityPolicyAdvancedOptionsConfig{}
			}
			aoc.RequestBodyInspectionSize = nil
			if want != "" {
				aoc.RequestBodyInspectionSize = direct.LazyPtr(want)
			} else {
				mask = append(mask, requestBodyInspectionSizePath)
			}
			out.AdvancedOptionsConfig = aoc
			changed = true
		}
	}
	if !changed {
		return nil, nil
	}
	return out, mask
}

func (a *securityPolicyFieldsAdapter) patch(ctx context.Context, body *pb.SecurityPolicy, mask []string) error {
	req := &pb.PatchSecurityPolicyRequest{Project: a.id.Project, SecurityPolicy: a.id.Name, SecurityPolicyResource: body}
	if len(mask) > 0 {
		req.UpdateMask = direct.LazyPtr(strings.Join(mask, ","))
	}
	op, err := a.gcpClient.Patch(ctx, req)
	if err == nil && !op.Done() {
		err = op.Wait(ctx)
	}
	if err != nil {
		return fmt.Errorf("setting fields of security policy %s: %w", a.id, err)
	}
	return nil
}

func (a *securityPolicyFieldsAdapter) Update(ctx context.Context, updateOp *directbase.UpdateOperation) error {
	if body, mask := a.body(true); body != nil {
		klog.FromContext(ctx).V(2).Info("setting fields of a security policy", "name", a.id)
		if err := a.patch(ctx, body, mask); err != nil {
			return err
		}
	}
	return updateOp.UpdateStatus(ctx, &krm.ComputeSecurityPolicyFieldsStatus{ExternalRef: direct.LazyPtr(a.id.String())}, nil)
}

// Delete clears the fields this object owns. The policy is never deleted.
// A policy that is gone is not reached: Find said not found.
func (a *securityPolicyFieldsAdapter) Delete(ctx context.Context, deleteOp *directbase.DeleteOperation) (bool, error) {
	body, mask := a.body(false)
	if body == nil {
		return true, nil
	}
	klog.FromContext(ctx).V(2).Info("clearing fields of a security policy", "name", a.id, "mask", mask)
	if err := a.patch(ctx, body, mask); err != nil {
		return false, err
	}
	return true, nil
}

func (a *securityPolicyFieldsAdapter) Export(ctx context.Context) (*unstructured.Unstructured, error) {
	return nil, nil
}
