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

// Package privateca holds the field kinds on Config Connector's Private CA
// kinds (docs/design.md section 6.6, pattern 2; decision 37):
// PrivateCACAPoolFields and PrivateCACertificateTemplateFields.
package privateca

// Written here: what both field kinds share.

import (
	"context"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	api "cloud.google.com/go/security/privateca/apiv1"
	pb "cloud.google.com/go/security/privateca/apiv1/privatecapb"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/config"
	"google.golang.org/protobuf/types/known/durationpb"

	krm "github.com/highfivelabs/config-connector-extensions/apis/privateca/v1alpha1"
)

// newClient is Config Connector's newCertificateAuthorityClient
// (pkg/controller/direct/privateca/client.go): the REST client.
func newClient(ctx context.Context, config *config.ControllerConfig) (*api.CertificateAuthorityClient, error) {
	opts, err := config.RESTClientOptions()
	if err != nil {
		return nil, err
	}
	c, err := api.NewCertificateAuthorityRESTClient(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("building service for certificate authority: %w", err)
	}
	return c, nil
}

// parseDuration reads a protobuf JSON duration ("3600s", "1.5s").
func parseDuration(field, s string) (*durationpb.Duration, error) {
	if !strings.HasSuffix(s, "s") {
		return nil, fmt.Errorf("%s: %q is not a duration in seconds, such as \"3600s\"", field, s)
	}
	f, err := strconv.ParseFloat(strings.TrimSuffix(s, "s"), 64)
	if err != nil || f < 0 {
		return nil, fmt.Errorf("%s: %q is not a duration in seconds, such as \"3600s\"", field, s)
	}
	return durationpb.New(time.Duration(f * float64(time.Second))), nil
}

func sameDuration(a, b *durationpb.Duration) bool {
	return a.GetSeconds() == b.GetSeconds() && a.GetNanos() == b.GetNanos()
}

// nameConstraintsToProto is the spec's name constraints as the API takes
// them.
func nameConstraintsToProto(in *krm.X509Parameters_NameConstraints) *pb.X509Parameters_NameConstraints {
	if in == nil {
		return nil
	}
	return &pb.X509Parameters_NameConstraints{
		Critical:                in.Critical != nil && *in.Critical,
		PermittedDnsNames:       in.PermittedDNSNames,
		ExcludedDnsNames:        in.ExcludedDNSNames,
		PermittedIpRanges:       in.PermittedIPRanges,
		ExcludedIpRanges:        in.ExcludedIPRanges,
		PermittedEmailAddresses: in.PermittedEmailAddresses,
		ExcludedEmailAddresses:  in.ExcludedEmailAddresses,
		PermittedUris:           in.PermittedUris,
		ExcludedUris:            in.ExcludedUris,
	}
}

// sameNameConstraints compares two name constraints, each list as a set
// (the API may return them in its own order); nil is the same as empty.
func sameNameConstraints(a, b *pb.X509Parameters_NameConstraints) bool {
	set := func(in []string) []string {
		out := slices.Clone(in)
		slices.Sort(out)
		return slices.Compact(out)
	}
	same := func(x, y []string) bool { return slices.Equal(set(x), set(y)) }
	return a.GetCritical() == b.GetCritical() &&
		same(a.GetPermittedDnsNames(), b.GetPermittedDnsNames()) &&
		same(a.GetExcludedDnsNames(), b.GetExcludedDnsNames()) &&
		same(a.GetPermittedIpRanges(), b.GetPermittedIpRanges()) &&
		same(a.GetExcludedIpRanges(), b.GetExcludedIpRanges()) &&
		same(a.GetPermittedEmailAddresses(), b.GetPermittedEmailAddresses()) &&
		same(a.GetExcludedEmailAddresses(), b.GetExcludedEmailAddresses()) &&
		same(a.GetPermittedUris(), b.GetPermittedUris()) &&
		same(a.GetExcludedUris(), b.GetExcludedUris())
}
