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

package main

import "testing"

// TestIsLRO: an API's own copy of google.longrunning.Operation is an
// operation whatever its schema is called (API Gateway's
// ApigatewayOperation, which the generator once missed, so the kinds'
// deletes never waited); a schema called ...Operation without its fields
// is not.
func TestIsLRO(t *testing.T) {
	op := func(done string) *Schema {
		return &Schema{Properties: map[string]*Schema{
			"done": {Type: done}, "name": {Type: "string"}, "error": {Ref: "Status"},
			"metadata": {Type: "object"}, "response": {Type: "object"},
		}}
	}
	d := &Discovery{Schemas: map[string]*Schema{
		"ApigatewayOperation": op("boolean"),
		"OddOperation":        op("string"),
		"BulkOperation":       {Properties: map[string]*Schema{"name": {Type: "string"}}},
		"Gateway":             op("boolean"),
	}}
	for ref, want := range map[string]bool{
		"Operation": true, "GoogleLongrunningOperation": true, "ApigatewayOperation": true,
		"OddOperation": false, "BulkOperation": false, "Gateway": false, "Missing": false,
	} {
		if got := isLRO(d, &Method{Response: &Schema{Ref: ref}}); got != want {
			t.Errorf("isLRO(%s) = %v, want %v", ref, got, want)
		}
	}
	if isLRO(d, nil) || isLRO(d, &Method{}) {
		t.Error("a method without a response is not an operation")
	}
}
