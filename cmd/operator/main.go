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

// Command operator runs this project's managers beside Config Connector's,
// each as its own service account carrying Config Connector's Workload
// Identity annotation, or with --identity=config-connector as Config
// Connector's own service accounts (pkg/operator). It holds no Google
// credentials and never calls Google Cloud.
package main

import (
	"flag"
	"os"
	"strings"
	"time"

	operatorv1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/operator/pkg/apis/core/v1beta1"
	appsv1 "k8s.io/api/apps/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	"github.com/highfivelabs/config-connector-extensions/pkg/operator"
)

type stringList []string

func (s *stringList) String() string     { return strings.Join(*s, ",") }
func (s *stringList) Set(v string) error { *s = append(*s, v); return nil }

func main() {
	var (
		image              string
		isolation          string
		identity           string
		resync             time.Duration
		metricsBindAddress string
		managerArgs        stringList
	)
	flag.StringVar(&image, "manager-image", os.Getenv("CCE_MANAGER_IMAGE"), "the config-connector-extensions manager image (required; default $CCE_MANAGER_IMAGE)")
	flag.StringVar(&isolation, "manager-namespace-isolation", operator.IsolationShared, "as Config Connector's operator flag: \"shared\" runs namespaced-mode managers in cnrm-system, \"dedicated\" in each context's spec.managerNamespace")
	flag.StringVar(&identity, "identity", operator.IdentityOwn, "whose Kubernetes service account managers run as: \"own\" (their own, with Config Connector's Workload Identity annotation; needs one roles/iam.workloadIdentityUser binding per Google service account) or \"config-connector\" (Config Connector's, with all its Kubernetes rights); the annotation "+operator.IdentityAnnotation+" on ConfigConnector or a ConfigConnectorContext overrides it")
	flag.DurationVar(&resync, "resync", 10*time.Minute, "how often to check the managers with no event")
	flag.StringVar(&metricsBindAddress, "metrics-bind-address", "0", "controller-runtime metrics address; \"0\" disables it")
	flag.Var(&managerArgs, "manager-arg", "a flag passed to every manager (repeatable)")
	klog.InitFlags(nil)
	flag.Parse()

	if image == "" {
		klog.Fatal("--manager-image is required")
	}
	if isolation != operator.IsolationShared && isolation != operator.IsolationDedicated {
		klog.Fatalf("--manager-namespace-isolation is %q; want %q or %q", isolation, operator.IsolationShared, operator.IsolationDedicated)
	}

	if identity != operator.IdentityOwn && identity != operator.IdentityConfigConnector {
		klog.Fatalf("--identity is %q; want %q or %q", identity, operator.IdentityOwn, operator.IdentityConfigConnector)
	}

	ctx := ctrl.SetupSignalHandler()
	ctrl.SetLogger(klog.NewKlogr())

	scheme := runtime.NewScheme()
	for _, add := range []func(*runtime.Scheme) error{clientgoscheme.AddToScheme, operatorv1beta1.AddToScheme} {
		if err := add(scheme); err != nil {
			klog.Fatalf("building scheme: %v", err)
		}
	}
	// Cache only what the operator made, among Deployments and bindings.
	ours := cache.ByObject{Label: labels.SelectorFromSet(labels.Set{operator.ManagedByLabel: operator.ManagedByValue})}
	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:  scheme,
		Metrics: metricsserver.Options{BindAddress: metricsBindAddress},
		Cache: cache.Options{ByObject: map[client.Object]cache.ByObject{
			&appsv1.Deployment{}:         ours,
			&rbacv1.ClusterRoleBinding{}: ours,
			&rbacv1.RoleBinding{}:        ours,
		}},
	})
	if err != nil {
		klog.Fatalf("creating manager: %v", err)
	}
	r := &operator.Reconciler{
		Client:    mgr.GetClient(),
		Settings:  operator.Settings{Image: image, ExtraArgs: managerArgs},
		Isolation: isolation,
		Identity:  identity,
		Resync:    resync,
	}
	if err := r.SetupWithManager(mgr); err != nil {
		klog.Fatalf("setting up the operator: %v", err)
	}
	klog.Info("starting config-connector-extensions operator")
	if err := mgr.Start(ctx); err != nil {
		klog.Errorf("operator exited: %v", err)
		os.Exit(1)
	}
}
