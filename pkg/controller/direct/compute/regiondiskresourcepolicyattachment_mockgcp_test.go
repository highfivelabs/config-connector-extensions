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

package compute_test

// ComputeRegionDiskResourcePolicyAttachment end to end on the fake cloud
// (pkg/test/mockenv) or a real project (docs/live-runs.md). The case: a
// daily snapshot schedule attached to a regional (two-zone) data disk. The
// test makes the disks (10 GB pd-balanced, the smallest regional disk) and
// the schedule through the API; the disks are deleted before the schedule
// (a schedule in use cannot be deleted).

import (
	"strings"
	"testing"
	"time"

	gcp "cloud.google.com/go/compute/apiv1"
	pb "cloud.google.com/go/compute/apiv1/computepb"
	upstreamcompute "github.com/GoogleCloudPlatform/k8s-config-connector/apis/compute/v1beta1"
	"github.com/GoogleCloudPlatform/k8s-config-connector/apis/refs"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/k8s"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	computev1beta1 "github.com/highfivelabs/config-connector-extensions/apis/compute/v1beta1"
	"github.com/highfivelabs/config-connector-extensions/pkg/test/mockenv"
)

func TestComputeRegionDiskResourcePolicyAttachment(t *testing.T) {
	const region = "us-central1"
	replicaZones := []string{"us-central1-a", "us-central1-b"}
	gvk := computev1beta1.ComputeRegionDiskResourcePolicyAttachmentGVK
	env := mockenv.New(t, "example")
	disks, err := gcp.NewRegionDisksRESTClient(env.Ctx, env.GCPOptions()...)
	if err != nil {
		t.Fatal(err)
	}
	// Closed after the disks' own cleanups (cleanups run last first): a
	// closed client panics on a call.
	t.Cleanup(func() { disks.Close() })
	policies, err := gcp.NewResourcePoliciesRESTClient(env.Ctx, env.GCPOptions()...)
	if err != nil {
		t.Fatal(err)
	}

	// The snapshot schedule, deleted after the disks (cleanups run last
	// first).
	schedule := mockenv.Name("daily")
	op, err := policies.Insert(env.Ctx, &pb.InsertResourcePolicyRequest{
		Project: mockenv.Project, Region: region,
		ResourcePolicyResource: &pb.ResourcePolicy{
			Name: mockenv.Ptr(schedule),
			SnapshotSchedulePolicy: &pb.ResourcePolicySnapshotSchedulePolicy{
				Schedule: &pb.ResourcePolicySnapshotSchedulePolicySchedule{
					DailySchedule: &pb.ResourcePolicyDailyCycle{DaysInCycle: mockenv.Ptr(int32(1)), StartTime: mockenv.Ptr("04:00")},
				},
				RetentionPolicy: &pb.ResourcePolicySnapshotSchedulePolicyRetentionPolicy{MaxRetentionDays: mockenv.Ptr(int32(1))},
			},
		},
	})
	if err == nil {
		err = op.Wait(env.Ctx)
	}
	if err != nil {
		t.Fatalf("making resource policy %s: %v", schedule, err)
	}
	t.Cleanup(func() {
		defer policies.Close()
		op, err := policies.Delete(env.Ctx, &pb.DeleteResourcePolicyRequest{Project: mockenv.Project, Region: region, ResourcePolicy: schedule})
		if err == nil {
			err = op.Wait(env.Ctx)
		}
		if err != nil && !direct.IsNotFound(err) {
			t.Errorf("deleting resource policy %s: %v", schedule, err)
		}
	})
	newDisk := func(base string) string {
		name := mockenv.Name(base)
		var zones []string
		for _, z := range replicaZones {
			zones = append(zones, "projects/"+mockenv.Project+"/zones/"+z)
		}
		op, err := disks.Insert(env.Ctx, &pb.InsertRegionDiskRequest{
			Project: mockenv.Project, Region: region,
			DiskResource: &pb.Disk{
				Name: mockenv.Ptr(name), SizeGb: mockenv.Ptr(int64(10)),
				Type:         mockenv.Ptr("projects/" + mockenv.Project + "/regions/" + region + "/diskTypes/pd-balanced"),
				ReplicaZones: zones,
				Labels:       mockenv.Labels(),
			},
		})
		if err == nil {
			err = op.Wait(env.Ctx)
		}
		if err != nil {
			t.Fatalf("making disk %s: %v", name, err)
		}
		t.Cleanup(func() {
			op, err := disks.Delete(env.Ctx, &pb.DeleteRegionDiskRequest{Project: mockenv.Project, Region: region, Disk: name})
			if err == nil {
				err = op.Wait(env.Ctx)
			}
			if err != nil && !direct.IsNotFound(err) {
				t.Errorf("deleting disk %s: %v", name, err)
			}
		})
		return name
	}
	data := newDisk("data")
	kept := newDisk("kept")

	attached := func(disk string) bool {
		d, err := disks.Get(env.Ctx, &pb.GetRegionDiskRequest{Project: mockenv.Project, Region: region, Disk: disk})
		if err != nil {
			t.Fatalf("reading disk %s: %v", disk, err)
		}
		for _, p := range d.GetResourcePolicies() {
			if strings.HasSuffix(p, "/resourcePolicies/"+schedule) {
				return true
			}
		}
		return false
	}
	writes := func() int {
		n := 0
		for _, r := range env.Requests() {
			if strings.HasPrefix(r, "POST ") && (strings.Contains(r, "/addResourcePolicies") || strings.Contains(r, "/removeResourcePolicies")) {
				n++
			}
		}
		return n
	}
	getObj := func(t *testing.T, name string) *computev1beta1.ComputeRegionDiskResourcePolicyAttachment {
		t.Helper()
		obj := &computev1beta1.ComputeRegionDiskResourcePolicyAttachment{}
		if err := env.Kube.Get(env.Ctx, types.NamespacedName{Namespace: "example", Name: name}, obj); err != nil {
			t.Fatal(err)
		}
		return obj
	}
	newAttachment := func(name, disk string) *computev1beta1.ComputeRegionDiskResourcePolicyAttachment {
		return &computev1beta1.ComputeRegionDiskResourcePolicyAttachment{
			ObjectMeta: metav1.ObjectMeta{Namespace: "example", Name: name, Labels: mockenv.Labels()},
			Spec: computev1beta1.ComputeRegionDiskResourcePolicyAttachmentSpec{
				ProjectRef: refs.ProjectRef{External: mockenv.Project},
				Region:     region,
				DiskRef:    upstreamcompute.ComputeDiskRef{External: "projects/" + mockenv.Project + "/regions/" + region + "/disks/" + disk},
				ResourceID: mockenv.Ptr(schedule),
			},
		}
	}

	if err := env.Kube.Create(env.Ctx, newAttachment("data-daily", data)); err != nil {
		t.Fatal(err)
	}

	t.Run("create attaches the schedule to the disk", func(t *testing.T) {
		env.WaitReady(gvk, "example", "data-daily")
		if !attached(data) {
			t.Errorf("disk %s does not have %s", data, schedule)
		}
		if n := writes(); n != 1 {
			t.Errorf("writes = %d, want one addResourcePolicies", n)
		}
	})

	t.Run("a resync writes nothing", func(t *testing.T) {
		before := writes()
		o := getObj(t, "data-daily")
		o.Annotations = map[string]string{k8s.InternalForceReconcileAnnotation: "1"}
		if err := env.Kube.Update(env.Ctx, o); err != nil {
			t.Fatal(err)
		}
		time.Sleep(3 * time.Second)
		env.WaitReady(gvk, "example", "data-daily")
		if n := writes() - before; n != 0 {
			t.Errorf("a resync wrote %d times", n)
		}
	})

	t.Run("the CRD refuses a change", func(t *testing.T) {
		o := getObj(t, "data-daily")
		o.Spec.Region = "us-east1"
		err := env.Kube.Update(env.Ctx, o)
		if err == nil || !strings.Contains(err.Error(), "the spec is immutable") {
			t.Errorf("changing the region: got %v, want the API server to refuse it", err)
		}
	})

	t.Run("delete detaches the schedule", func(t *testing.T) {
		if err := env.Kube.Delete(env.Ctx, getObj(t, "data-daily")); err != nil {
			t.Fatal(err)
		}
		env.WaitGone(&computev1beta1.ComputeRegionDiskResourcePolicyAttachment{ObjectMeta: metav1.ObjectMeta{Namespace: "example", Name: "data-daily"}})
		if attached(data) {
			t.Errorf("after delete, disk %s still has %s", data, schedule)
		}
	})

	t.Run("abandon keeps the schedule attached", func(t *testing.T) {
		if err := env.Kube.Create(env.Ctx, newAttachment("kept-daily", kept)); err != nil {
			t.Fatal(err)
		}
		env.WaitReady(gvk, "example", "kept-daily")
		o := getObj(t, "kept-daily")
		o.Annotations = map[string]string{"cnrm.cloud.google.com/deletion-policy": "abandon"}
		if err := env.Kube.Update(env.Ctx, o); err != nil {
			t.Fatal(err)
		}
		if err := env.Kube.Delete(env.Ctx, o); err != nil {
			t.Fatal(err)
		}
		env.WaitGone(&computev1beta1.ComputeRegionDiskResourcePolicyAttachment{ObjectMeta: metav1.ObjectMeta{Namespace: "example", Name: "kept-daily"}})
		if !attached(kept) {
			t.Errorf("after abandon, disk %s lost %s, want it kept", kept, schedule)
		}
	})
}
