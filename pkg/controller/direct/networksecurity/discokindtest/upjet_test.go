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

package discokindtest

import (
	"testing"

	"github.com/highfivelabs/config-connector-extensions/pkg/discokind/kindtest"
)

// TestNetworkSecurityDNSThreatDetectorUpjet makes the detector of
// Crossplane's Upjet example, with labels, and changes them; live it skips
// as the kind's own test does (testdata/.../upjet/live-skip).
func TestNetworkSecurityDNSThreatDetectorUpjet(t *testing.T) {
	kindtest.RunCase(t, "NetworkSecurityDNSThreatDetector", "upjet")
}

// TestNetworkSecurityGatewaySecurityPolicyTLS makes a policy that names a
// TLS inspection policy, the one the test makes first, by reference, and
// then by the string it resolves to; live it skips (tls/live-skip: the
// TLS inspection policy needs a CA pool).
func TestNetworkSecurityGatewaySecurityPolicyTLS(t *testing.T) {
	kindtest.RunCase(t, "NetworkSecurityGatewaySecurityPolicy", "tls")
}
