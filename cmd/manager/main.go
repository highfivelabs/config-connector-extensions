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

// Command manager runs the config-connector-extensions controllers. It is a cut-down
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

	"github.com/highfivelabs/config-connector-extensions/pkg/controller"
	"github.com/highfivelabs/config-connector-extensions/pkg/slotguard"
)

// useFakeCloud points the controllers at a fake Google Cloud. It is nil
// unless the binary is built with -tags fakecloud (fakecloud.go), so a
// production build cannot be sent to a fake.
var useFakeCloud func(*config.ControllerConfig)

func main() {
	var (
		scopedNamespace     string
		userProjectOverride bool
		billingProject      string
		metricsBindAddress  string
		slot                string
	)
	flag.StringVar(&scopedNamespace, "scoped-namespace", "", "watch only this namespace; if unset, watch the whole cluster")
	flag.BoolVar(&userProjectOverride, "user-project-override", false, "use the resource project for preconditions, quota and billing (as Config Connector's flag)")
	flag.StringVar(&billingProject, "billing-project", "", "project for preconditions, quota and billing when --user-project-override is set")
	flag.StringVar(&slot, "slot", "", "the version slot this manager is (default $"+slotguard.EnvVar+"); when set, the slot guard marks each resource with the slot and refuses to write one another slot holds (docs/design.md, \"Version slots\")")
	flag.StringVar(&metricsBindAddress, "metrics-bind-address", "0", "controller-runtime metrics address; \"0\" disables it")
	// No klog.InitFlags: Config Connector's dependencies already register
	// klog's flags on the default flag set (calling it again panics).
	flag.Parse()

	ctx := ctrl.SetupSignalHandler()
	ctrl.SetLogger(klog.NewKlogr())

	scheme := runtime.NewScheme()
	for _, add := range []func(*runtime.Scheme) error{
		clientgoscheme.AddToScheme,
		// directbase reads ConfigConnector / ConfigConnectorContext to honour
		// actuation mode, as Config Connector's own controllers do.
		operatorv1beta1.AddToScheme,
		controller.AddToScheme,
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
		UserAgent:           "config-connector-extensions",
		UserProjectOverride: userProjectOverride,
		BillingProject:      billingProject,
	}
	// Only a build with -tags fakecloud sets useFakeCloud (fakecloud.go); a
	// production build has no --fake-cloud-url flag and no fake-cloud code.
	if useFakeCloud != nil {
		useFakeCloud(cfg)
	}
	if err := cfg.Init(ctx); err != nil {
		klog.Fatalf("initializing controller config: %v", err)
	}

	if err := controller.AddToManager(ctx, mgr, cfg, controller.Options{Slot: slotguard.Resolve(slot)}); err != nil {
		klog.Fatalf("adding controllers: %v", err)
	}

	klog.Info("starting config-connector-extensions manager")
	if err := mgr.Start(ctx); err != nil {
		klog.Errorf("manager exited: %v", err)
		os.Exit(1)
	}
}
