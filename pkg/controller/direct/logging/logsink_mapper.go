// Copyright 2026 Google LLC
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

// SPDX-License-Identifier: Apache-2.0

// Modified by High Five Labs for config-connector-extensions: copied from Config Connector
// pkg/controller/direct/logging/mapper.go (UPSTREAM.md), its
// LoggingLogSinkSpec and LoggingLogSinkStatus functions, for our
// LoggingLogSink. Changed: they map to the Logging REST client's LogSink
// (google.golang.org/api/logging/v2), whose LogSink has interceptChildren
// (the protobuf's, at the pinned cloud.google.com/go/logging, has not);
// interceptChildren is mapped; a destination
// logging.googleapis.com/projects/{project} is a Cloud project
// (destination.projectRef), told apart from a log bucket by its length.
// The other destinations, BigQuery options and exclusions map as
// upstream's do.

package logging

import (
	"strings"

	bigqueryv1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/apis/bigquery/v1beta1"
	loggingv1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/apis/logging/v1beta1"
	pubsubv1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/apis/pubsub/v1beta1"
	refs "github.com/GoogleCloudPlatform/k8s-config-connector/apis/refs"
	storagev1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/apis/storage/v1beta1"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct"
	api "google.golang.org/api/logging/v2"

	krm "github.com/highfivelabs/config-connector-extensions/apis/logging/v1beta1"
)

// projectDestination is the project ID of a destination
// logging.googleapis.com/projects/{project} (a Cloud project, not one of
// its log buckets), or "".
func projectDestination(dest string) string {
	rest, ok := strings.CutPrefix(dest, "logging.googleapis.com/projects/")
	if !ok || rest == "" || strings.Contains(rest, "/") {
		return ""
	}
	return rest
}

func LoggingLogSinkSpec_FromAPI(mapCtx *direct.MapContext, in *api.LogSink) *krm.LoggingLogSinkSpec {
	if in == nil {
		return nil
	}
	out := &krm.LoggingLogSinkSpec{}
	out.Description = direct.LazyPtr(in.Description)
	out.Disabled = direct.LazyPtr(in.Disabled)
	out.Filter = direct.LazyPtr(in.Filter)
	out.IncludeChildren = direct.LazyPtr(in.IncludeChildren)
	out.InterceptChildren = direct.LazyPtr(in.InterceptChildren)

	if in.BigqueryOptions != nil {
		out.BigqueryOptions = &loggingv1beta1.LogSinkBigQueryOptions{
			UsePartitionedTables: in.BigqueryOptions.UsePartitionedTables,
		}
	}

	dest := in.Destination
	if dest != "" {
		if strings.HasPrefix(dest, "bigquery.googleapis.com/") {
			val := strings.TrimPrefix(dest, "bigquery.googleapis.com/")
			out.Destination.BigQueryDatasetRef = &bigqueryv1beta1.DatasetRef{External: val}
		} else if p := projectDestination(dest); p != "" {
			out.Destination.ProjectRef = &refs.ProjectRef{External: "projects/" + p}
		} else if strings.HasPrefix(dest, "logging.googleapis.com/") {
			val := strings.TrimPrefix(dest, "logging.googleapis.com/")
			out.Destination.LoggingLogBucketRef = &loggingv1beta1.LoggingLogBucketRef{External: val}
		} else if strings.HasPrefix(dest, "pubsub.googleapis.com/") {
			val := strings.TrimPrefix(dest, "pubsub.googleapis.com/")
			out.Destination.PubSubTopicRef = &pubsubv1beta1.PubSubTopicRef{External: val}
		} else if strings.HasPrefix(dest, "storage.googleapis.com/") {
			val := strings.TrimPrefix(dest, "storage.googleapis.com/")
			out.Destination.StorageBucketRef = &storagev1beta1.StorageBucketRef{External: val}
		}
	}

	if len(in.Exclusions) > 0 {
		out.Exclusions = make([]loggingv1beta1.LogSinkExclusions, len(in.Exclusions))
		for i, excl := range in.Exclusions {
			if excl == nil {
				continue
			}
			out.Exclusions[i] = loggingv1beta1.LogSinkExclusions{
				Name:        excl.Name,
				Description: direct.LazyPtr(excl.Description),
				Filter:      excl.Filter,
				Disabled:    direct.LazyPtr(excl.Disabled),
			}
		}
	}

	return out
}

// LoggingLogSinkSpec_ToAPI maps the spec to the REST client's LogSink. A
// project destination is the project's ID: the controller resolves
// destination.projectRef to it first.
func LoggingLogSinkSpec_ToAPI(mapCtx *direct.MapContext, in *krm.LoggingLogSinkSpec) *api.LogSink {
	if in == nil {
		return nil
	}
	out := &api.LogSink{}
	out.Description = direct.ValueOf(in.Description)
	out.Disabled = direct.ValueOf(in.Disabled)
	out.Filter = direct.ValueOf(in.Filter)
	out.IncludeChildren = direct.ValueOf(in.IncludeChildren)
	out.InterceptChildren = direct.ValueOf(in.InterceptChildren)

	if in.BigqueryOptions != nil {
		out.BigqueryOptions = &api.BigQueryOptions{
			UsePartitionedTables: in.BigqueryOptions.UsePartitionedTables,
		}
	}

	if in.Destination.BigQueryDatasetRef != nil && in.Destination.BigQueryDatasetRef.External != "" {
		ext := in.Destination.BigQueryDatasetRef.External
		if !strings.HasPrefix(ext, "bigquery.googleapis.com/") {
			ext = "bigquery.googleapis.com/" + ext
		}
		out.Destination = ext
	} else if in.Destination.LoggingLogBucketRef != nil && in.Destination.LoggingLogBucketRef.External != "" {
		ext := in.Destination.LoggingLogBucketRef.External
		if !strings.HasPrefix(ext, "logging.googleapis.com/") {
			ext = "logging.googleapis.com/" + ext
		}
		out.Destination = ext
	} else if in.Destination.PubSubTopicRef != nil && in.Destination.PubSubTopicRef.External != "" {
		ext := in.Destination.PubSubTopicRef.External
		if !strings.HasPrefix(ext, "pubsub.googleapis.com/") {
			ext = "pubsub.googleapis.com/" + ext
		}
		out.Destination = ext
	} else if in.Destination.StorageBucketRef != nil && in.Destination.StorageBucketRef.External != "" {
		ext := in.Destination.StorageBucketRef.External
		if strings.Contains(ext, "/buckets/") {
			parts := strings.Split(ext, "/buckets/")
			ext = parts[len(parts)-1]
		}
		if !strings.HasPrefix(ext, "storage.googleapis.com/") {
			ext = "storage.googleapis.com/" + ext
		}
		out.Destination = ext
	} else if in.Destination.ProjectRef != nil && in.Destination.ProjectRef.External != "" {
		out.Destination = "logging.googleapis.com/projects/" + strings.TrimPrefix(in.Destination.ProjectRef.External, "projects/")
	}

	if len(in.Exclusions) > 0 {
		out.Exclusions = make([]*api.LogExclusion, len(in.Exclusions))
		for i, excl := range in.Exclusions {
			out.Exclusions[i] = &api.LogExclusion{
				Name:        excl.Name,
				Description: direct.ValueOf(excl.Description),
				Filter:      excl.Filter,
				Disabled:    direct.ValueOf(excl.Disabled),
			}
		}
	}

	return out
}

func LoggingLogSinkStatus_FromAPI(mapCtx *direct.MapContext, in *api.LogSink) *krm.LoggingLogSinkStatus {
	if in == nil {
		return nil
	}
	out := &krm.LoggingLogSinkStatus{}
	out.WriterIdentity = direct.LazyPtr(in.WriterIdentity)
	return out
}
