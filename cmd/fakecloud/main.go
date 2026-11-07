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

// fakecloud runs Config Connector's own controller manager, unchanged and
// with every controller, plus gaps-gcp's controllers, against a Kubernetes
// cluster, with every Google API call answered in process by mockgcp. It is
// how Config Connector's e2e harness runs in mock mode
// (config/tests/samples/create/harness.go), outside a Go test: Config
// Connector has no flag or environment variable that points a deployed
// manager at mockgcp, because mockgcp is an in-process http.RoundTripper
// and gRPC server, not a network endpoint.
//
// The fake cloud is in memory: it starts empty, with the projects named by
// --projects, and forgets everything when the process stops.
package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/GoogleCloudPlatform/k8s-config-connector/mockgcp"
	"github.com/GoogleCloudPlatform/k8s-config-connector/mockgcp/pkg/storage"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/config"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/kccmanager"
	// Registers every direct controller, as Config Connector's cmd/manager does.
	_ "github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct/register"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/krmtotf"
	transport_tpg "github.com/hashicorp/terraform-provider-google-beta/google-beta/transport"
	"golang.org/x/oauth2"
	cloudresourcemanagerv1 "google.golang.org/api/cloudresourcemanager/v1"
	"google.golang.org/api/option"
	"google.golang.org/grpc"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	sqlv1beta1 "github.com/highfivelabs/gaps-gcp/apis/sql/v1beta1"
	"github.com/highfivelabs/gaps-gcp/pkg/controller"
)

func main() {
	var projects string
	flag.StringVar(&projects, "projects", "mock-project", "comma-separated project IDs to create in the fake cloud at start")
	flag.Parse()

	ctx := ctrl.SetupSignalHandler()
	ctrl.SetLogger(klog.NewKlogr())
	if err := run(ctx, strings.Split(projects, ",")); err != nil {
		klog.Errorf("fakecloud: %v", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, projects []string) error {
	restConfig := ctrl.GetConfigOrDie()
	kube, err := client.New(restConfig, client.Options{})
	if err != nil {
		return fmt.Errorf("building kube client: %w", err)
	}

	// The fake cloud: HTTP through the round tripper, gRPC through its
	// in-process connection.
	mock, err := mockgcp.NewMockRoundTripper(ctx, kube, storage.NewInMemoryStorage())
	if err != nil {
		return fmt.Errorf("building mockgcp: %w", err)
	}
	go func() {
		if err := mock.Run(ctx); err != nil {
			klog.Errorf("mockgcp gRPC server: %v", err)
		}
	}()
	mockConn := mock.NewGRPCConnection(ctx)
	httpClient := &http.Client{Transport: mock}
	ctx = context.WithValue(ctx, oauth2.HTTPClient, httpClient)

	// As the harness does for mock mode: Terraform-based and DCL-based
	// resources build their own HTTP clients, so hook those too.
	transport_tpg.DefaultHTTPClientTransformer = func(context.Context, *http.Client) *http.Client { return httpClient }
	transport_tpg.OAuth2HTTPClientTransformer = func(context.Context, *http.Client) *http.Client { return httpClient }
	toMock := func(ctx context.Context, method string, req, reply any, _ *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		return invoker(ctx, method, req, reply, mockConn, opts...)
	}
	transport_tpg.GRPCUnaryClientInterceptor = toMock
	krmtotf.SetUserAgentForTerraformProvider()

	for _, p := range projects {
		if p = strings.TrimSpace(p); p != "" {
			if err := createProject(ctx, httpClient, p); err != nil {
				return err
			}
		}
	}

	kccConfig := kccmanager.Config{
		HTTPClient:                 httpClient,
		GRPCUnaryClientInterceptor: toMock,
		GCPAccessToken:             "dummytoken",
	}
	mgr, err := kccmanager.New(ctx, restConfig, kccConfig)
	if err != nil {
		return fmt.Errorf("building Config Connector's manager: %w", err)
	}
	if err := sqlv1beta1.AddToScheme(mgr.GetScheme()); err != nil {
		return err
	}
	cfg := &config.ControllerConfig{HTTPClient: httpClient, UserAgent: "gaps-gcp-fakecloud"}
	if err := controller.AddToManager(ctx, mgr, cfg); err != nil {
		return fmt.Errorf("adding gaps-gcp controllers: %w", err)
	}

	klog.Infof("fakecloud: Config Connector and gaps-gcp against mockgcp; projects %v", projects)
	return mgr.Start(ctx)
}

func createProject(ctx context.Context, httpClient *http.Client, id string) error {
	crm, err := cloudresourcemanagerv1.NewService(ctx, option.WithHTTPClient(httpClient))
	if err != nil {
		return err
	}
	op, err := crm.Projects.Create(&cloudresourcemanagerv1.Project{ProjectId: id}).Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("creating fake project %q: %w", id, err)
	}
	for i := 0; i < 100 && !op.Done; i++ {
		time.Sleep(100 * time.Millisecond)
		if op, err = crm.Operations.Get(op.Name).Context(ctx).Do(); err != nil {
			return err
		}
	}
	if !op.Done {
		return fmt.Errorf("fake project %q was not created", id)
	}
	return nil
}
