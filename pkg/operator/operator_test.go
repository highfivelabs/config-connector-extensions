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

package operator_test

// The operator against a real kube-apiserver (envtest), with Config
// Connector's operator CRDs and the objects Config Connector's operator
// would have made (service accounts, ConfigConnector, contexts). Nothing
// runs the Deployments; tests/operator (kind) does that.

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	operatorv1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/operator/pkg/apis/core/v1beta1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	crconfig "sigs.k8s.io/controller-runtime/pkg/config"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	"github.com/highfivelabs/gaps-gcp/pkg/operator"
)

const image = "gaps-gcp-manager:test"

type env struct {
	t    *testing.T
	ctx  context.Context
	kube client.Client
}

func start(t *testing.T, isolation string) *env {
	t.Helper()
	if os.Getenv("KUBEBUILDER_ASSETS") == "" {
		t.Fatal("KUBEBUILDER_ASSETS is not set; run `make test`")
	}
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	te := &envtest.Environment{
		CRDDirectoryPaths:     []string{filepath.Join("..", "..", "third_party", "k8s-config-connector", "operator", "config", "crd", "bases")},
		ErrorIfCRDPathMissing: true,
	}
	cfg, err := te.Start()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = te.Stop() })
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = operatorv1beta1.AddToScheme(scheme)
	kube, err := client.New(cfg, client.Options{Scheme: scheme})
	if err != nil {
		t.Fatal(err)
	}
	mgr, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme:  scheme,
		Metrics: metricsserver.Options{BindAddress: "0"},
		// One manager per test, all in one binary.
		Controller: crconfig.Controller{SkipNameValidation: ptr.To(true)},
	})
	if err != nil {
		t.Fatal(err)
	}
	r := &operator.Reconciler{
		Client:    mgr.GetClient(),
		Settings:  operator.Settings{Image: image, ExtraArgs: []string{"--v=2"}},
		Isolation: isolation,
		Resync:    time.Minute,
	}
	if err := r.SetupWithManager(mgr); err != nil {
		t.Fatal(err)
	}
	mctx, stop := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() { defer close(done); _ = mgr.Start(mctx) }()
	t.Cleanup(func() { stop(); <-done })

	e := &env{t: t, ctx: ctx, kube: kube}
	e.create(&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: operator.CNRMSystemNamespace}})
	return e
}

func (e *env) create(obj client.Object) {
	e.t.Helper()
	if err := e.kube.Create(e.ctx, obj); err != nil {
		e.t.Fatal(err)
	}
}

// ksa is a Config Connector service account as its operator makes it.
func ksa(name, namespace, gsa string) *corev1.ServiceAccount {
	return &corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{
		Name: name, Namespace: namespace,
		Annotations: map[string]string{operator.WorkloadIdentityAnnotation: gsa},
	}}
}

// eventually polls f until it returns "" (or the deadline), and fails with
// its last message.
func (e *env) eventually(f func() string) {
	e.t.Helper()
	var msg string
	for deadline := time.Now().Add(20 * time.Second); time.Now().Before(deadline); time.Sleep(100 * time.Millisecond) {
		if msg = f(); msg == "" {
			return
		}
	}
	e.t.Fatal(msg)
}

func (e *env) deployment(ns, name string) (*appsv1.Deployment, string) {
	d := &appsv1.Deployment{}
	if err := e.kube.Get(e.ctx, types.NamespacedName{Namespace: ns, Name: name}, d); err != nil {
		return nil, "Deployment " + ns + "/" + name + ": " + err.Error()
	}
	return d, ""
}

func (e *env) gone(obj client.Object) string {
	err := e.kube.Get(e.ctx, client.ObjectKeyFromObject(obj), obj)
	if apierrors.IsNotFound(err) {
		return ""
	}
	return obj.GetName() + " still exists"
}

func TestClusterModeRunsAsConfigConnectorsServiceAccount(t *testing.T) {
	e := start(t, operator.IsolationShared)
	e.create(ksa(operator.ClusterModeKSA, operator.CNRMSystemNamespace, "kcc@p.iam.gserviceaccount.com"))
	e.create(&operatorv1beta1.ConfigConnector{
		ObjectMeta: metav1.ObjectMeta{Name: operator.ConfigConnectorName},
		Spec:       operatorv1beta1.ConfigConnectorSpec{Mode: "cluster", GoogleServiceAccount: "kcc@p.iam.gserviceaccount.com"},
	})

	var d *appsv1.Deployment
	e.eventually(func() (msg string) { d, msg = e.deployment(operator.CNRMSystemNamespace, "gaps-gcp-manager"); return })
	pod := d.Spec.Template.Spec
	if pod.ServiceAccountName != operator.ClusterModeKSA {
		t.Errorf("serviceAccountName = %q, want Config Connector's %q", pod.ServiceAccountName, operator.ClusterModeKSA)
	}
	c := pod.Containers[0]
	if c.Image != image || !slices.Equal(c.Args, []string{"--v=2"}) {
		t.Errorf("image/args = %q %v, want %q [--v=2] (cluster-wide: no --scoped-namespace)", c.Image, c.Args, image)
	}
	for _, env := range c.Env {
		if env.Name == "GOOGLE_APPLICATION_CREDENTIALS" {
			t.Errorf("Workload Identity mode mounts no key, but GOOGLE_APPLICATION_CREDENTIALS=%s", env.Value)
		}
	}
	crb := &rbacv1.ClusterRoleBinding{}
	if err := e.kube.Get(e.ctx, types.NamespacedName{Name: "gaps-gcp-manager"}, crb); err != nil {
		t.Fatal(err)
	}
	if crb.RoleRef.Name != operator.ManagerClusterRole || len(crb.Subjects) != 1 ||
		crb.Subjects[0].Name != operator.ClusterModeKSA || crb.Subjects[0].Namespace != operator.CNRMSystemNamespace {
		t.Errorf("ClusterRoleBinding = %+v %+v, want %s bound to cnrm-system/%s", crb.RoleRef, crb.Subjects, operator.ManagerClusterRole, operator.ClusterModeKSA)
	}
	if err := e.kube.Get(e.ctx, types.NamespacedName{Name: "gaps-gcp-manager:cluster-scoped"}, crb); err != nil {
		t.Fatal(err)
	}
	if crb.RoleRef.Name != operator.ManagerClusterScopedRole || crb.Subjects[0].Name != operator.ClusterModeKSA {
		t.Errorf("ClusterRoleBinding = %+v %+v, want %s bound to cnrm-system/%s", crb.RoleRef, crb.Subjects, operator.ManagerClusterScopedRole, operator.ClusterModeKSA)
	}
}

func TestClusterModeWithAKeyMountsConfigConnectorsSecret(t *testing.T) {
	e := start(t, operator.IsolationShared)
	e.create(&corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: operator.ClusterModeKSA, Namespace: operator.CNRMSystemNamespace}})
	e.create(&operatorv1beta1.ConfigConnector{
		ObjectMeta: metav1.ObjectMeta{Name: operator.ConfigConnectorName},
		Spec:       operatorv1beta1.ConfigConnectorSpec{Mode: "cluster", CredentialSecretName: "kcc-key"},
	})
	// The service account exists before the ConfigConnector object, so a
	// first pass may mount the install bundle's gcp-key; the next must
	// follow the ConfigConnector object.
	var pod corev1.PodSpec
	e.eventually(func() string {
		d, msg := e.deployment(operator.CNRMSystemNamespace, "gaps-gcp-manager")
		if msg != "" {
			return msg
		}
		pod = d.Spec.Template.Spec
		var secrets []string
		for _, v := range pod.Volumes {
			if v.Secret != nil {
				secrets = append(secrets, v.Secret.SecretName)
			}
		}
		if !slices.Equal(secrets, []string{"kcc-key"}) {
			return fmt.Sprintf("mounted Secrets = %v, want only Config Connector's kcc-key", secrets)
		}
		return ""
	})
	if !slices.Contains(pod.Containers[0].Env, corev1.EnvVar{Name: "GOOGLE_APPLICATION_CREDENTIALS", Value: "/var/secrets/google/key.json"}) {
		t.Errorf("env = %v, want GOOGLE_APPLICATION_CREDENTIALS at Config Connector's path", pod.Containers[0].Env)
	}
}

func TestNamespacedModeRunsOneManagerPerContext(t *testing.T) {
	e := start(t, operator.IsolationShared)
	e.create(ksa(operator.ClusterModeKSA, operator.CNRMSystemNamespace, "kcc@p.iam.gserviceaccount.com"))
	cc := &operatorv1beta1.ConfigConnector{
		ObjectMeta: metav1.ObjectMeta{Name: operator.ConfigConnectorName},
		Spec:       operatorv1beta1.ConfigConnectorSpec{Mode: "cluster", GoogleServiceAccount: "kcc@p.iam.gserviceaccount.com"},
	}
	e.create(cc)
	clusterManager := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Namespace: operator.CNRMSystemNamespace, Name: "gaps-gcp-manager"}}
	e.eventually(func() (msg string) { _, msg = e.deployment(operator.CNRMSystemNamespace, "gaps-gcp-manager"); return })

	// Switch Config Connector to namespaced mode, with two teams.
	if err := e.kube.Get(e.ctx, client.ObjectKeyFromObject(cc), cc); err != nil {
		t.Fatal(err)
	}
	cc.Spec = operatorv1beta1.ConfigConnectorSpec{Mode: "namespaced"}
	if err := e.kube.Update(e.ctx, cc); err != nil {
		t.Fatal(err)
	}
	for _, team := range []string{"team-a", "team-b"} {
		e.create(&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: team}})
		e.create(ksa(operator.NamespacedKSAPrefix+team, operator.CNRMSystemNamespace, team+"@p.iam.gserviceaccount.com"))
	}
	e.create(&operatorv1beta1.ConfigConnectorContext{
		ObjectMeta: metav1.ObjectMeta{Name: "configconnectorcontext.core.cnrm.cloud.google.com", Namespace: "team-a"},
		Spec:       operatorv1beta1.ConfigConnectorContextSpec{GoogleServiceAccount: "team-a@p.iam.gserviceaccount.com"},
	})
	e.create(&operatorv1beta1.ConfigConnectorContext{
		ObjectMeta: metav1.ObjectMeta{Name: "configconnectorcontext.core.cnrm.cloud.google.com", Namespace: "team-b"},
		Spec: operatorv1beta1.ConfigConnectorContextSpec{
			GoogleServiceAccount: "team-b@p.iam.gserviceaccount.com",
			RequestProjectPolicy: "BILLING_PROJECT", BillingProject: "billing-p",
		},
	})

	e.eventually(func() string { return e.gone(clusterManager) })
	for _, team := range []string{"team-a", "team-b"} {
		var d *appsv1.Deployment
		e.eventually(func() (msg string) {
			d, msg = e.deployment(operator.CNRMSystemNamespace, "gaps-gcp-manager-"+team)
			return
		})
		pod := d.Spec.Template.Spec
		if pod.ServiceAccountName != operator.NamespacedKSAPrefix+team {
			t.Errorf("%s: serviceAccountName = %q, want Config Connector's %s%s", team, pod.ServiceAccountName, operator.NamespacedKSAPrefix, team)
		}
		args := pod.Containers[0].Args
		if !slices.Contains(args, "--scoped-namespace="+team) {
			t.Errorf("%s: args = %v, want --scoped-namespace=%s", team, args, team)
		}
		if team == "team-b" && (!slices.Contains(args, "--user-project-override") || !slices.Contains(args, "--billing-project=billing-p")) {
			t.Errorf("team-b: args = %v, want the BILLING_PROJECT policy's flags", args)
		}
		rb := &rbacv1.RoleBinding{}
		if err := e.kube.Get(e.ctx, types.NamespacedName{Namespace: team, Name: "gaps-gcp-manager"}, rb); err != nil {
			t.Fatalf("%s: RoleBinding: %v", team, err)
		}
		if rb.RoleRef.Name != operator.ManagerClusterRole || rb.Subjects[0].Name != operator.NamespacedKSAPrefix+team {
			t.Errorf("%s: RoleBinding = %+v %+v", team, rb.RoleRef, rb.Subjects)
		}
		crb := &rbacv1.ClusterRoleBinding{}
		if err := e.kube.Get(e.ctx, types.NamespacedName{Name: "gaps-gcp-manager-" + team}, crb); err != nil {
			t.Fatalf("%s: ClusterRoleBinding: %v", team, err)
		}
		if crb.RoleRef.Name != operator.ManagerClusterScopedRole {
			t.Errorf("%s: a scoped manager is bound cluster-wide to %s, want only %s", team, crb.RoleRef.Name, operator.ManagerClusterScopedRole)
		}
	}
	for _, name := range []string{"gaps-gcp-manager", "gaps-gcp-manager:cluster-scoped"} {
		if msg := e.gone(&rbacv1.ClusterRoleBinding{ObjectMeta: metav1.ObjectMeta{Name: name}}); msg != "" {
			t.Errorf("cluster mode's ClusterRoleBinding outlived cluster mode: %s", msg)
		}
	}

	// A team leaves: its manager and bindings go.
	ccc := &operatorv1beta1.ConfigConnectorContext{ObjectMeta: metav1.ObjectMeta{Name: "configconnectorcontext.core.cnrm.cloud.google.com", Namespace: "team-a"}}
	if err := e.kube.Delete(e.ctx, ccc); err != nil {
		t.Fatal(err)
	}
	e.eventually(func() string {
		return e.gone(&appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Namespace: operator.CNRMSystemNamespace, Name: "gaps-gcp-manager-team-a"}})
	})
	e.eventually(func() string {
		return e.gone(&rbacv1.RoleBinding{ObjectMeta: metav1.ObjectMeta{Namespace: "team-a", Name: "gaps-gcp-manager"}})
	})
	if _, msg := e.deployment(operator.CNRMSystemNamespace, "gaps-gcp-manager-team-b"); msg != "" {
		t.Error(msg)
	}
}

func TestWaitsForConfigConnectorsServiceAccount(t *testing.T) {
	e := start(t, operator.IsolationShared)
	e.create(&operatorv1beta1.ConfigConnector{
		ObjectMeta: metav1.ObjectMeta{Name: operator.ConfigConnectorName},
		Spec:       operatorv1beta1.ConfigConnectorSpec{Mode: "cluster", GoogleServiceAccount: "kcc@p.iam.gserviceaccount.com"},
	})
	time.Sleep(time.Second)
	if _, msg := e.deployment(operator.CNRMSystemNamespace, "gaps-gcp-manager"); msg == "" {
		t.Fatal("a manager was created before Config Connector's service account exists; it would run as no identity")
	}
	e.create(ksa(operator.ClusterModeKSA, operator.CNRMSystemNamespace, "kcc@p.iam.gserviceaccount.com"))
	e.eventually(func() (msg string) { _, msg = e.deployment(operator.CNRMSystemNamespace, "gaps-gcp-manager"); return })
}

func TestWithoutConfigConnectorsOperator(t *testing.T) {
	// Config Connector from its install bundle: no ConfigConnector object,
	// only its service account, here with Workload Identity.
	e := start(t, operator.IsolationShared)
	e.create(ksa(operator.ClusterModeKSA, operator.CNRMSystemNamespace, "kcc@p.iam.gserviceaccount.com"))
	var d *appsv1.Deployment
	e.eventually(func() (msg string) { d, msg = e.deployment(operator.CNRMSystemNamespace, "gaps-gcp-manager"); return })
	if len(d.Spec.Template.Spec.Containers[0].Env) != 0 {
		t.Errorf("env = %v, want none (Workload Identity)", d.Spec.Template.Spec.Containers[0].Env)
	}
}
