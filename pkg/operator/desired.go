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

// Package operator runs gaps-gcp's managers where Config Connector runs its
// own, under Config Connector's own Kubernetes service accounts, so a gap
// kind acts as the same Google identity Config Connector would for the same
// namespace. It reads what Config Connector's operator reads
// (ConfigConnector, ConfigConnectorContext) and mirrors its layout:
//
//   - cluster mode: one manager in cnrm-system as the service account
//     cnrm-controller-manager, which Config Connector's operator annotates
//     with ConfigConnector.spec.googleServiceAccount for Workload Identity
//     (operator/pkg/controllers/configconnector,
//     objectTransformForWorkloadIdentity), or, with
//     spec.credentialSecretName instead, the same key Secret mounted the
//     same way (config/installbundle/components/manager/authmodes/
//     withsecretvolume);
//   - namespaced mode: for each ConfigConnectorContext in namespace N, one
//     manager watching only N (--scoped-namespace=N), as the service
//     account cnrm-controller-manager-N, which Config Connector's operator
//     annotates with that context's googleServiceAccount
//     (operator/channels/packages/configconnector/<v>/namespaced/
//     per-namespace-components.yaml), in cnrm-system, or in the context's
//     spec.managerNamespace under dedicated isolation.
//
// The operator never creates or annotates a service account and holds no
// Google credentials: identity stays Config Connector's to grant.
package operator

import (
	"fmt"
	"sort"

	operatorv1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/operator/pkg/apis/core/v1beta1"
	corev1 "k8s.io/api/core/v1"
)

// The names Config Connector's operator uses (operator/pkg/k8s/constants.go).
const (
	CNRMSystemNamespace        = "cnrm-system"
	ClusterModeKSA             = "cnrm-controller-manager"
	NamespacedKSAPrefix        = "cnrm-controller-manager-"
	ConfigConnectorName        = operatorv1beta1.ConfigConnectorAllowedName
	WorkloadIdentityAnnotation = "iam.gke.io/gcp-service-account"
	ClusterMode                = "cluster"
	NamespacedMode             = "namespaced"
	IsolationShared            = "shared"
	IsolationDedicated         = "dedicated"
	resourceProjectPolicy      = "RESOURCE_PROJECT"
	billingProjectPolicy       = "BILLING_PROJECT"
	credentialsMountPath       = "/var/secrets/google"
	credentialsVolumeName      = "gcp-service-account"
	credentialsFile            = credentialsMountPath + "/key.json"
	defaultCredentialsSecret   = "gcp-key"
)

// Labels on everything the operator makes, so it can find what to remove.
const (
	ManagedByLabel       = "app.kubernetes.io/managed-by"
	ManagedByValue       = "gaps-gcp-operator"
	NameLabel            = "app.kubernetes.io/name"
	ManagerName          = "gaps-gcp-manager"
	ScopedNamespaceLabel = "gcp.hfl.dev/scoped-namespace"
)

// Manager is one gaps-gcp manager the cluster should run.
type Manager struct {
	// Name of its Deployment.
	Name string
	// Namespace it runs in: where its service account is.
	Namespace string
	// ServiceAccount is Config Connector's, for the same scope.
	ServiceAccount string
	// ScopedNamespace is the one namespace it watches; "" for all.
	ScopedNamespace string
	// CredentialSecret, when set, is a key Secret in Namespace mounted as
	// Config Connector mounts it; "" for Workload Identity.
	CredentialSecret string
	// Args are the manager's flags from the context (project policy).
	Args []string
}

// Observed is what the operator reads from the cluster.
type Observed struct {
	// ConfigConnector is the ConfigConnector object; nil when there is none.
	ConfigConnector *operatorv1beta1.ConfigConnector
	// Contexts are the ConfigConnectorContexts in every namespace.
	Contexts []operatorv1beta1.ConfigConnectorContext
	// Isolation is Config Connector's --manager-namespace-isolation.
	Isolation string
	// ClusterKSA is cnrm-system/cnrm-controller-manager; nil when absent.
	ClusterKSA *corev1.ServiceAccount
}

// Desired is what should run. Without a ConfigConnector object (Config
// Connector installed from its install bundle, not its operator), a manager
// runs in cluster mode when cnrm-system/cnrm-controller-manager exists:
// with Workload Identity when that service account carries the
// iam.gke.io/gcp-service-account annotation, else with the bundle's key
// Secret, gcp-key, as the bundle's withsecretvolume variant mounts it.
func Desired(o Observed) ([]Manager, error) {
	cc := o.ConfigConnector
	if cc == nil {
		if o.ClusterKSA == nil {
			return nil, nil
		}
		secret := ""
		if o.ClusterKSA.Annotations[WorkloadIdentityAnnotation] == "" {
			secret = defaultCredentialsSecret
		}
		return []Manager{clusterManager(secret)}, nil
	}
	if cc.GetDeletionTimestamp() != nil {
		return nil, nil
	}
	switch cc.GetMode() {
	case ClusterMode:
		secret := ""
		if cc.Spec.GoogleServiceAccount == "" {
			secret = cc.Spec.CredentialSecretName
			if secret == "" {
				return nil, fmt.Errorf("ConfigConnector %s is in cluster mode with neither spec.googleServiceAccount nor spec.credentialSecretName", cc.Name)
			}
		}
		return []Manager{clusterManager(secret)}, nil
	case NamespacedMode:
		var out []Manager
		for i := range o.Contexts {
			ccc := &o.Contexts[i]
			if ccc.GetDeletionTimestamp() != nil {
				continue
			}
			ns := ccc.Namespace
			mgrNS := CNRMSystemNamespace
			if o.Isolation == IsolationDedicated && ccc.Spec.ManagerNamespace != "" {
				mgrNS = ccc.Spec.ManagerNamespace
			}
			var args []string
			switch ccc.GetRequestProjectPolicy() {
			case resourceProjectPolicy:
				args = append(args, "--user-project-override")
			case billingProjectPolicy:
				args = append(args, "--user-project-override", "--billing-project="+ccc.Spec.BillingProject)
			}
			out = append(out, Manager{
				Name:            ManagerName + "-" + ns,
				Namespace:       mgrNS,
				ServiceAccount:  NamespacedKSAPrefix + ns,
				ScopedNamespace: ns,
				Args:            args,
			})
		}
		sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
		return out, nil
	default:
		return nil, fmt.Errorf("ConfigConnector %s has spec.mode %q; want %q or %q", cc.Name, cc.Spec.Mode, ClusterMode, NamespacedMode)
	}
}

func clusterManager(secret string) Manager {
	return Manager{
		Name:             ManagerName,
		Namespace:        CNRMSystemNamespace,
		ServiceAccount:   ClusterModeKSA,
		CredentialSecret: secret,
	}
}
