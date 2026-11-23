// Copyright 2026 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// SPDX-License-Identifier: Apache-2.0

package privateca

// Modified by High Five Labs for config-connector-extensions: copied from
// Config Connector's pkg/controller/direct/privateca/privatecacertificateauthority_controller.go
// (the direct PrivateCACertificateAuthority): the same create, get and
// delete calls on the same client, the labels from metadata.labels.
// Changed (docs/design.md section 6.6, pattern 1: the API takes the
// authority's config, key, lifetime, type and bucket only at create, so the
// CRD makes them immutable and nothing of them is compared after create):
// an update changes only the labels and userDefinedAccessUrls, with a mask
// of those that differ; written here, as Terraform's
// google_privateca_certificate_authority does it (read, not copied): a
// self-signed authority is enabled after create (and when found staged),
// and an enabled one is disabled before the delete, which the API needs;
// a delete of an authority that was made resolves nothing
// (status.externalRef); the references are resolved with their Normalize
// methods.

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"strings"

	api "cloud.google.com/go/security/privateca/apiv1"
	pb "cloud.google.com/go/security/privateca/apiv1/privatecapb"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/config"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct/directbase"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct/registry"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/label"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"

	krm "github.com/highfivelabs/config-connector-extensions/apis/privateca/v1alpha1"
)

func init() {
	registry.RegisterModel(krm.PrivateCACertificateAuthorityGVK, NewCertificateAuthorityModel)
}

func NewCertificateAuthorityModel(ctx context.Context, config *config.ControllerConfig) (directbase.Model, error) {
	return &certificateAuthorityModel{config: config}, nil
}

type certificateAuthorityModel struct {
	config *config.ControllerConfig
}

var _ directbase.Model = &certificateAuthorityModel{}

type certificateAuthorityAdapter struct {
	id *krm.PrivateCACertificateAuthorityIdentity
	// skipGracePeriod is spec.skipGracePeriod, for the delete.
	skipGracePeriod bool
	desired         *pb.CertificateAuthority
	actual          *pb.CertificateAuthority
	caClient        *api.CertificateAuthorityClient
}

var _ directbase.Adapter = &certificateAuthorityAdapter{}

func (m *certificateAuthorityModel) AdapterForObject(ctx context.Context, op *directbase.AdapterForObjectOperation) (directbase.Adapter, error) {
	u := op.GetUnstructured()
	obj := &krm.PrivateCACertificateAuthority{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, obj); err != nil {
		return nil, fmt.Errorf("error converting to %T: %w", obj, err)
	}
	caClient, err := newClient(ctx, m.config)
	if err != nil {
		return nil, err
	}
	if ext := direct.ValueOf(obj.Status.ExternalRef); op.IsDeleting() && ext != "" {
		// Deleting an authority that was made: its name is in status, so
		// neither the pool nor the key is resolved.
		id := &krm.PrivateCACertificateAuthorityIdentity{}
		if err := id.FromExternal(ext); err != nil {
			return nil, err
		}
		return &certificateAuthorityAdapter{id: id, caClient: caClient, skipGracePeriod: direct.ValueOf(obj.Spec.SkipGracePeriod)}, nil
	}

	idVal, err := obj.GetIdentity(ctx, op.Reader)
	if err != nil {
		return nil, err
	}
	id, ok := idVal.(*krm.PrivateCACertificateAuthorityIdentity)
	if !ok {
		return nil, fmt.Errorf("unexpected identity type: %T", idVal)
	}
	spec := obj.Spec.DeepCopy()
	if ref := spec.KeySpec.CloudKmsKeyVersionRef; ref != nil {
		if err := ref.Normalize(ctx, op.Reader, obj.GetNamespace()); err != nil {
			return nil, err
		}
	}
	if ref := spec.GcsBucketRef; ref != nil {
		if err := ref.Normalize(ctx, op.Reader, obj.GetNamespace()); err != nil {
			return nil, err
		}
		// The API takes the bucket's name only.
		ref.External = ref.External[strings.LastIndex(ref.External, "/")+1:]
	}

	mapCtx := &direct.MapContext{}
	desired := PrivateCACertificateAuthoritySpec_ToProto(mapCtx, spec)
	if mapCtx.Err() != nil {
		return nil, mapCtx.Err()
	}
	desired.Labels = label.NewGCPLabelsFromK8sLabels(u.GetLabels())

	return &certificateAuthorityAdapter{
		id:              id,
		desired:         desired,
		caClient:        caClient,
		skipGracePeriod: direct.ValueOf(obj.Spec.SkipGracePeriod),
	}, nil
}

func (m *certificateAuthorityModel) AdapterForURL(ctx context.Context, url string) (directbase.Adapter, error) {
	return nil, nil
}

// Find implements the Adapter interface. An authority deleted within its
// grace period (state DELETED) is gone: its ID cannot be used again until
// the period ends, which the create's error then says.
func (a *certificateAuthorityAdapter) Find(ctx context.Context) (bool, error) {
	latest, err := a.caClient.GetCertificateAuthority(ctx, &pb.GetCertificateAuthorityRequest{Name: a.id.String()})
	if err != nil {
		if direct.IsNotFound(err) {
			return false, nil
		}
		return false, fmt.Errorf("getting PrivateCACertificateAuthority %s: %w", a.id.String(), err)
	}
	if latest.GetState() == pb.CertificateAuthority_DELETED {
		return false, nil
	}
	a.actual = latest
	return true, nil
}

func (a *certificateAuthorityAdapter) Create(ctx context.Context, createOp *directbase.CreateOperation) error {
	log := klog.FromContext(ctx)
	log.V(2).Info("creating PrivateCACertificateAuthority", "id", a.id.String())

	req := &pb.CreateCertificateAuthorityRequest{
		Parent:                 a.id.ParentString(),
		CertificateAuthorityId: a.id.CertificateAuthority,
		CertificateAuthority:   a.desired,
	}
	op, err := a.caClient.CreateCertificateAuthority(ctx, req)
	if err != nil {
		return fmt.Errorf("creating PrivateCACertificateAuthority %s: %w", a.id.String(), err)
	}
	_, err = op.Wait(ctx)
	if err != nil {
		return fmt.Errorf("waiting PrivateCACertificateAuthority %s creation: %w", a.id.String(), err)
	}
	log.V(2).Info("successfully created PrivateCACertificateAuthority", "name", a.id.String())

	latest, err := a.caClient.GetCertificateAuthority(ctx, &pb.GetCertificateAuthorityRequest{Name: a.id.String()})
	if err != nil {
		return err
	}
	if latest, err = a.enableStaged(ctx, latest); err != nil {
		return err
	}
	return a.updateStatus(ctx, createOp, latest)
}

// enableStaged enables a self-signed authority the API left staged (as it
// leaves every new one), as Terraform does by default; a subordinate one
// waits for its activation, which this kind does not do.
func (a *certificateAuthorityAdapter) enableStaged(ctx context.Context, latest *pb.CertificateAuthority) (*pb.CertificateAuthority, error) {
	if latest.GetType() != pb.CertificateAuthority_SELF_SIGNED || latest.GetState() != pb.CertificateAuthority_STAGED {
		return latest, nil
	}
	op, err := a.caClient.EnableCertificateAuthority(ctx, &pb.EnableCertificateAuthorityRequest{Name: a.id.String()})
	if err != nil {
		return nil, fmt.Errorf("enabling PrivateCACertificateAuthority %s: %w", a.id.String(), err)
	}
	if _, err := op.Wait(ctx); err != nil {
		return nil, fmt.Errorf("waiting PrivateCACertificateAuthority %s enable: %w", a.id.String(), err)
	}
	return a.caClient.GetCertificateAuthority(ctx, &pb.GetCertificateAuthorityRequest{Name: a.id.String()})
}

// Update implements the Adapter interface: only the labels and
// userDefinedAccessUrls can change.
func (a *certificateAuthorityAdapter) Update(ctx context.Context, updateOp *directbase.UpdateOperation) error {
	log := klog.FromContext(ctx)

	latest, err := a.enableStaged(ctx, a.actual)
	if err != nil {
		return err
	}
	var paths []string
	if !maps.Equal(a.desired.GetLabels(), latest.GetLabels()) {
		paths = append(paths, "labels")
	}
	if !sameUserDefinedAccessUrls(a.desired.GetUserDefinedAccessUrls(), latest.GetUserDefinedAccessUrls()) {
		paths = append(paths, "user_defined_access_urls")
	}
	if len(paths) > 0 {
		log.V(2).Info("updating PrivateCACertificateAuthority", "name", a.id.String(), "paths", paths)
		update := &pb.CertificateAuthority{
			Name:                  a.id.String(),
			Labels:                a.desired.GetLabels(),
			UserDefinedAccessUrls: a.desired.GetUserDefinedAccessUrls(),
		}
		op, err := a.caClient.UpdateCertificateAuthority(ctx, &pb.UpdateCertificateAuthorityRequest{
			UpdateMask:           &fieldmaskpb.FieldMask{Paths: paths},
			CertificateAuthority: update,
		})
		if err != nil {
			return fmt.Errorf("updating PrivateCACertificateAuthority %s: %w", a.id.String(), err)
		}
		if _, err := op.Wait(ctx); err != nil {
			return fmt.Errorf("waiting update PrivateCACertificateAuthority %s: %w", a.id.String(), err)
		}
		log.V(2).Info("successfully updated PrivateCACertificateAuthority", "name", a.id.String())
		latest, err = a.caClient.GetCertificateAuthority(ctx, &pb.GetCertificateAuthorityRequest{Name: a.id.String()})
		if err != nil {
			return err
		}
	}
	return a.updateStatus(ctx, updateOp, latest)
}

// sameUserDefinedAccessUrls compares the URL lists in order; unset is the
// same as empty.
func sameUserDefinedAccessUrls(a, b *pb.CertificateAuthority_UserDefinedAccessUrls) bool {
	return slices.Equal(a.GetAiaIssuingCertificateUrls(), b.GetAiaIssuingCertificateUrls()) &&
		slices.Equal(a.GetCrlAccessUrls(), b.GetCrlAccessUrls())
}

func (a *certificateAuthorityAdapter) Export(ctx context.Context) (*unstructured.Unstructured, error) {
	return nil, nil
}

// Delete implements the Adapter interface. An enabled authority is
// disabled first: the API deletes only a disabled or staged one. The
// delete keeps the API's 30-day grace period (Terraform's default), in
// which the authority can be undeleted and its ID is not free, unless
// spec.skipGracePeriod.
func (a *certificateAuthorityAdapter) Delete(ctx context.Context, deleteOp *directbase.DeleteOperation) (bool, error) {
	log := klog.FromContext(ctx)
	log.V(2).Info("deleting PrivateCACertificateAuthority", "name", a.id.String())

	if a.actual.GetState() == pb.CertificateAuthority_ENABLED {
		op, err := a.caClient.DisableCertificateAuthority(ctx, &pb.DisableCertificateAuthorityRequest{Name: a.id.String()})
		if err != nil {
			if direct.IsNotFound(err) {
				return true, nil
			}
			return false, fmt.Errorf("disabling PrivateCACertificateAuthority %s: %w", a.id.String(), err)
		}
		if _, err := op.Wait(ctx); err != nil {
			return false, fmt.Errorf("waiting PrivateCACertificateAuthority %s disable: %w", a.id.String(), err)
		}
	}
	op, err := a.caClient.DeleteCertificateAuthority(ctx, &pb.DeleteCertificateAuthorityRequest{Name: a.id.String(), SkipGracePeriod: a.skipGracePeriod})
	if err != nil {
		if direct.IsNotFound(err) {
			log.V(2).Info("skipping delete for non-existent PrivateCACertificateAuthority, assuming it was already deleted", "name", a.id.String())
			return true, nil
		}
		return false, fmt.Errorf("deleting PrivateCACertificateAuthority %s: %w", a.id.String(), err)
	}
	if _, err := op.Wait(ctx); err != nil {
		return false, fmt.Errorf("waiting delete PrivateCACertificateAuthority %s: %w", a.id.String(), err)
	}
	log.V(2).Info("successfully deleted PrivateCACertificateAuthority", "name", a.id.String())
	return true, nil
}

func (a *certificateAuthorityAdapter) updateStatus(ctx context.Context, op directbase.Operation, latest *pb.CertificateAuthority) error {
	status := &krm.PrivateCACertificateAuthorityStatus{
		ExternalRef:       direct.LazyPtr(a.id.String()),
		PemCaCertificates: latest.GetPemCaCertificates(),
		State:             direct.LazyPtr(latest.GetState().String()),
		Tier:              direct.LazyPtr(latest.GetTier().String()),
	}
	if u := latest.GetAccessUrls(); u != nil {
		status.AccessUrls = &krm.CertificateAuthority_AccessUrls{
			CaCertificateAccessUrl: direct.LazyPtr(u.GetCaCertificateAccessUrl()),
			CrlAccessUrls:          u.GetCrlAccessUrls(),
		}
	}
	return op.UpdateStatus(ctx, status, nil)
}

// CloudLabels are the found resource's labels, for the slot guard
// (pkg/slotguard). Written here.
func (a *certificateAuthorityAdapter) CloudLabels() map[string]string {
	if a.actual == nil {
		return nil
	}
	return a.actual.Labels
}
