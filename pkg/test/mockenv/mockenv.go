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

// Package mockenv runs this project's kinds end to end on a fake cloud, for
// tests: a real kube-apiserver and etcd (envtest), Config Connector's
// unchanged direct reconciler with every config-connector-extensions controller, and Config
// Connector's mockgcp (with our patches) as Google Cloud. No recordings.
//
// With E2E_GCP_TARGET=real the same tests run against a real project
// instead of mockgcp (target.go; task test-live, never task check).
//
// Needs KUBEBUILDER_ASSETS (task test sets it).
package mockenv

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/GoogleCloudPlatform/k8s-config-connector/mockgcp"
	"github.com/GoogleCloudPlatform/k8s-config-connector/mockgcp/pkg/storage"
	operatorv1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/operator/pkg/apis/core/v1beta1"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/config"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	cloudresourcemanagerv1 "google.golang.org/api/cloudresourcemanager/v1"
	"google.golang.org/api/option"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	k8sruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	"github.com/highfivelabs/config-connector-extensions/pkg/controller"
	chroniclefake "github.com/highfivelabs/config-connector-extensions/pkg/controller/direct/chronicle/fake"
	smfake "github.com/highfivelabs/config-connector-extensions/pkg/controller/direct/servicemanagement/fake"
	"github.com/highfivelabs/config-connector-extensions/pkg/discokind"
	discofake "github.com/highfivelabs/config-connector-extensions/pkg/discokind/fake"
)

// Env is one running fake cloud and cluster.
type Env struct {
	T    *testing.T
	Ctx  context.Context
	Kube client.Client
	// HTTP reaches the fake cloud; build Google API clients with
	// option.WithHTTPClient(HTTP) to read what the controllers did.
	HTTP *http.Client

	requests *recorder
	// slotRequests are each slot's own requests (NewSlots).
	slotRequests map[string]*recorder

	mockSQL    sync.Once
	mockSQLErr error
}

// recorder keeps the method and URL of every request to the fake cloud.
type recorder struct {
	inner http.RoundTripper
	mu    sync.Mutex
	urls  []string
}

func (r *recorder) RoundTrip(req *http.Request) (*http.Response, error) {
	r.mu.Lock()
	r.urls = append(r.urls, req.Method+" "+req.URL.String())
	r.mu.Unlock()
	return r.inner.RoundTrip(req)
}

// Requests is every request made to the fake cloud so far, as
// "METHOD URL", by the controllers and the test alike.
func (e *Env) Requests() []string {
	e.requests.mu.Lock()
	defer e.requests.mu.Unlock()
	return slices.Clone(e.requests.urls)
}

// repoRoot is the repository root, found from this file's path.
func repoRoot() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "..", "..")
}

// New starts envtest with our CRDs and Config Connector's operator CRDs
// (directbase reads ConfigConnector / ConfigConnectorContext), mockgcp with
// Project created (or, against a real project, Application Default
// Credentials), and a manager running every config-connector-extensions controller, and
// creates each namespace given. Everything stops when the test ends.
func New(t *testing.T, namespaces ...string) *Env {
	t.Helper()
	c := startCluster(t)
	c.startManager(t, "", "", c.requests)
	c.createNamespaces(t, namespaces)
	return c.Env
}

// NewSlots starts one cluster and one fake cloud (or one real project), and
// one manager per slot: slot i runs with --slot=slots[i] and watches only
// namespace "slot-<slot>", as two version slots of a hosted install do.
// The returned Env's HTTP and Requests are the test's own; each slot's
// requests are SlotRequests(slot).
func NewSlots(t *testing.T, slots ...string) *Env {
	t.Helper()
	c := startCluster(t)
	c.slotRequests = map[string]*recorder{}
	for _, slot := range slots {
		rec := &recorder{inner: c.requests.inner}
		c.slotRequests[slot] = rec
		c.startManager(t, slot, SlotNamespace(slot), rec)
		c.createNamespaces(t, []string{SlotNamespace(slot)})
	}
	return c.Env
}

// SlotNamespace is the namespace NewSlots gives a slot's manager.
func SlotNamespace(slot string) string { return "slot-" + slot }

// SlotRequests is every request the slot's controllers made to the
// cloud so far, as "METHOD URL".
func (e *Env) SlotRequests(slot string) []string {
	e.T.Helper()
	r, ok := e.slotRequests[slot]
	if !ok {
		e.T.Fatalf("no slot %q", slot)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return slices.Clone(r.urls)
}

type cluster struct {
	*Env
	restConfig *rest.Config
	scheme     *k8sruntime.Scheme
}

func startCluster(t *testing.T) *cluster {
	t.Helper()
	checkTarget(t)
	if os.Getenv("KUBEBUILDER_ASSETS") == "" {
		t.Fatal("KUBEBUILDER_ASSETS is not set; run `task test`, which installs envtest's kube-apiserver and etcd")
	}

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	root := repoRoot()
	kccCRDs := filepath.Join(root, "third_party", "k8s-config-connector", "config", "crds", "resources")
	env := &envtest.Environment{
		CRDDirectoryPaths: []string{
			filepath.Join(root, "config", "crds"),
			filepath.Join(root, "third_party", "k8s-config-connector", "operator", "config", "crd", "bases"),
			// Config Connector kinds a gap kind references by name
			// (ArtifactRegistryRepository's upstream password version).
			filepath.Join(kccCRDs, "apiextensions.k8s.io_v1_customresourcedefinition_secretmanagersecrets.secretmanager.cnrm.cloud.google.com.yaml"),
			filepath.Join(kccCRDs, "apiextensions.k8s.io_v1_customresourcedefinition_secretmanagersecretversions.secretmanager.cnrm.cloud.google.com.yaml"),
			// ComputeRegionNetworkEndpoint's group.
			filepath.Join(kccCRDs, "apiextensions.k8s.io_v1_customresourcedefinition_computeregionnetworkendpointgroups.compute.cnrm.cloud.google.com.yaml"),
			// ComputeRouterRoutePolicy's router.
			filepath.Join(kccCRDs, "apiextensions.k8s.io_v1_customresourcedefinition_computerouters.compute.cnrm.cloud.google.com.yaml"),
			// ComputeNetworkFirewallPolicyRule's policy.
			filepath.Join(kccCRDs, "apiextensions.k8s.io_v1_customresourcedefinition_computenetworkfirewallpolicies.compute.cnrm.cloud.google.com.yaml"),
			// StorageBucketObject's bucket.
			filepath.Join(kccCRDs, "apiextensions.k8s.io_v1_customresourcedefinition_storagebuckets.storage.cnrm.cloud.google.com.yaml"),
			// A managed folder, which IAMMember names by name.
			filepath.Join(kccCRDs, "apiextensions.k8s.io_v1_customresourcedefinition_storagemanagedfolders.storage.cnrm.cloud.google.com.yaml"),
			// ArtifactRegistryTag's repository.
			filepath.Join(kccCRDs, "apiextensions.k8s.io_v1_customresourcedefinition_artifactregistryrepositories.artifactregistry.cnrm.cloud.google.com.yaml"),
			// IAMWorkloadIdentityPoolNamespace's pool, and a provider key's
			// provider.
			filepath.Join(kccCRDs, "apiextensions.k8s.io_v1_customresourcedefinition_iamworkloadidentitypools.iam.cnrm.cloud.google.com.yaml"),
			filepath.Join(kccCRDs, "apiextensions.k8s.io_v1_customresourcedefinition_iamworkloadidentitypoolproviders.iam.cnrm.cloud.google.com.yaml"),
			// ChronicleRuleDeployment's rule.
			filepath.Join(kccCRDs, "apiextensions.k8s.io_v1_customresourcedefinition_chroniclerules.chronicle.cnrm.cloud.google.com.yaml"),
		},
		ErrorIfCRDPathMissing: true,
	}
	restConfig, err := env.Start()
	if err != nil {
		t.Fatalf("starting envtest: %v", err)
	}
	t.Cleanup(func() {
		if err := env.Stop(); err != nil {
			t.Errorf("stopping envtest: %v", err)
		}
	})

	scheme := k8sruntime.NewScheme()
	for _, add := range []func(*k8sruntime.Scheme) error{clientgoscheme.AddToScheme, operatorv1beta1.AddToScheme, controller.AddToScheme} {
		if err := add(scheme); err != nil {
			t.Fatal(err)
		}
	}
	kube, err := client.New(restConfig, client.Options{Scheme: scheme})
	if err != nil {
		t.Fatal(err)
	}

	var requests *recorder
	if Live() {
		// A real project, with the run's credentials; requests are still
		// recorded, so the same checks on them hold.
		authed, err := google.DefaultClient(ctx, "https://www.googleapis.com/auth/cloud-platform")
		if err != nil {
			t.Fatalf("Application Default Credentials for E2E_GCP_TARGET=real: %v", err)
		}
		requests = &recorder{inner: authed.Transport}
	} else {
		// The fake cloud, as Config Connector's own test harness builds it
		// (config/tests/samples/create/harness.go).
		mockCloud := mockgcp.NewMockRoundTripperForTest(t, kube, storage.NewInMemoryStorage())
		// The discokind kinds' collections are served by their own fake
		// (pkg/discokind/fake), and Service Management and the Chronicle API by their own, in front of mockgcp.
		requests = &recorder{inner: discofake.New(chroniclefake.New(smfake.New(mockCloud)), discokind.Kinds())}
	}
	gcpHTTP := &http.Client{Transport: requests}
	if !Live() {
		ctx = context.WithValue(ctx, oauth2.HTTPClient, gcpHTTP)
		createMockProject(t, ctx, gcpHTTP, Project)
	}
	return &cluster{
		Env:        &Env{T: t, Ctx: ctx, Kube: kube, HTTP: gcpHTTP, requests: requests},
		restConfig: restConfig,
		scheme:     scheme,
	}
}

// startManager runs every config-connector-extensions controller, as slot
// (none if ""), watching namespace (the whole cluster if ""), reaching the
// cloud through rec.
func (c *cluster) startManager(t *testing.T, slot, namespace string, rec *recorder) {
	t.Helper()
	gcpHTTP := &http.Client{Transport: rec}
	ctx := c.Ctx
	if !Live() {
		ctx = context.WithValue(ctx, oauth2.HTTPClient, gcpHTTP)
	}
	opts := ctrl.Options{
		Scheme:  c.scheme,
		Metrics: metricsserver.Options{BindAddress: "0"},
	}
	if namespace != "" {
		opts.Cache = cache.Options{DefaultNamespaces: map[string]cache.Config{namespace: {}}}
	}
	mgr, err := ctrl.NewManager(c.restConfig, opts)
	if err != nil {
		t.Fatal(err)
	}
	// Several Envs in one test binary register the same controller names;
	// each has its own manager, so the names never meet.
	cfg := &config.ControllerConfig{HTTPClient: gcpHTTP, SkipNameValidation: true}
	if err := controller.AddToManager(ctx, mgr, cfg, controller.Options{Slot: slot}); err != nil {
		t.Fatal(err)
	}
	// Stop the manager before envtest (cleanups run last-registered first),
	// or kube-apiserver waits on its open watches.
	mgrCtx, stopMgr := context.WithCancel(ctx)
	mgrDone := make(chan struct{})
	go func() {
		defer close(mgrDone)
		if err := mgr.Start(mgrCtx); err != nil {
			t.Errorf("manager: %v", err)
		}
	}()
	t.Cleanup(func() {
		stopMgr()
		<-mgrDone
	})
}

func (c *cluster) createNamespaces(t *testing.T, namespaces []string) {
	t.Helper()
	for _, ns := range namespaces {
		if err := c.Kube.Create(c.Ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: ns}}); err != nil {
			t.Fatal(err)
		}
	}
}

// GCPOptions are the client options that reach the fake cloud.
func (e *Env) GCPOptions() []option.ClientOption {
	return []option.ClientOption{option.WithHTTPClient(e.HTTP)}
}

func createMockProject(t *testing.T, ctx context.Context, httpClient *http.Client, projectID string) {
	t.Helper()
	crm, err := cloudresourcemanagerv1.NewService(ctx, option.WithHTTPClient(httpClient))
	if err != nil {
		t.Fatal(err)
	}
	op, err := crm.Projects.Create(&cloudresourcemanagerv1.Project{ProjectId: projectID}).Context(ctx).Do()
	if err != nil {
		t.Fatalf("creating mock project %q: %v", projectID, err)
	}
	for i := 0; i < 100 && !op.Done; i++ {
		time.Sleep(100 * time.Millisecond)
		if op, err = crm.Operations.Get(op.Name).Context(ctx).Do(); err != nil {
			t.Fatal(err)
		}
	}
	if !op.Done {
		t.Fatalf("mock project %q was not created", projectID)
	}
}

// WaitReady waits until the object's Ready condition is True for its
// current generation, and returns the object.
func (e *Env) WaitReady(gvk schema.GroupVersionKind, namespace, name string) *unstructured.Unstructured {
	e.T.Helper()
	var last *unstructured.Unstructured
	deadline := time.Now().Add(Timeout())
	for time.Now().Before(deadline) {
		u := &unstructured.Unstructured{}
		u.SetGroupVersionKind(gvk)
		if err := e.Kube.Get(e.Ctx, types.NamespacedName{Namespace: namespace, Name: name}, u); err != nil {
			e.T.Fatal(err)
		}
		last = u
		if ready, _ := Ready(u); ready {
			return u
		}
		time.Sleep(200 * time.Millisecond)
	}
	e.T.Fatalf("%s %s/%s did not become Ready; last status: %v", gvk.Kind, namespace, name, last.Object["status"])
	return nil
}

// WaitNotReady waits until the object's Ready condition is False for its
// current generation, and returns the condition's message.
func (e *Env) WaitNotReady(gvk schema.GroupVersionKind, namespace, name string) string {
	e.T.Helper()
	var last *unstructured.Unstructured
	deadline := time.Now().Add(Timeout())
	for time.Now().Before(deadline) {
		u := &unstructured.Unstructured{}
		u.SetGroupVersionKind(gvk)
		if err := e.Kube.Get(e.Ctx, types.NamespacedName{Namespace: namespace, Name: name}, u); err != nil {
			e.T.Fatal(err)
		}
		last = u
		observed, _, _ := unstructured.NestedInt64(u.Object, "status", "observedGeneration")
		conditions, _, _ := unstructured.NestedSlice(u.Object, "status", "conditions")
		for _, c := range conditions {
			cm, _ := c.(map[string]any)
			if cm["type"] == "Ready" && cm["status"] == "False" && observed == u.GetGeneration() {
				msg, _ := cm["message"].(string)
				return msg
			}
		}
		time.Sleep(200 * time.Millisecond)
	}
	e.T.Fatalf("%s %s/%s did not become not-Ready; last status: %v", gvk.Kind, namespace, name, last.Object["status"])
	return ""
}

// Ready reports whether the object's Ready condition is True for its
// current generation, and its message.
func Ready(u *unstructured.Unstructured) (bool, string) {
	observed, _, _ := unstructured.NestedInt64(u.Object, "status", "observedGeneration")
	conditions, _, _ := unstructured.NestedSlice(u.Object, "status", "conditions")
	for _, c := range conditions {
		cm, _ := c.(map[string]any)
		if cm["type"] == "Ready" {
			msg, _ := cm["message"].(string)
			return cm["status"] == "True" && observed == u.GetGeneration(), msg
		}
	}
	return false, ""
}

// WaitGone waits until the object no longer exists (its finalizers ran).
func (e *Env) WaitGone(obj client.Object) {
	e.T.Helper()
	deadline := time.Now().Add(Timeout())
	for {
		err := e.Kube.Get(e.Ctx, client.ObjectKeyFromObject(obj), obj)
		if apierrors.IsNotFound(err) {
			return
		}
		if time.Now().After(deadline) {
			e.T.Fatalf("%T %s was not deleted (finalizer not removed): %v", obj, obj.GetName(), err)
		}
		time.Sleep(200 * time.Millisecond)
	}
}

// Ptr returns a pointer to v.
func Ptr[T any](v T) *T { return &v }
