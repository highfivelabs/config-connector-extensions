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

package compute_test

// ComputeServiceAttachmentFields end to end on the fake cloud
// (pkg/test/mockenv) or a real project (docs/live-runs.md). The service
// attachment is the run's own: a network with a subnet for an internal
// passthrough load balancer and a Private Service Connect NAT subnet, a
// regional TCP health check, an internal backend service with no backends,
// its forwarding rule, and the service attachment publishing it, all
// deleted at the end (the forwarding rule is the only one billed by the
// hour).

import (
	"strings"
	"testing"
	"time"

	gcp "cloud.google.com/go/compute/apiv1"
	pb "cloud.google.com/go/compute/apiv1/computepb"
	refs "github.com/GoogleCloudPlatform/k8s-config-connector/apis/refs/v1beta1"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/k8s"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"

	krm "github.com/highfivelabs/config-connector-extensions/apis/compute/v1alpha1"
	"github.com/highfivelabs/config-connector-extensions/pkg/test/mockenv"
)

var configConnectorServiceAttachmentGVK = schema.GroupVersionKind{Group: "compute.cnrm.cloud.google.com", Version: "v1beta1", Kind: "ComputeServiceAttachment"}

// waitOp waits for a Compute Engine operation, failing the test on error.
func waitOp(t *testing.T, env *mockenv.Env, what string, op *gcp.Operation, err error) {
	t.Helper()
	if err == nil && !op.Done() {
		err = op.Wait(env.Ctx)
	}
	if err != nil {
		t.Fatalf("%s: %v", what, err)
	}
}

// cleanupOp deletes something at the end of the test, ignoring "not found".
func cleanupOp(t *testing.T, env *mockenv.Env, what string, del func() (*gcp.Operation, error)) {
	t.Cleanup(func() {
		op, err := del()
		if err == nil && !op.Done() {
			err = op.Wait(env.Ctx)
		}
		if err != nil && !direct.IsNotFound(err) {
			t.Errorf("deleting %s: %v", what, err)
		}
	})
}

// newServiceAttachment makes an internal passthrough load balancer and a
// service attachment publishing it, and returns the service attachment's
// name, projects/{p}/regions/{r}/serviceAttachments/{n}.
func newServiceAttachment(t *testing.T, env *mockenv.Env, region string) string {
	t.Helper()
	opts := env.GCPOptions()
	subnets, err := gcp.NewSubnetworksRESTClient(env.Ctx, opts...)
	if err != nil {
		t.Fatal(err)
	}
	healthChecks, err := gcp.NewRegionHealthChecksRESTClient(env.Ctx, opts...)
	if err != nil {
		t.Fatal(err)
	}
	backends, err := gcp.NewRegionBackendServicesRESTClient(env.Ctx, opts...)
	if err != nil {
		t.Fatal(err)
	}
	rules, err := gcp.NewForwardingRulesRESTClient(env.Ctx, opts...)
	if err != nil {
		t.Fatal(err)
	}
	attachments, err := gcp.NewServiceAttachmentsRESTClient(env.Ctx, opts...)
	if err != nil {
		t.Fatal(err)
	}
	// Registered first, so it runs after every delete below.
	t.Cleanup(func() {
		attachments.Close()
		rules.Close()
		backends.Close()
		healthChecks.Close()
		subnets.Close()
	})
	// Cleanups run last first: the service attachment goes before the
	// forwarding rule, and so on down to the network (newNetwork's).
	network := newNetwork(t, env, "psa-net")
	p := mockenv.Project
	regionPrefix := "projects/" + p + "/regions/" + region

	ilbSubnet := mockenv.Name("psa-ilb")
	op, err := subnets.Insert(env.Ctx, &pb.InsertSubnetworkRequest{Project: p, Region: region, SubnetworkResource: &pb.Subnetwork{
		Name: mockenv.Ptr(ilbSubnet), Network: mockenv.Ptr(network), IpCidrRange: mockenv.Ptr("10.10.0.0/24"),
	}})
	waitOp(t, env, "making the load balancer's subnet", op, err)
	cleanupOp(t, env, "subnet "+ilbSubnet, func() (*gcp.Operation, error) {
		return subnets.Delete(env.Ctx, &pb.DeleteSubnetworkRequest{Project: p, Region: region, Subnetwork: ilbSubnet})
	})
	natSubnet := mockenv.Name("psa-nat")
	op, err = subnets.Insert(env.Ctx, &pb.InsertSubnetworkRequest{Project: p, Region: region, SubnetworkResource: &pb.Subnetwork{
		Name: mockenv.Ptr(natSubnet), Network: mockenv.Ptr(network), IpCidrRange: mockenv.Ptr("10.10.1.0/24"),
		Purpose: mockenv.Ptr("PRIVATE_SERVICE_CONNECT"),
	}})
	waitOp(t, env, "making the NAT subnet", op, err)
	cleanupOp(t, env, "subnet "+natSubnet, func() (*gcp.Operation, error) {
		return subnets.Delete(env.Ctx, &pb.DeleteSubnetworkRequest{Project: p, Region: region, Subnetwork: natSubnet})
	})

	hc := mockenv.Name("psa-hc")
	op, err = healthChecks.Insert(env.Ctx, &pb.InsertRegionHealthCheckRequest{Project: p, Region: region, HealthCheckResource: &pb.HealthCheck{
		Name: mockenv.Ptr(hc), Type: mockenv.Ptr("TCP"), TcpHealthCheck: &pb.TCPHealthCheck{Port: mockenv.Ptr(int32(80))},
	}})
	waitOp(t, env, "making the health check", op, err)
	cleanupOp(t, env, "health check "+hc, func() (*gcp.Operation, error) {
		return healthChecks.Delete(env.Ctx, &pb.DeleteRegionHealthCheckRequest{Project: p, Region: region, HealthCheck: hc})
	})

	bs := mockenv.Name("psa-bs")
	op, err = backends.Insert(env.Ctx, &pb.InsertRegionBackendServiceRequest{Project: p, Region: region, BackendServiceResource: &pb.BackendService{
		Name: mockenv.Ptr(bs), LoadBalancingScheme: mockenv.Ptr("INTERNAL"), Protocol: mockenv.Ptr("TCP"),
		HealthChecks: []string{regionPrefix + "/healthChecks/" + hc},
	}})
	waitOp(t, env, "making the backend service", op, err)
	cleanupOp(t, env, "backend service "+bs, func() (*gcp.Operation, error) {
		return backends.Delete(env.Ctx, &pb.DeleteRegionBackendServiceRequest{Project: p, Region: region, BackendService: bs})
	})

	fr := mockenv.Name("psa-fr")
	op, err = rules.Insert(env.Ctx, &pb.InsertForwardingRuleRequest{Project: p, Region: region, ForwardingRuleResource: &pb.ForwardingRule{
		Name: mockenv.Ptr(fr), LoadBalancingScheme: mockenv.Ptr("INTERNAL"), IPProtocol: mockenv.Ptr("TCP"),
		Ports: []string{"80"}, BackendService: mockenv.Ptr(regionPrefix + "/backendServices/" + bs),
		Network: mockenv.Ptr(network), Subnetwork: mockenv.Ptr(regionPrefix + "/subnetworks/" + ilbSubnet),
		Labels: mockenv.Labels(),
	}})
	waitOp(t, env, "making the forwarding rule", op, err)
	cleanupOp(t, env, "forwarding rule "+fr, func() (*gcp.Operation, error) {
		return rules.Delete(env.Ctx, &pb.DeleteForwardingRuleRequest{Project: p, Region: region, ForwardingRule: fr})
	})

	sa := mockenv.Name("psa")
	op, err = attachments.Insert(env.Ctx, &pb.InsertServiceAttachmentRequest{Project: p, Region: region, ServiceAttachmentResource: &pb.ServiceAttachment{
		Name: mockenv.Ptr(sa), TargetService: mockenv.Ptr(regionPrefix + "/forwardingRules/" + fr),
		ConnectionPreference: mockenv.Ptr("ACCEPT_MANUAL"), NatSubnets: []string{regionPrefix + "/subnetworks/" + natSubnet},
		Description: mockenv.Ptr("made by the test"),
	}})
	waitOp(t, env, "making the service attachment", op, err)
	cleanupOp(t, env, "service attachment "+sa, func() (*gcp.Operation, error) {
		return attachments.Delete(env.Ctx, &pb.DeleteServiceAttachmentRequest{Project: p, Region: region, ServiceAttachment: sa})
	})
	return regionPrefix + "/serviceAttachments/" + sa
}

func TestComputeServiceAttachmentFields(t *testing.T) {
	const (
		namespace = "example"
		region    = "us-central1"
		name      = "fields"
	)
	gvk := krm.ComputeServiceAttachmentFieldsGVK
	env := mockenv.New(t, namespace)
	saName := newServiceAttachment(t, env, region)
	saShort := saName[strings.LastIndex(saName, "/")+1:]
	attachments, err := gcp.NewServiceAttachmentsRESTClient(env.Ctx, env.GCPOptions()...)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { attachments.Close() })
	get := func(t *testing.T) *pb.ServiceAttachment {
		t.Helper()
		sa, err := attachments.Get(env.Ctx, &pb.GetServiceAttachmentRequest{Project: mockenv.Project, Region: region, ServiceAttachment: saShort})
		if err != nil {
			t.Fatalf("reading the service attachment: %v", err)
		}
		return sa
	}
	patches := func() []string {
		var out []string
		for _, req := range env.Requests() {
			if strings.HasPrefix(req, "PATCH ") && strings.Contains(req, "/serviceAttachments/"+saShort) {
				out = append(out, req)
			}
		}
		return out
	}

	// Config Connector's ComputeServiceAttachment for it (not reconciled
	// here: read to resolve resourceRef by name, from its status.selfLink
	// as the DCL-based controller writes it).
	cc := &unstructured.Unstructured{Object: map[string]any{"spec": map[string]any{
		"location": region, "resourceID": saShort, "connectionPreference": "ACCEPT_MANUAL",
		"natSubnets":       []any{map[string]any{"external": "nat"}},
		"projectRef":       map[string]any{"external": mockenv.Project},
		"targetServiceRef": map[string]any{"external": "fr"},
	}}}
	cc.SetGroupVersionKind(configConnectorServiceAttachmentGVK)
	cc.SetNamespace(namespace)
	cc.SetName("attachment")
	if err := env.Kube.Create(env.Ctx, cc); err != nil {
		t.Fatal(err)
	}
	if err := unstructured.SetNestedField(cc.Object, "https://www.googleapis.com/compute/v1/"+saName, "status", "selfLink"); err != nil {
		t.Fatal(err)
	}
	if err := env.Kube.Status().Update(env.Ctx, cc); err != nil {
		t.Fatal(err)
	}

	getObj := func(t *testing.T, name string) *krm.ComputeServiceAttachmentFields {
		t.Helper()
		o := &krm.ComputeServiceAttachmentFields{}
		if err := env.Kube.Get(env.Ctx, types.NamespacedName{Namespace: namespace, Name: name}, o); err != nil {
			t.Fatal(err)
		}
		return o
	}
	resync := func(t *testing.T, name string) {
		t.Helper()
		o := getObj(t, name)
		o.Annotations = map[string]string{k8s.InternalForceReconcileAnnotation: time.Now().String()}
		if err := env.Kube.Update(env.Ctx, o); err != nil {
			t.Fatal(err)
		}
		time.Sleep(3 * time.Second)
		env.WaitReady(gvk, namespace, name)
	}
	newFields := func(name string, ref *refs.ComputeServiceAttachmentRef, reconcile *bool, limit *int32) *krm.ComputeServiceAttachmentFields {
		return &krm.ComputeServiceAttachmentFields{
			ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: name, Labels: mockenv.Labels()},
			Spec: krm.ComputeServiceAttachmentFieldsSpec{
				ResourceRef: ref, ReconcileConnections: reconcile, PropagatedConnectionLimit: limit,
			},
		}
	}

	obj := newFields(name, &refs.ComputeServiceAttachmentRef{Name: "attachment"}, mockenv.Ptr(true), mockenv.Ptr(int32(10)))
	if err := env.Kube.Create(env.Ctx, obj); err != nil {
		t.Fatal(err)
	}

	t.Run("create sets the two fields, and only them", func(t *testing.T) {
		u := env.WaitReady(gvk, namespace, name)
		if ref, _, _ := unstructured.NestedString(u.Object, "status", "externalRef"); ref != saName {
			t.Errorf("status.externalRef = %q, want %q", ref, saName)
		}
		sa := get(t)
		if !sa.GetReconcileConnections() || sa.GetPropagatedConnectionLimit() != 10 {
			t.Errorf("reconcileConnections = %v, propagatedConnectionLimit = %d; want true, 10", sa.GetReconcileConnections(), sa.GetPropagatedConnectionLimit())
		}
		if sa.GetDescription() != "made by the test" || sa.GetConnectionPreference() != "ACCEPT_MANUAL" {
			t.Errorf("the other fields changed: description %q, connectionPreference %q", sa.GetDescription(), sa.GetConnectionPreference())
		}
		if p := patches(); len(p) != 1 {
			t.Errorf("patches = %v, want one", p)
		}
	})

	t.Run("an update of Config Connector's fields keeps them", func(t *testing.T) {
		// What the DCL sends when a ComputeServiceAttachment's consumer
		// lists or description change: its own fields and the fingerprint,
		// never these two (newUpdateServiceAttachmentPatchRequest).
		before := get(t)
		op, err := attachments.Patch(env.Ctx, &pb.PatchServiceAttachmentRequest{
			Project: mockenv.Project, Region: region, ServiceAttachment: saShort,
			ServiceAttachmentResource: &pb.ServiceAttachment{
				Name: mockenv.Ptr(saShort), Description: mockenv.Ptr("changed by config connector"),
				ConnectionPreference: mockenv.Ptr("ACCEPT_MANUAL"), NatSubnets: before.GetNatSubnets(),
				ConsumerAcceptLists: []*pb.ServiceAttachmentConsumerProjectLimit{{ProjectIdOrNum: mockenv.Ptr(mockenv.Project), ConnectionLimit: mockenv.Ptr(uint32(5))}},
				Fingerprint:         before.Fingerprint,
			},
		})
		waitOp(t, env, "Config Connector's patch", op, err)
		sa := get(t)
		if sa.GetDescription() != "changed by config connector" {
			t.Errorf("description = %q, want the change", sa.GetDescription())
		}
		if !sa.GetReconcileConnections() || sa.GetPropagatedConnectionLimit() != 10 {
			t.Errorf("after Config Connector's patch reconcileConnections = %v, propagatedConnectionLimit = %d; want true, 10", sa.GetReconcileConnections(), sa.GetPropagatedConnectionLimit())
		}
	})

	t.Run("a resync writes nothing", func(t *testing.T) {
		before := len(patches())
		resync(t, name)
		if n := len(patches()) - before; n != 0 {
			t.Errorf("a resync patched the service attachment %d times", n)
		}
	})

	t.Run("a changed field is patched, and a field left out is not owned", func(t *testing.T) {
		o := getObj(t, name)
		o.Spec.ReconcileConnections = nil
		o.Spec.PropagatedConnectionLimit = mockenv.Ptr(int32(20))
		if err := env.Kube.Update(env.Ctx, o); err != nil {
			t.Fatal(err)
		}
		waitFor(t, func() bool { return get(t).GetPropagatedConnectionLimit() == 20 })
		env.WaitReady(gvk, namespace, name)
		if !get(t).GetReconcileConnections() {
			t.Errorf("a field the spec no longer sets was changed")
		}
		// Changed outside, a field no longer owned stays changed.
		op, err := attachments.Patch(env.Ctx, &pb.PatchServiceAttachmentRequest{
			Project: mockenv.Project, Region: region, ServiceAttachment: saShort,
			ServiceAttachmentResource: &pb.ServiceAttachment{ReconcileConnections: mockenv.Ptr(false), Fingerprint: get(t).Fingerprint},
		})
		waitOp(t, env, "changing reconcileConnections outside", op, err)
		resync(t, name)
		if get(t).GetReconcileConnections() {
			t.Errorf("a resync set reconcileConnections, which the spec no longer owns")
		}
	})

	t.Run("a field changed outside is set again", func(t *testing.T) {
		op, err := attachments.Patch(env.Ctx, &pb.PatchServiceAttachmentRequest{
			Project: mockenv.Project, Region: region, ServiceAttachment: saShort,
			ServiceAttachmentResource: &pb.ServiceAttachment{PropagatedConnectionLimit: mockenv.Ptr(uint32(3)), Fingerprint: get(t).Fingerprint},
		})
		waitOp(t, env, "changing propagatedConnectionLimit outside", op, err)
		resync(t, name)
		if got := get(t).GetPropagatedConnectionLimit(); got != 20 {
			t.Errorf("after a resync propagatedConnectionLimit = %d, want 20", got)
		}
	})

	t.Run("a second object for the service attachment is refused", func(t *testing.T) {
		dup := newFields("dup", &refs.ComputeServiceAttachmentRef{External: saName}, mockenv.Ptr(true), nil)
		if err := env.Kube.Create(env.Ctx, dup); err != nil {
			t.Fatal(err)
		}
		if msg := env.WaitNotReady(gvk, namespace, "dup"); !strings.Contains(msg, "already sets the fields of") {
			t.Errorf("Ready message = %q, want the second object refused", msg)
		}
		if get(t).GetReconcileConnections() {
			t.Errorf("the refused object changed the service attachment")
		}
		if err := env.Kube.Delete(env.Ctx, dup); err != nil {
			t.Fatal(err)
		}
		env.WaitGone(dup)
	})

	t.Run("a missing service attachment is waited for, never made", func(t *testing.T) {
		missing := "projects/" + mockenv.Project + "/regions/" + region + "/serviceAttachments/" + mockenv.Name("never-made")
		w := newFields("waits", &refs.ComputeServiceAttachmentRef{External: missing}, nil, mockenv.Ptr(int32(1)))
		if err := env.Kube.Create(env.Ctx, w); err != nil {
			t.Fatal(err)
		}
		if msg := env.WaitNotReady(gvk, namespace, "waits"); !strings.Contains(msg, "does not exist yet") {
			t.Errorf("Ready message = %q, want the missing service attachment waited for", msg)
		}
		if err := env.Kube.Delete(env.Ctx, w); err != nil {
			t.Fatal(err)
		}
		env.WaitGone(w)
	})

	t.Run("delete leaves the fields and the service attachment", func(t *testing.T) {
		before := len(patches())
		if err := env.Kube.Delete(env.Ctx, obj); err != nil {
			t.Fatal(err)
		}
		env.WaitGone(&krm.ComputeServiceAttachmentFields{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: name}})
		if n := len(patches()) - before; n != 0 {
			t.Errorf("delete patched the service attachment %d times", n)
		}
		sa := get(t)
		if sa.GetPropagatedConnectionLimit() != 20 || sa.GetDescription() != "changed by config connector" {
			t.Errorf("after delete propagatedConnectionLimit = %d, description %q", sa.GetPropagatedConnectionLimit(), sa.GetDescription())
		}
	})

	t.Run("abandon leaves the fields too", func(t *testing.T) {
		k := newFields("kept", &refs.ComputeServiceAttachmentRef{Name: "attachment"}, nil, mockenv.Ptr(int32(30)))
		k.Annotations = map[string]string{"cnrm.cloud.google.com/deletion-policy": "abandon"}
		if err := env.Kube.Create(env.Ctx, k); err != nil {
			t.Fatal(err)
		}
		env.WaitReady(gvk, namespace, "kept")
		if err := env.Kube.Delete(env.Ctx, k); err != nil {
			t.Fatal(err)
		}
		env.WaitGone(k)
		if got := get(t).GetPropagatedConnectionLimit(); got != 30 {
			t.Errorf("after an abandoning delete propagatedConnectionLimit = %d, want 30", got)
		}
	})
}
