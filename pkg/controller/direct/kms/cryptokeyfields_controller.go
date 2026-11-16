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

// Package kms reconciles KMSCryptoKeyFields, a field kind (docs/design.md
// section 6.6, pattern 2; decision 37).
package kms

// Written here: no Config Connector code or pull request sets a key's
// keyAccessJustificationsPolicy. The client is Config Connector's
// (pkg/controller/direct/kms/client.go builds the same REST client). The
// key is read with cryptoKeys.get and written with cryptoKeys.patch and an
// update mask of key_access_justifications_policy alone, so no other field
// of the key is sent: Config Connector keeps owning them. Terraform's update
// of a key (what Config Connector's KMSCryptoKey runs) also names only the
// fields it changed in its mask, so it leaves this one alone (section 6.6).

import (
	"context"
	"fmt"
	"slices"

	api "cloud.google.com/go/kms/apiv1"
	kmspb "cloud.google.com/go/kms/apiv1/kmspb"
	kmsv1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/apis/kms/v1beta1"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/config"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct/directbase"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct/registry"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"

	krm "github.com/highfivelabs/config-connector-extensions/apis/kms/v1alpha1"
	"github.com/highfivelabs/config-connector-extensions/pkg/fieldkind"
)

func init() {
	registry.RegisterModel(krm.KMSCryptoKeyFieldsGVK, NewCryptoKeyFieldsModel)
}

func NewCryptoKeyFieldsModel(ctx context.Context, config *config.ControllerConfig) (directbase.Model, error) {
	return &cryptoKeyFieldsModel{config: *config}, nil
}

type cryptoKeyFieldsModel struct {
	config config.ControllerConfig
}

var _ directbase.Model = &cryptoKeyFieldsModel{}

// kajMask is the only path this kind ever writes.
const kajMask = "key_access_justifications_policy"

func (m *cryptoKeyFieldsModel) AdapterForObject(ctx context.Context, op *directbase.AdapterForObjectOperation) (directbase.Adapter, error) {
	obj := &krm.KMSCryptoKeyFields{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(op.GetUnstructured().Object, obj); err != nil {
		return nil, fmt.Errorf("error converting to %T: %w", obj, err)
	}
	ref := obj.Spec.ResourceRef
	if ref == nil || (ref.External == "" && ref.Name == "") {
		return nil, fmt.Errorf("spec.resourceRef needs external or name")
	}
	prev := direct.ValueOf(obj.Status.ExternalRef)
	if obj.GetDeletionTimestamp() != nil && prev != "" {
		// Deleting: the key this object set is in its status; Config
		// Connector's KMSCryptoKey may be gone already.
		ref = &kmsv1beta1.KMSCryptoKeyRef{External: prev}
	} else if err := ref.Normalize(ctx, op.Reader, obj.GetNamespace()); err != nil {
		return nil, err
	}
	id := &kmsv1beta1.KMSCryptoKeyIdentity{}
	if err := id.FromExternal(ref.External); err != nil {
		return nil, err
	}
	if prev != "" && prev != id.String() {
		return nil, fmt.Errorf("cannot change the key of KMSCryptoKeyFields (old=%q, new=%q)", prev, id.String())
	}
	desired, err := desiredPolicy(obj.Spec.KeyAccessJustificationsPolicy)
	if err != nil {
		return nil, err
	}
	if err := fieldkind.CheckSoleOwner(ctx, op.Reader, krm.KMSCryptoKeyFieldsGVK, obj, id.String()); err != nil {
		return nil, err
	}
	opts, err := m.config.RESTClientOptions()
	if err != nil {
		return nil, err
	}
	c, err := api.NewKeyManagementRESTClient(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("building kms client: %w", err)
	}
	return &cryptoKeyFieldsAdapter{id: id, gcpClient: c, desired: desired}, nil
}

// desiredPolicy is the spec's policy as the API takes it; an unknown reason
// is refused here rather than by the API.
func desiredPolicy(in *krm.KeyAccessJustificationsPolicy) (*kmspb.KeyAccessJustificationsPolicy, error) {
	if in == nil {
		return nil, fmt.Errorf("spec.keyAccessJustificationsPolicy is required")
	}
	out := &kmspb.KeyAccessJustificationsPolicy{AllowedAccessReasons: []kmspb.AccessReason{}}
	for _, r := range in.AllowedAccessReasons {
		v, ok := kmspb.AccessReason_value[r]
		if !ok || v == 0 {
			return nil, fmt.Errorf("spec.keyAccessJustificationsPolicy.allowedAccessReasons: unknown access reason %q", r)
		}
		out.AllowedAccessReasons = append(out.AllowedAccessReasons, kmspb.AccessReason(v))
	}
	return out, nil
}

func (m *cryptoKeyFieldsModel) AdapterForURL(ctx context.Context, url string) (directbase.Adapter, error) {
	return nil, nil
}

type cryptoKeyFieldsAdapter struct {
	id        *kmsv1beta1.KMSCryptoKeyIdentity
	gcpClient *api.KeyManagementClient
	desired   *kmspb.KeyAccessJustificationsPolicy
	actual    *kmspb.CryptoKey
}

var _ directbase.Adapter = &cryptoKeyFieldsAdapter{}

// Find reports whether the key exists. A missing key is "not found" so that
// a delete finishes when the key is gone; a create then waits (Create).
func (a *cryptoKeyFieldsAdapter) Find(ctx context.Context) (bool, error) {
	key, err := a.gcpClient.GetCryptoKey(ctx, &kmspb.GetCryptoKeyRequest{Name: a.id.String()})
	if err != nil {
		if direct.IsNotFound(err) {
			return false, nil
		}
		return false, fmt.Errorf("getting KMS CryptoKey %s: %w", a.id, err)
	}
	a.actual = key
	return true, nil
}

// Create is called only while the key does not exist: this kind never makes
// it, so it waits for it.
func (a *cryptoKeyFieldsAdapter) Create(ctx context.Context, createOp *directbase.CreateOperation) error {
	return fieldkind.NotFoundError("KMSCryptoKey", a.id.String())
}

// sameReasons compares two lists of reasons as sets: the API may return
// them in its own order.
func sameReasons(a, b []kmspb.AccessReason) bool {
	a, b = slices.Clone(a), slices.Clone(b)
	slices.Sort(a)
	slices.Sort(b)
	return slices.Equal(slices.Compact(a), slices.Compact(b))
}

func (a *cryptoKeyFieldsAdapter) Update(ctx context.Context, updateOp *directbase.UpdateOperation) error {
	actual := a.actual.GetKeyAccessJustificationsPolicy()
	if actual != nil && sameReasons(actual.GetAllowedAccessReasons(), a.desired.GetAllowedAccessReasons()) {
		return a.updateStatus(ctx, updateOp)
	}
	klog.FromContext(ctx).V(2).Info("setting the keyAccessJustificationsPolicy of a KMS CryptoKey", "name", a.id)
	if err := a.patch(ctx, a.desired); err != nil {
		return err
	}
	return a.updateStatus(ctx, updateOp)
}

func (a *cryptoKeyFieldsAdapter) patch(ctx context.Context, policy *kmspb.KeyAccessJustificationsPolicy) error {
	updated, err := a.gcpClient.UpdateCryptoKey(ctx, &kmspb.UpdateCryptoKeyRequest{
		CryptoKey:  &kmspb.CryptoKey{Name: a.id.String(), KeyAccessJustificationsPolicy: policy},
		UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{kajMask}},
	})
	if err != nil {
		return fmt.Errorf("setting the keyAccessJustificationsPolicy of KMS CryptoKey %s: %w", a.id, err)
	}
	a.actual = updated
	return nil
}

// Delete clears the policy (the key then has none, as before this object),
// and never deletes the key. A key that is gone is not reached: Find said
// not found.
func (a *cryptoKeyFieldsAdapter) Delete(ctx context.Context, deleteOp *directbase.DeleteOperation) (bool, error) {
	if a.actual.GetKeyAccessJustificationsPolicy() == nil {
		return true, nil
	}
	klog.FromContext(ctx).V(2).Info("clearing the keyAccessJustificationsPolicy of a KMS CryptoKey", "name", a.id)
	if err := a.patch(ctx, nil); err != nil {
		return false, err
	}
	return true, nil
}

func (a *cryptoKeyFieldsAdapter) Export(ctx context.Context) (*unstructured.Unstructured, error) {
	return nil, nil
}

func (a *cryptoKeyFieldsAdapter) updateStatus(ctx context.Context, op directbase.Operation) error {
	return op.UpdateStatus(ctx, &krm.KMSCryptoKeyFieldsStatus{ExternalRef: direct.LazyPtr(a.id.String())}, nil)
}
