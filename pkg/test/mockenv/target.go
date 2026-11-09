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

package mockenv

// The switch between the fake cloud and a real project. The same tests run
// against either; Config Connector's own harness uses the same variable
// (E2E_GCP_TARGET=mock|real in config/tests/samples/create/harness.go) and
// GCP_PROJECT_ID for the real project (pkg/test/gcp). hack/live/run.sh
// (task test-live) is the only thing that sets the real target; it also
// creates what a run shares (a Cloud SQL instance) and sweeps afterwards.
// docs/live-runs.md.

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	sqladmin "google.golang.org/api/sqladmin/v1beta4"
)

const (
	// TargetMock is mockgcp, in process: the default, what task check runs.
	TargetMock = "mock"
	// TargetReal is the Google Cloud project in GCP_PROJECT_ID, with
	// Application Default Credentials.
	TargetReal = "real"
)

// Target is E2E_GCP_TARGET: "mock" (or unset) or "real".
func Target() string {
	switch t := os.Getenv("E2E_GCP_TARGET"); t {
	case "", TargetMock:
		return TargetMock
	default:
		return t
	}
}

// Live reports whether the tests run against a real project.
func Live() bool { return Target() == TargetReal }

// Project is the project the tests use: mockgcp's "mock-project", or
// GCP_PROJECT_ID against a real project.
var Project = projectFromEnv()

func projectFromEnv() string {
	if Live() {
		return os.Getenv("GCP_PROJECT_ID")
	}
	return "mock-project"
}

// RunID is E2E_RUN_ID against a real project (hack/live/run.sh sets it,
// for example cce-11091530-a3f2): every name a test makes starts with it,
// and every resource that takes labels carries it, so the run's leftovers
// can be found and swept.
var RunID = os.Getenv("E2E_RUN_ID")

// Name is the cloud-unique name for a test's resource: base in the fake
// cloud, RunID-base in a real project.
func Name(base string) string {
	if Live() {
		return RunID + "-" + base
	}
	return base
}

// Labels are the labels every object a test makes carries against a real
// project (Config Connector copies metadata.labels to the resource's
// labels), and none in the fake cloud.
func Labels() map[string]string {
	if !Live() {
		return nil
	}
	return map[string]string{"cce-e2e": "true", "cce-run": RunID}
}

// Timeout bounds every wait: real operations (a Cloud SQL backup, a user
// on an instance busy with another operation) take minutes.
func Timeout() time.Duration {
	if Live() {
		return 15 * time.Minute
	}
	return 60 * time.Second
}

// checkTarget fails the test unless the target is known and, for a real
// project, the run's project and ID are set.
func checkTarget(t *testing.T) {
	t.Helper()
	switch Target() {
	case TargetMock:
	case TargetReal:
		if Project == "" {
			t.Fatal("E2E_GCP_TARGET=real needs GCP_PROJECT_ID; run `task test-live PROJECT=...`")
		}
		if RunID == "" {
			t.Fatal("E2E_GCP_TARGET=real needs E2E_RUN_ID (the run's name prefix and label); run `task test-live PROJECT=...`")
		}
		t.Logf("target: real project %s, run %s", Project, RunID)
	default:
		t.Fatalf("E2E_GCP_TARGET=%q not supported (mock or real)", Target())
	}
}

// ServiceAccount is a Google service account email for tests that need an
// IAM principal: a made-up one in the fake cloud, the run's own (from
// E2E_SERVICE_ACCOUNT) in a real project, where it must exist.
func ServiceAccount(t *testing.T) string {
	t.Helper()
	if !Live() {
		return "app-runtime@" + Project + ".iam.gserviceaccount.com"
	}
	sa := os.Getenv("E2E_SERVICE_ACCOUNT")
	if sa == "" {
		t.Fatal("E2E_SERVICE_ACCOUNT is not set; hack/live/run.sh sets it to the run's service account")
	}
	return sa
}

// LoggingFolder is the folder whose Log Router settings LoggingSettings'
// test changes, as folders/{id}: a made-up one in the fake cloud, and
// against a real project the folder in E2E_LOGGING_FOLDER, which must be a
// test folder the live-test service account administers logging on. The
// test puts the folder's settings back as it found them. A live run never
// changes an organization's settings.
func LoggingFolder(t *testing.T) string {
	t.Helper()
	if !Live() {
		return "folders/123456"
	}
	f := os.Getenv("E2E_LOGGING_FOLDER")
	if f == "" {
		t.Fatal("E2E_LOGGING_FOLDER is not set; a live LoggingSettings test needs a test folder (docs/live-runs.md)")
	}
	return f
}

// LoggingKMSKey is a Cloud KMS key for the Log Router of LoggingFolder: a
// made-up one in the fake cloud, which does not check keys, and against a
// real project the key in E2E_LOGGING_KMS_KEY, which the folder's Log
// Router service account must be able to use.
func LoggingKMSKey(t *testing.T) string {
	t.Helper()
	if !Live() {
		return "projects/" + Project + "/locations/europe-west1/keyRings/logs/cryptoKeys/router"
	}
	k := os.Getenv("E2E_LOGGING_KMS_KEY")
	if k == "" {
		t.Fatal("E2E_LOGGING_KMS_KEY is not set; a live LoggingSettings test needs a key the folder's Log Router can use (docs/live-runs.md)")
	}
	return k
}

// SQLInstance is a Cloud SQL Postgres instance for the SQL kinds, which
// are sub-resources of one. In the fake cloud it is created here, once per
// Env; against a real project it is the run's shared instance
// (E2E_SQL_INSTANCE), which hack/live/run.sh creates once per run because
// a real one takes minutes to make.
func (e *Env) SQLInstance() string {
	e.T.Helper()
	if Live() {
		name := os.Getenv("E2E_SQL_INSTANCE")
		if name == "" {
			e.T.Fatal("E2E_SQL_INSTANCE is not set; hack/live/run.sh creates the run's Cloud SQL instance and sets it")
		}
		return name
	}
	const name = "example"
	e.mockSQL.Do(func() { e.mockSQLErr = createMockSQLInstance(e.Ctx, e, name) })
	if e.mockSQLErr != nil {
		e.T.Fatal(e.mockSQLErr)
	}
	return name
}

func createMockSQLInstance(ctx context.Context, e *Env, name string) error {
	svc, err := sqladmin.NewService(ctx, e.GCPOptions()...)
	if err != nil {
		return err
	}
	op, err := svc.Instances.Insert(Project, &sqladmin.DatabaseInstance{
		Name:            name,
		DatabaseVersion: "POSTGRES_16",
		Region:          "us-central1",
		Settings:        &sqladmin.Settings{Tier: "db-custom-1-3840"},
	}).Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("creating the instance in the fake cloud: %w", err)
	}
	for i := 0; i < 100 && op.Status != "DONE"; i++ {
		time.Sleep(100 * time.Millisecond)
		if op, err = svc.Operations.Get(Project, op.Name).Context(ctx).Do(); err != nil {
			return err
		}
	}
	return nil
}
