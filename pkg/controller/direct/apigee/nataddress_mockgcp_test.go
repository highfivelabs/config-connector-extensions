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

package apigee_test

// ApigeeNATAddress end to end on the fake cloud (pkg/test/mockenv) or a
// real project (docs/live-runs.md). The case: an Apigee instance's egress
// gets a reserved NAT address that a backend's firewall can allow, as the
// catalog's apigee module does.
//
// A live run needs a provisioned Apigee organization and instance in the
// project (named by E2E_APIGEE_INSTANCE, organizations/O/instances/I) and
// roles/apigee.admin for the live-test service account; without them the
// test skips live. Provisioning Apigee is billed and slow, so no test makes
// one.

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/k8s"
	api "google.golang.org/api/apigee/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"

	apigeev1beta1 "github.com/highfivelabs/config-connector-extensions/apis/apigee/v1beta1"
	"github.com/highfivelabs/config-connector-extensions/pkg/test/mockenv"
)

const namespace = "example"

func TestApigeeNATAddress(t *testing.T) {
	instance := "organizations/" + mockenv.Project + "/instances/" + mockenv.Name("instance")
	if mockenv.Live() {
		instance = os.Getenv("E2E_APIGEE_INSTANCE")
		if instance == "" {
			t.Skip("a live ApigeeNATAddress needs a provisioned Apigee instance (E2E_APIGEE_INSTANCE=organizations/O/instances/I) and roles/apigee.admin for the live-test service account (docs/live-runs.md)")
		}
	}
	gvk := apigeev1beta1.ApigeeNATAddressGVK
	env := mockenv.New(t, namespace)
	s, err := api.NewService(env.Ctx, env.GCPOptions()...)
	if err != nil {
		t.Fatal(err)
	}
	nats := api.NewOrganizationsInstancesNatAddressesService(s)
	ops := api.NewOrganizationsOperationsService(s)

	name := func(base string) string { return instance + "/natAddresses/" + mockenv.Name(base) }
	get := func(n string) (*api.GoogleCloudApigeeV1NatAddress, error) {
		return nats.Get(n).Context(env.Ctx).Do()
	}
	writes := func() []string {
		var out []string
		for _, r := range env.Requests() {
			if (strings.HasPrefix(r, "POST ") || strings.HasPrefix(r, "PATCH ")) && strings.Contains(r, "/natAddresses") {
				out = append(out, r)
			}
		}
		return out
	}
	getObj := func(t *testing.T, n string) *apigeev1beta1.ApigeeNATAddress {
		t.Helper()
		obj := &apigeev1beta1.ApigeeNATAddress{}
		if err := env.Kube.Get(env.Ctx, types.NamespacedName{Namespace: namespace, Name: n}, obj); err != nil {
			t.Fatal(err)
		}
		return obj
	}
	// Delete what the test made, the abandoned address included, and what
	// a failed step left.
	t.Cleanup(func() {
		for _, base := range []string{"egress", "kept"} {
			op, err := nats.Delete(name(base)).Context(env.Ctx).Do()
			if err == nil {
				deadline := time.Now().Add(mockenv.Timeout())
				for !op.Done && err == nil && time.Now().Before(deadline) {
					time.Sleep(time.Second)
					op, err = ops.Get(op.Name).Context(env.Ctx).Do()
				}
			}
			if err != nil && !direct.IsNotFound(err) {
				t.Errorf("cleaning up %s: %v", name(base), err)
			}
		}
	})

	newNAT := func(n, base string) *apigeev1beta1.ApigeeNATAddress {
		return &apigeev1beta1.ApigeeNATAddress{
			ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: n, Labels: mockenv.Labels()},
			Spec: apigeev1beta1.ApigeeNATAddressSpec{
				InstanceId: instance,
				ResourceID: mockenv.Ptr(mockenv.Name(base)),
			},
		}
	}

	if err := env.Kube.Create(env.Ctx, newNAT("egress", "egress")); err != nil {
		t.Fatal(err)
	}

	t.Run("create reserves the address", func(t *testing.T) {
		u := env.WaitReady(gvk, namespace, "egress")
		if ref, _, _ := unstructured.NestedString(u.Object, "status", "externalRef"); ref != name("egress") {
			t.Errorf("status.externalRef = %q, want %q", ref, name("egress"))
		}
		got, err := get(name("egress"))
		if err != nil {
			t.Fatalf("reading %s: %v", name("egress"), err)
		}
		if st, _, _ := unstructured.NestedString(u.Object, "status", "state"); st == "" || st != got.State {
			t.Errorf("status.state = %q, want the API's %q", st, got.State)
		}
		if ip, _, _ := unstructured.NestedString(u.Object, "status", "ipAddress"); ip != got.IpAddress {
			t.Errorf("status.ipAddress = %q, want the API's %q", ip, got.IpAddress)
		}
	})

	t.Run("a resync writes nothing", func(t *testing.T) {
		before := len(writes())
		o := getObj(t, "egress")
		o.Annotations = map[string]string{k8s.InternalForceReconcileAnnotation: "1"}
		if err := env.Kube.Update(env.Ctx, o); err != nil {
			t.Fatal(err)
		}
		time.Sleep(3 * time.Second)
		env.WaitReady(gvk, namespace, "egress")
		if w := writes()[before:]; len(w) != 0 {
			t.Errorf("a resync wrote %v", w)
		}
	})

	t.Run("the CRD refuses a change of instance or name", func(t *testing.T) {
		o := getObj(t, "egress")
		o.Spec.InstanceId = instance + "-other"
		if err := env.Kube.Update(env.Ctx, o); err == nil || !strings.Contains(err.Error(), "instanceId is immutable") {
			t.Errorf("changing instanceId: got %v, want the API server to refuse it", err)
		}
		o = getObj(t, "egress")
		o.Spec.ResourceID = mockenv.Ptr(mockenv.Name("renamed"))
		if err := env.Kube.Update(env.Ctx, o); err == nil || !strings.Contains(err.Error(), "resourceID is immutable") {
			t.Errorf("changing resourceID: got %v, want the API server to refuse it", err)
		}
	})

	t.Run("delete releases the address", func(t *testing.T) {
		if err := env.Kube.Delete(env.Ctx, getObj(t, "egress")); err != nil {
			t.Fatal(err)
		}
		env.WaitGone(&apigeev1beta1.ApigeeNATAddress{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: "egress"}})
		if _, err := get(name("egress")); !direct.IsNotFound(err) {
			t.Errorf("after delete, reading the address: %v, want not found", err)
		}
	})

	t.Run("abandon keeps the address, and a new object acquires it", func(t *testing.T) {
		if err := env.Kube.Create(env.Ctx, newNAT("kept", "kept")); err != nil {
			t.Fatal(err)
		}
		env.WaitReady(gvk, namespace, "kept")
		o := getObj(t, "kept")
		o.Annotations = map[string]string{"cnrm.cloud.google.com/deletion-policy": "abandon"}
		if err := env.Kube.Update(env.Ctx, o); err != nil {
			t.Fatal(err)
		}
		if err := env.Kube.Delete(env.Ctx, o); err != nil {
			t.Fatal(err)
		}
		env.WaitGone(&apigeev1beta1.ApigeeNATAddress{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: "kept"}})
		if _, err := get(name("kept")); err != nil {
			t.Fatalf("after abandon, reading the address: %v, want it kept", err)
		}

		before := len(writes())
		if err := env.Kube.Create(env.Ctx, newNAT("acquired", "kept")); err != nil {
			t.Fatal(err)
		}
		u := env.WaitReady(gvk, namespace, "acquired")
		if ref, _, _ := unstructured.NestedString(u.Object, "status", "externalRef"); ref != name("kept") {
			t.Errorf("status.externalRef = %q, want %q", ref, name("kept"))
		}
		if w := writes()[before:]; len(w) != 0 {
			t.Errorf("acquiring the address wrote %v", w)
		}
		if err := env.Kube.Delete(env.Ctx, getObj(t, "acquired")); err != nil {
			t.Fatal(err)
		}
		env.WaitGone(&apigeev1beta1.ApigeeNATAddress{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: "acquired"}})
		if _, err := get(name("kept")); !direct.IsNotFound(err) {
			t.Errorf("after delete, reading the address: %v, want not found", err)
		}
	})
}
