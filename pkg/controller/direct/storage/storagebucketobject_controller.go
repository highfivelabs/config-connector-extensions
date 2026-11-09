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

// Package storage has the direct controllers for this project's Cloud
// Storage kinds.
//
// Written here: no Config Connector code or pull request has a bucket
// object. This controller follows Terraform's google_storage_bucket_object
// (read, not copied): the object is uploaded with objects.insert; a change
// to its content (seen by its MD5) or to its other fields uploads it again,
// which makes a new generation, since Terraform replaces the object for
// each of them; delete is objects.delete. It uses the JSON API client
// (google.golang.org/api/storage/v1), already in the pinned module graph.
package storage

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/base64"
	"fmt"
	"maps"

	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/config"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct/common"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct/directbase"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct/registry"
	"google.golang.org/api/googleapi"
	api "google.golang.org/api/storage/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	krm "github.com/highfivelabs/config-connector-extensions/apis/storage/v1alpha1"
)

func init() {
	registry.RegisterModel(krm.StorageBucketObjectGVK, NewStorageBucketObjectModel)
}

func NewStorageBucketObjectModel(ctx context.Context, config *config.ControllerConfig) (directbase.Model, error) {
	return &objectModel{config: *config}, nil
}

type objectModel struct {
	config config.ControllerConfig
}

var _ directbase.Model = &objectModel{}

func (m *objectModel) AdapterForObject(ctx context.Context, op *directbase.AdapterForObjectOperation) (directbase.Adapter, error) {
	obj := &krm.StorageBucketObject{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(op.GetUnstructured().Object, obj); err != nil {
		return nil, fmt.Errorf("error converting to %T: %w", obj, err)
	}
	if err := common.NormalizeReferences(ctx, op.Reader, obj, nil); err != nil {
		return nil, fmt.Errorf("normalizing references: %w", err)
	}
	id, err := obj.GetIdentity(ctx, op.Reader)
	if err != nil {
		return nil, err
	}
	content, err := resolveContent(ctx, op.Reader, obj)
	if err != nil {
		return nil, err
	}
	opts, err := m.config.RESTClientOptions()
	if err != nil {
		return nil, err
	}
	svc, err := api.NewService(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("building storage client: %w", err)
	}
	return &objectAdapter{id: id.(*krm.StorageBucketObjectIdentity), svc: svc, desired: obj, content: content}, nil
}

// resolveContent reads the content from the spec, or from a Secret's or a
// ConfigMap's key in the object's namespace.
func resolveContent(ctx context.Context, reader client.Reader, obj *krm.StorageBucketObject) ([]byte, error) {
	s := obj.Spec
	from := s.ContentFrom
	switch {
	case s.Content != nil && from != nil:
		return nil, fmt.Errorf("spec.content and spec.contentFrom are both set; set one")
	case s.Content != nil:
		return []byte(*s.Content), nil
	case from == nil || (from.SecretKeyRef == nil) == (from.ConfigMapKeyRef == nil):
		return nil, fmt.Errorf("set spec.content, or spec.contentFrom with one of secretKeyRef and configMapKeyRef")
	case from.SecretKeyRef != nil:
		secret := &corev1.Secret{}
		if err := reader.Get(ctx, types.NamespacedName{Namespace: obj.Namespace, Name: from.SecretKeyRef.Name}, secret); err != nil {
			return nil, fmt.Errorf("reading Secret %s: %w", from.SecretKeyRef.Name, err)
		}
		v, ok := secret.Data[from.SecretKeyRef.Key]
		if !ok {
			return nil, fmt.Errorf("Secret %s has no key %q", from.SecretKeyRef.Name, from.SecretKeyRef.Key)
		}
		return v, nil
	default:
		cm := &corev1.ConfigMap{}
		if err := reader.Get(ctx, types.NamespacedName{Namespace: obj.Namespace, Name: from.ConfigMapKeyRef.Name}, cm); err != nil {
			return nil, fmt.Errorf("reading ConfigMap %s: %w", from.ConfigMapKeyRef.Name, err)
		}
		if v, ok := cm.Data[from.ConfigMapKeyRef.Key]; ok {
			return []byte(v), nil
		}
		if v, ok := cm.BinaryData[from.ConfigMapKeyRef.Key]; ok {
			return v, nil
		}
		return nil, fmt.Errorf("ConfigMap %s has no key %q", from.ConfigMapKeyRef.Name, from.ConfigMapKeyRef.Key)
	}
}

func (m *objectModel) AdapterForURL(ctx context.Context, url string) (directbase.Adapter, error) {
	return nil, nil
}

type objectAdapter struct {
	id      *krm.StorageBucketObjectIdentity
	svc     *api.Service
	desired *krm.StorageBucketObject
	content []byte
	actual  *api.Object
}

var _ directbase.Adapter = &objectAdapter{}

func (a *objectAdapter) Find(ctx context.Context) (bool, error) {
	actual, err := a.svc.Objects.Get(a.id.Bucket, a.id.Object).Context(ctx).Do()
	if err != nil {
		if direct.IsNotFound(err) {
			return false, nil
		}
		return false, fmt.Errorf("getting StorageBucketObject %s: %w", a.id, err)
	}
	a.actual = actual
	return true, nil
}

func (a *objectAdapter) md5() string {
	sum := md5.Sum(a.content)
	return base64.StdEncoding.EncodeToString(sum[:])
}

// differs reports what of the desired object differs from the stored one,
// or "" if nothing does. Fields the spec leaves unset are not compared.
func (a *objectAdapter) differs() string {
	s, o := a.desired.Spec, a.actual
	switch {
	case o.Md5Hash != a.md5():
		return "content"
	case s.ContentType != nil && *s.ContentType != o.ContentType:
		return "contentType"
	case s.CacheControl != nil && *s.CacheControl != o.CacheControl:
		return "cacheControl"
	case s.ContentDisposition != nil && *s.ContentDisposition != o.ContentDisposition:
		return "contentDisposition"
	case s.ContentEncoding != nil && *s.ContentEncoding != o.ContentEncoding:
		return "contentEncoding"
	case s.ContentLanguage != nil && *s.ContentLanguage != o.ContentLanguage:
		return "contentLanguage"
	case s.Metadata != nil && !maps.Equal(s.Metadata, o.Metadata):
		return "metadata"
	}
	return ""
}

func (a *objectAdapter) upload(ctx context.Context, op directbase.Operation) error {
	s := a.desired.Spec
	obj := &api.Object{
		Name:               a.id.Object,
		ContentType:        direct.ValueOf(s.ContentType),
		CacheControl:       direct.ValueOf(s.CacheControl),
		ContentDisposition: direct.ValueOf(s.ContentDisposition),
		ContentEncoding:    direct.ValueOf(s.ContentEncoding),
		ContentLanguage:    direct.ValueOf(s.ContentLanguage),
		Metadata:           s.Metadata,
	}
	call := a.svc.Objects.Insert(a.id.Bucket, obj).Context(ctx)
	if s.ContentType != nil {
		call = call.Media(bytes.NewReader(a.content), googleapi.ContentType(*s.ContentType))
	} else {
		call = call.Media(bytes.NewReader(a.content))
	}
	stored, err := call.Do()
	if err != nil {
		return fmt.Errorf("uploading StorageBucketObject %s: %w", a.id, err)
	}
	return a.updateStatus(ctx, op, stored)
}

func (a *objectAdapter) Create(ctx context.Context, createOp *directbase.CreateOperation) error {
	klog.FromContext(ctx).V(2).Info("uploading StorageBucketObject", "name", a.id)
	return a.upload(ctx, createOp)
}

func (a *objectAdapter) Update(ctx context.Context, updateOp *directbase.UpdateOperation) error {
	if what := a.differs(); what != "" {
		klog.FromContext(ctx).V(2).Info("uploading StorageBucketObject again", "name", a.id, "changed", what)
		return a.upload(ctx, updateOp)
	}
	return a.updateStatus(ctx, updateOp, a.actual)
}

func (a *objectAdapter) Delete(ctx context.Context, deleteOp *directbase.DeleteOperation) (bool, error) {
	klog.FromContext(ctx).V(2).Info("deleting StorageBucketObject", "name", a.id)
	if err := a.svc.Objects.Delete(a.id.Bucket, a.id.Object).Context(ctx).Do(); err != nil {
		if direct.IsNotFound(err) {
			return true, nil
		}
		return false, fmt.Errorf("deleting StorageBucketObject %s: %w", a.id, err)
	}
	return true, nil
}

func (a *objectAdapter) Export(ctx context.Context) (*unstructured.Unstructured, error) {
	return nil, nil
}

func (a *objectAdapter) updateStatus(ctx context.Context, op directbase.Operation, o *api.Object) error {
	size := int64(o.Size)
	return op.UpdateStatus(ctx, &krm.StorageBucketObjectStatus{
		ExternalRef: direct.LazyPtr(a.id.String()),
		ObservedState: &krm.StorageBucketObjectObservedState{
			Generation: direct.LazyPtr(o.Generation),
			MD5Hash:    direct.LazyPtr(o.Md5Hash),
			CRC32C:     direct.LazyPtr(o.Crc32c),
			Size:       &size,
			MediaLink:  direct.LazyPtr(o.MediaLink),
		},
	}, nil)
}
