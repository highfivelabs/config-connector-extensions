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

package iam

// Written here: the plumbing both IAM binding kinds share. A resourceRef
// is resolved to a resource type and its full name (pkg/iampolicy), and the
// policy is read and written over HTTP with the manager's credentials.

import (
	"context"
	"fmt"
	"net/http"

	"cloud.google.com/go/iam/apiv1/iampb"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/config"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct/directbase"
	"google.golang.org/api/option"
	htransport "google.golang.org/api/transport/http"
	"google.golang.org/protobuf/proto"

	krm "github.com/highfivelabs/config-connector-extensions/apis/iam/v1alpha1"
	"github.com/highfivelabs/config-connector-extensions/pkg/iampolicy"
)

// policyClient builds the HTTP client for policy calls from the manager's
// configuration: the fake cloud's client in tests, Application Default
// Credentials otherwise.
func policyClient(ctx context.Context, cfg config.ControllerConfig) (*iampolicy.Client, error) {
	opts, err := cfg.RESTClientOptions()
	if err != nil {
		return nil, err
	}
	opts = append(opts, option.WithScopes("https://www.googleapis.com/auth/cloud-platform"))
	var h *http.Client
	if h, _, err = htransport.NewClient(ctx, opts...); err != nil {
		return nil, fmt.Errorf("building the HTTP client: %w", err)
	}
	return &iampolicy.Client{HTTP: h}, nil
}

// policyRef and conditionOf take IAMMember's v1alpha1 types; IAMBinding's
// v1beta1 ones have their own (binding_controller.go).
func policyRef(r krm.IAMResourceReference) iampolicy.Ref {
	return iampolicy.Ref{APIVersion: r.APIVersion, Kind: r.Kind, Name: r.Name, Namespace: r.Namespace, External: r.External}
}

func conditionOf(c *krm.IAMCondition) *iampolicy.Condition {
	if c == nil {
		return nil
	}
	return &iampolicy.Condition{Title: c.Title, Description: c.Description, Expression: c.Expression}
}

// resolveResource finds the resource a binding is on. While the binding is
// being deleted, a resource that is gone, or whose object is being deleted
// with it, leaves nothing to remove: gone is true and the error nil, and
// Delete makes no call.
func resolveResource(ctx context.Context, op *directbase.AdapterForObjectOperation, ref iampolicy.Ref) (t *iampolicy.ResourceType, name string, gone bool, err error) {
	t, name, err = iampolicy.Resolve(ctx, op.Reader, op.GetUnstructured().GetNamespace(), ref)
	if err != nil && op.IsDeleting() {
		if _, ok := err.(*iampolicy.ErrGone); ok {
			return nil, "", true, nil
		}
	}
	return t, name, false, err
}

func clonePolicy(p *iampb.Policy) *iampb.Policy { return proto.Clone(p).(*iampb.Policy) }
