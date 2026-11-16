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
// pkg/controller/direct/container/containernodepool_controller.go
// (nodePoolAdapter.waitForOperation and the client it builds; UPSTREAM.md),
// as a function of the client and the operation's project and location
// so that both field kinds use it.

// Package container reconciles ContainerClusterFields and
// ContainerNodePoolFields, field kinds (docs/design.md section 6.6,
// pattern 2; decision 37).
package container

import (
	"context"
	"fmt"
	"strings"
	"time"

	gcp "cloud.google.com/go/container/apiv1"
	pb "cloud.google.com/go/container/apiv1/containerpb"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/config"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct/common"
)

func newClient(ctx context.Context, cfg *config.ControllerConfig) (*gcp.ClusterManagerClient, error) {
	opts, err := cfg.RESTClientOptions()
	if err != nil {
		return nil, err
	}
	return gcp.NewClusterManagerRESTClient(ctx, opts...)
}

// pollInterval is how often an operation is read while it runs.
var pollInterval = 2 * time.Second

func waitForOperation(ctx context.Context, client *gcp.ClusterManagerClient, project, location string, op *pb.Operation) error {
	if op == nil {
		return nil
	}
	opName := op.GetName()
	if !strings.HasPrefix(opName, "projects/") {
		opName = fmt.Sprintf("projects/%s/locations/%s/operations/%s", project, location, op.GetName())
	}
	_, err := common.WaitForOperation(ctx, pollInterval, func(op *pb.Operation) (bool, error) {
		if op.GetStatus() == pb.Operation_DONE {
			if op.GetError() != nil && op.GetError().GetCode() != 0 {
				return false, fmt.Errorf("operation %s failed: %s", op.GetName(), op.GetError().GetMessage())
			}
			return true, nil
		}
		if op.GetStatus() == pb.Operation_ABORTING {
			return false, fmt.Errorf("operation %s aborted: %s", op.GetName(), op.GetStatusMessage())
		}
		return false, nil
	}, func() (*pb.Operation, error) {
		return client.GetOperation(ctx, &pb.GetOperationRequest{Name: opName})
	})
	return err
}
