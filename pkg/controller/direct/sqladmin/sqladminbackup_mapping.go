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

// Modified by High Five Labs for gaps-gcp. The mapping of Config Connector's
// open pull request #12553 (head 9611a6de, pkg/controller/direct/sqladmin/
// mapper.generated.go and mapper.go; UPSTREAM.md), field for field, written
// against the sqladmin v1beta4 REST types instead of the cloud.google.com/go/
// sql protos (sqladminbackup_controller.go says why). Fields the PR marks
// MISSING (kind, selfLink, maxChargeableBytes) stay unmapped.

package sqladmin

import (
	"slices"

	kmsv1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/apis/kms/v1beta1"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct"
	api "google.golang.org/api/sqladmin/v1beta4"

	krm "github.com/highfivelabs/gaps-gcp/apis/sqladmin/v1alpha1"
)

// SQLAdminBackupSpec_ToAPI is the body of backupRuns.insert. The instance is
// in the request path, so it is not repeated in the body.
func SQLAdminBackupSpec_ToAPI(in *krm.SQLAdminBackupSpec) *api.BackupRun {
	if in == nil {
		return nil
	}
	out := &api.BackupRun{}
	out.Description = direct.ValueOf(in.Description)
	out.Location = direct.ValueOf(in.Location)
	out.BackupKind = direct.ValueOf(in.BackupKind)
	if in.DiskEncryptionConfiguration != nil {
		out.DiskEncryptionConfiguration = &api.DiskEncryptionConfiguration{}
		if in.DiskEncryptionConfiguration.KMSKeyRef != nil {
			out.DiskEncryptionConfiguration.KmsKeyName = in.DiskEncryptionConfiguration.KMSKeyRef.External
		}
	}
	return out
}

// SQLAdminBackupSpec_FromAPI is the spec of a backup run, for Export.
func SQLAdminBackupSpec_FromAPI(in *api.BackupRun) *krm.SQLAdminBackupSpec {
	if in == nil {
		return nil
	}
	out := &krm.SQLAdminBackupSpec{}
	out.Description = direct.LazyPtr(in.Description)
	out.Location = direct.LazyPtr(in.Location)
	out.BackupKind = direct.LazyPtr(in.BackupKind)
	if in.DiskEncryptionConfiguration != nil {
		out.DiskEncryptionConfiguration = &krm.DiskEncryptionConfiguration{}
		if in.DiskEncryptionConfiguration.KmsKeyName != "" {
			out.DiskEncryptionConfiguration.KMSKeyRef = &kmsv1beta1.KMSCryptoKeyRef{External: in.DiskEncryptionConfiguration.KmsKeyName}
		}
	}
	return out
}

// SQLAdminBackupObservedState_FromAPI is status.observedState.
func SQLAdminBackupObservedState_FromAPI(in *api.BackupRun) *krm.SQLAdminBackupObservedState {
	if in == nil {
		return nil
	}
	out := &krm.SQLAdminBackupObservedState{}
	out.Status = direct.LazyPtr(in.Status)
	out.EnqueuedTime = direct.LazyPtr(in.EnqueuedTime)
	out.ID = direct.LazyPtr(in.Id)
	out.StartTime = direct.LazyPtr(in.StartTime)
	out.EndTime = direct.LazyPtr(in.EndTime)
	if in.Error != nil {
		out.Error = &krm.OperationError{
			Kind:    direct.LazyPtr(in.Error.Kind),
			Code:    direct.LazyPtr(in.Error.Code),
			Message: direct.LazyPtr(in.Error.Message),
		}
	}
	out.Type = direct.LazyPtr(in.Type)
	out.WindowStartTime = direct.LazyPtr(in.WindowStartTime)
	if in.DiskEncryptionStatus != nil {
		out.DiskEncryptionStatus = &krm.DiskEncryptionStatus{
			KMSKeyVersionName: direct.LazyPtr(in.DiskEncryptionStatus.KmsKeyVersionName),
			Kind:              direct.LazyPtr(in.DiskEncryptionStatus.Kind),
		}
	}
	out.TimeZone = direct.LazyPtr(in.TimeZone)
	return out
}

// specDiff names the spec fields set in desired that differ from actual.
// The PR compares the same fields (its OnlySpecFields mask, then a
// top-level diff of the fields desired sets).
func specDiff(actual, desired *api.BackupRun) []string {
	var diff []string
	if desired.Description != "" && desired.Description != actual.Description {
		diff = append(diff, "description")
	}
	if desired.Location != "" && desired.Location != actual.Location {
		diff = append(diff, "location")
	}
	if desired.BackupKind != "" && desired.BackupKind != actual.BackupKind {
		diff = append(diff, "backupKind")
	}
	if desired.DiskEncryptionConfiguration != nil {
		var actualKey string
		if actual.DiskEncryptionConfiguration != nil {
			actualKey = actual.DiskEncryptionConfiguration.KmsKeyName
		}
		if desired.DiskEncryptionConfiguration.KmsKeyName != actualKey {
			diff = append(diff, "diskEncryptionConfiguration")
		}
	}
	slices.Sort(diff)
	return diff
}
