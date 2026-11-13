// Copyright 2026 High Five Labs
// SPDX-License-Identifier: Apache-2.0

// Written by High Five Labs for config-connector-extensions after our
// principalaccessboundarypolicy_controller.go (copied from Config
// Connector): an IAM v3 policy binding (IAMPolicyBinding). Config
// Connector's pull request #14133 has the types but no controller. The
// binding's parent is an organization, folder or project; its policy is an
// IAMPrincipalAccessBoundaryPolicy by name or a policy's resource name.
// Display name, annotations and the condition change in place (patch with
// an update mask); the rest is immutable (CEL in the CRD).

package iam

import (
	"context"
	"fmt"
	"maps"
	"sort"
	"strings"

	gcp "cloud.google.com/go/iam/apiv3"
	iampb "cloud.google.com/go/iam/apiv3/iampb"
	refsv1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/apis/refs/v1beta1"
	k8sv1alpha1 "github.com/GoogleCloudPlatform/k8s-config-connector/pkg/apis/k8s/v1alpha1"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/config"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct/directbase"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct/registry"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	krm "github.com/highfivelabs/config-connector-extensions/apis/iam/v1alpha1"
	krmv1beta1 "github.com/highfivelabs/config-connector-extensions/apis/iam/v1beta1"
)

func init() {
	registry.RegisterModel(krm.IAMPolicyBindingGVK, NewPolicyBindingModel)
}

func NewPolicyBindingModel(ctx context.Context, config *config.ControllerConfig) (directbase.Model, error) {
	return &modelPolicyBinding{config: *config}, nil
}

var _ directbase.Model = &modelPolicyBinding{}

type modelPolicyBinding struct {
	config config.ControllerConfig
}

func (m *modelPolicyBinding) AdapterForObject(ctx context.Context, op *directbase.AdapterForObjectOperation) (directbase.Adapter, error) {
	u := op.GetUnstructured()
	obj := &krm.IAMPolicyBinding{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, obj); err != nil {
		return nil, fmt.Errorf("error converting to %T: %w", obj, err)
	}
	i, err := obj.GetIdentity(ctx, op.Reader)
	if err != nil {
		return nil, err
	}
	policy, err := resolvePolicyBindingPolicy(ctx, op.Reader, obj)
	if err != nil {
		return nil, err
	}
	gcpClient, err := m.client(ctx)
	if err != nil {
		return nil, err
	}
	return &PolicyBindingAdapter{
		id:        i.(*krm.IAMPolicyBindingIdentity),
		gcpClient: gcpClient,
		desired:   obj,
		policy:    policy,
	}, nil
}

// resolvePolicyBindingPolicy is the policy's resource name: policyRef's
// external, or the status.externalRef of the IAMPrincipalAccessBoundaryPolicy
// it names.
func resolvePolicyBindingPolicy(ctx context.Context, reader client.Reader, obj *krm.IAMPolicyBinding) (string, error) {
	ref := obj.Spec.PolicyRef
	if ref == nil {
		return "", fmt.Errorf("spec.policyRef is required")
	}
	if (ref.External == "") == (ref.Name == "") {
		return "", fmt.Errorf("spec.policyRef takes exactly one of external and name")
	}
	if ref.External != "" {
		return strings.TrimPrefix(strings.TrimPrefix(ref.External, "//iam.googleapis.com/"), "v3/"), nil
	}
	ns := ref.Namespace
	if ns == "" {
		ns = obj.GetNamespace()
	}
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(krmv1beta1.IAMPrincipalAccessBoundaryPolicyGVK)
	if err := reader.Get(ctx, types.NamespacedName{Namespace: ns, Name: ref.Name}, u); err != nil {
		return "", fmt.Errorf("reading spec.policyRef IAMPrincipalAccessBoundaryPolicy %s/%s: %w", ns, ref.Name, err)
	}
	external, _, _ := unstructured.NestedString(u.Object, "status", "externalRef")
	if external == "" {
		return "", fmt.Errorf("spec.policyRef IAMPrincipalAccessBoundaryPolicy %s/%s is not ready (no status.externalRef)", ns, ref.Name)
	}
	return external, nil
}

func (m *modelPolicyBinding) AdapterForURL(ctx context.Context, url string) (directbase.Adapter, error) {
	if !strings.HasPrefix(url, "//iam.googleapis.com/") {
		return nil, nil
	}
	id := &krm.IAMPolicyBindingIdentity{}
	if err := id.FromExternal(url); err != nil {
		return nil, nil
	}
	gcpClient, err := m.client(ctx)
	if err != nil {
		return nil, err
	}
	return &PolicyBindingAdapter{id: id, gcpClient: gcpClient}, nil
}

func (m *modelPolicyBinding) client(ctx context.Context) (*gcp.PolicyBindingsClient, error) {
	opts, err := m.config.RESTClientOptions()
	if err != nil {
		return nil, err
	}
	gcpClient, err := gcp.NewPolicyBindingsRESTClient(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("building PolicyBinding client: %w", err)
	}
	return gcpClient, nil
}

type PolicyBindingAdapter struct {
	id        *krm.IAMPolicyBindingIdentity
	gcpClient *gcp.PolicyBindingsClient
	desired   *krm.IAMPolicyBinding
	policy    string
	actual    *iampb.PolicyBinding
}

var _ directbase.Adapter = &PolicyBindingAdapter{}

func (a *PolicyBindingAdapter) Find(ctx context.Context) (bool, error) {
	klog.FromContext(ctx).V(2).Info("getting PolicyBinding", "name", a.id)
	got, err := a.gcpClient.GetPolicyBinding(ctx, &iampb.GetPolicyBindingRequest{Name: a.id.String()})
	if err != nil {
		if direct.IsNotFound(err) {
			return false, nil
		}
		return false, fmt.Errorf("getting PolicyBinding %q: %w", a.id, err)
	}
	a.actual = got
	return true, nil
}

func (a *PolicyBindingAdapter) Create(ctx context.Context, createOp *directbase.CreateOperation) error {
	log := klog.FromContext(ctx)
	log.V(2).Info("creating PolicyBinding", "name", a.id)
	mapCtx := &direct.MapContext{}
	resource := IAMPolicyBindingSpec_ToProto(mapCtx, &a.desired.Spec)
	if mapCtx.Err() != nil {
		return mapCtx.Err()
	}
	resource.Policy = a.policy
	op, err := a.gcpClient.CreatePolicyBinding(ctx, &iampb.CreatePolicyBindingRequest{
		Parent:          a.id.ParentString(),
		PolicyBindingId: a.id.PolicyBinding,
		PolicyBinding:   resource,
	})
	if err != nil {
		return fmt.Errorf("creating PolicyBinding %s: %w", a.id, err)
	}
	created, err := op.Wait(ctx)
	if err != nil {
		return fmt.Errorf("PolicyBinding %s waiting creation: %w", a.id, err)
	}
	log.V(2).Info("successfully created PolicyBinding", "name", a.id)
	return a.updateStatus(ctx, createOp.UpdateStatus, created)
}

// policyBindingMask is the mutable fields whose desired value the binding
// does not hold, as update mask paths.
func policyBindingMask(desired, actual *iampb.PolicyBinding) []string {
	var paths []string
	if desired.GetDisplayName() != actual.GetDisplayName() {
		paths = append(paths, "display_name")
	}
	if !maps.Equal(desired.GetAnnotations(), actual.GetAnnotations()) {
		paths = append(paths, "annotations")
	}
	if !proto.Equal(desired.GetCondition(), actual.GetCondition()) {
		paths = append(paths, "condition")
	}
	sort.Strings(paths)
	return paths
}

func (a *PolicyBindingAdapter) Update(ctx context.Context, updateOp *directbase.UpdateOperation) error {
	log := klog.FromContext(ctx)
	mapCtx := &direct.MapContext{}
	desired := IAMPolicyBindingSpec_ToProto(mapCtx, &a.desired.Spec)
	if mapCtx.Err() != nil {
		return mapCtx.Err()
	}
	updated := a.actual
	if paths := policyBindingMask(desired, a.actual); len(paths) == 0 {
		log.V(2).Info("no field needs update", "name", a.id)
	} else {
		log.V(2).Info("updating PolicyBinding", "name", a.id, "mask", paths)
		desired.Name = a.id.String()
		desired.Etag = a.actual.GetEtag()
		op, err := a.gcpClient.UpdatePolicyBinding(ctx, &iampb.UpdatePolicyBindingRequest{
			PolicyBinding: desired,
			UpdateMask:    &fieldmaskpb.FieldMask{Paths: paths},
		})
		if err != nil {
			return fmt.Errorf("updating PolicyBinding %s: %w", a.id, err)
		}
		updated, err = op.Wait(ctx)
		if err != nil {
			return fmt.Errorf("PolicyBinding %s waiting update: %w", a.id, err)
		}
		log.V(2).Info("successfully updated PolicyBinding", "name", a.id)
	}
	return a.updateStatus(ctx, updateOp.UpdateStatus, updated)
}

func (a *PolicyBindingAdapter) updateStatus(ctx context.Context, write func(context.Context, any, *k8sv1alpha1.Condition) error, got *iampb.PolicyBinding) error {
	mapCtx := &direct.MapContext{}
	status := &krm.IAMPolicyBindingStatus{}
	status.ObservedState = IAMPolicyBindingObservedState_FromProto(mapCtx, got)
	if mapCtx.Err() != nil {
		return mapCtx.Err()
	}
	status.ExternalRef = direct.LazyPtr(a.id.String())
	return write(ctx, status, nil)
}

func (a *PolicyBindingAdapter) Export(ctx context.Context) (*unstructured.Unstructured, error) {
	if a.actual == nil {
		return nil, fmt.Errorf("Find() not called")
	}
	obj := &krm.IAMPolicyBinding{}
	mapCtx := &direct.MapContext{}
	obj.Spec = direct.ValueOf(IAMPolicyBindingSpec_FromProto(mapCtx, a.actual))
	if mapCtx.Err() != nil {
		return nil, mapCtx.Err()
	}
	switch parent := a.id.Parent; {
	case strings.HasPrefix(parent, "organizations/"):
		obj.Spec.OrganizationRef = &refsv1beta1.OrganizationRef{External: parent}
	case strings.HasPrefix(parent, "folders/"):
		obj.Spec.FolderRef = &refsv1beta1.FolderRefDeprecated{External: parent}
	default:
		obj.Spec.ProjectRef = &refsv1beta1.ProjectRef{External: parent}
	}
	obj.Spec.Location = direct.LazyPtr(a.id.Location)
	uObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
	if err != nil {
		return nil, err
	}
	u := &unstructured.Unstructured{Object: uObj}
	u.SetName(a.id.PolicyBinding)
	u.SetGroupVersionKind(krm.IAMPolicyBindingGVK)
	return u, nil
}

func (a *PolicyBindingAdapter) Delete(ctx context.Context, deleteOp *directbase.DeleteOperation) (bool, error) {
	log := klog.FromContext(ctx)
	log.V(2).Info("deleting PolicyBinding", "name", a.id)
	op, err := a.gcpClient.DeletePolicyBinding(ctx, &iampb.DeletePolicyBindingRequest{Name: a.id.String()})
	if err != nil {
		if direct.IsNotFound(err) {
			log.V(2).Info("skipping delete for non-existent PolicyBinding, assuming it was already deleted", "name", a.id)
			return true, nil
		}
		return false, fmt.Errorf("deleting PolicyBinding %s: %w", a.id, err)
	}
	if err := op.Wait(ctx); err != nil {
		if direct.IsNotFound(err) || strings.Contains(err.Error(), "missing \"value\" field") {
			return true, nil
		}
		return false, fmt.Errorf("waiting delete PolicyBinding %s: %w", a.id, err)
	}
	log.V(2).Info("successfully deleted PolicyBinding", "name", a.id)
	return true, nil
}
