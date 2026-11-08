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

// Package operator runs config-connector-extensions' managers where Config
// Connector runs its own, so a gap kind acts as the same Google identity
// Config Connector would for the same namespace. It reads what Config
// Connector's operator reads (ConfigConnector, ConfigConnectorContext) and
// mirrors its layout:
//
//   - cluster mode: one manager in cnrm-system, for the scope of Config
//     Connector's service account cnrm-controller-manager, which Config
//     Connector's operator annotates with
//     ConfigConnector.spec.googleServiceAccount for Workload Identity
//     (operator/pkg/controllers/configconnector,
//     objectTransformForWorkloadIdentity), or, with
//     spec.credentialSecretName instead, the same key Secret mounted the
//     same way (config/installbundle/components/manager/authmodes/
//     withsecretvolume);
//   - namespaced mode: for each ConfigConnectorContext in namespace N, one
//     manager watching only N (--scoped-namespace=N), for the scope of
//     Config Connector's service account cnrm-controller-manager-N, which
//     Config Connector's operator annotates with that context's
//     googleServiceAccount (operator/channels/packages/configconnector/<v>/
//     namespaced/per-namespace-components.yaml), in cnrm-system, or in the
//     context's spec.managerNamespace under dedicated isolation.
//
// Identity. By default (IdentityOwn) a manager runs as its own Kubernetes
// service account, beside Config Connector's in the same namespace, bound
// only to this project's roles, and carrying the same Workload Identity
// annotation as Config Connector's (copied from it), so it acts as the same
// Google service account. Workload Identity then needs one more
// roles/iam.workloadIdentityUser binding on that Google service account for
// the new Kubernetes service account; a key Secret needs nothing more. With
// IdentityConfigConnector (the --identity flag, or the annotation
// gcp.hfl.dev/identity on the ConfigConnector object or on one
// ConfigConnectorContext) a manager runs as Config Connector's own service
// account instead: no extra binding, but the manager then holds every
// Kubernetes right Config Connector's service account has
// (docs/design.md, section 9.4).
//
// The operator holds no Google credentials and never changes Config
// Connector's service accounts; the only service accounts it writes are
// its own managers'.
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

// Name is the project's name; every name the operator gives an object
// starts with it (the Makefile's NAME, for a rename).
const Name = "config-connector-extensions"

// Labels on everything the operator makes, so it can find what to remove.
const (
	ManagedByLabel       = "app.kubernetes.io/managed-by"
	ManagedByValue       = Name + "-operator"
	NameLabel            = "app.kubernetes.io/name"
	ManagerName          = Name + "-manager"
	ScopedNamespaceLabel = "gcp.hfl.dev/scoped-namespace"
)

// Identity: whose Kubernetes service account a manager runs as.
const (
	// IdentityOwn: the manager's own service account, with Config
	// Connector's Workload Identity annotation copied onto it (the default).
	IdentityOwn = "own"
	// IdentityConfigConnector: Config Connector's service account itself.
	IdentityConfigConnector = "config-connector"
	// IdentityAnnotation chooses the identity on the ConfigConnector object
	// (every manager) or on a ConfigConnectorContext (that namespace's
	// manager), over the operator's --identity flag.
	IdentityAnnotation = "gcp.hfl.dev/identity"
)

// Manager is one config-connector-extensions manager the cluster should run.
type Manager struct {
	// Name of its Deployment.
	Name string
	// Namespace it runs in: where its service account is, beside Config
	// Connector's.
	Namespace string
	// ServiceAccount is the one it runs as: its own (named Name) with
	// IdentityOwn, Config Connector's with IdentityConfigConnector.
	ServiceAccount string
	// ConfigConnectorServiceAccount is Config Connector's service account
	// for the same scope, in Namespace: the operator waits for it, and
	// copies its Workload Identity annotation onto the manager's own.
	ConfigConnectorServiceAccount string
	// Identity is IdentityOwn or IdentityConfigConnector.
	Identity string
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
	// Identity is the operator's default (its --identity flag); "" means
	// IdentityOwn.
	Identity string
}

// identity is the identity for one scope: the context's annotation, else
// the ConfigConnector object's, else the operator's default.
func identity(o Observed, ccc *operatorv1beta1.ConfigConnectorContext) (string, error) {
	pick := func(kind, name, v string) (string, bool, error) {
		switch v {
		case "":
			return "", false, nil
		case IdentityOwn, IdentityConfigConnector:
			return v, true, nil
		default:
			return "", false, fmt.Errorf("%s %s has %s: %q; want %q or %q", kind, name, IdentityAnnotation, v, IdentityOwn, IdentityConfigConnector)
		}
	}
	if ccc != nil {
		if v, ok, err := pick("ConfigConnectorContext", ccc.Namespace+"/"+ccc.Name, ccc.Annotations[IdentityAnnotation]); ok || err != nil {
			return v, err
		}
	}
	if cc := o.ConfigConnector; cc != nil {
		if v, ok, err := pick("ConfigConnector", cc.Name, cc.Annotations[IdentityAnnotation]); ok || err != nil {
			return v, err
		}
	}
	switch o.Identity {
	case "", IdentityOwn:
		return IdentityOwn, nil
	case IdentityConfigConnector:
		return IdentityConfigConnector, nil
	default:
		return "", fmt.Errorf("--identity is %q; want %q or %q", o.Identity, IdentityOwn, IdentityConfigConnector)
	}
}

// withIdentity sets who m runs as.
func withIdentity(m Manager, id string) Manager {
	m.Identity = id
	m.ServiceAccount = m.Name
	if id == IdentityConfigConnector {
		m.ServiceAccount = m.ConfigConnectorServiceAccount
	}
	return m
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
		id, err := identity(o, nil)
		if err != nil {
			return nil, err
		}
		return []Manager{withIdentity(clusterManager(secret), id)}, nil
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
		id, err := identity(o, nil)
		if err != nil {
			return nil, err
		}
		return []Manager{withIdentity(clusterManager(secret), id)}, nil
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
			id, err := identity(o, ccc)
			if err != nil {
				return nil, err
			}
			out = append(out, withIdentity(Manager{
				Name:                          ManagerName + "-" + ns,
				Namespace:                     mgrNS,
				ConfigConnectorServiceAccount: NamespacedKSAPrefix + ns,
				ScopedNamespace:               ns,
				Args:                          args,
			}, id))
		}
		sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
		return out, nil
	default:
		return nil, fmt.Errorf("ConfigConnector %s has spec.mode %q; want %q or %q", cc.Name, cc.Spec.Mode, ClusterMode, NamespacedMode)
	}
}

func clusterManager(secret string) Manager {
	return Manager{
		Name:                          ManagerName,
		Namespace:                     CNRMSystemNamespace,
		ConfigConnectorServiceAccount: ClusterModeKSA,
		CredentialSecret:              secret,
	}
}
