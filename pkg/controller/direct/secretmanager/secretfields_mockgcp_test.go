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

package secretmanager_test

// The secret fields Config Connector's SecretManagerSecret has as NOTYET
// or lacks, which ours serves (foundation item 26): versionDestroyTtl
// (delayed destruction of versions), a regional secret's own
// customerManagedEncryption, and tags (resource manager tags bound at
// create, input only).

import (
	"maps"
	"strings"
	"testing"
	"time"

	pb "cloud.google.com/go/secretmanager/apiv1/secretmanagerpb"
	kmsv1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/apis/kms/v1beta1"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/k8s"
	"k8s.io/apimachinery/pkg/types"

	smv1beta1 "github.com/highfivelabs/config-connector-extensions/apis/secretmanager/v1beta1"
	"github.com/highfivelabs/config-connector-extensions/pkg/test/mockenv"
)

// TestSecretFields: a regional secret made with versionDestroyTtl (and,
// on the fake, a key and tags) holds them; a resync writes nothing; a
// changed versionDestroyTtl is patched alone; tags cannot change. Live the
// key and the tags are left out: the live-test project has no key the
// Secret Manager service agent may use, and no tag to bind.
func TestSecretFields(t *testing.T) {
	env := mockenv.New(t, namespace)
	cloud := regionalClient(t, env, region)
	gvk := smv1beta1.SecretManagerSecretGVK

	name := mockenv.Name("secret-fields")
	key := "projects/" + mockenv.Project + "/locations/" + region + "/keyRings/ring/cryptoKeys/key"
	tags := map[string]string{"123/environment": "production"}
	secret := &smv1beta1.SecretManagerSecret{
		ObjectMeta: meta(name),
		Spec: smv1beta1.SecretManagerSecretSpec{
			Location:          mockenv.Ptr(region),
			VersionDestroyTtl: mockenv.Ptr("86400s"),
		},
	}
	if !mockenv.Live() {
		secret.Spec.CustomerManagedEncryption = &smv1beta1.CustomerManagedEncryption{KmsKeyRef: &kmsv1beta1.KMSCryptoKeyRef{External: key}}
		secret.Spec.Tags = tags
	}
	if err := env.Kube.Create(env.Ctx, secret); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		obj := &smv1beta1.SecretManagerSecret{}
		if err := env.Kube.Get(env.Ctx, types.NamespacedName{Namespace: namespace, Name: name}, obj); err == nil {
			_ = env.Kube.Delete(env.Ctx, obj)
			env.WaitGone(obj)
		}
	})
	get := func(t *testing.T) *smv1beta1.SecretManagerSecret {
		t.Helper()
		obj := &smv1beta1.SecretManagerSecret{}
		if err := env.Kube.Get(env.Ctx, types.NamespacedName{Namespace: namespace, Name: name}, obj); err != nil {
			t.Fatal(err)
		}
		return obj
	}
	patches := func() int {
		n := 0
		for _, r := range env.Requests() {
			if strings.HasPrefix(r, "PATCH ") && strings.Contains(r, "/secrets/"+name) {
				n++
			}
		}
		return n
	}
	var ref string

	t.Run("create sends the fields", func(t *testing.T) {
		ref = externalRef(t, env.WaitReady(gvk, namespace, name))
		got, err := cloud.GetSecret(env.Ctx, &pb.GetSecretRequest{Name: ref})
		if err != nil {
			t.Fatal(err)
		}
		if d := got.GetVersionDestroyTtl().AsDuration(); d != 24*time.Hour {
			t.Errorf("versionDestroyTtl = %v, want 24h", d)
		}
		if !mockenv.Live() {
			if k := got.GetCustomerManagedEncryption().GetKmsKeyName(); k != key {
				t.Errorf("customerManagedEncryption.kmsKeyName = %q, want %q", k, key)
			}
			// The fake keeps the input-only tags; the real API does not
			// return them.
			if !maps.Equal(got.GetTags(), tags) {
				t.Errorf("the create's tags = %v, want %v", got.GetTags(), tags)
			}
		}
	})

	t.Run("a resync writes nothing", func(t *testing.T) {
		before := patches()
		obj := get(t)
		obj.Annotations = map[string]string{k8s.InternalForceReconcileAnnotation: time.Now().Format(time.RFC3339Nano)}
		maps.Copy(obj.Annotations, projectAnnotation)
		if err := env.Kube.Update(env.Ctx, obj); err != nil {
			t.Fatal(err)
		}
		time.Sleep(3 * time.Second)
		env.WaitReady(gvk, namespace, name)
		if n := patches() - before; n != 0 {
			t.Errorf("a resync patched the secret %d times", n)
		}
	})

	t.Run("a changed versionDestroyTtl is patched alone", func(t *testing.T) {
		before := len(env.Requests())
		obj := get(t)
		obj.Spec.VersionDestroyTtl = mockenv.Ptr("172800s")
		if err := env.Kube.Update(env.Ctx, obj); err != nil {
			t.Fatal(err)
		}
		deadline := time.Now().Add(mockenv.Timeout())
		for {
			got, err := cloud.GetSecret(env.Ctx, &pb.GetSecretRequest{Name: ref})
			if err == nil && got.GetVersionDestroyTtl().AsDuration() == 48*time.Hour {
				if !mockenv.Live() && got.GetCustomerManagedEncryption().GetKmsKeyName() != key {
					t.Errorf("the patch changed the key: %v", got.GetCustomerManagedEncryption())
				}
				break
			}
			if time.Now().After(deadline) {
				t.Fatalf("versionDestroyTtl did not reach the cloud (err %v)", err)
			}
			time.Sleep(500 * time.Millisecond)
		}
		for _, r := range env.Requests()[before:] {
			if strings.HasPrefix(r, "PATCH ") && strings.Contains(r, "/secrets/"+name) && !strings.Contains(r, "updateMask=versionDestroyTtl") {
				t.Errorf("the patch's mask is not versionDestroyTtl alone: %s", r)
			}
		}
	})

	t.Run("tags cannot change", func(t *testing.T) {
		if mockenv.Live() {
			t.Skip("no tags live")
		}
		obj := get(t)
		obj.Spec.Tags = map[string]string{"123/environment": "staging"}
		err := env.Kube.Update(env.Ctx, obj)
		if err == nil || !strings.Contains(err.Error(), "tags is immutable") {
			t.Errorf("update error = %v, want tags is immutable", err)
		}
	})
}
