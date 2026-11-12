// Copyright 2025 Google LLC
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

// SPDX-License-Identifier: Apache-2.0

// Modified by High Five Labs for config-connector-extensions: copied from Config Connector
// pkg/controller/direct/datacatalog/datacatalogtag_controller.go (UPSTREAM.md).
// Changed: our KRM types (stable, datacatalog.gcp.hfl.dev/v1beta1,
// docs/design.md decision 26); the model is returned to our manager, not
// registered with Config Connector's registry; an update keeps
// status.externalRef (upstream's wrote a status without it, so the next
// reconcile lost the server-assigned ID and made a second tag); fields are
// compared without the display name and order the API adds (upstream
// compared them with reflect.DeepEqual, so every resync wrote); Export is
// not supported (no kind here exports).

package datacatalog

import (
	"context"
	"errors"
	"fmt"

	api "cloud.google.com/go/datacatalog/apiv1"
	pb "cloud.google.com/go/datacatalog/apiv1/datacatalogpb"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/config"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct/directbase"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/structuredreporting"

	krm "github.com/highfivelabs/config-connector-extensions/apis/datacatalog/v1beta1"
)

func NewTagModel(ctx context.Context, config *config.ControllerConfig) (directbase.Model, error) {
	return &tagModel{config: *config}, nil
}

var _ directbase.Model = &tagModel{}

type tagModel struct {
	config config.ControllerConfig
}

func (m *tagModel) client(ctx context.Context, projectID string) (*api.Client, error) {
	var opts []option.ClientOption

	config := m.config

	// the service requires that a quota project be set
	if !config.UserProjectOverride || config.BillingProject == "" {
		config.UserProjectOverride = true
		config.BillingProject = projectID // Use project from identity
	}

	opts, err := config.RESTClientOptions()
	if err != nil {
		return nil, err
	}

	gcpClient, err := api.NewRESTClient(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("building datacatalog tag client: %w", err)
	}

	return gcpClient, nil
}

func (m *tagModel) AdapterForObject(ctx context.Context, op *directbase.AdapterForObjectOperation) (directbase.Adapter, error) {
	u := op.GetUnstructured()
	reader := op.Reader
	obj := &krm.DataCatalogTag{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, &obj); err != nil {
		return nil, fmt.Errorf("error converting to %T: %w", obj, err)
	}

	// The tag's ID is server-generated: the identity is the parent entry and,
	// once known, the ID from status.externalRef or spec.resourceID.
	id, err := krm.NewTagIdentity(ctx, reader, obj)
	if err != nil {
		return nil, err
	}

	projectID := id.Parent().ProjectID

	gcpClient, err := m.client(ctx, projectID)
	if err != nil {
		return nil, err
	}

	return &tagAdapter{
		gcpClient: gcpClient,
		id:        id,
		desired:   obj,
		reader:    reader,
	}, nil
}

// AdapterForURL is not supported: no kind here is exported.
func (m *tagModel) AdapterForURL(ctx context.Context, url string) (directbase.Adapter, error) {
	return nil, nil
}

type tagAdapter struct {
	gcpClient *api.Client
	id        *krm.TagIdentity
	desired   *krm.DataCatalogTag
	actual    *pb.Tag
	reader    client.Reader
}

var _ directbase.Adapter = &tagAdapter{}

// Find implements directbase.Adapter.
// Data Catalog Tags don't have a Get RPC. We list tags on the parent and filter.
func (a *tagAdapter) Find(ctx context.Context) (bool, error) {
	log := klog.FromContext(ctx)

	// This resource has a server-generated ID. This means user should not know
	// the ID before the resource is created, and 'metadata.name' won't be used
	// as the default resource ID. So empty value for 'spec.resourceID' should
	// also be valid:
	// 1. When 'spec.resourceID' is not set or set to an empty value, the
	//    intention is to create the resource.
	// 2. When 'spec.resourceID' is set, the intention is to acquire an existing
	//    resource.
	//    2.1. When 'spec.resourceID' is set but the corresponding GCP resource
	//         is not found, then it is a real error.
	if a.id.ID() == "" {
		log.V(2).Info("no resource ID in get indicates the create intention", "name", a.id)
		return false, nil
	}

	parentName := a.id.Parent().String()
	log.V(2).Info("listing datacatalog tags", "parent", parentName)

	req := &pb.ListTagsRequest{Parent: parentName}
	it := a.gcpClient.ListTags(ctx, req)
	for {
		tag, err := it.Next()
		if errors.Is(err, iterator.Done) {
			break
		}
		if err != nil {
			// PermissionDenied might occur if parent doesn't exist
			if direct.IsPermissionDenied(err) {
				log.V(1).Info("permission denied listing tags, parent likely missing", "parent", parentName, "error", err)
				return false, nil // Treat as not found
			}
			return false, fmt.Errorf("listing datacatalog tags for parent %q: %w", parentName, err)
		}

		if a.id.String() == tag.Name {
			a.actual = tag
			return true, nil
		}
	}

	log.V(2).Info("no matching datacatalog tag found", "parent", parentName, "id", a.id.ID())
	return false, nil
}

// Create implements directbase.Adapter.
func (a *tagAdapter) Create(ctx context.Context, createOp *directbase.CreateOperation) error {
	log := klog.FromContext(ctx)
	mapCtx := &direct.MapContext{}

	parentName := a.id.Parent().String()
	log.V(2).Info("creating datacatalog tag", "parent", parentName)

	// Resolve the tag template reference first to get the full template name.
	tagTemplateName, err := a.desired.Spec.TemplateRef.NormalizedExternal(ctx, a.reader, a.desired.GetNamespace())
	if err != nil {
		return fmt.Errorf("resolving template reference: %w", err)
	}

	desiredProto := DataCatalogTagSpec_ToProto(mapCtx, &a.desired.Spec)
	if mapCtx.Err() != nil {
		return mapCtx.Err()
	}

	// Set the fully resolved template name
	desiredProto.Template = tagTemplateName

	req := &pb.CreateTagRequest{
		Parent: parentName,
		Tag:    desiredProto,
	}
	created, err := a.gcpClient.CreateTag(ctx, req)
	if err != nil {
		return fmt.Errorf("creating datacatalog tag for parent %q: %w", parentName, err)
	}
	log.V(2).Info("successfully created datacatalog tag in gcp", "name", created.Name)

	status := &krm.DataCatalogTagStatus{}
	status.ObservedState = DataCatalogTagObservedState_FromProto(mapCtx, created)
	if mapCtx.Err() != nil {
		return mapCtx.Err()
	}
	// Store the generated name for future Find/Update/Delete operations.
	status.ExternalRef = direct.LazyPtr(created.Name)
	return createOp.UpdateStatus(ctx, status, nil)
}

// comparableField is a tag field as the spec can state it: without the
// display name and order the API copies from the template.
func comparableField(in *pb.TagField) *pb.TagField {
	if in == nil {
		return nil
	}
	out := proto.Clone(in).(*pb.TagField)
	out.DisplayName = ""
	out.Order = 0
	return out
}

// fieldsDiffer reports whether the desired fields differ from the actual
// ones, by key and value.
func fieldsDiffer(desired, actual map[string]*pb.TagField) bool {
	if len(desired) != len(actual) {
		return true
	}
	for k, d := range desired {
		a, ok := actual[k]
		if !ok || !proto.Equal(comparableField(d), comparableField(a)) {
			return true
		}
	}
	return false
}

// Update implements directbase.Adapter.
// Only the 'fields' map is mutable. Template and scope (column) are immutable.
func (a *tagAdapter) Update(ctx context.Context, updateOp *directbase.UpdateOperation) error {
	log := klog.FromContext(ctx)
	mapCtx := &direct.MapContext{}

	// The name (including the system-generated ID) is required for update.
	if a.actual == nil || a.actual.Name == "" {
		return fmt.Errorf("cannot update tag: actual state or name not found")
	}
	tagName := a.actual.Name
	log.V(2).Info("updating datacatalog tag", "name", tagName)

	// Resolve the tag template reference first to get the full template name.
	_, err := a.desired.Spec.TemplateRef.NormalizedExternal(ctx, a.reader, a.desired.GetNamespace())
	if err != nil {
		return fmt.Errorf("resolving template reference: %w", err)
	}

	desiredProto := DataCatalogTagSpec_ToProto(mapCtx, &a.desired.Spec)
	if mapCtx.Err() != nil {
		return mapCtx.Err()
	}

	report := &structuredreporting.Diff{Object: updateOp.GetUnstructured()}

	// Set required fields for the update request payload
	desiredProto.Name = tagName
	desiredProto.Template = a.actual.Template // Template is immutable, use actual value

	// Check if only mutable fields ('fields') have changed.
	updateMask := &fieldmaskpb.FieldMask{}
	if fieldsDiffer(desiredProto.Fields, a.actual.Fields) {
		report.AddField("fields", a.actual.Fields, desiredProto.Fields)
		updateMask.Paths = append(updateMask.Paths, "fields")
	}

	// The column is immutable (CEL in the CRD); refuse it here too.
	if desiredProto.GetColumn() != a.actual.GetColumn() {
		return fmt.Errorf("attempting to update immutable field 'column' for tag %q", tagName)
	}

	var updated *pb.Tag
	if len(updateMask.Paths) == 0 {
		log.V(2).Info("no mutable field needs update", "name", tagName)
		updated = a.actual // Use current actual state for status update
	} else {
		structuredreporting.ReportDiff(ctx, report)
		log.V(2).Info("updating fields", "name", tagName, "fields", updateMask.Paths)
		req := &pb.UpdateTagRequest{
			Tag:        desiredProto,
			UpdateMask: updateMask,
		}
		updated, err = a.gcpClient.UpdateTag(ctx, req)
		if err != nil {
			return fmt.Errorf("updating datacatalog tag %q: %w", tagName, err)
		}
		log.V(2).Info("successfully updated datacatalog tag in gcp", "name", tagName)
	}

	status := &krm.DataCatalogTagStatus{}
	status.ObservedState = DataCatalogTagObservedState_FromProto(mapCtx, updated)
	if mapCtx.Err() != nil {
		return mapCtx.Err()
	}
	status.ExternalRef = direct.LazyPtr(tagName)
	return updateOp.UpdateStatus(ctx, status, nil)
}

// Export implements directbase.Adapter.
func (a *tagAdapter) Export(ctx context.Context) (*unstructured.Unstructured, error) {
	return nil, fmt.Errorf("export of DataCatalogTag is not supported")
}

// Delete implements directbase.Adapter.
func (a *tagAdapter) Delete(ctx context.Context, deleteOp *directbase.DeleteOperation) (bool, error) {
	log := klog.FromContext(ctx)

	// Get the tag name (including system-generated ID) from actual state or status
	tagName := ""
	if a.actual != nil && a.actual.Name != "" {
		tagName = a.actual.Name
	} else if a.desired.Status.ExternalRef != nil {
		tagName = *a.desired.Status.ExternalRef
	}

	if tagName == "" {
		// Resource was likely never created or Find failed. Assume deleted.
		log.V(2).Info("skipping delete for datacatalog tag: resource name not found", "namespace", a.desired.Namespace, "name", a.desired.Name)
		return true, nil
	}

	log.V(2).Info("deleting datacatalog tag", "name", tagName)

	req := &pb.DeleteTagRequest{Name: tagName}
	err := a.gcpClient.DeleteTag(ctx, req)
	if err != nil {
		if direct.IsNotFound(err) {
			log.V(2).Info("skipping delete for non-existent datacatalog tag, assuming it was already deleted", "name", tagName)
			return true, nil
		}
		// PermissionDenied might also indicate not found or access removed
		if direct.IsPermissionDenied(err) {
			log.V(2).Info("skipping delete for datacatalog tag due to permission denied, potentially already deleted or access revoked", "name", tagName)
			return true, nil // Treat as deleted
		}
		return false, fmt.Errorf("deleting datacatalog tag %q: %w", tagName, err)
	}
	log.V(2).Info("successfully deleted datacatalog tag", "name", tagName)

	return true, nil
}
