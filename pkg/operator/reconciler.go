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

package operator

import (
	"context"
	"fmt"
	"strings"
	"time"

	operatorv1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/operator/pkg/apis/core/v1beta1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const fieldOwner = client.FieldOwner(ManagedByValue)

// Reconciler keeps the managers, and their RBAC bindings, matching
// Desired. Every event leads to one full pass: read Config Connector's
// objects, apply what should exist, delete what the operator made that
// should not.
type Reconciler struct {
	Client    client.Client
	Settings  Settings
	Isolation string
	// Identity is the default identity, IdentityOwn or
	// IdentityConfigConnector ("" is IdentityOwn); the annotation
	// IdentityAnnotation overrides it per scope.
	Identity string
	// Resync is how often a pass runs with no event.
	Resync time.Duration
}

// The single key every event maps to.
var syncKey = reconcile.Request{NamespacedName: types.NamespacedName{Name: Name + "-managers"}}

// SetupWithManager watches ConfigConnector, ConfigConnectorContext, the
// operator's Deployments, and the service accounts: Config Connector's and
// the managers' own.
func (r *Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	all := handler.EnqueueRequestsFromMapFunc(func(context.Context, client.Object) []reconcile.Request {
		return []reconcile.Request{syncKey}
	})
	ours := builder.WithPredicates(predicate.NewPredicateFuncs(func(o client.Object) bool {
		return o.GetLabels()[ManagedByLabel] == ManagedByValue
	}))
	serviceAccounts := builder.WithPredicates(predicate.NewPredicateFuncs(func(o client.Object) bool {
		n := o.GetName()
		return n == ClusterModeKSA || strings.HasPrefix(n, NamespacedKSAPrefix) || o.GetLabels()[ManagedByLabel] == ManagedByValue
	}))
	return ctrl.NewControllerManagedBy(mgr).
		Named(Name+"-operator").
		Watches(&operatorv1beta1.ConfigConnector{}, all).
		Watches(&operatorv1beta1.ConfigConnectorContext{}, all).
		Watches(&appsv1.Deployment{}, all, ours).
		Watches(&corev1.ServiceAccount{}, all, serviceAccounts).
		Complete(r)
}

// Reconcile is one pass.
func (r *Reconciler) Reconcile(ctx context.Context, _ reconcile.Request) (reconcile.Result, error) {
	log := klog.FromContext(ctx)
	observed, err := r.observe(ctx)
	if err != nil {
		return reconcile.Result{}, err
	}
	managers, err := Desired(observed)
	if err != nil {
		return reconcile.Result{}, err
	}

	keepDeployments := map[types.NamespacedName]bool{}
	keepCRBs := map[string]bool{}
	keepRBs := map[types.NamespacedName]bool{}
	keepSAs := map[types.NamespacedName]bool{}
	for _, m := range managers {
		// Config Connector's operator makes its service account for the
		// scope; until it exists Config Connector is not set up there (and
		// there is no identity to copy), so wait for it.
		kcc := &corev1.ServiceAccount{}
		if err := r.Client.Get(ctx, types.NamespacedName{Namespace: m.Namespace, Name: m.ConfigConnectorServiceAccount}, kcc); err != nil {
			if apierrors.IsNotFound(err) {
				log.Info("waiting for Config Connector's service account", "serviceAccount", m.Namespace+"/"+m.ConfigConnectorServiceAccount, "manager", m.Name)
				continue
			}
			return reconcile.Result{}, err
		}
		if sa := ServiceAccount(m, kcc); sa != nil {
			keepSAs[client.ObjectKeyFromObject(sa)] = true
			existing := &corev1.ServiceAccount{}
			switch err := r.Client.Get(ctx, client.ObjectKeyFromObject(sa), existing); {
			case apierrors.IsNotFound(err):
				if gsa := sa.Annotations[WorkloadIdentityAnnotation]; gsa != "" {
					// The one step outside the cluster: without it Google
					// refuses the manager's tokens (docs/design.md 9.4).
					log.Info("manager runs as its own service account; for Workload Identity the Google service account must allow it",
						"manager", m.Name, "googleServiceAccount", gsa,
						"role", "roles/iam.workloadIdentityUser",
						"member", "serviceAccount:<project>.svc.id.goog["+sa.Namespace+"/"+sa.Name+"]")
				}
			case err != nil:
				return reconcile.Result{}, err
			}
			if err := r.Client.Patch(ctx, sa, client.Apply, fieldOwner, client.ForceOwnership); err != nil {
				return reconcile.Result{}, fmt.Errorf("applying ServiceAccount %s/%s: %w", sa.Namespace, sa.Name, err)
			}
		}
		for _, crb := range ClusterRoleBindings(m) {
			keepCRBs[crb.Name] = true
			if err := r.Client.Patch(ctx, crb, client.Apply, fieldOwner, client.ForceOwnership); err != nil {
				return reconcile.Result{}, fmt.Errorf("applying ClusterRoleBinding %s: %w", crb.Name, err)
			}
		}
		for _, rb := range RoleBindings(m) {
			keepRBs[client.ObjectKeyFromObject(rb)] = true
			if err := r.Client.Patch(ctx, rb, client.Apply, fieldOwner, client.ForceOwnership); err != nil {
				return reconcile.Result{}, fmt.Errorf("applying RoleBinding %s/%s: %w", rb.Namespace, rb.Name, err)
			}
		}
		d := Deployment(m, r.Settings)
		keepDeployments[client.ObjectKeyFromObject(d)] = true
		if err := r.Client.Patch(ctx, d, client.Apply, fieldOwner, client.ForceOwnership); err != nil {
			return reconcile.Result{}, fmt.Errorf("applying Deployment %s/%s: %w", d.Namespace, d.Name, err)
		}
	}

	if err := r.prune(ctx, keepDeployments, keepCRBs, keepRBs, keepSAs); err != nil {
		return reconcile.Result{}, err
	}
	log.V(1).Info("managers in sync", "count", len(keepDeployments))
	return reconcile.Result{RequeueAfter: r.Resync}, nil
}

func (r *Reconciler) observe(ctx context.Context) (Observed, error) {
	o := Observed{Isolation: r.Isolation, Identity: r.Identity}
	cc := &operatorv1beta1.ConfigConnector{}
	switch err := r.Client.Get(ctx, types.NamespacedName{Name: ConfigConnectorName}, cc); {
	case err == nil:
		o.ConfigConnector = cc
	case apierrors.IsNotFound(err) || meta.IsNoMatchError(err):
	default:
		return o, err
	}
	contexts := &operatorv1beta1.ConfigConnectorContextList{}
	switch err := r.Client.List(ctx, contexts); {
	case err == nil:
		o.Contexts = contexts.Items
	case meta.IsNoMatchError(err):
	default:
		return o, err
	}
	ksa := &corev1.ServiceAccount{}
	switch err := r.Client.Get(ctx, types.NamespacedName{Namespace: CNRMSystemNamespace, Name: ClusterModeKSA}, ksa); {
	case err == nil:
		o.ClusterKSA = ksa
	case apierrors.IsNotFound(err):
	default:
		return o, err
	}
	return o, nil
}

// prune deletes what the operator made that no manager needs now.
func (r *Reconciler) prune(ctx context.Context, deployments map[types.NamespacedName]bool, crbs map[string]bool, rbs map[types.NamespacedName]bool, sas map[types.NamespacedName]bool) error {
	selector := client.MatchingLabels{ManagedByLabel: ManagedByValue}
	dl := &appsv1.DeploymentList{}
	if err := r.Client.List(ctx, dl, selector); err != nil {
		return err
	}
	for i := range dl.Items {
		d := &dl.Items[i]
		if !deployments[client.ObjectKeyFromObject(d)] {
			if err := client.IgnoreNotFound(r.Client.Delete(ctx, d)); err != nil {
				return err
			}
		}
	}
	crbl := &rbacv1.ClusterRoleBindingList{}
	if err := r.Client.List(ctx, crbl, selector); err != nil {
		return err
	}
	for i := range crbl.Items {
		if !crbs[crbl.Items[i].Name] {
			if err := client.IgnoreNotFound(r.Client.Delete(ctx, &crbl.Items[i])); err != nil {
				return err
			}
		}
	}
	rbl := &rbacv1.RoleBindingList{}
	if err := r.Client.List(ctx, rbl, selector); err != nil {
		return err
	}
	for i := range rbl.Items {
		if !rbs[client.ObjectKeyFromObject(&rbl.Items[i])] {
			if err := client.IgnoreNotFound(r.Client.Delete(ctx, &rbl.Items[i])); err != nil {
				return err
			}
		}
	}
	// Service accounts after the Deployments that ran as them.
	sal := &corev1.ServiceAccountList{}
	if err := r.Client.List(ctx, sal, selector); err != nil {
		return err
	}
	for i := range sal.Items {
		if !sas[client.ObjectKeyFromObject(&sal.Items[i])] {
			if err := client.IgnoreNotFound(r.Client.Delete(ctx, &sal.Items[i])); err != nil {
				return err
			}
		}
	}
	return nil
}
