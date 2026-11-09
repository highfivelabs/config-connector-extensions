// Copyright 2026 High Five Labs
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/bufbuild/protocompile"
	"github.com/bufbuild/protocompile/reporter"
	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// The denominator for services Discovery does not describe (gRPC-only
// APIs): googleapis protos. A service is a proto service with a
// google.api.default_host; a resource is the message a Create<Name> RPC
// creates (AIP-133); its attributes are that message's fields that are not
// OUTPUT_ONLY.

const (
	extDefaultHost   = 1049 // google.api.default_host on ServiceOptions
	extFieldBehavior = 1052 // google.api.field_behavior on FieldOptions
	behRequired      = 2
	behOutputOnly    = 3
)

var protoVersion = regexp.MustCompile(`\.(v\d+(?:[a-z]+\d*)?)$`)

// protoSkipDirs are googleapis directories that are not Google Cloud
// infrastructure (advertising, Maps, Workspace, consumer, shopping).
var protoSkipDirs = []string{
	"google/ads/", "google/maps/", "google/shopping/", "google/apps/", "google/chat/", "google/area120/",
	"google/actions/", "google/assistant/", "google/geo/", "google/marketingplatform/", "google/analytics/",
	"google/youtube/", "google/home/", "google/search/", "google/chromeos/", "google/chrome/",
	"google/identity/", "google/streetview/", "google/photos/", "google/gdata/", "google/ccc/",
}

type protoDoc struct {
	host    string
	pkg     string // google.cloud.visionai.v1
	version string
	files   []protoreflect.FileDescriptor
}

func unknownVarints(b []byte, num protowire.Number) []uint64 {
	var out []uint64
	for len(b) > 0 {
		n, typ, l := protowire.ConsumeTag(b)
		if l < 0 {
			return out
		}
		b = b[l:]
		switch typ {
		case protowire.VarintType:
			v, l := protowire.ConsumeVarint(b)
			if l < 0 {
				return out
			}
			if n == num {
				out = append(out, v)
			}
			b = b[l:]
		case protowire.BytesType:
			v, l := protowire.ConsumeBytes(b)
			if l < 0 {
				return out
			}
			if n == num { // packed
				for len(v) > 0 {
					x, k := protowire.ConsumeVarint(v)
					if k < 0 {
						break
					}
					out = append(out, x)
					v = v[k:]
				}
			}
			b = b[l:]
		default:
			l := protowire.ConsumeFieldValue(n, typ, b)
			if l < 0 {
				return out
			}
			b = b[l:]
		}
	}
	return out
}

func unknownString(b []byte, num protowire.Number) string {
	for len(b) > 0 {
		n, typ, l := protowire.ConsumeTag(b)
		if l < 0 {
			return ""
		}
		b = b[l:]
		if typ == protowire.BytesType {
			v, l := protowire.ConsumeBytes(b)
			if l < 0 {
				return ""
			}
			if n == num {
				return string(v)
			}
			b = b[l:]
			continue
		}
		l = protowire.ConsumeFieldValue(n, typ, b)
		if l < 0 {
			return ""
		}
		b = b[l:]
	}
	return ""
}

// optionBytes is the wire form of an options message, in which the
// google.api extensions (unknown to the Go descriptor types) sit.
func optionBytes(m protoreflect.ProtoMessage) []byte {
	if m == nil {
		return nil
	}
	b, err := proto.Marshal(m)
	if err != nil {
		return nil
	}
	return b
}

func listProtoFiles(root string) ([]string, error) {
	var out []string
	err := filepath.WalkDir(filepath.Join(root, "google"), func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(p, ".proto") {
			return err
		}
		rel, _ := filepath.Rel(root, p)
		rel = filepath.ToSlash(rel)
		for _, s := range protoSkipDirs {
			if strings.HasPrefix(rel, s) {
				return nil
			}
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		if strings.Contains(string(b), "default_host") && strings.Contains(string(b), "\nservice ") {
			out = append(out, rel)
		}
		return nil
	})
	sort.Strings(out)
	return out, err
}

// loadProtos compiles the service protos under root/google and returns,
// per service host not in skipHosts, the documents to read, best first.
func loadProtos(root string, skipHosts map[string]bool) (map[string][]*protoDoc, []string, error) {
	files, err := listProtoFiles(root)
	if err != nil {
		return nil, nil, err
	}
	var failed []string
	rep := reporter.NewReporter(func(e reporter.ErrorWithPos) error {
		failed = append(failed, e.Error())
		return nil
	}, nil)
	comp := protocompile.Compiler{
		Resolver: protocompile.WithStandardImports(&protocompile.SourceResolver{ImportPaths: []string{root}}),
		Reporter: rep,
	}
	res, cerr := comp.Compile(context.Background(), files...)
	if res == nil && cerr != nil {
		return nil, failed, fmt.Errorf("compiling googleapis: %w", cerr)
	}
	byHostPkg := map[string]*protoDoc{}
	for _, f := range res {
		if f == nil {
			continue
		}
		svcs := f.Services()
		for i := 0; i < svcs.Len(); i++ {
			sd := svcs.Get(i)
			host := unknownString(optionBytes(sd.Options()), extDefaultHost)
			if host == "" || skipHosts[host] || !strings.HasSuffix(host, ".googleapis.com") {
				continue
			}
			pkg := string(f.Package())
			m := protoVersion.FindStringSubmatch(pkg)
			if m == nil {
				continue
			}
			k := host + "|" + pkg
			d := byHostPkg[k]
			if d == nil {
				d = &protoDoc{host: host, pkg: pkg, version: m[1]}
				byHostPkg[k] = d
			}
			d.files = append(d.files, f)
		}
	}
	out := map[string][]*protoDoc{}
	for _, d := range byHostPkg {
		out[d.host] = append(out[d.host], d)
	}
	for h, ds := range out {
		sort.SliceStable(ds, func(i, j int) bool {
			ri, ni := versionRank(ds[i].version)
			rj, nj := versionRank(ds[j].version)
			if ri != rj {
				return ri > rj
			}
			if ni != nj {
				return ni > nj
			}
			return ds[i].pkg < ds[j].pkg
		})
		if r, _ := versionRank(ds[0].version); r == 3 {
			var stable []*protoDoc
			for _, d := range ds {
				if r, _ := versionRank(d.version); r == 3 {
					stable = append(stable, d)
				}
			}
			out[h] = stable
		} else {
			out[h] = ds[:1]
		}
	}
	return out, failed, nil
}

var wellKnownLeaf = map[string]string{
	"google.protobuf.Timestamp": "string", "google.protobuf.Duration": "string",
	"google.protobuf.FieldMask": "string", "google.protobuf.Struct": "object",
	"google.protobuf.Value": "any", "google.protobuf.Any": "object", "google.protobuf.ListValue": "array",
	"google.protobuf.Empty": "object", "google.type.Date": "object", "google.type.TimeOfDay": "object",
	"google.type.Money": "object", "google.type.LatLng": "object", "google.type.Interval": "object",
	"google.protobuf.BoolValue": "boolean", "google.protobuf.StringValue": "string",
	"google.protobuf.Int32Value": "integer", "google.protobuf.Int64Value": "integer",
	"google.protobuf.DoubleValue": "number", "google.protobuf.FloatValue": "number",
	"google.protobuf.UInt32Value": "integer", "google.protobuf.UInt64Value": "integer",
	"google.protobuf.BytesValue": "string",
}

func kindName(k protoreflect.Kind) string {
	switch k {
	case protoreflect.BoolKind:
		return "boolean"
	case protoreflect.StringKind, protoreflect.BytesKind, protoreflect.EnumKind:
		return "string"
	case protoreflect.FloatKind, protoreflect.DoubleKind:
		return "number"
	case protoreflect.MessageKind, protoreflect.GroupKind:
		return "object"
	}
	return "integer"
}

type protoFlattener struct {
	res   *APIResource
	stack []string
}

func fieldBehaviors(fd protoreflect.FieldDescriptor) (required, outputOnly bool) {
	for _, v := range unknownVarints(optionBytes(fd.Options()), extFieldBehavior) {
		switch v {
		case behRequired:
			required = true
		case behOutputOnly:
			outputOnly = true
		}
	}
	return
}

func (f *protoFlattener) message(md protoreflect.MessageDescriptor, path string, depth int) {
	f.stack = append(f.stack, string(md.FullName()))
	defer func() { f.stack = f.stack[:len(f.stack)-1] }()
	fs := md.Fields()
	type pf struct {
		name string
		fd   protoreflect.FieldDescriptor
	}
	var list []pf
	for i := 0; i < fs.Len(); i++ {
		fd := fs.Get(i)
		list = append(list, pf{fd.JSONName(), fd})
	}
	sort.Slice(list, func(i, j int) bool { return list[i].name < list[j].name })
	for _, e := range list {
		req, out := fieldBehaviors(e.fd)
		if out {
			continue
		}
		f.field(e.fd, join(path, e.name), req, depth+1)
	}
}

func (f *protoFlattener) field(fd protoreflect.FieldDescriptor, path string, req bool, depth int) {
	emit := func(p, typ string) {
		f.res.Attrs = append(f.res.Attrs, Attr{Path: p, Type: typ, Required: req})
	}
	switch {
	case fd.IsMap():
		v := fd.MapValue()
		if v.Kind() == protoreflect.MessageKind || v.Kind() == protoreflect.GroupKind {
			if t, ok := wellKnownLeaf[string(v.Message().FullName())]; ok {
				emit(path+"{}", t)
				return
			}
			f.nested(v.Message(), path+"{}", req, depth)
			return
		}
		emit(path+"{}", kindName(v.Kind()))
	case fd.Kind() == protoreflect.MessageKind || fd.Kind() == protoreflect.GroupKind:
		p := path
		if fd.IsList() {
			p += "[]"
		}
		if t, ok := wellKnownLeaf[string(fd.Message().FullName())]; ok {
			emit(p, t)
			return
		}
		f.nested(fd.Message(), p, req, depth)
	default:
		p := path
		if fd.IsList() {
			p += "[]"
		}
		emit(p, kindName(fd.Kind()))
	}
}

func (f *protoFlattener) nested(md protoreflect.MessageDescriptor, path string, req bool, depth int) {
	for _, s := range f.stack {
		if s == string(md.FullName()) {
			f.res.Truncated++
			f.res.Attrs = append(f.res.Attrs, Attr{Path: path, Type: "object", Required: req})
			return
		}
	}
	if depth > 24 {
		f.res.Truncated++
		f.res.Attrs = append(f.res.Attrs, Attr{Path: path, Type: "object", Required: req})
		return
	}
	f.message(md, path, depth)
}

var camelSplit = regexp.MustCompile(`[A-Z][a-z0-9]*|[0-9]+`)

// buildProtoResources makes the resources of one host from its protos.
func buildProtoResources(host string, docs []*protoDoc) []*APIResource {
	type cand struct {
		name string
		rpc  string
		req  protoreflect.MessageDescriptor
		pkg  string
	}
	var cs []cand
	seen := map[string]bool{}
	for _, d := range docs {
		var batch []cand
		for _, f := range d.files {
			svcs := f.Services()
			for i := 0; i < svcs.Len(); i++ {
				sd := svcs.Get(i)
				if unknownString(optionBytes(sd.Options()), extDefaultHost) != host {
					continue
				}
				ms := sd.Methods()
				for j := 0; j < ms.Len(); j++ {
					m := ms.Get(j)
					n := string(m.Name())
					if !strings.HasPrefix(n, "Create") || len(n) == len("Create") || m.IsStreamingClient() || m.IsStreamingServer() {
						continue
					}
					if seen[norm(n[len("Create"):])] {
						continue
					}
					batch = append(batch, cand{n[len("Create"):], string(sd.FullName()) + "." + n, m.Input(), d.pkg})
				}
			}
		}
		for _, c := range batch {
			seen[norm(c.name)] = true
		}
		cs = append(cs, batch...)
	}
	var out []*APIResource
	used := map[string]bool{}
	for _, c := range cs {
		// The message the RPC creates: the request field whose message is
		// named like the resource, else its one non-parent message field.
		var body protoreflect.FieldDescriptor
		fs := c.req.Fields()
		var msgs []protoreflect.FieldDescriptor
		for i := 0; i < fs.Len(); i++ {
			fd := fs.Get(i)
			if fd.Kind() != protoreflect.MessageKind || fd.IsList() || fd.IsMap() {
				continue
			}
			if _, wk := wellKnownLeaf[string(fd.Message().FullName())]; wk {
				continue
			}
			msgs = append(msgs, fd)
			if norm(string(fd.Message().Name())) == norm(c.name) || norm(string(fd.Name())) == norm(c.name) {
				body = fd
			}
		}
		if body == nil && len(msgs) == 1 {
			body = msgs[0]
		}
		if body == nil {
			continue
		}
		name := c.name
		if used[name] {
			continue
		}
		used[name] = true
		r := &APIResource{Service: host, Name: name, Disco: protoVersion.FindStringSubmatch(c.pkg)[1] + ":" + c.rpc}
		words := camelSplit.FindAllString(name, -1)
		r.Names = []string{norm(name)}
		for i := len(words) - 1; i >= 1; i-- {
			r.Names = append(r.Names, norm(strings.Join(words[i:], "")))
		}
		// (the full name first, then its trailing words)
		pf := &protoFlattener{res: r}
		pf.message(body.Message(), "", 0)
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}
