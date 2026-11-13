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

// Package chronicle has the direct controller for ChronicleRuleDeployment,
// the deployment of a Google SecOps (Chronicle) rule. Config Connector has
// rule kinds but no deployment kind (UPSTREAM.md), so it is written here on
// Config Connector's direct reconciler (directbase), shaped after
// servicemanagement's EndpointsService controller. It does what Terraform's
// google_chronicle_rule_deployment does: the deployment exists with its
// rule and has no create or delete, so "create" and "update" read it and
// patch the fields the spec sets that differ, and delete leaves it as it
// is. It speaks the API's REST JSON (pkg/discokind's API) rather than
// cloud.google.com/go/chronicle, whose v1.5.0 has neither the customizable
// run frequencies nor scheduleCustomizations; like Terraform it sends to the
// location's host, https://<location>-chronicle.googleapis.com/v1/.
package chronicle

import (
	"context"
	"fmt"
	"net/http"
	"reflect"
	"sort"
	"strings"

	kccchroniclev1alpha1 "github.com/GoogleCloudPlatform/k8s-config-connector/apis/chronicle/v1alpha1"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/config"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct/directbase"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/k8s"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	krm "github.com/highfivelabs/config-connector-extensions/apis/chronicle/v1alpha1"
	krmv1beta1 "github.com/highfivelabs/config-connector-extensions/apis/chronicle/v1beta1"
	"github.com/highfivelabs/config-connector-extensions/pkg/discokind"
)

// NewRuleDeploymentModel is the direct reconciler's model.
func NewRuleDeploymentModel(ctx context.Context, cfg *config.ControllerConfig) (directbase.Model, error) {
	return &ruleDeploymentModel{config: *cfg}, nil
}

type ruleDeploymentModel struct {
	config config.ControllerConfig
}

var _ directbase.Model = &ruleDeploymentModel{}

// BaseURL is the API's base URL for a location, as Terraform's Chronicle
// provider builds it.
func BaseURL(location string) string {
	return "https://" + location + "-chronicle.googleapis.com/v1/"
}

func (m *ruleDeploymentModel) AdapterForObject(ctx context.Context, op *directbase.AdapterForObjectOperation) (directbase.Adapter, error) {
	u := op.GetUnstructured()
	obj := &krm.ChronicleRuleDeployment{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, obj); err != nil {
		return nil, fmt.Errorf("error converting to %T: %w", obj, err)
	}
	if obj.Spec.RuleRef == nil {
		return nil, fmt.Errorf("spec.ruleRef is required")
	}
	httpClient, err := m.config.NewAuthenticatedHTTPClient(ctx)
	if err != nil {
		return nil, err
	}
	ext := ""
	if obj.Status.ExternalRef != nil {
		ext = *obj.Status.ExternalRef
	}
	if !obj.GetDeletionTimestamp().IsZero() {
		// Delete leaves the deployment, so it needs no rule: a rule object
		// deleted with this one (everything deleted at once) does not hold
		// it. With no status.externalRef, Find finds nothing.
		location := ""
		if id := (&kccchroniclev1alpha1.ChronicleRuleIdentity{}); ext != "" && id.FromExternal(strings.TrimSuffix(ext, "/deployment")) == nil {
			location = id.Location
		}
		return &ruleDeploymentAdapter{api: &discokind.API{HTTP: httpClient, BaseURL: BaseURL(location)}, name: ext, desired: &obj.Spec}, nil
	}
	external, err := resolveRuleRef(ctx, op.Reader, obj.GetNamespace(), obj.Spec.RuleRef)
	if err != nil {
		return nil, err
	}
	rule := &kccchroniclev1alpha1.ChronicleRuleIdentity{}
	if err := rule.FromExternal(external); err != nil {
		return nil, err
	}
	if strings.Contains(rule.Rule, "@") {
		return nil, fmt.Errorf("spec.ruleRef %q names a revision; a deployment belongs to the rule (%s)", external, strings.SplitN(rule.Rule, "@", 2)[0])
	}
	name := rule.String() + "/deployment"
	if ext != "" && ext != name {
		return nil, fmt.Errorf("cannot change ChronicleRuleDeployment identity (old=%q, new=%q)", ext, name)
	}
	return &ruleDeploymentAdapter{
		api:     &discokind.API{HTTP: httpClient, BaseURL: BaseURL(rule.Location)},
		name:    name,
		desired: &obj.Spec,
	}, nil
}

// resolveRuleRef is the rule's name. A ruleRef by name finds our
// ChronicleRule (chronicle.gcp.hfl.dev) first, then Config Connector's
// (whose alpha kind cannot resolve an instance, so it never has an
// externalRef).
func resolveRuleRef(ctx context.Context, reader client.Reader, namespace string, in *kccchroniclev1alpha1.ChronicleRuleRef) (string, error) {
	ref := in.DeepCopy()
	if ref.External == "" && ref.Name != "" {
		key := types.NamespacedName{Namespace: ref.Namespace, Name: ref.Name}
		if key.Namespace == "" {
			key.Namespace = namespace
		}
		ours := &unstructured.Unstructured{}
		ours.SetGroupVersionKind(krmv1beta1.ChronicleRuleGVK)
		err := reader.Get(ctx, key, ours)
		switch {
		case err == nil:
			ext, _, _ := unstructured.NestedString(ours.Object, "status", "externalRef")
			if ext == "" {
				return "", k8s.NewReferenceNotReadyError(krmv1beta1.ChronicleRuleGVK, key)
			}
			return ext, nil
		case !apierrors.IsNotFound(err):
			return "", fmt.Errorf("reading ChronicleRule %v: %w", key, err)
		}
	}
	if err := ref.Normalize(ctx, reader, namespace); err != nil {
		return "", err
	}
	return ref.External, nil
}

func (m *ruleDeploymentModel) AdapterForURL(ctx context.Context, url string) (directbase.Adapter, error) {
	return nil, nil
}

type ruleDeploymentAdapter struct {
	api     *discokind.API
	name    string
	desired *krm.ChronicleRuleDeploymentSpec
	actual  map[string]any
}

var _ directbase.Adapter = &ruleDeploymentAdapter{}

// Find reads the deployment. A rule that does not exist has none: Create
// then fails with a message that says so.
func (a *ruleDeploymentAdapter) Find(ctx context.Context) (bool, error) {
	if a.name == "" {
		// Being deleted, never set (AdapterForObject).
		return false, nil
	}
	got, err := a.api.Get(ctx, a.name)
	if err != nil {
		if direct.IsNotFound(err) {
			return false, nil
		}
		return false, fmt.Errorf("getting %s: %w", a.name, err)
	}
	a.actual = got
	return true, nil
}

// Create sets the deployment the rule already has: there is no create
// method, and Find found no deployment only when the rule is missing.
func (a *ruleDeploymentAdapter) Create(ctx context.Context, op *directbase.CreateOperation) error {
	klog.FromContext(ctx).V(2).Info("setting the deployment of a rule", "name", a.name)
	if err := a.reconcile(ctx, map[string]any{}); err != nil {
		if direct.IsNotFound(err) {
			return fmt.Errorf("%s was not found: does the rule exist? (%w)", a.name, err)
		}
		return err
	}
	return op.UpdateStatus(ctx, a.status(), nil)
}

func (a *ruleDeploymentAdapter) Update(ctx context.Context, op *directbase.UpdateOperation) error {
	if err := a.reconcile(ctx, a.actual); err != nil {
		return err
	}
	return op.UpdateStatus(ctx, a.status(), nil)
}

// patch is one PATCH: the fields and the update mask.
type patch struct {
	body map[string]any
	mask []string
}

// reconcile patches the fields the spec sets that differ from actual, in
// the order archiving allows: an archive goes after the other fields (the
// rule must be disabled first), an unarchive before them (an archived
// rule's enabled, alerting and run frequency cannot change). With nothing
// to change it writes nothing.
func (a *ruleDeploymentAdapter) reconcile(ctx context.Context, actual map[string]any) error {
	fields, archive := a.changes(actual)
	var steps []patch
	switch {
	case archive == nil:
		steps = append(steps, fields)
	case *archive:
		steps = append(steps, fields, patch{body: map[string]any{"archived": true}, mask: []string{"archived"}})
	default:
		steps = append(steps, patch{body: map[string]any{"archived": false}, mask: []string{"archived"}}, fields)
	}
	for _, p := range steps {
		if len(p.mask) == 0 {
			continue
		}
		klog.FromContext(ctx).V(2).Info("patching", "name", a.name, "mask", p.mask)
		got, err := a.api.Do(ctx, http.MethodPatch, a.name, map[string]string{"updateMask": strings.Join(p.mask, ",")}, p.body)
		if err != nil {
			return fmt.Errorf("patching %s (%s): %w", a.name, strings.Join(p.mask, ","), err)
		}
		actual = got
	}
	if len(actual) == 0 {
		got, err := a.api.Get(ctx, a.name)
		if err != nil {
			return err
		}
		actual = got
	}
	a.actual = actual
	return nil
}

// changes are the fields other than archived the spec sets that differ
// from actual, and the archived value to send, if it differs.
func (a *ruleDeploymentAdapter) changes(actual map[string]any) (patch, *bool) {
	p := patch{body: map[string]any{}}
	d := a.desired
	for _, f := range []struct {
		name string
		want *bool
	}{{"alerting", d.Alerting}, {"enabled", d.Enabled}} {
		if f.want != nil && boolOf(actual[f.name]) != *f.want {
			p.body[f.name] = *f.want
			p.mask = append(p.mask, f.name)
		}
	}
	// runFrequency and scheduleCustomizations share the runFrequency mask:
	// a change to either sends both (Terraform's pre_update).
	actualRF, _ := actual["runFrequency"].(string)
	rfChanged := d.RunFrequency != nil && !sameRunFrequency(actualRF, *d.RunFrequency)
	scChanged := d.ScheduleCustomizations != nil && !scheduleHolds(d.ScheduleCustomizations, actual["scheduleCustomizations"])
	if rfChanged || scChanged {
		rf := actualRF
		if d.RunFrequency != nil {
			rf = *d.RunFrequency
		}
		if rf != "" {
			p.body["runFrequency"] = rf
		}
		if d.ScheduleCustomizations != nil {
			p.body["scheduleCustomizations"] = schedule(d.ScheduleCustomizations)
		} else if sc, ok := actual["scheduleCustomizations"]; ok {
			p.body["scheduleCustomizations"] = sc
		}
		p.mask = append(p.mask, "runFrequency")
	}
	sort.Strings(p.mask)
	if d.Archived != nil && boolOf(actual["archived"]) != *d.Archived {
		v := *d.Archived
		return p, &v
	}
	return p, nil
}

func boolOf(v any) bool {
	b, _ := v.(bool)
	return b
}

// sameRunFrequency: the API stores some legacy run frequencies as others
// (Terraform's chronicleRuleDeploymentNilRunFrequencyDiffSuppressFunc):
// LIVE as LIVE_CUSTOMIZABLE and HOURLY or DAILY as HOURLY_CUSTOMIZABLE for
// a multi-event rule, HOURLY or DAILY as LIVE for a single-event rule.
func sameRunFrequency(actual, want string) bool {
	switch {
	case actual == want:
		return true
	case actual == "LIVE_CUSTOMIZABLE" && want == "LIVE":
		return true
	case actual == "HOURLY_CUSTOMIZABLE" && (want == "HOURLY" || want == "DAILY"):
		return true
	case actual == "LIVE" && (want == "HOURLY" || want == "DAILY"):
		return true
	}
	return false
}

func schedule(s *krm.ChronicleRuleDeploymentScheduleCustomizations) map[string]any {
	out := map[string]any{}
	if s.EnsureEnrichmentCompleteness != nil {
		out["ensureEnrichmentCompleteness"] = *s.EnsureEnrichmentCompleteness
	}
	if s.LateArrivingDataAdjustment != nil {
		out["lateArrivingDataAdjustment"] = *s.LateArrivingDataAdjustment
	}
	return out
}

// scheduleHolds: what the spec sets of the schedule, actual holds.
func scheduleHolds(want *krm.ChronicleRuleDeploymentScheduleCustomizations, actual any) bool {
	got, _ := actual.(map[string]any)
	for k, v := range schedule(want) {
		g, ok := got[k]
		if b, isBool := v.(bool); isBool && !b && !ok {
			// false is the API's default, which it leaves out.
			continue
		}
		if !ok || !reflect.DeepEqual(g, v) {
			return false
		}
	}
	return true
}

func (a *ruleDeploymentAdapter) status() *krm.ChronicleRuleDeploymentStatus {
	str := func(k string) *string {
		if s, _ := a.actual[k].(string); s != "" {
			return &s
		}
		return nil
	}
	strs := func(k string) []string {
		var out []string
		l, _ := a.actual[k].([]any)
		for _, v := range l {
			if s, ok := v.(string); ok {
				out = append(out, s)
			}
		}
		return out
	}
	name := a.name
	return &krm.ChronicleRuleDeploymentStatus{
		ExternalRef: &name,
		ObservedState: &krm.ChronicleRuleDeploymentObservedState{
			RunFrequency:              str("runFrequency"),
			ArchiveTime:               str("archiveTime"),
			ExecutionState:            str("executionState"),
			ProducerRules:             strs("producerRules"),
			ConsumerRules:             strs("consumerRules"),
			LastAlertStatusChangeTime: str("lastAlertStatusChangeTime"),
		},
	}
}

func (a *ruleDeploymentAdapter) Export(ctx context.Context) (*unstructured.Unstructured, error) {
	return nil, fmt.Errorf("ChronicleRuleDeployment does not support export")
}

// Delete leaves the deployment as it is: the API has no delete, and the
// deployment lasts as long as its rule (Terraform's exclude_delete).
func (a *ruleDeploymentAdapter) Delete(ctx context.Context, op *directbase.DeleteOperation) (bool, error) {
	klog.FromContext(ctx).V(2).Info("leaving the deployment: the API has no delete", "name", a.name)
	return true, nil
}
