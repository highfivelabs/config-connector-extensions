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

// PrivateCACertificateAuthority (ours) end to end on the fake cloud
// (pkg/test/mockenv; mockgcp patch 0062) or a real project
// (docs/live-runs.md; E2E_PRIVATECA=true, as the other Private CA tests).
// The case: a root authority in the run's own DevOps tier CA pool, with
// the fields Config Connector's kind lacks (name constraints, an OCSP
// server, a subject key ID, user-defined access URLs), enabled after
// create; its labels and URLs changed; and deleted without the grace
// period, so the test can delete the pool. A live authority is billed
// while it exists (by the hour, prorated); the test deletes it within
// minutes.

import (
	"slices"
	"strings"
	"testing"

	pb "cloud.google.com/go/security/privateca/apiv1/privatecapb"
	"github.com/GoogleCloudPlatform/k8s-config-connector/apis/privateca/privatecarefs"
	refsv1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/apis/refs/v1beta1"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"

	krm "github.com/highfivelabs/config-connector-extensions/apis/privateca/v1alpha1"
	"github.com/highfivelabs/config-connector-extensions/pkg/test/mockenv"
)

func TestPrivateCACertificateAuthority(t *testing.T) {
	const namespace = "example"
	skipLiveWithoutPrivateCA(t)
	gvk := krm.PrivateCACertificateAuthorityGVK
	env := mockenv.New(t, namespace)
	c := newCAClient(t, env)

	poolID := mockenv.Name("cas")
	parent := "projects/" + mockenv.Project + "/locations/" + location
	poolName := parent + "/caPools/" + poolID
	op, err := c.CreateCaPool(env.Ctx, &pb.CreateCaPoolRequest{
		Parent: parent, CaPoolId: poolID,
		CaPool: &pb.CaPool{Tier: pb.CaPool_DEVOPS, Labels: mockenv.Labels()},
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

	caID := mockenv.Name("root")
	caName := poolName + "/certificateAuthorities/" + caID
	getCA := func(t *testing.T) *pb.CertificateAuthority {
		t.Helper()
		ca, err := c.GetCertificateAuthority(env.Ctx, &pb.GetCertificateAuthorityRequest{Name: caName})
		if err != nil {
			t.Fatalf("reading the authority: %v", err)
		}
		return ca
	}
	writes := func(method string) int {
		n := 0
		for _, r := range env.Requests() {
			if strings.HasPrefix(r, method+" ") && strings.Contains(r, "/certificateAuthorities/"+caID) {
				n++
			}
		}
		return n
	}
	getObj := func(t *testing.T, name string) *krm.PrivateCACertificateAuthority {
		t.Helper()
		o := &krm.PrivateCACertificateAuthority{}
		if err := env.Kube.Get(env.Ctx, types.NamespacedName{Namespace: namespace, Name: name}, o); err != nil {
			t.Fatal(err)
		}
		return o
	}
	update := func(t *testing.T, name string, change func(*krm.PrivateCACertificateAuthority)) error {
		t.Helper()
		o := getObj(t, name)
		change(o)
		return env.Kube.Update(env.Ctx, o)
	}
	labels := mockenv.Labels()
	if labels == nil {
		labels = map[string]string{}
	}
	labels["team"] = "platform"
	newCA := func(name string) *krm.PrivateCACertificateAuthority {
		return &krm.PrivateCACertificateAuthority{
			ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: name, Labels: labels},
			Spec: krm.PrivateCACertificateAuthoritySpec{
				ProjectRef: refsv1beta1.ProjectRef{External: "projects/" + mockenv.Project},
				Location:   location,
				ResourceID: direct.LazyPtr(caID),
				CaPoolRef:  privatecarefs.PrivateCACAPoolRef{External: poolName},
				Type:       "SELF_SIGNED",
				Lifetime:   "86400s",
				KeySpec:    krm.CertificateAuthority_KeyVersionSpec{Algorithm: direct.LazyPtr("EC_P256_SHA256")},
				Config: krm.CertificateConfig{
					SubjectConfig: krm.CertificateConfig_SubjectConfig{
						Subject: krm.Subject{CommonName: direct.LazyPtr("cce test root"), Organization: direct.LazyPtr("cce tests")},
					},
					X509Config: krm.CertificateConfig_X509Config{
						CaOptions: &krm.X509Parameters_CAOptions{IsCA: direct.LazyPtr(true)},
						KeyUsage: &krm.CertificateAuthority_KeyUsage{
							BaseKeyUsage: &krm.KeyUsage_KeyUsageOptions{CertSign: direct.LazyPtr(true), CrlSign: direct.LazyPtr(true)},
						},
						AiaOcspServers: []string{"http://ocsp.example.com"},
						NameConstraints: &krm.X509Parameters_NameConstraints{
							Critical:          direct.LazyPtr(true),
							PermittedDNSNames: []string{"example.com"},
							ExcludedDNSNames:  []string{"internal.example.com"},
						},
					},
					SubjectKeyID: &krm.CertificateConfig_KeyID{KeyID: "0123456789abcdef0123456789abcdef01234567"},
				},
				UserDefinedAccessUrls: &krm.CertificateAuthority_UserDefinedAccessUrls{
					AiaIssuingCertificateUrls: []string{"http://pki.example.com/ca.crt"},
				},
				SkipGracePeriod: direct.LazyPtr(true),
			},
		}
	}

	t.Run("refused by the CRD: a key spec with both or neither", func(t *testing.T) {
		bad := newCA("bad-key")
		bad.Spec.KeySpec.CloudKmsKeyVersionRef = nil
		bad.Spec.KeySpec.Algorithm = nil
		if err := env.Kube.Create(env.Ctx, bad); err == nil || !strings.Contains(err.Error(), "keySpec takes one of algorithm and cloudKmsKeyVersionRef") {
			t.Errorf("creating an authority without a key: %v, want refused", err)
		}
	})

	const name = "root"
	if err := env.Kube.Create(env.Ctx, newCA(name)); err != nil {
		t.Fatal(err)
	}

	t.Run("create makes the authority with the fields Config Connector's kind lacks, and enables it", func(t *testing.T) {
		u := env.WaitReady(gvk, namespace, name)
		ca := getCA(t)
		if ca.GetState() != pb.CertificateAuthority_ENABLED {
			t.Errorf("state = %v, want ENABLED", ca.GetState())
		}
		x := ca.GetConfig().GetX509Config()
		if nc := x.GetNameConstraints(); !nc.GetCritical() || !slices.Equal(nc.GetPermittedDnsNames(), []string{"example.com"}) || !slices.Equal(nc.GetExcludedDnsNames(), []string{"internal.example.com"}) {
			t.Errorf("name constraints = %v", nc)
		}
		if !slices.Equal(x.GetAiaOcspServers(), []string{"http://ocsp.example.com"}) {
			t.Errorf("aiaOcspServers = %v", x.GetAiaOcspServers())
		}
		if !mockenv.Live() {
			// The real API may compute its own key ID; the fake keeps the
			// one sent.
			if got := ca.GetConfig().GetSubjectKeyId().GetKeyId(); got != "0123456789abcdef0123456789abcdef01234567" {
				t.Errorf("subjectKeyId = %q", got)
			}
		}
		if got := ca.GetUserDefinedAccessUrls().GetAiaIssuingCertificateUrls(); !slices.Equal(got, []string{"http://pki.example.com/ca.crt"}) {
			t.Errorf("userDefinedAccessUrls = %v", ca.GetUserDefinedAccessUrls())
		}
		if ca.GetLabels()["team"] != "platform" {
			t.Errorf("labels = %v", ca.GetLabels())
		}
		if ref, _, _ := unstructured.NestedString(u.Object, "status", "externalRef"); ref != caName {
			t.Errorf("status.externalRef = %q, want %q", ref, caName)
		}
		if state, _, _ := unstructured.NestedString(u.Object, "status", "state"); state != "ENABLED" {
			t.Errorf("status.state = %q, want ENABLED", state)
		}
		if pems, _, _ := unstructured.NestedStringSlice(u.Object, "status", "pemCaCertificates"); len(pems) == 0 {
			t.Errorf("status.pemCaCertificates is empty")
		}
	})

	t.Run("a resync writes nothing", func(t *testing.T) {
		before := writes("PATCH") + writes("POST")
		resync(t, env, gvk, getObj(t, name))
		if n := writes("PATCH") + writes("POST") - before; n != 0 {
			t.Errorf("a resync wrote %d times", n)
		}
	})

	t.Run("labels and user-defined access URLs change with one update", func(t *testing.T) {
		before := writes("PATCH")
		if err := update(t, name, func(o *krm.PrivateCACertificateAuthority) {
			delete(o.Labels, "team")
			o.Spec.UserDefinedAccessUrls = &krm.CertificateAuthority_UserDefinedAccessUrls{
				AiaIssuingCertificateUrls: []string{"http://pki2.example.com/ca.crt"},
				CrlAccessUrls:             []string{"http://pki2.example.com/ca.crl"},
			}
		}); err != nil {
			t.Fatal(err)
		}
		waitFor(t, "the authority's labels and URLs", func() bool {
			ca := getCA(t)
			u := ca.GetUserDefinedAccessUrls()
			_, team := ca.GetLabels()["team"]
			return !team && slices.Equal(u.GetAiaIssuingCertificateUrls(), []string{"http://pki2.example.com/ca.crt"}) &&
				slices.Equal(u.GetCrlAccessUrls(), []string{"http://pki2.example.com/ca.crl"})
		})
		env.WaitReady(gvk, namespace, name)
		if n := writes("PATCH") - before; n != 1 {
			t.Errorf("the change sent %d updates, want 1", n)
		}
	})

	t.Run("what the API takes only at create is immutable", func(t *testing.T) {
		for field, change := range map[string]func(*krm.PrivateCACertificateAuthority){
			"config is immutable": func(o *krm.PrivateCACertificateAuthority) {
				o.Spec.Config.X509Config.AiaOcspServers = []string{"http://other.example.com"}
			},
			"lifetime is immutable": func(o *krm.PrivateCACertificateAuthority) { o.Spec.Lifetime = "172800s" },
			"keySpec is immutable": func(o *krm.PrivateCACertificateAuthority) {
				o.Spec.KeySpec.Algorithm = direct.LazyPtr("EC_P384_SHA384")
			},
			"type is immutable":      func(o *krm.PrivateCACertificateAuthority) { o.Spec.Type = "SUBORDINATE" },
			"caPoolRef is immutable": func(o *krm.PrivateCACertificateAuthority) { o.Spec.CaPoolRef.External = poolName + "x" },
		} {
			if err := update(t, name, change); err == nil || !strings.Contains(err.Error(), field) {
				t.Errorf("%s: got %v, want the API server to refuse it", field, err)
			}
		}
	})

	t.Run("delete disables the authority, then deletes it", func(t *testing.T) {
		disables := writes("POST")
		if err := env.Kube.Delete(env.Ctx, getObj(t, name)); err != nil {
			t.Fatal(err)
		}
		env.WaitGone(&krm.PrivateCACertificateAuthority{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: name}})
		ca, err := c.GetCertificateAuthority(env.Ctx, &pb.GetCertificateAuthorityRequest{Name: caName})
		if err == nil && ca.GetState() != pb.CertificateAuthority_DELETED {
			t.Errorf("after delete, the authority is %v, want it gone", ca.GetState())
		}
		var disabled bool
		for _, r := range env.Requests() {
			if strings.HasPrefix(r, "POST ") && strings.Contains(r, "/certificateAuthorities/"+caID+":disable") {
				disabled = true
			}
		}
		if !disabled || writes("POST") == disables {
			t.Errorf("the delete did not disable the authority first")
		}
	})
}
