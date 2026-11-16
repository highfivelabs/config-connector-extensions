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

package deleting

import (
	"context"
	"errors"
	"testing"

	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct/directbase"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/k8s"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// inner stands for a kind's model: it fails as a kind does when a
// reference no longer resolves, and counts its calls.
type inner struct {
	calls int
	err   error
}

func (m *inner) AdapterForObject(ctx context.Context, op *directbase.AdapterForObjectOperation) (directbase.Adapter, error) {
	m.calls++
	if m.err != nil {
		return nil, m.err
	}
	return Gone{}, nil
}

func (m *inner) AdapterForURL(ctx context.Context, url string) (directbase.Adapter, error) {
	return nil, nil
}

func TestWrap(t *testing.T) {
	notFound := errors.New("referenced Folder catalog-folder/mock not found")
	object := func(deleting bool, ext string, annotations map[string]string) *unstructured.Unstructured {
		u := &unstructured.Unstructured{Object: map[string]any{}}
		u.SetAnnotations(annotations)
		if deleting {
			now := metav1.Now()
			u.SetDeletionTimestamp(&now)
		}
		if ext != "" {
			_ = unstructured.SetNestedField(u.Object, ext, "status", "externalRef")
		}
		return u
	}
	selfLinked := func() *unstructured.Unstructured {
		u := object(true, "", nil)
		_ = unstructured.SetNestedField(u.Object, "https://www.googleapis.com/compute/v1/projects/p/regions/r/routers/x", "status", "selfLink")
		return u
	}
	abandon := map[string]string{k8s.DeletionPolicyAnnotation: k8s.DeletionPolicyAbandon}
	for _, tc := range []struct {
		name      string
		u         *unstructured.Unstructured
		innerErr  error
		wantCalls int
		wantErr   bool
		wantGone  bool
		noRecord  bool
	}{
		{"abandon resolves nothing", object(true, "folders/1/settings", abandon), notFound, 0, false, true, false},
		{"abandon of an object never made", object(true, "", abandon), notFound, 0, false, true, false},
		{"never made, reference gone: nothing to delete", object(true, "", nil), notFound, 1, false, true, false},
		{"made, reference gone: the kind's own error", object(true, "folders/1/settings", nil), notFound, 1, true, false, false},
		{"not deleting: the error stands", object(false, "", nil), notFound, 1, true, false, false},
		{"abandon annotation without a delete: the kind's adapter", object(false, "", abandon), nil, 1, false, true, false},
		{"never made, a kind that records nothing: the error stands", object(true, "", nil), notFound, 1, true, false, true},
		{"abandon, a kind that records nothing: resolves nothing", object(true, "", abandon), notFound, 0, false, true, true},
		{"made (a selfLink, no externalRef), reference gone: the error stands", selfLinked(), notFound, 1, true, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			in := &inner{err: tc.innerErr}
			m, err := Wrap(in, Options{NoRecord: tc.noRecord})
			if err != nil {
				t.Fatal(err)
			}
			a, err := m.AdapterForObject(context.Background(), &directbase.AdapterForObjectOperation{Object: tc.u})
			if in.calls != tc.wantCalls {
				t.Errorf("the kind's AdapterForObject ran %d times, want %d", in.calls, tc.wantCalls)
			}
			if (err != nil) != tc.wantErr {
				t.Fatalf("err = %v, want error %v", err, tc.wantErr)
			}
			if _, gone := a.(Gone); gone != tc.wantGone {
				t.Errorf("adapter = %T, want Gone %v", a, tc.wantGone)
			}
			if tc.wantGone {
				if found, err := a.Find(context.Background()); found || err != nil {
					t.Errorf("Find = %v, %v; want not found", found, err)
				}
			}
		})
	}
}
