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

// Modified by High Five Labs for config-connector-extensions. Copied from Config Connector
// pkg/controller/direct/compute/computerouter_controller.go (UPSTREAM.md;
// PR #13960, head 9b916158, changes only its migration diff, which is not
// ours). Our ComputeRouter owns the whole router (docs/design.md decision
// 14), so, changed:
//   - it is written with routers.update (PUT), not routers.patch: patch keeps
//     a list the body leaves out and the client leaves an empty list out (as
//     the regional firewall policy rule showed live), so a removed peer, NAT,
//     interface or key would stay. The body is the router as read, with the
//     owned fields replaced, so what the kind does not model is sent back as
//     it was; peers and interfaces the API manages for an interconnect
//     attachment (managementType MANAGED_BY_ATTACHMENT) are kept;
//   - the comparison is written here: a field the spec sets must match the
//     cloud's, a field it leaves out is the API's to default (the API fills
//     in BFD, timeouts, IP versions and the like), the four lists must have
//     the same members, and links compare without their URL prefix;
//   - MD5 key values are read from Secrets and sent with every write; the API
//     does not return them, so status.md5AuthenticationKeysVersion (a digest
//     of the Secrets' names, keys and resourceVersions, not of the values)
//     says when a Secret changed and the keys must be written again;
//   - the network and encryptedInterconnectRouter are immutable: a change is
//     an error, not a write;
//   - VPN tunnel and interconnect attachment references by name are resolved
//     from the Config Connector object's status.selfLink (upstream's
//     reference types for them have no resolver);
//   - the REST client is built here, as for our other compute kinds.

package compute

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"slices"
	"sort"

	gcp "cloud.google.com/go/compute/apiv1"
	pb "cloud.google.com/go/compute/apiv1/computepb"
	apirefs "github.com/GoogleCloudPlatform/k8s-config-connector/apis/refs"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/config"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct/common"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct/directbase"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct/registry"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/export"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	krm "github.com/highfivelabs/config-connector-extensions/apis/compute/v1alpha1"
)

func init() {
	registry.RegisterModel(krm.ComputeRouterGVK, NewRouterModel)
}

func NewRouterModel(ctx context.Context, config *config.ControllerConfig) (directbase.Model, error) {
	return &routerModel{config: *config}, nil
}

var _ directbase.Model = &routerModel{}

type routerModel struct {
	config config.ControllerConfig
}

func (m *routerModel) client(ctx context.Context) (*gcp.RoutersClient, error) {
	opts, err := m.config.RESTClientOptions()
	if err != nil {
		return nil, err
	}
	c, err := gcp.NewRoutersRESTClient(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("building routers client: %w", err)
	}
	return c, nil
}

func (m *routerModel) AdapterForObject(ctx context.Context, op *directbase.AdapterForObjectOperation) (directbase.Adapter, error) {
	u := op.GetUnstructured()
	reader := op.Reader
	obj := &krm.ComputeRouter{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, &obj); err != nil {
		return nil, fmt.Errorf("error converting to %T: %w", obj, err)
	}

	id, err := obj.GetIdentity(ctx, reader)
	if err != nil {
		return nil, err
	}

	if err := common.NormalizeReferences(ctx, reader, obj, nil); err != nil {
		return nil, fmt.Errorf("normalizing references: %w", err)
	}
	for i := range obj.Spec.Interfaces {
		in := &obj.Spec.Interfaces[i]
		if in.VPNTunnelRef != nil {
			ext, err := configConnectorSelfLink(ctx, reader, obj.GetNamespace(), "ComputeVPNTunnel", in.VPNTunnelRef.External, in.VPNTunnelRef.Name, in.VPNTunnelRef.Namespace)
			if err != nil {
				return nil, fmt.Errorf("spec.interfaces[%d].vpnTunnelRef: %w", i, err)
			}
			in.VPNTunnelRef.External = ext
		}
		if in.InterconnectAttachmentRef != nil {
			ext, err := configConnectorSelfLink(ctx, reader, obj.GetNamespace(), "ComputeInterconnectAttachment", in.InterconnectAttachmentRef.External, in.InterconnectAttachmentRef.Name, in.InterconnectAttachmentRef.Namespace)
			if err != nil {
				return nil, fmt.Errorf("spec.interfaces[%d].interconnectAttachmentRef: %w", i, err)
			}
			in.InterconnectAttachmentRef.External = ext
		}
	}

	keys, version, err := readMD5Keys(ctx, reader, obj)
	if err != nil {
		return nil, err
	}

	mapCtx := &direct.MapContext{}
	desired := ComputeRouterSpec_ToProto(mapCtx, &obj.Spec)
	if mapCtx.Err() != nil {
		return nil, mapCtx.Err()
	}

	gcpClient, err := m.client(ctx)
	if err != nil {
		return nil, err
	}
	return &RouterAdapter{
		gcpClient:      gcpClient,
		id:             id.(*krm.ComputeRouterIdentity),
		desired:        desired,
		keys:           keys,
		keysVersion:    version,
		appliedVersion: direct.ValueOf(obj.Status.MD5AuthenticationKeysVersion),
	}, nil
}

func (m *routerModel) AdapterForURL(ctx context.Context, url string) (directbase.Adapter, error) {
	id := &krm.ComputeRouterIdentity{}
	if err := id.FromExternal(url); err != nil {
		// Not recognized
		return nil, nil
	}
	gcpClient, err := m.client(ctx)
	if err != nil {
		return nil, err
	}
	return &RouterAdapter{
		gcpClient: gcpClient,
		id:        id,
	}, nil
}

type RouterAdapter struct {
	gcpClient *gcp.RoutersClient
	id        *krm.ComputeRouterIdentity
	desired   *pb.Router
	actual    *pb.Router
	// keys are the MD5 key values by key name, read from their Secrets.
	keys map[string]string
	// keysVersion is the digest of where keys came from; appliedVersion is
	// the one the keys were last written from (status).
	keysVersion, appliedVersion string
}

var _ directbase.Adapter = &RouterAdapter{}

func (a *RouterAdapter) Find(ctx context.Context) (bool, error) {
	log := klog.FromContext(ctx)
	log.V(2).Info("getting ComputeRouter", "name", a.id)

	actual, err := a.get(ctx)
	if err != nil {
		if direct.IsNotFound(err) {
			return false, nil
		}
		return false, fmt.Errorf("getting ComputeRouter %q: %w", a.id, err)
	}

	a.actual = actual
	return true, nil
}

func (a *RouterAdapter) Create(ctx context.Context, createOp *directbase.CreateOperation) error {
	log := klog.FromContext(ctx)
	log.V(2).Info("creating ComputeRouter", "name", a.id)

	// The API refuses to insert a router with more than one of its
	// features ("Creating a router with multiple features is currently
	// unsupported", seen live), so the router is inserted without its
	// interfaces, peers, NATs and keys, and they are written next with
	// routers.update, as an update writes them.
	resource := proto.CloneOf(a.desired)
	resource.Name = proto.String(a.id.Router)
	resource.Interfaces, resource.BgpPeers, resource.Nats, resource.Md5AuthenticationKeys = nil, nil, nil, nil

	req := &pb.InsertRouterRequest{
		Project:        a.id.Project,
		Region:         a.id.Region,
		RouterResource: resource,
	}
	op, err := a.gcpClient.Insert(ctx, req)
	if err != nil {
		return fmt.Errorf("creating ComputeRouter %s: %w", a.id, err)
	}

	err = op.Wait(ctx)
	if err != nil {
		return fmt.Errorf("compute ComputeRouter %s waiting creation: %w", a.id.String(), err)
	}
	log.Info("successfully created compute ComputeRouter in gcp", "name", a.id)

	created, err := a.get(ctx)
	if err != nil {
		return fmt.Errorf("getting ComputeRouter %s: %w", a.id, err)
	}
	if len(a.desired.Interfaces)+len(a.desired.BgpPeers)+len(a.desired.Nats)+len(a.desired.Md5AuthenticationKeys) > 0 {
		a.actual = created
		if created, err = a.write(ctx); err != nil {
			return err
		}
	}

	return a.updateStatus(ctx, createOp, created, a.keysVersion)
}

// write puts updateBody with routers.update, waits, and reads the router.
func (a *RouterAdapter) write(ctx context.Context) (*pb.Router, error) {
	req := &pb.UpdateRouterRequest{
		Project:        a.id.Project,
		Region:         a.id.Region,
		Router:         a.id.Router,
		RouterResource: a.updateBody(),
	}
	op, err := a.gcpClient.Update(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("updating compute ComputeRouter %s: %w", a.id.String(), err)
	}
	if err := op.Wait(ctx); err != nil {
		return nil, fmt.Errorf("compute ComputeRouter %s waiting for update: %w", a.id.String(), err)
	}
	klog.FromContext(ctx).V(2).Info("successfully updated compute ComputeRouter", "name", a.id.String())
	updated, err := a.get(ctx)
	if err != nil {
		return nil, fmt.Errorf("getting ComputeRouter %s: %w", a.id, err)
	}
	return updated, nil
}

func (a *RouterAdapter) Update(ctx context.Context, updateOp *directbase.UpdateOperation) error {
	log := klog.FromContext(ctx)
	log.V(2).Info("updating ComputeRouter", "name", a.id)

	if d := a.desired.GetNetwork(); d != "" && apirefs.TrimComputeURIPrefix(d) != apirefs.TrimComputeURIPrefix(a.actual.GetNetwork()) {
		return fmt.Errorf("ComputeRouter %s: networkRef is immutable (the router is in %q, the spec says %q)", a.id, a.actual.GetNetwork(), d)
	}
	if d := a.desired.EncryptedInterconnectRouter; d != nil && *d != a.actual.GetEncryptedInterconnectRouter() {
		return fmt.Errorf("ComputeRouter %s: encryptedInterconnectRouter is immutable (the router has %t)", a.id, a.actual.GetEncryptedInterconnectRouter())
	}

	keysChanged := len(a.keys) > 0 && a.keysVersion != a.appliedVersion
	if routerMatches(a.desired, a.actual) && !keysChanged {
		log.V(2).Info("no field needs update", "name", a.id.String())
		applied := a.appliedVersion
		if len(a.keys) == 0 {
			applied = ""
		}
		return a.updateStatus(ctx, updateOp, a.actual, applied)
	}

	updateOp.RecordUpdatingEvent()
	updated, err := a.write(ctx)
	if err != nil {
		return err
	}
	return a.updateStatus(ctx, updateOp, updated, a.keysVersion)
}

// updateBody is the router to PUT: the router as read, with the fields this
// kind owns replaced by the spec's. A peer or interface the API manages for
// an interconnect attachment stays unless the spec names it.
func (a *RouterAdapter) updateBody() *pb.Router {
	body := proto.CloneOf(a.actual)
	body.Description = proto.String(a.desired.GetDescription())
	if a.desired.Bgp != nil {
		body.Bgp = a.desired.Bgp
	}
	body.BgpPeers = slices.Clone(a.desired.BgpPeers)
	for _, p := range a.actual.BgpPeers {
		if !userManaged(p.GetManagementType()) && !slices.ContainsFunc(a.desired.BgpPeers, func(d *pb.RouterBgpPeer) bool { return d.GetName() == p.GetName() }) {
			body.BgpPeers = append(body.BgpPeers, p)
		}
	}
	body.Interfaces = slices.Clone(a.desired.Interfaces)
	for _, in := range a.actual.Interfaces {
		if !userManaged(in.GetManagementType()) && !slices.ContainsFunc(a.desired.Interfaces, func(d *pb.RouterInterface) bool { return d.GetName() == in.GetName() }) {
			body.Interfaces = append(body.Interfaces, in)
		}
	}
	body.Nats = a.desired.Nats
	body.Md5AuthenticationKeys = a.desired.Md5AuthenticationKeys
	body = proto.CloneOf(body)
	a.withKeyValues(body)
	return body
}

// withKeyValues fills in the MD5 key values read from the Secrets.
func (a *RouterAdapter) withKeyValues(r *pb.Router) {
	for _, k := range r.Md5AuthenticationKeys {
		if v, ok := a.keys[k.GetName()]; ok {
			k.Key = proto.String(v)
		}
	}
}

func (a *RouterAdapter) Export(ctx context.Context) (*unstructured.Unstructured, error) {
	if a.actual == nil {
		return nil, fmt.Errorf("Find() not called")
	}
	u := &unstructured.Unstructured{}

	obj := &krm.ComputeRouter{}
	mapCtx := &direct.MapContext{}
	obj.Spec = direct.ValueOf(ComputeRouterSpec_FromProto(mapCtx, a.actual))
	if mapCtx.Err() != nil {
		return nil, mapCtx.Err()
	}

	obj.Spec.Region = a.id.Region
	obj.Spec.ResourceID = direct.LazyPtr(a.id.Router)
	// A key's value is in no Secret we know of; peers keep the key's name.
	obj.Spec.MD5AuthenticationKeys = nil

	if obj.Spec.EncryptedInterconnectRouter != nil && !*obj.Spec.EncryptedInterconnectRouter {
		obj.Spec.EncryptedInterconnectRouter = nil
	}

	uObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
	if err != nil {
		return nil, err
	}

	u.Object = uObj
	u.SetName(a.id.Router)
	u.SetGroupVersionKind(krm.ComputeRouterGVK)

	export.SetProjectID(u, a.id.Project)

	return u, nil
}

func (a *RouterAdapter) Delete(ctx context.Context, deleteOp *directbase.DeleteOperation) (bool, error) {
	log := klog.FromContext(ctx)
	log.V(2).Info("deleting ComputeRouter", "name", a.id)

	req := &pb.DeleteRouterRequest{
		Project: a.id.Project,
		Region:  a.id.Region,
		Router:  a.id.Router,
	}
	op, err := a.gcpClient.Delete(ctx, req)
	if err != nil {
		if direct.IsNotFound(err) {
			return true, nil
		}
		return false, fmt.Errorf("deleting compute ComputeRouter %s: %w", a.id.String(), err)
	}
	log.Info("successfully deleted compute ComputeRouter", "name", a.id)

	if !op.Done() {
		err = op.Wait(ctx)
		if err != nil {
			return false, fmt.Errorf("waiting for deletion of compute ComputeRouter %s: %w", a.id.String(), err)
		}
	}

	return true, nil
}

func (a *RouterAdapter) get(ctx context.Context) (*pb.Router, error) {
	getReq := &pb.GetRouterRequest{
		Project: a.id.Project,
		Region:  a.id.Region,
		Router:  a.id.Router,
	}
	return a.gcpClient.Get(ctx, getReq)
}

func (a *RouterAdapter) updateStatus(ctx context.Context, op directbase.Operation, latest *pb.Router, keysVersion string) error {
	mapCtx := &direct.MapContext{}
	status := ComputeRouterStatus_FromProto(mapCtx, latest)
	if mapCtx.Err() != nil {
		return mapCtx.Err()
	}
	if keysVersion != "" {
		status.MD5AuthenticationKeysVersion = direct.LazyPtr(keysVersion)
	}
	return op.UpdateStatus(ctx, status, nil)
}

// userManaged says whether a peer or interface is the user's: one the API
// made for an interconnect attachment is MANAGED_BY_ATTACHMENT.
func userManaged(managementType string) bool {
	return managementType == "" || managementType == "MANAGED_BY_USER"
}

// readMD5Keys reads each MD5 key's value from its Secret, in the router's
// namespace, and the digest of where the values came from (each key's name
// and its Secret's name, key and resourceVersion; never the value).
func readMD5Keys(ctx context.Context, reader client.Reader, obj *krm.ComputeRouter) (map[string]string, string, error) {
	if len(obj.Spec.MD5AuthenticationKeys) == 0 {
		return nil, "", nil
	}
	values := map[string]string{}
	h := sha256.New()
	for i, k := range obj.Spec.MD5AuthenticationKeys {
		path := fmt.Sprintf("spec.md5AuthenticationKeys[%d].keyRef", i)
		if k.KeyRef.Name == "" || k.KeyRef.Key == "" {
			return nil, "", fmt.Errorf("%s: name and key are required", path)
		}
		s := &corev1.Secret{}
		if err := reader.Get(ctx, types.NamespacedName{Namespace: obj.GetNamespace(), Name: k.KeyRef.Name}, s); err != nil {
			if apierrors.IsNotFound(err) {
				return nil, "", fmt.Errorf("%s: Secret %s/%s not found", path, obj.GetNamespace(), k.KeyRef.Name)
			}
			return nil, "", fmt.Errorf("%s: reading Secret %s/%s: %w", path, obj.GetNamespace(), k.KeyRef.Name, err)
		}
		v, ok := s.Data[k.KeyRef.Key]
		if !ok {
			return nil, "", fmt.Errorf("%s: Secret %s/%s has no key %q", path, obj.GetNamespace(), k.KeyRef.Name, k.KeyRef.Key)
		}
		values[k.Name] = string(v)
		fmt.Fprintf(h, "%s\x00%s\x00%s\x00%s\x00", k.Name, k.KeyRef.Name, k.KeyRef.Key, s.ResourceVersion)
	}
	return values, hex.EncodeToString(h.Sum(nil)), nil
}

// configConnectorSelfLink resolves a reference to a Config Connector compute
// object: an external link as given (without its URL prefix), or the named
// object's status.selfLink.
func configConnectorSelfLink(ctx context.Context, reader client.Reader, defaultNamespace, kind, external, name, namespace string) (string, error) {
	if external != "" {
		return apirefs.TrimComputeURIPrefix(external), nil
	}
	if name == "" {
		return "", fmt.Errorf("one of external and name is required")
	}
	if namespace == "" {
		namespace = defaultNamespace
	}
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(schema.GroupVersionKind{Group: "compute.cnrm.cloud.google.com", Version: "v1beta1", Kind: kind})
	if err := reader.Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, u); err != nil {
		return "", fmt.Errorf("reading %s %s/%s: %w", kind, namespace, name, err)
	}
	link, _, _ := unstructured.NestedString(u.Object, "status", "selfLink")
	if link == "" {
		return "", fmt.Errorf("%s %s/%s has no status.selfLink yet", kind, namespace, name)
	}
	return apirefs.TrimComputeURIPrefix(link), nil
}

// routerMatches says whether the cloud's router is what the spec asks for.
// Peers and interfaces the API manages for an interconnect attachment are
// not the spec's and are left out of the comparison.
func routerMatches(desired, actual *pb.Router) bool {
	if desired.GetDescription() != actual.GetDescription() {
		return false
	}
	if desired.Bgp != nil && !protoSubset(desired.Bgp.ProtoReflect(), actual.GetBgp().ProtoReflect()) {
		return false
	}
	var peers []*pb.RouterBgpPeer
	for _, p := range actual.BgpPeers {
		if userManaged(p.GetManagementType()) {
			peers = append(peers, p)
		}
	}
	var interfaces []*pb.RouterInterface
	for _, in := range actual.Interfaces {
		if userManaged(in.GetManagementType()) {
			interfaces = append(interfaces, in)
		}
	}
	if !messagesMatch(desired.BgpPeers, peers) || !messagesMatch(desired.Interfaces, interfaces) || !messagesMatch(desired.Nats, actual.Nats) {
		return false
	}
	var want, have []string
	for _, k := range desired.Md5AuthenticationKeys {
		want = append(want, k.GetName())
	}
	for _, k := range actual.Md5AuthenticationKeys {
		have = append(have, k.GetName())
	}
	sort.Strings(want)
	sort.Strings(have)
	return slices.Equal(want, have)
}

func messagesMatch[T proto.Message](desired, actual []T) bool {
	d := make([]protoreflect.Message, len(desired))
	for i, m := range desired {
		d[i] = m.ProtoReflect()
	}
	a := make([]protoreflect.Message, len(actual))
	for i, m := range actual {
		a[i] = m.ProtoReflect()
	}
	return messageListsMatch(d, a)
}

// messageListsMatch pairs every desired element with a distinct actual one
// it is a subset of, in any order; the lists must be the same length.
func messageListsMatch(desired, actual []protoreflect.Message) bool {
	if len(desired) != len(actual) {
		return false
	}
	used := make([]bool, len(actual))
	for _, d := range desired {
		found := false
		for j, a := range actual {
			if !used[j] && protoSubset(d, a) {
				used[j], found = true, true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

// serverDefaultedLists are the lists the API fills in when they are left
// out, so an empty one in the spec says nothing.
var serverDefaultedLists = map[protoreflect.Name]bool{"endpoint_types": true}

// orderedLists are the lists whose order means something: route policies
// are evaluated in the order given. Other lists compare as sets.
var orderedLists = map[protoreflect.Name]bool{"import_policies": true, "export_policies": true}

// protoSubset says whether actual has every field desired sets, with the
// same value: a scalar equal to the cloud's (a link without its URL prefix,
// a missing one as its zero value), a message recursively, a list with the
// same members in any order. A list desired leaves empty must be empty in
// actual too, unless the API defaults it.
func protoSubset(desired, actual protoreflect.Message) bool {
	fields := desired.Descriptor().Fields()
	for i := 0; i < fields.Len(); i++ {
		fd := fields.Get(i)
		if fd.IsMap() {
			continue
		}
		if fd.IsList() {
			dl, al := desired.Get(fd).List(), actual.Get(fd).List()
			if dl.Len() == 0 && serverDefaultedLists[fd.Name()] {
				continue
			}
			if !listsMatch(fd, dl, al) {
				return false
			}
			continue
		}
		if !desired.Has(fd) {
			continue
		}
		dv, av := desired.Get(fd), actual.Get(fd)
		switch fd.Kind() {
		case protoreflect.MessageKind, protoreflect.GroupKind:
			if !protoSubset(dv.Message(), av.Message()) {
				return false
			}
		case protoreflect.StringKind:
			if apirefs.TrimComputeURIPrefix(dv.String()) != apirefs.TrimComputeURIPrefix(av.String()) {
				return false
			}
		default:
			if !dv.Equal(av) {
				return false
			}
		}
	}
	return true
}

func listsMatch(fd protoreflect.FieldDescriptor, desired, actual protoreflect.List) bool {
	if desired.Len() != actual.Len() {
		return false
	}
	if fd.Kind() == protoreflect.MessageKind {
		d := make([]protoreflect.Message, desired.Len())
		a := make([]protoreflect.Message, actual.Len())
		for i := range d {
			d[i], a[i] = desired.Get(i).Message(), actual.Get(i).Message()
		}
		return messageListsMatch(d, a)
	}
	d := make([]string, desired.Len())
	a := make([]string, actual.Len())
	for i := range d {
		d[i] = apirefs.TrimComputeURIPrefix(desired.Get(i).String())
		a[i] = apirefs.TrimComputeURIPrefix(actual.Get(i).String())
	}
	if !orderedLists[fd.Name()] {
		sort.Strings(d)
		sort.Strings(a)
	}
	return slices.Equal(d, a)
}
