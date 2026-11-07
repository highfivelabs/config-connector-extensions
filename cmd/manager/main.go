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

// Command manager runs the gaps-gcp controllers. It is a cut-down
// counterpart of Config Connector's cmd/manager: one controller-runtime
// manager, cluster-scoped or scoped to one namespace, authenticating to
// Google Cloud with Application Default Credentials (Workload Identity in a
// cluster). It runs beside Config Connector, never inside it.
package main

import (
	"flag"
	"os"

	operatorv1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/operator/pkg/apis/core/v1beta1"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/config"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	sqlv1beta1 "github.com/highfivelabs/gaps-gcp/apis/sql/v1beta1"
	"github.com/highfivelabs/gaps-gcp/pkg/controller"
)

func main() {
	var (
		scopedNamespace     string
		userProjectOverride bool
		billingProject      string
		metricsBindAddress  string
	)
	flag.StringVar(&scopedNamespace, "scoped-namespace", "", "watch only this namespace; if unset, watch the whole cluster")
	flag.BoolVar(&userProjectOverride, "user-project-override", false, "use the resource project for preconditions, quota and billing (as Config Connector's flag)")
	flag.StringVar(&billingProject, "billing-project", "", "project for preconditions, quota and billing when --user-project-override is set")
	flag.StringVar(&metricsBindAddress, "metrics-bind-address", "0", "controller-runtime metrics address; \"0\" disables it")
	klog.InitFlags(nil)
	flag.Parse()

	ctx := ctrl.SetupSignalHandler()
	ctrl.SetLogger(klog.NewKlogr())

	scheme := runtime.NewScheme()
	for _, add := range []func(*runtime.Scheme) error{
		clientgoscheme.AddToScheme,
		// directbase reads ConfigConnector / ConfigConnectorContext to honour
		// actuation mode, as Config Connector's own controllers do.
		operatorv1beta1.AddToScheme,
		sqlv1beta1.AddToScheme,
	} {
		if err := add(scheme); err != nil {
			klog.Fatalf("building scheme: %v", err)
		}
	}

	opts := ctrl.Options{
		Scheme:  scheme,
		Metrics: metricsserver.Options{BindAddress: metricsBindAddress},
	}
	if scopedNamespace != "" {
		opts.Cache = cache.Options{DefaultNamespaces: map[string]cache.Config{scopedNamespace: {}}}
	}
	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), opts)
	if err != nil {
		klog.Fatalf("creating manager: %v", err)
	}

	cfg := &config.ControllerConfig{
		UserAgent:           "gaps-gcp",
		UserProjectOverride: userProjectOverride,
		BillingProject:      billingProject,
	}
	if err := cfg.Init(ctx); err != nil {
		klog.Fatalf("initializing controller config: %v", err)
	}

	if err := controller.AddToManager(ctx, mgr, cfg); err != nil {
		klog.Fatalf("adding controllers: %v", err)
	}

	klog.Info("starting gaps-gcp manager")
	if err := mgr.Start(ctx); err != nil {
		klog.Errorf("manager exited: %v", err)
		os.Exit(1)
	}
}
