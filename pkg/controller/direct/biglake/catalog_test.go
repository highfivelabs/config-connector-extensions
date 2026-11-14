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

package biglake_test

import (
	"testing"

	"github.com/highfivelabs/config-connector-extensions/pkg/test/mockenv"
)

// TestBigLakeCatalogMissingIs403: a catalog (the catalog's live run in
// hfl-dev-514205 took the 403 before its create for a denial and never
// made the catalog).
func TestBigLakeCatalogMissingIs403(t *testing.T) {
	testMissingIs403(t, "BigLakeCatalog", "biglakecatalog-403", func(env *mockenv.Env) map[string]any {
		return map[string]any{}
	})
}
