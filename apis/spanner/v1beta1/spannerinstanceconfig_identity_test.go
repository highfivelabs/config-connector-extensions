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

package v1beta1

import "testing"

func TestParseInstanceConfigExternal(t *testing.T) {
	const name = "projects/p/instanceConfigs/custom-nam3-ro"
	parent, id, err := ParseInstanceConfigExternal(name)
	if err != nil {
		t.Fatal(err)
	}
	if id != "custom-nam3-ro" || parent.String() != "projects/p" {
		t.Errorf("ParseInstanceConfigExternal(%q) = %v, %q", name, parent, id)
	}
	if got := (&InstanceConfigIdentity{parent: parent, id: id}).String(); got != name {
		t.Errorf("String() = %q, want %q", got, name)
	}
	for _, bad := range []string{"", "projects/p", "projects/p/locations/l/instanceConfigs/c", "projects/p/instances/i"} {
		if _, _, err := ParseInstanceConfigExternal(bad); err == nil {
			t.Errorf("ParseInstanceConfigExternal(%q): no error", bad)
		}
	}
}
