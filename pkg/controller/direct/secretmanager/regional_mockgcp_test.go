// Copyright 2026 High Five Labs
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

// Regional Secret Manager secrets end to end on the fake cloud
// (pkg/test/mockenv). Bundt's case: an Instance's database password kept in
// a secret in the Environment's region (data residency), its value written
// from a Kubernetes Secret as a version.

import (
	"strings"
	"testing"

	gcp "cloud.google.com/go/secretmanager/apiv1"
	pb "cloud.google.com/go/secretmanager/apiv1/secretmanagerpb"
	refsv1beta1secret "github.com/GoogleCloudPlatform/k8s-config-connector/apis/refs/v1beta1/secret"
	"google.golang.org/api/option"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"

	smv1beta1 "github.com/highfivelabs/gaps-gcp/apis/secretmanager/v1beta1"
	"github.com/highfivelabs/gaps-gcp/pkg/test/mockenv"
)

const (
	namespace = "bundt"
	region    = "us-central1"
)

var projectAnnotation = map[string]string{"cnrm.cloud.google.com/project-id": mockenv.Project}

// regionalClient reads the fake cloud through the regional endpoint, as
// the controller does.
func regionalClient(t *testing.T, env *mockenv.Env, location string) *gcp.Client {
	t.Helper()
	opts := env.GCPOptions()
	if location != "global" {
		opts = append(opts, option.WithEndpoint("secretmanager."+location+".rep.googleapis.com:443"))
	}
	c, err := gcp.NewRESTClient(env.Ctx, opts...)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { c.Close() })
	return c
}

func externalRef(t *testing.T, u *unstructured.Unstructured) string {
	t.Helper()
	ref, _, _ := unstructured.NestedString(u.Object, "status", "externalRef")
	if ref == "" {
		t.Fatalf("%s has no status.externalRef", u.GetName())
	}
	return ref
}

func TestRegionalSecret(t *testing.T) {
	env := mockenv.New(t, namespace)
	cloud := regionalClient(t, env, region)

	const secretName = "bundt-db-password"
	if err := env.Kube.Create(env.Ctx, &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: "db"},
		Data:       map[string][]byte{"password": []byte("s3cret")},
	}); err != nil {
		t.Fatal(err)
	}

	secret := &smv1beta1.SecretManagerSecret{
		ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: secretName, Annotations: projectAnnotation},
		Spec:       smv1beta1.SecretManagerSecretSpec{Location: mockenv.Ptr(region)},
	}
	if err := env.Kube.Create(env.Ctx, secret); err != nil {
		t.Fatal(err)
	}

	var secretRef string
	t.Run("a regional secret is created in its region", func(t *testing.T) {
		u := env.WaitReady(smv1beta1.SecretManagerSecretGVK, namespace, secretName)
		secretRef = externalRef(t, u)
		want := "projects/" + mockenv.Project + "/locations/" + region + "/secrets/" + secretName
		if secretRef != want {
			t.Errorf("status.externalRef = %q, want %q", secretRef, want)
		}
		got, err := cloud.GetSecret(env.Ctx, &pb.GetSecretRequest{Name: secretRef})
		if err != nil {
			t.Fatalf("reading the secret from the fake cloud: %v", err)
		}
		if !strings.Contains(got.Name, "/locations/"+region+"/secrets/"+secretName) {
			t.Errorf("cloud secret name = %q, want it in %s", got.Name, region)
		}
		if got.Replication != nil {
			t.Errorf("a regional secret has replication %v; regional secrets have none", got.Replication)
		}
		// The controller went to the regional endpoint, never the global one.
		var regional int
		for _, r := range env.Requests() {
			if strings.Contains(r, "/secrets") && strings.Contains(r, "://secretmanager.googleapis.com") {
				t.Errorf("request to the global endpoint for a regional secret: %s", r)
			}
			if strings.Contains(r, "://secretmanager."+region+".rep.googleapis.com") {
				regional++
			}
		}
		if regional == 0 {
			t.Errorf("no request reached secretmanager.%s.rep.googleapis.com", region)
		}
	})

	const versionName = "bundt-db-password-1"
	version := &smv1beta1.SecretManagerSecretVersion{
		ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: versionName, Annotations: projectAnnotation},
		Spec: smv1beta1.SecretManagerSecretVersionSpec{
			SecretRef: &smv1beta1.SecretManagerSecretRef{Name: secretName},
			SecretData: &smv1beta1.SecretData_OneOf{Legacy: &refsv1beta1secret.Legacy{
				ValueFrom: &refsv1beta1secret.LegacyValueFrom{SecretKeyRef: &refsv1beta1secret.LegacyValueFromSecretKeyRef{Name: "db", Key: "password"}},
			}},
		},
	}
	if err := env.Kube.Create(env.Ctx, version); err != nil {
		t.Fatal(err)
	}

	var versionRef string
	t.Run("a version holds the Kubernetes Secret's value", func(t *testing.T) {
		u := env.WaitReady(smv1beta1.SecretManagerSecretVersionGVK, namespace, versionName)
		versionRef = externalRef(t, u)
		if !strings.HasPrefix(versionRef, secretRef+"/versions/") {
			t.Errorf("version status.externalRef = %q, want it under %s", versionRef, secretRef)
		}
		got, err := cloud.AccessSecretVersion(env.Ctx, &pb.AccessSecretVersionRequest{Name: versionRef})
		if err != nil {
			t.Fatalf("accessing the version in the fake cloud: %v", err)
		}
		if string(got.GetPayload().GetData()) != "s3cret" {
			t.Errorf("version payload = %q, want the Secret's value", got.GetPayload().GetData())
		}
	})

	t.Run("a secret cannot move region", func(t *testing.T) {
		obj := &smv1beta1.SecretManagerSecret{}
		if err := env.Kube.Get(env.Ctx, types.NamespacedName{Namespace: namespace, Name: secretName}, obj); err != nil {
			t.Fatal(err)
		}
		obj.Spec.Location = mockenv.Ptr("europe-west1")
		if err := env.Kube.Update(env.Ctx, obj); err != nil {
			t.Fatal(err)
		}
		msg := env.WaitNotReady(smv1beta1.SecretManagerSecretGVK, namespace, secretName)
		if !strings.Contains(msg, "Location changed") {
			t.Errorf("Ready message = %q, want it to say the location changed", msg)
		}
		obj = &smv1beta1.SecretManagerSecret{}
		if err := env.Kube.Get(env.Ctx, types.NamespacedName{Namespace: namespace, Name: secretName}, obj); err != nil {
			t.Fatal(err)
		}
		obj.Spec.Location = mockenv.Ptr(region)
		if err := env.Kube.Update(env.Ctx, obj); err != nil {
			t.Fatal(err)
		}
		env.WaitReady(smv1beta1.SecretManagerSecretGVK, namespace, secretName)
	})

	t.Run("delete removes the version and the secret", func(t *testing.T) {
		if err := env.Kube.Delete(env.Ctx, version); err != nil {
			t.Fatal(err)
		}
		env.WaitGone(&smv1beta1.SecretManagerSecretVersion{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: versionName}})
		got, err := cloud.GetSecretVersion(env.Ctx, &pb.GetSecretVersionRequest{Name: versionRef})
		if err == nil && got.State != pb.SecretVersion_DESTROYED {
			t.Errorf("version state after delete = %v, want DESTROYED or gone", got.State)
		}
		if err := env.Kube.Delete(env.Ctx, secret); err != nil {
			t.Fatal(err)
		}
		env.WaitGone(&smv1beta1.SecretManagerSecret{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: secretName}})
		if _, err := cloud.GetSecret(env.Ctx, &pb.GetSecretRequest{Name: secretRef}); status.Code(err) != codes.NotFound {
			t.Errorf("reading the deleted secret: got %v, want NotFound", err)
		}
	})
}

// A secret without a location stays global, as Config Connector's own.
func TestGlobalSecretUnchanged(t *testing.T) {
	env := mockenv.New(t, namespace)
	cloud := regionalClient(t, env, "global")

	const name = "global-secret"
	secret := &smv1beta1.SecretManagerSecret{
		ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: name, Annotations: projectAnnotation},
		Spec: smv1beta1.SecretManagerSecretSpec{
			Replication: &smv1beta1.Replication{LegacyAutomatic: &smv1beta1.Replication_Automatic{}},
		},
	}
	if err := env.Kube.Create(env.Ctx, secret); err != nil {
		t.Fatal(err)
	}
	u := env.WaitReady(smv1beta1.SecretManagerSecretGVK, namespace, name)
	ref := externalRef(t, u)
	if want := "projects/" + mockenv.Project + "/secrets/" + name; ref != want {
		t.Errorf("status.externalRef = %q, want %q", ref, want)
	}
	if _, err := cloud.GetSecret(env.Ctx, &pb.GetSecretRequest{Name: ref}); err != nil {
		t.Errorf("reading the global secret from the fake cloud: %v", err)
	}
}
