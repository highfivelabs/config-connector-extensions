// Copyright 2025 Google LLC
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

// Modified by High Five Labs for config-connector-extensions: copied from Config Connector
// apis/privateca/v1beta1/types.generated.go (UPSTREAM.md), where
// X509Parameters_NameConstraints is generated but commented out as an
// unreachable type (no kind of Config Connector's has the field). The
// lists are atomic: a field kind owns each list whole.

package v1alpha1

// +kcc:proto=google.cloud.security.privateca.v1.X509Parameters.NameConstraints
type X509Parameters_NameConstraints struct {
	// Indicates whether or not the name constraints are marked critical.
	// +kcc:proto:field=google.cloud.security.privateca.v1.X509Parameters.NameConstraints.critical
	Critical *bool `json:"critical,omitempty"`

	// Contains permitted DNS names. Any DNS name that can be
	//  constructed by simply adding zero or more labels to
	//  the left-hand side of the name satisfies the name constraint.
	//  For example, `example.com`, `www.example.com`, `www.sub.example.com`
	//  would satisfy `example.com` while `example1.com` does not.
	// +kcc:proto:field=google.cloud.security.privateca.v1.X509Parameters.NameConstraints.permitted_dns_names
	// +listType=atomic
	PermittedDNSNames []string `json:"permittedDNSNames,omitempty"`

	// Contains excluded DNS names. Any DNS name that can be
	//  constructed by simply adding zero or more labels to
	//  the left-hand side of the name satisfies the name constraint.
	//  For example, `example.com`, `www.example.com`, `www.sub.example.com`
	//  would satisfy `example.com` while `example1.com` does not.
	// +kcc:proto:field=google.cloud.security.privateca.v1.X509Parameters.NameConstraints.excluded_dns_names
	// +listType=atomic
	ExcludedDNSNames []string `json:"excludedDNSNames,omitempty"`

	// Contains the permitted IP ranges. For IPv4 addresses, the ranges
	//  are expressed using CIDR notation as specified in RFC 4632.
	//  For IPv6 addresses, the ranges are expressed in similar encoding as IPv4
	//  addresses.
	// +kcc:proto:field=google.cloud.security.privateca.v1.X509Parameters.NameConstraints.permitted_ip_ranges
	// +listType=atomic
	PermittedIPRanges []string `json:"permittedIPRanges,omitempty"`

	// Contains the excluded IP ranges. For IPv4 addresses, the ranges
	//  are expressed using CIDR notation as specified in RFC 4632.
	//  For IPv6 addresses, the ranges are expressed in similar encoding as IPv4
	//  addresses.
	// +kcc:proto:field=google.cloud.security.privateca.v1.X509Parameters.NameConstraints.excluded_ip_ranges
	// +listType=atomic
	ExcludedIPRanges []string `json:"excludedIPRanges,omitempty"`

	// Contains the permitted email addresses. The value can be a particular
	//  email address, a hostname to indicate all email addresses on that host or
	//  a domain with a leading period (e.g. `.example.com`) to indicate
	//  all email addresses in that domain.
	// +kcc:proto:field=google.cloud.security.privateca.v1.X509Parameters.NameConstraints.permitted_email_addresses
	// +listType=atomic
	PermittedEmailAddresses []string `json:"permittedEmailAddresses,omitempty"`

	// Contains the excluded email addresses. The value can be a particular
	//  email address, a hostname to indicate all email addresses on that host or
	//  a domain with a leading period (e.g. `.example.com`) to indicate
	//  all email addresses in that domain.
	// +kcc:proto:field=google.cloud.security.privateca.v1.X509Parameters.NameConstraints.excluded_email_addresses
	// +listType=atomic
	ExcludedEmailAddresses []string `json:"excludedEmailAddresses,omitempty"`

	// Contains the permitted URIs that apply to the host part of the name.
	//  The value can be a hostname or a domain with a
	//  leading period (like `.example.com`)
	// +kcc:proto:field=google.cloud.security.privateca.v1.X509Parameters.NameConstraints.permitted_uris
	// +listType=atomic
	PermittedUris []string `json:"permittedUris,omitempty"`

	// Contains the excluded URIs that apply to the host part of the name.
	//  The value can be a hostname or a domain with a
	//  leading period (like `.example.com`)
	// +kcc:proto:field=google.cloud.security.privateca.v1.X509Parameters.NameConstraints.excluded_uris
	// +listType=atomic
	ExcludedUris []string `json:"excludedUris,omitempty"`
}
