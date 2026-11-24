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

package privateca_test

// PrivateCACAPoolFields end to end on the fake cloud (pkg/test/mockenv)
// or a real project (docs/live-runs.md). The CA pool is the run's own
// (mockenv.Name), DevOps tier in us-central1, with no certificate
// authority in it, so it costs nothing; the test deletes it.
//
// A live run needs privateca.googleapis.com on and roles/privateca.admin
// for the live-test service account; the Private CA tests skip unless
// E2E_PRIVATECA=true.

import (
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	api "cloud.google.com/go/security/privateca/apiv1"
	pb "cloud.google.com/go/security/privateca/apiv1/privatecapb"
	"github.com/GoogleCloudPlatform/k8s-config-connector/apis/privateca/privatecarefs"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/k8s"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	krm "github.com/highfivelabs/config-connector-extensions/apis/privateca/v1alpha1"
	"github.com/highfivelabs/config-connector-extensions/pkg/test/mockenv"
)

const location = "us-central1"

var configConnectorCAPoolGVK = schema.GroupVersionKind{Group: "privateca.cnrm.cloud.google.com", Version: "v1beta1", Kind: "PrivateCACAPool"}

// skipLiveWithoutPrivateCA skips a live run where the account may not use
// Certificate Authority Service.
func skipLiveWithoutPrivateCA(t *testing.T) {
	t.Helper()
	if mockenv.Live() && os.Getenv("E2E_PRIVATECA") != "true" {
		t.Skip("a live Private CA test needs privateca.googleapis.com on and roles/privateca.admin for the live-test service account; set E2E_PRIVATECA=true where it has them (docs/live-runs.md)")
	}
}

func newCAClient(t *testing.T, env *mockenv.Env) *api.CertificateAuthorityClient {
	t.Helper()
	c, err := api.NewCertificateAuthorityRESTClient(env.Ctx, env.GCPOptions()...)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { c.Close() })
	return c
}

// patchesOf is the PATCH requests the run made to the resource named by
// path (".../caPools/<id>", ".../certificateTemplates/<id>").
func patchesOf(env *mockenv.Env, path string) []string {
	var out []string
	for _, req := range env.Requests() {
		if strings.HasPrefix(req, "PATCH ") && strings.Contains(req, path+"?") {
			out = append(out, req)
		}
	}
	return out
}

func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(mockenv.Timeout())
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(time.Second)
	}
}

// resync makes the controller reconcile obj again and waits for Ready.
func resync(t *testing.T, env *mockenv.Env, gvk schema.GroupVersionKind, obj client.Object) {
	t.Helper()
	if err := env.Kube.Get(env.Ctx, client.ObjectKeyFromObject(obj), obj); err != nil {
		t.Fatal(err)
	}
	obj.SetAnnotations(map[string]string{k8s.InternalForceReconcileAnnotation: time.Now().String()})
	if err := env.Kube.Update(env.Ctx, obj); err != nil {
		t.Fatal(err)
	}
	time.Sleep(3 * time.Second)
	env.WaitReady(gvk, obj.GetNamespace(), obj.GetName())
}

func TestPrivateCACAPoolFields(t *testing.T) {
	const (
		namespace = "example"
		name      = "fields"
	)
	skipLiveWithoutPrivateCA(t)
	gvk := krm.PrivateCACAPoolFieldsGVK
	env := mockenv.New(t, namespace)
	c := newCAClient(t, env)

	poolID := mockenv.Name("cpf")
	parent := "projects/" + mockenv.Project + "/locations/" + location
	poolName := parent + "/caPools/" + poolID
	op, err := c.CreateCaPool(env.Ctx, &pb.CreateCaPoolRequest{
		Parent: parent, CaPoolId: poolID,
		CaPool: &pb.CaPool{
			Tier:   pb.CaPool_DEVOPS,
			Labels: mockenv.Labels(),
			IssuancePolicy: &pb.CaPool_IssuancePolicy{
				MaximumLifetime: durationpb.New(24 * time.Hour),
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := op.Wait(env.Ctx); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		op, err := c.DeleteCaPool(env.Ctx, &pb.DeleteCaPoolRequest{Name: poolName})
		if err == nil {
			err = op.Wait(env.Ctx)
		}
		if err != nil {
			t.Errorf("deleting CA pool %s: %v", poolName, err)
		}
	})

	// Config Connector's PrivateCACAPool for the pool (not reconciled
	// here: read to resolve resourceRef by name, from its spec, as Config
	// Connector's reference does for a DCL object with no externalRef).
	cc := &unstructured.Unstructured{Object: map[string]any{"spec": map[string]any{
		"projectRef": map[string]any{"external": "projects/" + mockenv.Project},
		"location":   location, "resourceID": poolID, "tier": "DEVOPS",
	}}}
	cc.SetGroupVersionKind(configConnectorCAPoolGVK)
	cc.SetNamespace(namespace)
	cc.SetName("pool")
	if err := env.Kube.Create(env.Ctx, cc); err != nil {
		t.Fatal(err)
	}

	getPool := func(t *testing.T) *pb.CaPool {
		t.Helper()
		p, err := c.GetCaPool(env.Ctx, &pb.GetCaPoolRequest{Name: poolName})
		if err != nil {
			t.Fatalf("reading the pool: %v", err)
		}
		return p
	}
	patches := func() []string { return patchesOf(env, "/caPools/"+poolID) }
	getObj := func(t *testing.T, name string) *krm.PrivateCACAPoolFields {
		t.Helper()
		o := &krm.PrivateCACAPoolFields{}
		if err := env.Kube.Get(env.Ctx, types.NamespacedName{Namespace: namespace, Name: name}, o); err != nil {
			t.Fatal(err)
		}
		return o
	}
	newFields := func(name string, ref *privatecarefs.PrivateCACAPoolRef) *krm.PrivateCACAPoolFields {
		return &krm.PrivateCACAPoolFields{
			ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: name, Labels: mockenv.Labels()},
			Spec:       krm.PrivateCACAPoolFieldsSpec{ResourceRef: ref, IssuancePolicy: &krm.CAPoolFields_IssuancePolicy{}},
		}
	}
	checkOwned := func(t *testing.T, p *pb.CaPool, backdate time.Duration, dns []string) {
		t.Helper()
		if got := p.GetIssuancePolicy().GetBackdateDuration().AsDuration(); got != backdate {
			t.Errorf("backdateDuration = %v, want %v", got, backdate)
		}
		nc := p.GetIssuancePolicy().GetBaselineValues().GetNameConstraints()
		if got := nc.GetPermittedDnsNames(); !slices.Equal(got, dns) {
			t.Errorf("nameConstraints.permittedDnsNames = %v, want %v", got, dns)
		}
	}

	obj := newFields(name, &privatecarefs.PrivateCACAPoolRef{Name: "pool"})
	obj.Spec.IssuancePolicy.BackdateDuration = mockenv.Ptr("3600s")
	obj.Spec.IssuancePolicy.BaselineValues = &krm.CAPoolFields_BaselineValues{NameConstraints: &krm.X509Parameters_NameConstraints{
		Critical:          mockenv.Ptr(true),
		PermittedDNSNames: []string{"example.com"},
		ExcludedDNSNames:  []string{"bad.example.com"},
	}}
	if err := env.Kube.Create(env.Ctx, obj); err != nil {
		t.Fatal(err)
	}

	t.Run("create sets the fields and keeps the pool's own", func(t *testing.T) {
		u := env.WaitReady(gvk, namespace, name)
		if ref, _, _ := unstructured.NestedString(u.Object, "status", "externalRef"); ref != poolName {
			t.Errorf("status.externalRef = %q, want %q", ref, poolName)
		}
		p := getPool(t)
		checkOwned(t, p, time.Hour, []string{"example.com"})
		nc := p.GetIssuancePolicy().GetBaselineValues().GetNameConstraints()
		if !nc.GetCritical() || !slices.Equal(nc.GetExcludedDnsNames(), []string{"bad.example.com"}) {
			t.Errorf("nameConstraints = %v", nc)
		}
		if got := p.GetIssuancePolicy().GetMaximumLifetime().AsDuration(); got != 24*time.Hour {
			t.Errorf("the pool's maximumLifetime = %v, want 24h kept", got)
		}
		if p.GetTier() != pb.CaPool_DEVOPS {
			t.Errorf("tier = %v", p.GetTier())
		}
		if pp := patches(); len(pp) != 1 || !strings.Contains(pp[0], "updateMask=issuancePolicy") {
			t.Errorf("patches = %v, want one with updateMask=issuancePolicy", pp)
		}
	})

	t.Run("a resync writes nothing", func(t *testing.T) {
		before := len(patches())
		resync(t, env, gvk, getObj(t, name))
		if n := len(patches()) - before; n != 0 {
			t.Errorf("a resync patched the pool %d times", n)
		}
	})

	t.Run("an update of Config Connector's issuance policy is undone for ours at the next reconcile", func(t *testing.T) {
		// What Config Connector's update of a PrivateCACAPool sends when
		// its issuance policy changes: the policy it knows, with a mask
		// of issuancePolicy (the DCL's top-level mask).
		op, err := c.UpdateCaPool(env.Ctx, &pb.UpdateCaPoolRequest{
			CaPool: &pb.CaPool{Name: poolName, IssuancePolicy: &pb.CaPool_IssuancePolicy{
				MaximumLifetime: durationpb.New(48 * time.Hour),
			}},
			UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"issuance_policy"}},
		})
		if err == nil {
			_, err = op.Wait(env.Ctx)
		}
		if err != nil {
			t.Fatal(err)
		}
		checkOwned(t, getPool(t), 0, nil)
		resync(t, env, gvk, getObj(t, name))
		p := getPool(t)
		checkOwned(t, p, time.Hour, []string{"example.com"})
		if got := p.GetIssuancePolicy().GetMaximumLifetime().AsDuration(); got != 48*time.Hour {
			t.Errorf("Config Connector's maximumLifetime = %v, want its change (48h) kept", got)
		}
	})

	t.Run("a changed field is written", func(t *testing.T) {
		o := getObj(t, name)
		o.Spec.IssuancePolicy.BackdateDuration = mockenv.Ptr("7200s")
		if err := env.Kube.Update(env.Ctx, o); err != nil {
			t.Fatal(err)
		}
		waitFor(t, "the backdate duration to change", func() bool {
			return getPool(t).GetIssuancePolicy().GetBackdateDuration().AsDuration() == 2*time.Hour
		})
		env.WaitReady(gvk, namespace, name)
		checkOwned(t, getPool(t), 2*time.Hour, []string{"example.com"})
	})

	t.Run("a second object for the pool is refused", func(t *testing.T) {
		dup := newFields("dup", &privatecarefs.PrivateCACAPoolRef{External: poolName})
		dup.Spec.IssuancePolicy.BackdateDuration = mockenv.Ptr("60s")
		if err := env.Kube.Create(env.Ctx, dup); err != nil {
			t.Fatal(err)
		}
		if msg := env.WaitNotReady(gvk, namespace, "dup"); !strings.Contains(msg, "already sets the fields of") {
			t.Errorf("Ready message = %q, want the second object refused", msg)
		}
		dup.Annotations = map[string]string{"cnrm.cloud.google.com/deletion-policy": "abandon"}
		if err := env.Kube.Delete(env.Ctx, dup); err != nil {
			t.Fatal(err)
		}
		env.WaitGone(dup)
		checkOwned(t, getPool(t), 2*time.Hour, []string{"example.com"})
	})

	t.Run("a missing pool is waited for, never made", func(t *testing.T) {
		w := newFields("waits", &privatecarefs.PrivateCACAPoolRef{External: parent + "/caPools/" + mockenv.Name("never-made")})
		w.Spec.IssuancePolicy.BackdateDuration = mockenv.Ptr("60s")
		if err := env.Kube.Create(env.Ctx, w); err != nil {
			t.Fatal(err)
		}
		if msg := env.WaitNotReady(gvk, namespace, "waits"); !strings.Contains(msg, "does not exist yet") {
			t.Errorf("Ready message = %q, want the missing pool waited for", msg)
		}
		if err := env.Kube.Delete(env.Ctx, w); err != nil {
			t.Fatal(err)
		}
		env.WaitGone(w)
	})

	t.Run("delete clears the fields and keeps the pool's own", func(t *testing.T) {
		if err := env.Kube.Delete(env.Ctx, obj); err != nil {
			t.Fatal(err)
		}
		env.WaitGone(&krm.PrivateCACAPoolFields{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: name}})
		p := getPool(t)
		checkOwned(t, p, 0, nil)
		if p.GetIssuancePolicy().GetBaselineValues().GetNameConstraints() != nil {
			t.Errorf("after delete the name constraints are %v, want none", p.GetIssuancePolicy().GetBaselineValues().GetNameConstraints())
		}
		if got := p.GetIssuancePolicy().GetMaximumLifetime().AsDuration(); got != 48*time.Hour {
			t.Errorf("delete changed the pool's maximumLifetime: %v", got)
		}
	})

	t.Run("abandon keeps the fields", func(t *testing.T) {
		k := newFields("kept", &privatecarefs.PrivateCACAPoolRef{Name: "pool"})
		k.Spec.IssuancePolicy.BackdateDuration = mockenv.Ptr("120s")
		k.Annotations = map[string]string{"cnrm.cloud.google.com/deletion-policy": "abandon"}
		if err := env.Kube.Create(env.Ctx, k); err != nil {
			t.Fatal(err)
		}
		env.WaitReady(gvk, namespace, "kept")
		if err := env.Kube.Delete(env.Ctx, k); err != nil {
			t.Fatal(err)
		}
		env.WaitGone(k)
		if got := getPool(t).GetIssuancePolicy().GetBackdateDuration().AsDuration(); got != 2*time.Minute {
			t.Errorf("after an abandoning delete backdateDuration = %v, want 2m", got)
		}
	})
}
