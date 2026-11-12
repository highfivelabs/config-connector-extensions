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

// Package slotmove moves one cloud resource between two version slots, end
// to end, as a hosted Bundt does (docs/design.md, "Version slots"): two
// managers (mockenv.NewSlots), the old slot pauses, the new slot adopts the
// resource by name with the takeover annotation, nothing is recreated or
// changed but the slot label, the old slot is refused by the slot guard,
// and the old object is deleted with deletion-policy abandon, leaving the
// resource to the new slot.
package slotmove

import (
	"maps"
	"strings"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/highfivelabs/config-connector-extensions/pkg/slotguard"
	"github.com/highfivelabs/config-connector-extensions/pkg/test/mockenv"
)

// The two slots: the one the resource moves from and the one it moves to.
const (
	Old = "blue"
	New = "green"
)

// Cloud is what the test reads of the cloud resource.
type Cloud struct {
	// ID and Created must not change across the move: a recreated
	// resource has a new creation time (and often a new ID).
	ID, Created string
	// Labels are the resource's labels (nil for a kind without them).
	Labels map[string]string
	// Fields is every other field the object sets, in some canonical
	// form; the move must not change it.
	Fields string
}

// Case is one kind's move.
type Case struct {
	GVK schema.GroupVersionKind
	// Name is the object's name in both slots.
	Name string
	// Object is the object as a slot applies it, in namespace ns.
	Object func(ns string) client.Object
	// Read reads the cloud resource; found is false when it is gone.
	Read func(t *testing.T) (c Cloud, found bool)
	// DeleteCloud deletes the cloud resource directly: the last resort of
	// the cleanup, when a failed step left it.
	DeleteCloud func(t *testing.T)
	// Labeled says the cloud resource carries labels, so the guard can
	// mark and check it. A kind without labels moves the same way, but
	// nothing in the cloud stops the old slot (docs/design.md).
	Labeled bool
}

// Run moves c from slot Old to slot New on env (mockenv.NewSlots(t, Old, New)).
func Run(t *testing.T, env *mockenv.Env, c Case) {
	oldNS, newNS := mockenv.SlotNamespace(Old), mockenv.SlotNamespace(New)

	// Whatever happens, the new slot's object deletes the resource last
	// (a live run leaves nothing); the old one is abandoned first. If a
	// step failed before the new slot had it, the old one deletes it, and
	// failing that the test does.
	t.Cleanup(func() {
		newObj := c.Object(newNS)
		if err := env.Kube.Get(env.Ctx, client.ObjectKeyFromObject(newObj), newObj); err == nil {
			abandonAndDelete(env, c, oldNS)
			if err := env.Kube.Delete(env.Ctx, newObj); err == nil {
				waitGoneQuietly(env, newObj)
			}
		} else if oldObj := c.Object(oldNS); env.Kube.Delete(env.Ctx, oldObj) == nil {
			waitGoneQuietly(env, oldObj)
		}
		if _, found := c.Read(t); found {
			t.Errorf("%s %s was still in the cloud after the test; deleting it directly", c.GVK.Kind, c.Name)
			c.DeleteCloud(t)
		}
	})

	var before Cloud
	t.Run("the old slot creates the resource and marks it", func(t *testing.T) {
		if err := env.Kube.Create(env.Ctx, c.Object(oldNS)); err != nil {
			t.Fatal(err)
		}
		u := env.WaitReady(c.GVK, oldNS, c.Name)
		if got := u.GetLabels()[slotguard.ObjectLabel]; got != Old {
			t.Errorf("object label %s = %q, want %q", slotguard.ObjectLabel, got, Old)
		}
		var found bool
		if before, found = c.Read(t); !found {
			t.Fatal("the resource is not in the cloud")
		}
		if c.Labeled {
			if got := before.Labels[slotguard.CloudLabel]; got != Old {
				t.Errorf("cloud label %s = %q, want %q (labels %v)", slotguard.CloudLabel, got, Old, before.Labels)
			}
		}
	})
	if t.Failed() {
		t.FailNow()
	}

	pausedAt := len(env.SlotRequests(Old))
	t.Run("the old slot pauses", func(t *testing.T) {
		setAnnotations(t, env, c, oldNS, map[string]string{"cnrm.cloud.google.com/actuation-mode": "Paused"})
	})

	newStart := len(env.SlotRequests(New))
	if c.Labeled {
		t.Run("without the takeover annotation the new slot is refused", func(t *testing.T) {
			if err := env.Kube.Create(env.Ctx, c.Object(newNS)); err != nil {
				t.Fatal(err)
			}
			waitReason(t, env, c, newNS, slotguard.Reason, slotguard.Message(Old, New))
			if got := nonGET(env.SlotRequests(New)[newStart:]); len(got) > 0 {
				t.Errorf("the refused new slot wrote to the cloud: %v", got)
			}
		})
	}
	t.Run("the new slot adopts it by name, unchanged but for the slot label", func(t *testing.T) {
		if c.Labeled {
			setAnnotations(t, env, c, newNS, map[string]string{slotguard.TakeoverAnnotation: Old})
		} else {
			obj := c.Object(newNS)
			obj.SetAnnotations(mergeAnnotations(obj.GetAnnotations(), slotguard.TakeoverAnnotation, Old))
			if err := env.Kube.Create(env.Ctx, obj); err != nil {
				t.Fatal(err)
			}
		}
		env.WaitReady(c.GVK, newNS, c.Name)
		after, found := c.Read(t)
		if !found {
			t.Fatal("the resource is gone from the cloud")
		}
		sameResource(t, before, after)
		writes := nonGET(env.SlotRequests(New)[newStart:])
		if c.Labeled {
			if got := after.Labels[slotguard.CloudLabel]; got != New {
				t.Errorf("cloud label %s = %q after adoption, want %q", slotguard.CloudLabel, got, New)
			}
			if !sameExceptSlot(before.Labels, after.Labels) {
				t.Errorf("labels changed beyond the slot label: before %v, after %v", before.Labels, after.Labels)
			}
			if len(writes) > 1 {
				t.Errorf("the new slot wrote %d times to adopt (want only the slot label): %v", len(writes), writes)
			}
		} else if len(writes) > 0 {
			t.Errorf("the new slot wrote to adopt an unchanged resource: %v", writes)
		}
		for _, w := range writes {
			if strings.HasPrefix(w, "DELETE ") {
				t.Errorf("the new slot deleted something: %s", w)
			}
		}
	})
	if got := nonGET(env.SlotRequests(Old)[pausedAt:]); len(got) > 0 {
		t.Errorf("the paused old slot wrote to the cloud: %v", got)
	}

	if c.Labeled {
		t.Run("the old slot, resumed, is refused by the guard", func(t *testing.T) {
			at := len(env.SlotRequests(Old))
			setAnnotations(t, env, c, oldNS, map[string]string{"cnrm.cloud.google.com/actuation-mode": ""})
			waitReason(t, env, c, oldNS, slotguard.Reason, "")
			waitEvent(t, env, oldNS, c.Name)
			if got := nonGET(env.SlotRequests(Old)[at:]); len(got) > 0 {
				t.Errorf("the refused old slot wrote to the cloud: %v", got)
			}
			if after, _ := c.Read(t); after.Labels[slotguard.CloudLabel] != New {
				t.Errorf("cloud label %s = %q after the old slot ran, want %q", slotguard.CloudLabel, after.Labels[slotguard.CloudLabel], New)
			}
		})

		t.Run("the old slot cannot delete it", func(t *testing.T) {
			at := len(env.SlotRequests(Old))
			obj := c.Object(oldNS)
			if err := env.Kube.Delete(env.Ctx, obj); err != nil {
				t.Fatal(err)
			}
			waitReason(t, env, c, oldNS, "DeleteFailed", slotguard.Reason)
			if got := nonGET(env.SlotRequests(Old)[at:]); len(got) > 0 {
				t.Errorf("the refused old slot wrote to the cloud: %v", got)
			}
			if _, found := c.Read(t); !found {
				t.Fatal("the old slot deleted the resource")
			}
		})
	}

	t.Run("the old object is deleted with deletion-policy abandon", func(t *testing.T) {
		at := len(env.SlotRequests(Old))
		abandonAndDelete(env, c, oldNS)
		if got := nonGET(env.SlotRequests(Old)[at:]); len(got) > 0 {
			t.Errorf("abandoning wrote to the cloud: %v", got)
		}
		after, found := c.Read(t)
		if !found {
			t.Fatal("the resource is gone from the cloud")
		}
		sameResource(t, before, after)
		if c.Labeled && after.Labels[slotguard.CloudLabel] != New {
			t.Errorf("cloud label %s = %q, want %q", slotguard.CloudLabel, after.Labels[slotguard.CloudLabel], New)
		}
		if u := env.WaitReady(c.GVK, newNS, c.Name); u.GetLabels()[slotguard.ObjectLabel] != New {
			t.Errorf("new object label %s = %q, want %q", slotguard.ObjectLabel, u.GetLabels()[slotguard.ObjectLabel], New)
		}
	})
}

func mergeAnnotations(ann map[string]string, k, v string) map[string]string {
	out := maps.Clone(ann)
	if out == nil {
		out = map[string]string{}
	}
	out[k] = v
	return out
}

func sameResource(t *testing.T, before, after Cloud) {
	t.Helper()
	if after.ID != before.ID || after.Created != before.Created {
		t.Errorf("the resource was recreated: ID %q created %q, was ID %q created %q", after.ID, after.Created, before.ID, before.Created)
	}
	if after.Fields != before.Fields {
		t.Errorf("the resource changed:\nbefore %s\nafter  %s", before.Fields, after.Fields)
	}
}

func sameExceptSlot(a, b map[string]string) bool {
	a, b = maps.Clone(a), maps.Clone(b)
	delete(a, slotguard.CloudLabel)
	delete(b, slotguard.CloudLabel)
	return maps.Equal(a, b)
}

func nonGET(reqs []string) []string {
	var out []string
	for _, r := range reqs {
		if !strings.HasPrefix(r, "GET ") {
			out = append(out, r)
		}
	}
	return out
}

// setAnnotations sets each annotation, or removes it when the value is "".
func setAnnotations(t *testing.T, env *mockenv.Env, c Case, ns string, set map[string]string) {
	t.Helper()
	for {
		u := get(t, env, c, ns)
		ann := u.GetAnnotations()
		if ann == nil {
			ann = map[string]string{}
		}
		for k, v := range set {
			if v == "" {
				delete(ann, k)
			} else {
				ann[k] = v
			}
		}
		u.SetAnnotations(ann)
		err := env.Kube.Update(env.Ctx, u)
		if err == nil {
			return
		}
		if !apierrors.IsConflict(err) {
			t.Fatal(err)
		}
	}
}

func abandonAndDelete(env *mockenv.Env, c Case, ns string) {
	t := env.T
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(c.GVK)
	if err := env.Kube.Get(env.Ctx, types.NamespacedName{Namespace: ns, Name: c.Name}, u); apierrors.IsNotFound(err) {
		return
	}
	setAnnotations(t, env, c, ns, map[string]string{
		"cnrm.cloud.google.com/deletion-policy": "abandon",
		"cnrm.cloud.google.com/actuation-mode":  "",
	})
	if err := env.Kube.Delete(env.Ctx, u); err != nil && !apierrors.IsNotFound(err) {
		t.Fatal(err)
	}
	env.WaitGone(u)
}

func waitGoneQuietly(env *mockenv.Env, obj client.Object) {
	deadline := time.Now().Add(mockenv.Timeout())
	for time.Now().Before(deadline) {
		if err := env.Kube.Get(env.Ctx, client.ObjectKeyFromObject(obj), obj); apierrors.IsNotFound(err) {
			return
		}
		time.Sleep(200 * time.Millisecond)
	}
	env.T.Errorf("%s was not deleted", obj.GetName())
}

func get(t *testing.T, env *mockenv.Env, c Case, ns string) *unstructured.Unstructured {
	t.Helper()
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(c.GVK)
	if err := env.Kube.Get(env.Ctx, types.NamespacedName{Namespace: ns, Name: c.Name}, u); err != nil {
		t.Fatal(err)
	}
	return u
}

// waitReason waits for the Ready condition to be False with reason, its
// message containing msg.
func waitReason(t *testing.T, env *mockenv.Env, c Case, ns, reason, msg string) {
	t.Helper()
	var last any
	deadline := time.Now().Add(mockenv.Timeout())
	for time.Now().Before(deadline) {
		u := get(t, env, c, ns)
		conditions, _, _ := unstructured.NestedSlice(u.Object, "status", "conditions")
		last = conditions
		for _, cond := range conditions {
			cm, _ := cond.(map[string]any)
			m, _ := cm["message"].(string)
			if cm["type"] == "Ready" && cm["status"] == "False" && cm["reason"] == reason && strings.Contains(m, msg) {
				return
			}
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.Fatalf("%s %s/%s: no Ready=False reason %s (message with %q); conditions %v", c.GVK.Kind, ns, c.Name, reason, msg, last)
}

// waitEvent waits for a Warning event with reason SlotMismatch on the object.
func waitEvent(t *testing.T, env *mockenv.Env, ns, name string) {
	t.Helper()
	deadline := time.Now().Add(mockenv.Timeout())
	for time.Now().Before(deadline) {
		events := &corev1.EventList{}
		if err := env.Kube.List(env.Ctx, events, client.InNamespace(ns)); err != nil {
			t.Fatal(err)
		}
		for _, e := range events.Items {
			if e.InvolvedObject.Name == name && e.Reason == slotguard.Reason && e.Type == corev1.EventTypeWarning &&
				strings.Contains(e.Message, "held by slot "+New) && strings.Contains(e.Message, "this controller is slot "+Old) {
				return
			}
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.Fatalf("no Warning event %s on %s/%s", slotguard.Reason, ns, name)
}
