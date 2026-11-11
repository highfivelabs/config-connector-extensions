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

package mockenv_test

// The fake cloud's Service Usage enables any public Google API (mockgcp
// patch 0034), as a real project does: charts that enable services, such
// as the catalog's project wrapper, need iam, logging, monitoring and
// artifactregistry at least. A made-up name is still refused.

import (
	"testing"
	"time"

	serviceusage "google.golang.org/api/serviceusage/v1"

	"github.com/highfivelabs/config-connector-extensions/pkg/test/mockenv"
)

func TestFakeServiceUsageEnablesPublicServices(t *testing.T) {
	if mockenv.Live() {
		t.Skip("a test of the fake cloud only; enabling APIs in a real project is never done by a test")
	}
	env := mockenv.New(t)
	su, err := serviceusage.NewService(env.Ctx, env.GCPOptions()...)
	if err != nil {
		t.Fatal(err)
	}
	for _, svc := range []string{
		"iam.googleapis.com", "logging.googleapis.com", "monitoring.googleapis.com",
		"artifactregistry.googleapis.com", "secretmanager.googleapis.com", "run.googleapis.com",
		"sqladmin.googleapis.com", "dns.googleapis.com", "cloudbilling.googleapis.com",
	} {
		name := "projects/" + mockenv.Project + "/services/" + svc
		op, err := su.Services.Enable(name, &serviceusage.EnableServiceRequest{}).Context(env.Ctx).Do()
		if err != nil {
			t.Errorf("enabling %s: %v", svc, err)
			continue
		}
		for i := 0; i < 50 && !op.Done; i++ {
			time.Sleep(100 * time.Millisecond)
			if op, err = su.Operations.Get(op.Name).Context(env.Ctx).Do(); err != nil {
				t.Fatalf("operation for %s: %v", svc, err)
			}
		}
		got, err := su.Services.Get(name).Context(env.Ctx).Do()
		if err != nil {
			t.Errorf("reading %s: %v", svc, err)
			continue
		}
		if got.State != "ENABLED" {
			t.Errorf("%s is %s after enable", svc, got.State)
		}
	}
	if _, err := su.Services.Enable("projects/"+mockenv.Project+"/services/no-such-api.googleapis.com", &serviceusage.EnableServiceRequest{}).Context(env.Ctx).Do(); err == nil {
		t.Error("enabling a service that does not exist succeeded")
	}
}
