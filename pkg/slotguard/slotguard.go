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

// Package slotguard is the version-slot guard (docs/design.md, "Version
// slots"): a soft lease on each cloud resource, so that two installs of
// this operator (two slots, for example the current and the previous
// version of a hosted Bundt) never both write one resource.
//
// It is off unless the manager is given a slot (--slot, or HFL_SLOT). With
// a slot, every kind's model is wrapped:
//
//   - The Kubernetes object gets the label hfl.dev/slot=<slot>.
//   - A kind whose cloud resource has labels gets the label
//     hfl-dev-slot=<slot> written on create and on adopt (Google label keys
//     cannot hold "." or "/").
//   - Before an update or a delete, the resource's hfl-dev-slot label is
//     read (Find, a read, always runs). If it names another slot, nothing
//     is sent: an update sets Ready False with reason SlotMismatch and a
//     Warning event SlotMismatch; a delete is refused with an error the
//     reconciler reports as DeleteFailed, with the same message and event.
//     deletion-policy: abandon still releases the object, since it sends
//     nothing.
//   - A resource with no slot label is claimed. A resource held by the
//     slot named in the object's hfl.dev/slot-takeover annotation is
//     claimed too: that is how a move hands it from the old slot to the
//     new one. A refused update is retried with the workqueue's backoff.
//
// Config Connector's reconciler (directbase) is unchanged: this wraps the
// Model and Adapter it calls.
package slotguard

import (
	"context"
	"fmt"
	"os"
	"regexp"

	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/apis/k8s/v1alpha1"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct/directbase"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/record"
)

const (
	// EnvVar names the slot when --slot is not given.
	EnvVar = "HFL_SLOT"
	// ObjectLabel is the Kubernetes object's label naming its slot.
	ObjectLabel = "hfl.dev/slot"
	// CloudLabel is the cloud resource's label naming the slot that holds it.
	CloudLabel = "hfl-dev-slot"
	// TakeoverAnnotation, on an object, names the one other slot whose
	// resource this slot may claim (the slot a move comes from).
	TakeoverAnnotation = "hfl.dev/slot-takeover"
	// Reason is the Ready condition's reason and the event's when a write is
	// refused.
	Reason = "SlotMismatch"
)

// LabelSource is where a kind's cloud labels come from in its object.
type LabelSource int

const (
	// None: the cloud resource has no labels; the guard cannot mark it.
	None LabelSource = iota
	// Metadata: metadata.labels (keys with "/" left out, as Config
	// Connector does), or spec.labels when the object sets it.
	Metadata
	// Spec: spec.labels.
	Spec
)

// CloudLabeler is an Adapter whose cloud resource has labels; CloudLabels
// is valid after Find returned true.
type CloudLabeler interface {
	CloudLabels() map[string]string
}

// Resolve is the slot: the flag's value, else HFL_SLOT.
func Resolve(flagValue string) string {
	if flagValue != "" {
		return flagValue
	}
	return os.Getenv(EnvVar)
}

// A slot must be both a Google label value and a Kubernetes label value.
var slotRE = regexp.MustCompile(`^[a-z0-9]([a-z0-9_-]{0,61}[a-z0-9])?$`)

// Validate fails for a slot that cannot be a label value.
func Validate(slot string) error {
	if !slotRE.MatchString(slot) {
		return fmt.Errorf("slot %q: want 1-63 lowercase letters, digits, '-' or '_', starting and ending with a letter or digit", slot)
	}
	return nil
}

// Guard is one manager's slot.
type Guard struct {
	Slot     string
	Recorder record.EventRecorder
}

// Message is the refusal's text.
func Message(holder, slot string) string {
	return fmt.Sprintf("cloud resource is held by slot %s; this controller is slot %s", holder, slot)
}

// Wrap guards m, a kind whose labels come from src.
func (g *Guard) Wrap(gvk schema.GroupVersionKind, m directbase.Model, src LabelSource) (directbase.Model, error) {
	if _, ok := m.(directbase.SensitiveFieldModel); ok {
		return nil, fmt.Errorf("slot guard: %v's model watches Secrets (SensitiveFieldModel), which the wrapper does not pass on", gvk)
	}
	if _, ok := m.(directbase.IAMModel); ok {
		return nil, fmt.Errorf("slot guard: %v's model is an IAMModel, which the wrapper does not pass on", gvk)
	}
	return &model{inner: m, g: g, gvk: gvk, src: src}, nil
}

type model struct {
	inner directbase.Model
	g     *Guard
	gvk   schema.GroupVersionKind
	src   LabelSource
}

func (m *model) AdapterForObject(ctx context.Context, op *directbase.AdapterForObjectOperation) (directbase.Adapter, error) {
	u := op.GetUnstructured()
	// The object's own label, written by the reconciler's next update of
	// the object (its finalizers on the first reconcile).
	if u.GetLabels()[ObjectLabel] == "" {
		labels := u.GetLabels()
		if labels == nil {
			labels = map[string]string{}
		}
		labels[ObjectLabel] = m.g.Slot
		u.SetLabels(labels)
	}
	// The kind's adapter sees the slot's cloud label among the desired
	// ones, so create writes it and an update (an adoption) adds it; the
	// copy keeps it out of the object itself.
	desired := u
	if m.src != None {
		desired = u.DeepCopy()
		if err := addCloudLabel(desired, m.src, m.g.Slot); err != nil {
			return nil, fmt.Errorf("slot guard: %w", err)
		}
	}
	inner, err := m.inner.AdapterForObject(ctx, &directbase.AdapterForObjectOperation{Reader: op.Reader, Object: desired})
	if err != nil {
		return nil, err
	}
	return &adapter{inner: inner, m: m, obj: u, takeover: u.GetAnnotations()[TakeoverAnnotation]}, nil
}

func (m *model) AdapterForURL(ctx context.Context, url string) (directbase.Adapter, error) {
	return m.inner.AdapterForURL(ctx, url)
}

func addCloudLabel(u *unstructured.Unstructured, src LabelSource, slot string) error {
	specLabels, hasSpecLabels, err := unstructured.NestedStringMap(u.Object, "spec", "labels")
	if err != nil {
		return fmt.Errorf("reading spec.labels: %w", err)
	}
	if src == Spec || hasSpecLabels {
		if specLabels == nil {
			specLabels = map[string]string{}
		}
		specLabels[CloudLabel] = slot
		if err := unstructured.SetNestedStringMap(u.Object, specLabels, "spec", "labels"); err != nil {
			return fmt.Errorf("writing spec.labels: %w", err)
		}
	}
	if src == Metadata {
		labels := u.GetLabels()
		if labels == nil {
			labels = map[string]string{}
		}
		labels[CloudLabel] = slot
		u.SetLabels(labels)
	}
	return nil
}

type adapter struct {
	inner    directbase.Adapter
	m        *model
	obj      *unstructured.Unstructured
	takeover string
	// holder is the other slot that holds the resource, if one does.
	holder string
}

// Find reads the resource, and with it the slot that holds it.
func (a *adapter) Find(ctx context.Context) (bool, error) {
	found, err := a.inner.Find(ctx)
	if err != nil || !found || a.m.src == None {
		return found, err
	}
	l, ok := a.inner.(CloudLabeler)
	if !ok {
		return false, fmt.Errorf("slot guard: %v has cloud labels but its adapter cannot read them (CloudLabels)", a.m.gvk)
	}
	if h := l.CloudLabels()[CloudLabel]; h != "" && h != a.m.g.Slot && h != a.takeover {
		a.holder = h
	}
	return true, nil
}

// Create runs only when Find found nothing, so there is no holder.
func (a *adapter) Create(ctx context.Context, op *directbase.CreateOperation) error {
	return a.inner.Create(ctx, op)
}

func (a *adapter) Update(ctx context.Context, op *directbase.UpdateOperation) error {
	if a.holder == "" {
		return a.inner.Update(ctx, op)
	}
	msg := Message(a.holder, a.m.g.Slot)
	a.m.g.Recorder.Event(a.obj, corev1.EventTypeWarning, Reason, msg)
	// Try again with the workqueue's backoff, as after a failed update: the
	// reconciler's event filter ignores most annotation changes (the
	// takeover annotation among them), so this is how a refused object
	// notices the holder letting go.
	op.RequestRequeue()
	// The status as it is, with Ready False: UpdateStatus replaces status
	// with what it is given.
	u := op.GetUnstructured()
	status, _, _ := unstructured.NestedMap(u.Object, "status")
	if status == nil {
		status = map[string]any{}
	}
	return op.UpdateStatus(ctx, &unstructured.Unstructured{Object: runtime.DeepCopyJSON(status)}, &v1alpha1.Condition{
		Type:    v1alpha1.ReadyConditionType,
		Status:  corev1.ConditionFalse,
		Reason:  Reason,
		Message: msg,
	})
}

func (a *adapter) Delete(ctx context.Context, op *directbase.DeleteOperation) (bool, error) {
	if a.holder == "" {
		return a.inner.Delete(ctx, op)
	}
	msg := Message(a.holder, a.m.g.Slot)
	a.m.g.Recorder.Event(a.obj, corev1.EventTypeWarning, Reason, msg)
	return false, fmt.Errorf("%s: %s", Reason, msg)
}

func (a *adapter) Export(ctx context.Context) (*unstructured.Unstructured, error) {
	return a.inner.Export(ctx)
}

// IsUnreadableButDeletable passes on the kind's own answer, if it has one.
func (a *adapter) IsUnreadableButDeletable(err error) bool {
	d, ok := a.inner.(directbase.UnreadableDeleter)
	return ok && d.IsUnreadableButDeletable(err)
}
