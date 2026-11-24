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
	"github.com/highfivelabs/config-connector-extensions/pkg/test/mockenv"
)

func init() {
	kindtest.RegisterSetup("ComputeSubnetwork", networkSetup("computesubnetwork"))
	kindtest.RegisterSetup("ComputeSubnetwork/insertonly", networkSetup("computesubnetwork-insertonly"))
}

// networkSetup makes the network ${NAME}-net the fixtures of a kind whose
// lower-case name is base put their subnet in, for the test's object and
// for its abandoned one (whose ${NAME} ends in -kept), on the fake and
// live. Each network is deleted after the test's own resources (t.Cleanup
// runs last-in first-out) and swept by name (hack/live/sweep.sh).
func networkSetup(base string) kindtest.Setup {
	return func(t *testing.T, env *mockenv.Env) {
		for _, b := range []string{base, base + "-kept"} {
			newNetwork(t, env, b+"-net")
		}
	}
}

// TestComputeSubnetworkInsertOnly makes a subnet with the fields the API
// takes only at insert (a BYOIP IPv6 sub-prefix, subnet mask resolution,
// resource manager tags) on the fake cloud; live it skips (live-skip).
func TestComputeSubnetworkInsertOnly(t *testing.T) {
	kindtest.RunCase(t, "ComputeSubnetwork", "insertonly")
}
