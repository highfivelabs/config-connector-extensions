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

package cloudfunctions2_test

// What a CloudFunctions2Function needs live: its source, a zip of one
// Python HTTP function in a Cloud Storage bucket (the fixtures'
// ${NAME}-source/function.zip), for the test's function and its abandoned
// one. The fake does not build, so nothing is made there. When the test
// ends, the buckets go, and what Cloud Functions made for the run's
// functions beside them: the copies of their source in the project's
// gcf-v2-sources-* bucket and their images in the gcf-artifacts
// repository.

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct"

	"github.com/highfivelabs/config-connector-extensions/pkg/discokind"
	"github.com/highfivelabs/config-connector-extensions/pkg/discokind/kindtest"
	"github.com/highfivelabs/config-connector-extensions/pkg/test/mockenv"
)

const region = "us-central1"

func init() { kindtest.RegisterSetup("CloudFunctions2Function", makeSources) }

const mainPy = `def main(request):
    return "made by a config-connector-extensions test"
`

func makeSources(t *testing.T, env *mockenv.Env) {
	if !mockenv.Live() {
		return
	}
	gcs := &discokind.API{HTTP: env.HTTP, BaseURL: "https://storage.googleapis.com/storage/v1/"}
	labels := map[string]any{}
	for k, v := range mockenv.Labels() {
		labels[k] = v
	}
	var archive bytes.Buffer
	zw := zip.NewWriter(&archive)
	w, err := zw.Create("main.py")
	if err == nil {
		_, err = io.WriteString(w, mainPy)
	}
	if err == nil {
		err = zw.Close()
	}
	if err != nil {
		t.Fatal(err)
	}
	var functions []string
	for _, base := range []string{"cloudfunctions2function", "cloudfunctions2function-kept"} {
		fn := mockenv.Name(base)
		functions = append(functions, fn)
		bucket := fn + "-source"
		if _, err := gcs.Do(env.Ctx, http.MethodPost, "b", map[string]string{"project": mockenv.Project}, map[string]any{
			"name": bucket, "location": region, "labels": labels,
			"iamConfiguration": map[string]any{"uniformBucketLevelAccess": map[string]any{"enabled": true}},
		}); err != nil {
			t.Fatalf("creating the bucket %s: %v", bucket, err)
		}
		t.Logf("created the bucket %s", bucket)
		t.Cleanup(func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
			defer cancel()
			if _, err := gcs.Do(ctx, http.MethodDelete, "b/"+bucket+"/o/function.zip", nil, nil); err != nil && !direct.IsNotFound(err) {
				t.Errorf("cleanup: deleting gs://%s/function.zip: %v", bucket, err)
			}
			if _, err := gcs.Do(ctx, http.MethodDelete, "b/"+bucket, nil, nil); err != nil && !direct.IsNotFound(err) {
				t.Errorf("cleanup: deleting the bucket %s: %v", bucket, err)
			}
		})
		u := "https://storage.googleapis.com/upload/storage/v1/b/" + bucket + "/o?uploadType=media&name=function.zip"
		req, err := http.NewRequestWithContext(env.Ctx, http.MethodPost, u, bytes.NewReader(archive.Bytes()))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Content-Type", "application/zip")
		resp, err := env.HTTP.Do(req)
		if err == nil {
			b, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				err = fmt.Errorf("%s: %s", resp.Status, b)
			}
		}
		if err != nil {
			t.Fatalf("uploading gs://%s/function.zip: %v", bucket, err)
		}
	}
	// Registered after the buckets' cleanups, so it runs before them, and
	// after the kindtest's own (the functions are gone by then).
	t.Cleanup(func() { removeBuildLeftovers(t, env, functions) })
}

// removeBuildLeftovers deletes what Cloud Functions made for the
// functions beside them: their source copies in gcf-v2-sources-* and their
// images in the gcf-artifacts repository.
func removeBuildLeftovers(t *testing.T, env *mockenv.Env, functions []string) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	gcs := &discokind.API{HTTP: env.HTTP, BaseURL: "https://storage.googleapis.com/storage/v1/"}
	buckets, err := gcs.Do(ctx, http.MethodGet, "b", map[string]string{"project": mockenv.Project, "prefix": "gcf-v2-sources-"}, nil)
	if err != nil {
		t.Errorf("cleanup: listing the gcf-v2-sources buckets: %v", err)
	}
	items, _ := buckets["items"].([]any)
	for _, b := range items {
		bucket, _ := b.(map[string]any)["name"].(string)
		for _, fn := range functions {
			objs, err := gcs.Do(ctx, http.MethodGet, "b/"+bucket+"/o", map[string]string{"prefix": fn + "/"}, nil)
			if err != nil {
				t.Errorf("cleanup: listing gs://%s/%s/: %v", bucket, fn, err)
				continue
			}
			list, _ := objs["items"].([]any)
			for _, o := range list {
				name, _ := o.(map[string]any)["name"].(string)
				if _, err := gcs.Do(ctx, http.MethodDelete, "b/"+bucket+"/o/"+url.PathEscape(name), nil, nil); err != nil && !direct.IsNotFound(err) {
					t.Errorf("cleanup: deleting gs://%s/%s: %v", bucket, name, err)
				} else {
					t.Logf("deleted gs://%s/%s", bucket, name)
				}
			}
		}
	}
	ar := &discokind.API{HTTP: env.HTTP, BaseURL: "https://artifactregistry.googleapis.com/v1/"}
	repo := "projects/" + mockenv.Project + "/locations/" + region + "/repositories/gcf-artifacts"
	pkgs, err := ar.Do(ctx, http.MethodGet, repo+"/packages", map[string]string{"pageSize": "1000"}, nil)
	if direct.IsNotFound(err) {
		return
	}
	if err != nil {
		t.Errorf("cleanup: listing %s's packages: %v", repo, err)
		return
	}
	list, _ := pkgs["packages"].([]any)
	for _, p := range list {
		name, _ := p.(map[string]any)["name"].(string)
		for _, fn := range functions {
			// An image is named after the project, region and function,
			// each dash doubled: ...__us--central1__cce--1114...
			if !strings.HasSuffix(name, "__"+strings.ReplaceAll(fn, "-", "--")) && !strings.HasSuffix(name, "/"+fn) {
				continue
			}
			op, err := ar.Do(ctx, http.MethodDelete, name, nil, nil)
			if err == nil {
				_, err = ar.Wait(ctx, op)
			}
			if err != nil && !direct.IsNotFound(err) {
				t.Errorf("cleanup: deleting the image package %s: %v", name, err)
			} else {
				t.Logf("deleted the image package %s", name)
			}
		}
	}
}
