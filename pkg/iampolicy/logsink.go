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

package iampolicy

// Written here: memberFrom.logSinkRef, a LoggingLogSink's writer identity
// as a member, as Config Connector's IAMPolicyMember resolves it
// (pkg/controller/iam/iamclient: the sink's status.writerIdentity), for
// this project's LoggingLogSink as well as Config Connector's.

import (
	"context"
	"fmt"

	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/k8s"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// LogSinkGVKs are the LoggingLogSink kinds a logSinkRef can name, in the
// order an unset apiVersion looks for them: this project's, then Config
// Connector's.
var LogSinkGVKs = []schema.GroupVersionKind{
	{Group: "logging.gcp.hfl.dev", Version: "v1beta1", Kind: "LoggingLogSink"},
	{Group: "logging.cnrm.cloud.google.com", Version: "v1beta1", Kind: "LoggingLogSink"},
}

// LogSinkWriter is the writer identity of the LoggingLogSink named, as the
// API writes it (serviceAccount:...). A sink that is missing, not Ready,
// or has no writer identity yet is a reference error, which the
// reconciler waits on and retries.
func LogSinkWriter(ctx context.Context, reader client.Reader, defaultNamespace, apiVersion, name, namespace string) (string, error) {
	if name == "" {
		return "", fmt.Errorf("logSinkRef.name is required")
	}
	candidates := LogSinkGVKs
	if apiVersion != "" {
		gv, err := schema.ParseGroupVersion(apiVersion)
		if err != nil {
			return "", fmt.Errorf("logSinkRef.apiVersion %q: %w", apiVersion, err)
		}
		candidates = []schema.GroupVersionKind{gv.WithKind("LoggingLogSink")}
	}
	key := types.NamespacedName{Namespace: namespace, Name: name}
	if key.Namespace == "" {
		key.Namespace = defaultNamespace
	}
	var obj *unstructured.Unstructured
	for _, gvk := range candidates {
		u := &unstructured.Unstructured{}
		u.SetGroupVersionKind(gvk)
		err := reader.Get(ctx, key, u)
		if err == nil {
			obj = u
			break
		}
		if !apierrors.IsNotFound(err) && !isNoMatch(err) {
			return "", fmt.Errorf("reading referenced LoggingLogSink %s: %w", key, err)
		}
	}
	if obj == nil {
		return "", k8s.NewReferenceNotFoundError(candidates[0], key)
	}
	writer := nestedString(obj, "status", "writerIdentity")
	if !ready(obj) || writer == "" {
		return "", k8s.NewReferenceNotReadyError(obj.GroupVersionKind(), key)
	}
	return writer, nil
}
