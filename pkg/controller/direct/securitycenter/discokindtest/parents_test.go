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

// The other two parents of the Security Command Center kinds (decision
// 16): a folder's and an organization's, from
// testdata/discokind/<Kind>/folder/ and organization/; the generated tests
// run a project's. On the fake cloud only (each case's live-skip).

func TestSecurityCenterBigQueryExportFolder(t *testing.T) {
	kindtest.RunCase(t, "SecurityCenterBigQueryExport", "folder")
}

func TestSecurityCenterBigQueryExportOrganization(t *testing.T) {
	kindtest.RunCase(t, "SecurityCenterBigQueryExport", "organization")
}

func TestSecurityCenterNotificationConfigFolder(t *testing.T) {
	kindtest.RunCase(t, "SecurityCenterNotificationConfig", "folder")
}

func TestSecurityCenterNotificationConfigOrganization(t *testing.T) {
	kindtest.RunCase(t, "SecurityCenterNotificationConfig", "organization")
}
