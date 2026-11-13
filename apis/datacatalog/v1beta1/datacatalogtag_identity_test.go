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

func TestParseTagExternal(t *testing.T) {
	const name = "projects/p/locations/us-central1/entryGroups/g/entries/e/tags/123"
	parent, id, err := ParseTagExternal(name)
	if err != nil {
		t.Fatal(err)
	}
	if id != "123" || parent.String() != "projects/p/locations/us-central1/entryGroups/g/entries/e" {
		t.Errorf("ParseTagExternal(%q) = %v, %q", name, parent, id)
	}
	if got := (&TagIdentity{parent: parent, id: id}).String(); got != name {
		t.Errorf("String() = %q, want %q", got, name)
	}
	for _, bad := range []string{"", "projects/p/locations/l/entryGroups/g/entries/e", "projects/p/locations/l/tagTemplates/t/tags/1"} {
		if _, _, err := ParseTagExternal(bad); err == nil {
			t.Errorf("ParseTagExternal(%q): no error", bad)
		}
	}
}
