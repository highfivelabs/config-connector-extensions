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

package controller

import (
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/highfivelabs/config-connector-extensions/pkg/discokind"
)

// A generated kind's reference by name finds our kind of the target's
// name before Config Connector's (pkg/discokind refs.go). Read when a
// reference resolves, after every init has added its kinds.
func init() {
	discokind.OurKinds = func() []schema.GroupVersionKind {
		out := make([]schema.GroupVersionKind, 0, len(Kinds))
		for _, k := range Kinds {
			out = append(out, k.gvk)
		}
		return out
	}
}
