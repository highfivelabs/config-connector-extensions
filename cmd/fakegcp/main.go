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

// Command fakegcp serves Config Connector's mockgcp over HTTP inside a
// cluster, for managers started with --fake-cloud-url (pkg/fakecloud). The
// fake cloud is in memory: it starts with the projects named by --projects
// and forgets everything when the process stops. For tests only.
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
	"golang.org/x/oauth2"
	cloudresourcemanagerv1 "google.golang.org/api/cloudresourcemanager/v1"
	"google.golang.org/api/option"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/highfivelabs/gaps-gcp/pkg/fakecloud"
)

func main() {
	var (
		projects string
		addr     string
	)
	flag.StringVar(&projects, "projects", "mock-project", "comma-separated project IDs to create in the fake cloud at start")
	flag.StringVar(&addr, "listen", ":8080", "address to serve the fake cloud on")
	// No klog.InitFlags: mockgcp's dependencies already register klog's
	// flags on the default flag set.
	flag.Parse()

	ctx := ctrl.SetupSignalHandler()
	ctrl.SetLogger(klog.NewKlogr())
	if err := run(ctx, addr, strings.Split(projects, ",")); err != nil {
		klog.Errorf("fakegcp: %v", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, addr string, projects []string) error {
	kube, err := client.New(ctrl.GetConfigOrDie(), client.Options{})
	if err != nil {
		return fmt.Errorf("building kube client: %w", err)
	}
	mock, err := mockgcp.NewMockRoundTripper(ctx, kube, storage.NewInMemoryStorage())
	if err != nil {
		return fmt.Errorf("building mockgcp: %w", err)
	}
	go func() {
		if err := mock.Run(ctx); err != nil {
			klog.Errorf("mockgcp gRPC server: %v", err)
		}
	}()
	httpClient := &http.Client{Transport: mock}
	ctx = context.WithValue(ctx, oauth2.HTTPClient, httpClient)
	for _, p := range projects {
		if p = strings.TrimSpace(p); p != "" {
			if err := createProject(ctx, httpClient, p); err != nil {
				return err
			}
		}
	}

	srv := &http.Server{Addr: addr, Handler: fakecloud.Handler(mock), ReadHeaderTimeout: 10 * time.Second}
	go func() {
		<-ctx.Done()
		_ = srv.Close()
	}()
	klog.Infof("fake cloud serving on %s with projects %v", addr, projects)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}

func createProject(ctx context.Context, httpClient *http.Client, projectID string) error {
	crm, err := cloudresourcemanagerv1.NewService(ctx, option.WithHTTPClient(httpClient))
	if err != nil {
		return err
	}
	op, err := crm.Projects.Create(&cloudresourcemanagerv1.Project{ProjectId: projectID}).Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("creating project %q in the fake cloud: %w", projectID, err)
	}
	for i := 0; i < 100 && !op.Done; i++ {
		time.Sleep(100 * time.Millisecond)
		if op, err = crm.Operations.Get(op.Name).Context(ctx).Do(); err != nil {
			return err
		}
	}
	if !op.Done {
		return fmt.Errorf("project %q was not created in the fake cloud", projectID)
	}
	return nil
}
