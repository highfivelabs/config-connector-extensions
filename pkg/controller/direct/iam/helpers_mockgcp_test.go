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

package iam_test

// What the IAM tests share against a real project (docs/live-runs.md): a
// workload identity pool of the test's own, and for provider keys a SAML
// provider in it. mockgcp does not serve pools or providers (its
// namespaces, managed identities and keys do not check their parent), so
// in the fake cloud these are only the Config Connector objects the tests
// already make. A deleted pool is kept for 30 days and its ID cannot be
// reused, so each test has its own pool ID, mockenv.Name(base).

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"fmt"
	"math/big"
	"testing"
	"time"

	api "google.golang.org/api/iam/v1"

	"github.com/highfivelabs/config-connector-extensions/pkg/test/mockenv"
)

// usePool sets the package's pool to mockenv.Name(base) for this test and,
// against a real project, makes it (in mode, "" for the default) and
// deletes it when the test ends, with everything in it.
func usePool(t *testing.T, env *mockenv.Env, cloud *api.Service, base, mode string) {
	t.Helper()
	pool = mockenv.Name(base)
	if !mockenv.Live() {
		return
	}
	parent := "projects/" + mockenv.Project + "/locations/global"
	op, err := cloud.Projects.Locations.WorkloadIdentityPools.Create(parent, &api.WorkloadIdentityPool{
		DisplayName: base, Description: "config-connector-extensions live test " + mockenv.RunID, Mode: mode,
	}).WorkloadIdentityPoolId(pool).Context(env.Ctx).Do()
	if err == nil {
		err = waitIAM(env, cloud, op)
	}
	if err != nil {
		t.Fatalf("making pool %s: %v", pool, err)
	}
	name := parent + "/workloadIdentityPools/" + pool
	t.Cleanup(func() {
		if _, err := cloud.Projects.Locations.WorkloadIdentityPools.Delete(name).Context(env.Ctx).Do(); err != nil {
			t.Errorf("deleting pool %s: %v", name, err)
		}
	})
}

// useSAMLProvider makes, against a real project, a SAML provider id in the
// test's pool, with identity provider metadata around a fresh self-signed
// certificate. It goes when the pool does.
func useSAMLProvider(t *testing.T, env *mockenv.Env, cloud *api.Service, id string) {
	t.Helper()
	if !mockenv.Live() {
		return
	}
	parent := "projects/" + mockenv.Project + "/locations/global/workloadIdentityPools/" + pool
	op, err := cloud.Projects.Locations.WorkloadIdentityPools.Providers.Create(parent, &api.WorkloadIdentityPoolProvider{
		DisplayName:      id,
		AttributeMapping: map[string]string{"google.subject": "assertion.subject"},
		Saml:             &api.Saml{IdpMetadataXml: samlMetadata(t)},
	}).WorkloadIdentityPoolProviderId(id).Context(env.Ctx).Do()
	if err == nil {
		err = waitIAM(env, cloud, op)
	}
	if err != nil {
		t.Fatalf("making SAML provider %s/providers/%s: %v", parent, id, err)
	}
}

// samlMetadata is a SAML 2.0 identity provider's metadata, signing with a
// new 2048-bit RSA key whose certificate is valid for a day.
func samlMetadata(t *testing.T) string {
	t.Helper()
	k, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(time.Now().UnixNano()),
		Subject:      pkix.Name{CommonName: "idp.example.com"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &k.PublicKey, k)
	if err != nil {
		t.Fatal(err)
	}
	return `<?xml version="1.0" encoding="UTF-8"?>
<md:EntityDescriptor xmlns:md="urn:oasis:names:tc:SAML:2.0:metadata" entityID="https://idp.example.com/saml">
  <md:IDPSSODescriptor protocolSupportEnumeration="urn:oasis:names:tc:SAML:2.0:protocol">
    <md:KeyDescriptor use="signing">
      <ds:KeyInfo xmlns:ds="http://www.w3.org/2000/09/xmldsig#">
        <ds:X509Data><ds:X509Certificate>` + base64.StdEncoding.EncodeToString(der) + `</ds:X509Certificate></ds:X509Data>
      </ds:KeyInfo>
    </md:KeyDescriptor>
    <md:SingleSignOnService Binding="urn:oasis:names:tc:SAML:2.0:bindings:HTTP-Redirect" Location="https://idp.example.com/sso"/>
  </md:IDPSSODescriptor>
</md:EntityDescriptor>`
}

// waitIAM waits for an IAM long-running operation.
func waitIAM(env *mockenv.Env, cloud *api.Service, op *api.Operation) error {
	deadline := time.Now().Add(mockenv.Timeout())
	for !op.Done {
		if time.Now().After(deadline) {
			return fmt.Errorf("timed out waiting for operation %s", op.Name)
		}
		time.Sleep(2 * time.Second)
		var err error
		if op, err = cloud.Projects.Locations.WorkloadIdentityPools.Operations.Get(op.Name).Context(env.Ctx).Do(); err != nil {
			return err
		}
	}
	if op.Error != nil {
		return fmt.Errorf("operation %s: %s", op.Name, op.Error.Message)
	}
	return nil
}
