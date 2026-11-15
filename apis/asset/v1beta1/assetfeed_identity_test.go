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

import (
	"context"
	"fmt"
	"strings"
	"testing"

	upstream "github.com/GoogleCloudPlatform/k8s-config-connector/apis/asset/v1beta1"
	refs "github.com/GoogleCloudPlatform/k8s-config-connector/apis/refs/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// The bug this kind fixes (catalog PR #215): Google Cloud names a project's
// feed projects/<number>/feeds/<id>, and Config Connector's AssetFeed
// compares that number with the project ID its projectRef resolves to on
// every reconcile after the create.
func TestUpstreamFeedIdentityRefusesItsOwnFeed(t *testing.T) {
	obj := &upstream.AssetFeed{
		ObjectMeta: metav1.ObjectMeta{Namespace: "ns", Name: "feed"},
		Spec: upstream.AssetFeedSpec{
			Parent: upstream.AssetFeedParent{ProjectRef: &refs.ProjectRef{External: "catalog-mock-prj"}},
		},
		Status: upstream.AssetFeedStatus{ExternalRef: ptr("projects/886048300/feeds/feed")},
	}
	_, err := upstream.NewFeedIdentity(context.Background(), nil, obj)
	if err == nil || !strings.Contains(err.Error(), "spec.projectRef changed, expect 886048300, got catalog-mock-prj") {
		t.Fatalf("upstream NewFeedIdentity: err = %v, want the projectRef-changed refusal (if upstream fixed it, retire this kind: docs/design.md section 3.4)", err)
	}
}

func TestFeedIdentityProjectNumberAndID(t *testing.T) {
	numbers := map[string]string{"catalog-mock-prj": "886048300", "other-prj": "111"}
	calls := 0
	resolve := func(ctx context.Context, id string) (string, error) {
		calls++
		n, ok := numbers[id]
		if !ok {
			return "", fmt.Errorf("project %q not found", id)
		}
		return n, nil
	}
	feed := func(project, external string) *AssetFeed {
		o := &AssetFeed{
			ObjectMeta: metav1.ObjectMeta{Namespace: "ns", Name: "feed"},
			Spec:       AssetFeedSpec{AssetFeedParent: AssetFeedParent{ProjectRef: &refs.ProjectRef{External: project}}},
		}
		if external != "" {
			o.Status.ExternalRef = ptr(external)
		}
		return o
	}
	for _, tc := range []struct {
		name, project, external string
		want, wantErr           string
		wantCalls               int
	}{
		{name: "before the create", project: "catalog-mock-prj", want: "projects/catalog-mock-prj/feeds/feed"},
		{name: "the ID and its number", project: "catalog-mock-prj", external: "projects/886048300/feeds/feed", want: "projects/886048300/feeds/feed", wantCalls: 1},
		{name: "projects/ID and its number", project: "projects/catalog-mock-prj", external: "projects/886048300/feeds/feed", want: "projects/886048300/feeds/feed", wantCalls: 1},
		{name: "the number itself", project: "886048300", external: "projects/886048300/feeds/feed", want: "projects/886048300/feeds/feed"},
		{name: "another project", project: "other-prj", external: "projects/886048300/feeds/feed", wantErr: "spec.projectRef changed, expect 886048300, got other-prj", wantCalls: 1},
		{name: "another number", project: "111", external: "projects/886048300/feeds/feed", wantErr: "spec.projectRef changed"},
		{name: "an unknown project", project: "gone-prj", external: "projects/886048300/feeds/feed", wantErr: `reading the number of project "gone-prj"`, wantCalls: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls = 0
			id, err := NewFeedIdentity(context.Background(), nil, feed(tc.project, tc.external), resolve)
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Errorf("err = %v, want %q", err, tc.wantErr)
				}
			} else if err != nil {
				t.Errorf("err = %v", err)
			} else if id.String() != tc.want {
				t.Errorf("identity = %q, want %q", id.String(), tc.want)
			}
			if calls != tc.wantCalls {
				t.Errorf("projects.get calls = %d, want %d", calls, tc.wantCalls)
			}
		})
	}
}

func TestFeedIdentityFolderAndOrganization(t *testing.T) {
	folder := &AssetFeed{
		ObjectMeta: metav1.ObjectMeta{Namespace: "ns", Name: "feed"},
		Spec:       AssetFeedSpec{AssetFeedParent: AssetFeedParent{FolderRef: &refs.FolderRefDeprecated{External: "folders/123"}}},
		Status:     AssetFeedStatus{ExternalRef: ptr("folders/123/feeds/feed")},
	}
	if id, err := NewFeedIdentity(context.Background(), nil, folder, nil); err != nil || id.String() != "folders/123/feeds/feed" {
		t.Errorf("folder: %v, %v", id, err)
	}
	folder.Status.ExternalRef = ptr("organizations/9/feeds/feed")
	if _, err := NewFeedIdentity(context.Background(), nil, folder, nil); err == nil {
		t.Errorf("a folder's feed recorded under an organization: no error")
	}
	if id, err := NewFeedIdentityFromExternal("projects/886048300/feeds/feed"); err != nil || id.String() != "projects/886048300/feeds/feed" {
		t.Errorf("from external: %v, %v", id, err)
	}
}

func ptr[T any](v T) *T { return &v }
