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
// LoggingLogSinkSpec and LoggingLogSinkStatus functions, renamed for
// LoggingBillingAccountSink. Changed: includeChildren is not mapped (the
// kind has no such field); the status carries externalRef too. The
// destination, BigQuery options and exclusions map as upstream's do.

package logging

import (
	"strings"

	pb "cloud.google.com/go/logging/apiv2/loggingpb"
	bigqueryv1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/apis/bigquery/v1beta1"
	loggingv1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/apis/logging/v1beta1"
	pubsubv1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/apis/pubsub/v1beta1"
	storagev1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/apis/storage/v1beta1"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct"

	krm "github.com/highfivelabs/config-connector-extensions/apis/logging/v1alpha1"
)

func LoggingBillingAccountSinkSpec_FromProto(mapCtx *direct.MapContext, in *pb.LogSink) *krm.LoggingBillingAccountSinkSpec {
	if in == nil {
		return nil
	}
	out := &krm.LoggingBillingAccountSinkSpec{}
	out.Description = direct.LazyPtr(in.GetDescription())
	out.Disabled = direct.LazyPtr(in.GetDisabled())
	out.Filter = direct.LazyPtr(in.GetFilter())

	if in.GetBigqueryOptions() != nil {
		out.BigqueryOptions = &loggingv1beta1.LogSinkBigQueryOptions{
			UsePartitionedTables: in.GetBigqueryOptions().GetUsePartitionedTables(),
		}
	}

	dest := in.GetDestination()
	if dest != "" {
		if strings.HasPrefix(dest, "bigquery.googleapis.com/") {
			val := strings.TrimPrefix(dest, "bigquery.googleapis.com/")
			out.Destination.BigQueryDatasetRef = &bigqueryv1beta1.DatasetRef{External: val}
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

	if len(in.GetExclusions()) > 0 {
		out.Exclusions = make([]loggingv1beta1.LogSinkExclusions, len(in.GetExclusions()))
		for i, excl := range in.GetExclusions() {
			out.Exclusions[i] = loggingv1beta1.LogSinkExclusions{
				Name:        excl.GetName(),
				Description: direct.LazyPtr(excl.GetDescription()),
				Filter:      excl.GetFilter(),
				Disabled:    direct.LazyPtr(excl.GetDisabled()),
			}
		}
	}

	return out
}

func LoggingBillingAccountSinkSpec_ToProto(mapCtx *direct.MapContext, in *krm.LoggingBillingAccountSinkSpec) *pb.LogSink {
	if in == nil {
		return nil
	}
	out := &pb.LogSink{}
	out.Description = direct.ValueOf(in.Description)
	out.Disabled = direct.ValueOf(in.Disabled)
	out.Filter = direct.ValueOf(in.Filter)

	if in.BigqueryOptions != nil {
		out.Options = &pb.LogSink_BigqueryOptions{
			BigqueryOptions: &pb.BigQueryOptions{
				UsePartitionedTables: in.BigqueryOptions.UsePartitionedTables,
			},
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
	}

	if len(in.Exclusions) > 0 {
		out.Exclusions = make([]*pb.LogExclusion, len(in.Exclusions))
		for i, excl := range in.Exclusions {
			out.Exclusions[i] = &pb.LogExclusion{
				Name:        excl.Name,
				Description: direct.ValueOf(excl.Description),
				Filter:      excl.Filter,
				Disabled:    direct.ValueOf(excl.Disabled),
			}
		}
	}

	return out
}

func LoggingBillingAccountSinkStatus_FromProto(mapCtx *direct.MapContext, in *pb.LogSink) *krm.LoggingBillingAccountSinkStatus {
	if in == nil {
		return nil
	}
	out := &krm.LoggingBillingAccountSinkStatus{}
	out.WriterIdentity = direct.LazyPtr(in.GetWriterIdentity())
	return out
}
