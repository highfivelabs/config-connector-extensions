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
// pkg/controller/direct/firestore/mapper.generated.go (UPSTREAM.md), the backup
// schedule's functions only. Changed: our KRM types for the kind (stable,
// firestore.gcp.hfl.dev/v1beta1, docs/design.md decision 26), upstream's for
// the recurrence types; the v1alpha1 suffix dropped from the kind's functions.

package firestore

import (
	pb "cloud.google.com/go/firestore/apiv1/admin/adminpb"
	upstream "github.com/GoogleCloudPlatform/k8s-config-connector/apis/firestore/v1alpha1"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct"
	dayofweekpb "google.golang.org/genproto/googleapis/type/dayofweek"

	krm "github.com/highfivelabs/config-connector-extensions/apis/firestore/v1beta1"
)

func DailyRecurrence_FromProto(mapCtx *direct.MapContext, in *pb.DailyRecurrence) *upstream.DailyRecurrence {
	if in == nil {
		return nil
	}
	out := &upstream.DailyRecurrence{}
	return out
}

func DailyRecurrence_ToProto(mapCtx *direct.MapContext, in *upstream.DailyRecurrence) *pb.DailyRecurrence {
	if in == nil {
		return nil
	}
	out := &pb.DailyRecurrence{}
	return out
}

func FirestoreBackupScheduleObservedState_FromProto(mapCtx *direct.MapContext, in *pb.BackupSchedule) *krm.FirestoreBackupScheduleObservedState {
	if in == nil {
		return nil
	}
	out := &krm.FirestoreBackupScheduleObservedState{}
	out.Name = direct.LazyPtr(in.GetName())
	out.CreateTime = direct.StringTimestamp_FromProto(mapCtx, in.GetCreateTime())
	out.UpdateTime = direct.StringTimestamp_FromProto(mapCtx, in.GetUpdateTime())
	return out
}

func FirestoreBackupScheduleSpec_FromProto(mapCtx *direct.MapContext, in *pb.BackupSchedule) *krm.FirestoreBackupScheduleSpec {
	if in == nil {
		return nil
	}
	out := &krm.FirestoreBackupScheduleSpec{}
	out.Retention = direct.StringDuration_FromProto(mapCtx, in.GetRetention())
	out.DailyRecurrence = DailyRecurrence_FromProto(mapCtx, in.GetDailyRecurrence())
	out.WeeklyRecurrence = WeeklyRecurrence_FromProto(mapCtx, in.GetWeeklyRecurrence())
	return out
}

func FirestoreBackupScheduleSpec_ToProto(mapCtx *direct.MapContext, in *krm.FirestoreBackupScheduleSpec) *pb.BackupSchedule {
	if in == nil {
		return nil
	}
	out := &pb.BackupSchedule{}
	out.Retention = direct.StringDuration_ToProto(mapCtx, in.Retention)
	if oneof := DailyRecurrence_ToProto(mapCtx, in.DailyRecurrence); oneof != nil {
		out.Recurrence = &pb.BackupSchedule_DailyRecurrence{DailyRecurrence: oneof}
	}
	if oneof := WeeklyRecurrence_ToProto(mapCtx, in.WeeklyRecurrence); oneof != nil {
		out.Recurrence = &pb.BackupSchedule_WeeklyRecurrence{WeeklyRecurrence: oneof}
	}
	return out
}

func WeeklyRecurrence_FromProto(mapCtx *direct.MapContext, in *pb.WeeklyRecurrence) *upstream.WeeklyRecurrence {
	if in == nil {
		return nil
	}
	out := &upstream.WeeklyRecurrence{}
	out.Day = direct.Enum_FromProto(mapCtx, in.GetDay())
	return out
}

func WeeklyRecurrence_ToProto(mapCtx *direct.MapContext, in *upstream.WeeklyRecurrence) *pb.WeeklyRecurrence {
	if in == nil {
		return nil
	}
	out := &pb.WeeklyRecurrence{}
	out.Day = direct.Enum_ToProto[dayofweekpb.DayOfWeek](mapCtx, in.Day)
	return out
}
