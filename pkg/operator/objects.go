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
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

// The ClusterRoles the operator binds a manager's service account to
// (config/operator/rbac.yaml defines them; the operator may only bind
// these two, by name).
const (
	// ManagerClusterRole: this project's kinds and what reconciling them
	// reads.
	ManagerClusterRole = Name + "-manager"
	// ManagerClusterScopedRole: the cluster-scoped reads a manager scoped
	// to one namespace still needs (CRDs, ConfigConnector, namespaces).
	ManagerClusterScopedRole = Name + "-manager-cluster"
)

// Settings are how the operator builds a manager's Deployment.
type Settings struct {
	// Image is the config-connector-extensions manager image.
	Image string
	// ExtraArgs are appended to every manager's flags (for the fake cloud
	// in tests: --fake-cloud-url).
	ExtraArgs []string
}

func labels(m Manager) map[string]string {
	l := map[string]string{
		ManagedByLabel: ManagedByValue,
		NameLabel:      ManagerName,
	}
	if m.ScopedNamespace != "" {
		l[ScopedNamespaceLabel] = m.ScopedNamespace
	}
	return l
}

// ServiceAccount is the manager's own service account, for IdentityOwn: in
// the namespace of Config Connector's (kcc), carrying kcc's Workload
// Identity annotation, so it acts as the same Google service account once
// that account allows it (roles/iam.workloadIdentityUser for
// <pool>[<namespace>/<name>]). Without the annotation (a key Secret) it
// carries none. nil for IdentityConfigConnector.
func ServiceAccount(m Manager, kcc *corev1.ServiceAccount) *corev1.ServiceAccount {
	if m.Identity != IdentityOwn {
		return nil
	}
	sa := &corev1.ServiceAccount{
		TypeMeta:   metav1.TypeMeta{APIVersion: "v1", Kind: "ServiceAccount"},
		ObjectMeta: metav1.ObjectMeta{Name: m.ServiceAccount, Namespace: m.Namespace, Labels: labels(m)},
	}
	if gsa := kcc.Annotations[WorkloadIdentityAnnotation]; gsa != "" {
		sa.Annotations = map[string]string{WorkloadIdentityAnnotation: gsa}
	}
	return sa
}

// Deployment is the manager's Deployment.
func Deployment(m Manager, s Settings) *appsv1.Deployment {
	args := []string{}
	if m.ScopedNamespace != "" {
		args = append(args, "--scoped-namespace="+m.ScopedNamespace)
	}
	args = append(args, m.Args...)
	args = append(args, s.ExtraArgs...)

	container := corev1.Container{
		Name:  "manager",
		Image: s.Image,
		Args:  args,
		Resources: corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("50m"),
				corev1.ResourceMemory: resource.MustParse("128Mi"),
			},
			Limits: corev1.ResourceList{
				corev1.ResourceMemory: resource.MustParse("512Mi"),
			},
		},
		SecurityContext: &corev1.SecurityContext{
			AllowPrivilegeEscalation: ptr.To(false),
			ReadOnlyRootFilesystem:   ptr.To(true),
			RunAsNonRoot:             ptr.To(true),
			Capabilities:             &corev1.Capabilities{Drop: []corev1.Capability{"ALL"}},
		},
	}
	// A writable /tmp on a read-only root filesystem.
	container.VolumeMounts = []corev1.VolumeMount{{Name: "tmp", MountPath: "/tmp"}}
	pod := corev1.PodSpec{
		ServiceAccountName: m.ServiceAccount,
		Containers:         []corev1.Container{container},
		Volumes:            []corev1.Volume{{Name: "tmp", VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}}}},
		SecurityContext: &corev1.PodSecurityContext{
			RunAsNonRoot:   ptr.To(true),
			SeccompProfile: &corev1.SeccompProfile{Type: corev1.SeccompProfileTypeRuntimeDefault},
		},
	}
	if m.CredentialSecret != "" {
		// As Config Connector's withsecretvolume auth mode.
		pod.Containers[0].Env = []corev1.EnvVar{{Name: "GOOGLE_APPLICATION_CREDENTIALS", Value: credentialsFile}}
		pod.Containers[0].VolumeMounts = append(pod.Containers[0].VolumeMounts, corev1.VolumeMount{Name: credentialsVolumeName, MountPath: credentialsMountPath, ReadOnly: true})
		pod.Volumes = append(pod.Volumes, corev1.Volume{
			Name:         credentialsVolumeName,
			VolumeSource: corev1.VolumeSource{Secret: &corev1.SecretVolumeSource{SecretName: m.CredentialSecret}},
		})
	}

	l := labels(m)
	selector := map[string]string{NameLabel: ManagerName}
	if m.ScopedNamespace != "" {
		selector[ScopedNamespaceLabel] = m.ScopedNamespace
	}
	return &appsv1.Deployment{
		TypeMeta:   metav1.TypeMeta{APIVersion: "apps/v1", Kind: "Deployment"},
		ObjectMeta: metav1.ObjectMeta{Name: m.Name, Namespace: m.Namespace, Labels: l},
		Spec: appsv1.DeploymentSpec{
			Replicas: ptr.To(int32(1)),
			// One manager at a time per scope: Recreate, so two never
			// reconcile the same objects during a rollout.
			Strategy: appsv1.DeploymentStrategy{Type: appsv1.RecreateDeploymentStrategyType},
			Selector: &metav1.LabelSelector{MatchLabels: selector},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: l},
				Spec:       pod,
			},
		},
	}
}

func subject(m Manager) rbacv1.Subject {
	return rbacv1.Subject{Kind: rbacv1.ServiceAccountKind, Name: m.ServiceAccount, Namespace: m.Namespace}
}

// ClusterRoleBindings are the cluster-wide bindings for a manager: in
// cluster mode both roles; for a manager scoped to one namespace only
// ManagerClusterScopedRole (its namespaced rights come from RoleBindings).
func ClusterRoleBindings(m Manager) []*rbacv1.ClusterRoleBinding {
	crb := func(name, role string) *rbacv1.ClusterRoleBinding {
		return &rbacv1.ClusterRoleBinding{
			TypeMeta:   metav1.TypeMeta{APIVersion: "rbac.authorization.k8s.io/v1", Kind: "ClusterRoleBinding"},
			ObjectMeta: metav1.ObjectMeta{Name: name, Labels: labels(m)},
			RoleRef:    rbacv1.RoleRef{APIGroup: "rbac.authorization.k8s.io", Kind: "ClusterRole", Name: role},
			Subjects:   []rbacv1.Subject{subject(m)},
		}
	}
	if m.ScopedNamespace != "" {
		return []*rbacv1.ClusterRoleBinding{crb(m.Name, ManagerClusterScopedRole)}
	}
	return []*rbacv1.ClusterRoleBinding{
		crb(m.Name, ManagerClusterRole),
		crb(m.Name+":cluster-scoped", ManagerClusterScopedRole),
	}
}

// RoleBindings are the namespaced bindings: ManagerClusterRole in the one
// namespace a scoped manager watches; none in cluster mode.
func RoleBindings(m Manager) []*rbacv1.RoleBinding {
	if m.ScopedNamespace == "" {
		return nil
	}
	return []*rbacv1.RoleBinding{{
		TypeMeta:   metav1.TypeMeta{APIVersion: "rbac.authorization.k8s.io/v1", Kind: "RoleBinding"},
		ObjectMeta: metav1.ObjectMeta{Name: ManagerName, Namespace: m.ScopedNamespace, Labels: labels(m)},
		RoleRef:    rbacv1.RoleRef{APIGroup: "rbac.authorization.k8s.io", Kind: "ClusterRole", Name: ManagerClusterRole},
		Subjects:   []rbacv1.Subject{subject(m)},
	}}
}
