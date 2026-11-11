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
// apis/logging/v1beta1/logginglogsink_identity.go (UPSTREAM.md), keeping
// only its billingAccounts/{billingaccount}/sinks/{sink} format (upstream's
// identity already has it; its spec has no billing account parent). The
// parent comes from spec.billingAccountRef, resolved as Config Connector's
// BillingAccountRef does, and the identity is checked against
// status.externalRef, as upstream's newer identities are.

package v1alpha1

import (
	"context"
	"fmt"

	billingv1alpha1 "github.com/GoogleCloudPlatform/k8s-config-connector/apis/billing/v1alpha1"
	"github.com/GoogleCloudPlatform/k8s-config-connector/apis/common"
	"github.com/GoogleCloudPlatform/k8s-config-connector/apis/common/identity"
	refsv1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/apis/refs/v1beta1"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/gcpurls"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	_ identity.IdentityV2 = &LoggingBillingAccountSinkIdentity{}
	_ identity.Resource   = &LoggingBillingAccountSink{}
)

var LoggingBillingAccountSinkIdentityFormat = gcpurls.Template[LoggingBillingAccountSinkIdentity]("logging.googleapis.com", "billingAccounts/{billingaccount}/sinks/{sink}")

// +k8s:deepcopy-gen=false

// LoggingBillingAccountSinkIdentity is the identity of a billing account's log sink.
type LoggingBillingAccountSinkIdentity struct {
	BillingAccount string
	Sink           string
}

func (i *LoggingBillingAccountSinkIdentity) String() string {
	return LoggingBillingAccountSinkIdentityFormat.ToString(*i)
}

func (i *LoggingBillingAccountSinkIdentity) ParentString() string {
	return fmt.Sprintf("billingAccounts/%s", i.BillingAccount)
}

func (i *LoggingBillingAccountSinkIdentity) ID() string {
	return i.Sink
}

func (i *LoggingBillingAccountSinkIdentity) FromExternal(ref string) error {
	parsed, match, err := LoggingBillingAccountSinkIdentityFormat.Parse(ref)
	if err != nil {
		return fmt.Errorf("format of LoggingBillingAccountSink external=%q was not known (use %s): %w", ref, LoggingBillingAccountSinkIdentityFormat.CanonicalForm(), err)
	}
	if !match {
		return fmt.Errorf("format of LoggingBillingAccountSink external=%q was not known (use %s)", ref, LoggingBillingAccountSinkIdentityFormat.CanonicalForm())
	}
	*i = *parsed
	return nil
}

func (i *LoggingBillingAccountSinkIdentity) Host() string {
	return LoggingBillingAccountSinkIdentityFormat.Host()
}

func getIdentityFromLoggingBillingAccountSinkSpec(ctx context.Context, reader client.Reader, obj *LoggingBillingAccountSink) (*LoggingBillingAccountSinkIdentity, error) {
	resourceID, err := refsv1beta1.GetResourceID(obj)
	if err != nil {
		return nil, fmt.Errorf("cannot resolve resource ID: %w", err)
	}

	ref := obj.Spec.BillingAccountRef
	if ref.External == "" {
		if ref.Name == "" {
			return nil, fmt.Errorf("spec.billingAccountRef needs external or name")
		}
		if err := ref.Normalize(ctx, reader, obj.GetNamespace()); err != nil {
			return nil, fmt.Errorf("resolving spec.billingAccountRef: %w", err)
		}
	}
	account := &billingv1alpha1.BillingAccountIdentity{}
	if err := account.FromExternal(ref.External); err != nil {
		return nil, fmt.Errorf("resolving spec.billingAccountRef: %w", err)
	}

	return &LoggingBillingAccountSinkIdentity{
		BillingAccount: account.BillingAccount,
		Sink:           resourceID,
	}, nil
}

func (obj *LoggingBillingAccountSink) GetIdentity(ctx context.Context, reader client.Reader) (identity.Identity, error) {
	specIdentity, err := getIdentityFromLoggingBillingAccountSinkSpec(ctx, reader, obj)
	if err != nil {
		return nil, err
	}
	if externalRef := common.ValueOf(obj.Status.ExternalRef); externalRef != "" && externalRef != specIdentity.String() {
		return nil, fmt.Errorf("cannot change LoggingBillingAccountSink identity (old=%q, new=%q)", externalRef, specIdentity.String())
	}
	return specIdentity, nil
}
