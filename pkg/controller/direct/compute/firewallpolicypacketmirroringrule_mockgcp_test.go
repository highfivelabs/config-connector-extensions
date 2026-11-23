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

// ComputeFirewallPolicyPacketMirroringRule end to end on the fake cloud
// (pkg/test/mockenv; mockgcp patch 0058) or a real folder
// (docs/live-runs.md: the test folder in E2E_TEST_FOLDER, where the
// live-test service account administers firewall policies). The case is
// the global policy's (TestComputeNetworkFirewallPolicyPacketMirroringRule)
// on a folder's hierarchical policy: web traffic is not mirrored, and the
// rest goes to the next rule. The policy is made in the cloud, under the
// folder, and named by external; the test deletes it.

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	pb "cloud.google.com/go/compute/apiv1/computepb"
	kcccomputev1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/apis/compute/v1beta1"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/k8s"
	computeapi "google.golang.org/api/compute/v1"
	htransport "google.golang.org/api/transport/http"
	"google.golang.org/protobuf/encoding/protojson"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"

	computev1alpha1 "github.com/highfivelabs/config-connector-extensions/apis/compute/v1alpha1"
	computev1beta1 "github.com/highfivelabs/config-connector-extensions/apis/compute/v1beta1"
	"github.com/highfivelabs/config-connector-extensions/pkg/test/mockenv"
)

// waitOrganizationOperation waits for a global organization operation and
// returns it done.
func waitOrganizationOperation(ctx context.Context, svc *computeapi.Service, op *computeapi.Operation) (*computeapi.Operation, error) {
	for op.Status != "DONE" {
		time.Sleep(time.Second)
		var err error
		if op, err = svc.GlobalOrganizationOperations.Get(op.Name).Context(ctx).Do(); err != nil {
			return nil, err
		}
	}
	if op.Error != nil && len(op.Error.Errors) > 0 {
		return nil, fmt.Errorf("%s: %s", op.Error.Errors[0].Code, op.Error.Errors[0].Message)
	}
	return op, nil
}

func TestComputeFirewallPolicyPacketMirroringRule(t *testing.T) {
	if mockenv.Live() && os.Getenv("E2E_TEST_FOLDER") == "" {
		t.Skip("a live hierarchical firewall policy needs the test folder, E2E_TEST_FOLDER, where the live-test service account administers firewall policies (docs/live-runs.md)")
	}
	gvk := computev1alpha1.ComputeFirewallPolicyPacketMirroringRuleGVK
	env := mockenv.New(t, namespace)
	svc, err := computeapi.NewService(env.Ctx, env.GCPOptions()...)
	if err != nil {
		t.Fatal(err)
	}
	httpClient, _, err := htransport.NewClient(env.Ctx, env.GCPOptions()...)
	if err != nil {
		t.Fatal(err)
	}

	// The folder's policy, made in the cloud and deleted when the test ends
	// (with what rules are left in it, the abandoned one included).
	op, err := svc.FirewallPolicies.Insert(&computeapi.FirewallPolicy{
		ShortName:   mockenv.Name("mirroring"),
		Description: "packet mirroring rules under test",
	}).ParentId(mockenv.TestFolder(t)).Context(env.Ctx).Do()
	if err == nil {
		op, err = waitOrganizationOperation(env.Ctx, svc, op)
	}
	if err != nil {
		t.Fatalf("making the folder's firewall policy: %v", err)
	}
	// The policy is named by the number the API gives it.
	policyID := strconv.FormatUint(op.TargetId, 10)
	if op.TargetId == 0 {
		policyID = op.TargetLink[strings.LastIndex(op.TargetLink, "/")+1:]
	}
	t.Cleanup(func() {
		op, err := svc.FirewallPolicies.Delete(policyID).Context(env.Ctx).Do()
		if err == nil {
			_, err = waitOrganizationOperation(env.Ctx, svc, op)
		}
		if err != nil {
			t.Errorf("deleting firewall policy %s: %v", policyID, err)
		}
	})
	policyPath := "locations/global/firewallPolicies/" + policyID

	// cloudRule reads a rule with the beta API's getPacketMirroringRule.
	cloudRule := func(priority int32) (*pb.FirewallPolicyRule, error) {
		u := fmt.Sprintf("https://compute.googleapis.com/compute/beta/%s/getPacketMirroringRule?priority=%d", policyPath, priority)
		req, err := http.NewRequestWithContext(env.Ctx, http.MethodGet, u, nil)
		if err != nil {
			return nil, err
		}
		resp, err := httpClient.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}
		if resp.StatusCode != http.StatusOK {
			var e struct {
				Error struct {
					Message string `json:"message"`
				} `json:"error"`
			}
			_ = json.Unmarshal(body, &e)
			return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, e.Error.Message)
		}
		rule := &pb.FirewallPolicyRule{}
		return rule, (protojson.UnmarshalOptions{DiscardUnknown: true}).Unmarshal(body, rule)
	}
	missing := func(err error) bool {
		return err != nil && (strings.Contains(err.Error(), "HTTP 404") || strings.Contains(err.Error(), "does not contain a"))
	}
	writes := func(method string) int {
		n := 0
		for _, r := range env.Requests() {
			if strings.HasPrefix(r, "POST ") && strings.Contains(r, "/firewallPolicies/"+policyID+"/"+method) {
				n++
			}
		}
		return n
	}
	getObj := func(t *testing.T, name string) *computev1alpha1.ComputeFirewallPolicyPacketMirroringRule {
		t.Helper()
		obj := &computev1alpha1.ComputeFirewallPolicyPacketMirroringRule{}
		if err := env.Kube.Get(env.Ctx, types.NamespacedName{Namespace: namespace, Name: name}, obj); err != nil {
			t.Fatal(err)
		}
		return obj
	}
	update := func(t *testing.T, name string, change func(*computev1alpha1.ComputeFirewallPolicyPacketMirroringRule)) {
		t.Helper()
		o := getObj(t, name)
		change(o)
		if err := env.Kube.Update(env.Ctx, o); err != nil {
			t.Fatal(err)
		}
	}
	newRule := func(name string, priority int64, action string) *computev1alpha1.ComputeFirewallPolicyPacketMirroringRule {
		return &computev1alpha1.ComputeFirewallPolicyPacketMirroringRule{
			ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: name, Labels: mockenv.Labels()},
			Spec: computev1alpha1.ComputeFirewallPolicyPacketMirroringRuleSpec{
				FirewallPolicyRef: kcccomputev1beta1.ComputeFirewallPolicyRef{External: policyPath},
				Priority:          priority,
				Action:            action,
				Direction:         "INGRESS",
				Description:       mockenv.Ptr("web traffic is not mirrored"),
				Match: computev1beta1.NetworkFirewallPolicyPacketMirroringRuleMatch{
					SrcIPRanges:  []string{"10.0.0.0/8"},
					DestIPRanges: []string{"10.20.0.0/16"},
					Layer4Configs: []computev1beta1.NetworkFirewallPolicyPacketMirroringRuleLayer4Config{
						{IPProtocol: "tcp", Ports: []string{"80", "443"}},
					},
				},
			},
		}
	}

	const web = "no-web-mirroring"
	if err := env.Kube.Create(env.Ctx, newRule(web, 1000, "do_not_mirror")); err != nil {
		t.Fatal(err)
	}

	t.Run("create adds the packet mirroring rule at its priority", func(t *testing.T) {
		u := env.WaitReady(gvk, namespace, web)
		got, err := cloudRule(1000)
		if err != nil {
			t.Fatalf("reading the rule from the cloud: %v", err)
		}
		if got.GetAction() != "do_not_mirror" || got.GetDirection() != "INGRESS" || got.GetDescription() != "web traffic is not mirrored" {
			t.Errorf("cloud rule = %v", got)
		}
		if l := got.GetMatch().GetLayer4Configs(); len(l) != 1 || l[0].GetIpProtocol() != "tcp" || len(l[0].GetPorts()) != 2 {
			t.Errorf("cloud layer 4 configs = %v", l)
		}
		want := policyPath + "/packetMirroringRules/1000"
		if ref, _, _ := unstructured.NestedString(u.Object, "status", "externalRef"); ref != want {
			t.Errorf("status.externalRef = %q, want %q", ref, want)
		}
		if n, _, _ := unstructured.NestedInt64(u.Object, "status", "ruleTupleCount"); n == 0 {
			t.Errorf("status.ruleTupleCount is not set")
		}
		if n := writes("addRule") + writes("patchRule") + writes("removeRule"); n != 0 {
			t.Errorf("%d writes of the policy's firewall rules, want 0", n)
		}
	})

	t.Run("a resync writes nothing", func(t *testing.T) {
		before := writes("patchPacketMirroringRule") + writes("addPacketMirroringRule")
		update(t, web, func(o *computev1alpha1.ComputeFirewallPolicyPacketMirroringRule) {
			o.Annotations = map[string]string{k8s.InternalForceReconcileAnnotation: time.Now().Format(time.RFC3339Nano)}
		})
		time.Sleep(3 * time.Second)
		env.WaitReady(gvk, namespace, web)
		if n := writes("patchPacketMirroringRule") + writes("addPacketMirroringRule") - before; n != 0 {
			t.Errorf("a resync wrote %d times", n)
		}
	})

	t.Run("changed fields and an emptied list are one patch", func(t *testing.T) {
		before := writes("patchPacketMirroringRule")
		update(t, web, func(o *computev1alpha1.ComputeFirewallPolicyPacketMirroringRule) {
			o.Spec.Description = mockenv.Ptr("web traffic to the frontends is not mirrored")
			o.Spec.Match.SrcIPRanges = []string{"10.10.0.0/16"}
			o.Spec.Match.DestIPRanges = nil
			o.Spec.RuleName = mockenv.Ptr("no-web")
		})
		deadline := time.Now().Add(mockenv.Timeout())
		for {
			got, err := cloudRule(1000)
			if err == nil && got.GetDescription() == "web traffic to the frontends is not mirrored" &&
				len(got.GetMatch().GetDestIpRanges()) == 0 && slices.Equal(got.GetMatch().GetSrcIpRanges(), []string{"10.10.0.0/16"}) &&
				got.GetRuleName() == "no-web" {
				break
			}
			if time.Now().After(deadline) {
				t.Fatalf("the cloud rule did not change: %v, %v", got, err)
			}
			time.Sleep(time.Second)
		}
		env.WaitReady(gvk, namespace, web)
		if n := writes("patchPacketMirroringRule") - before; n != 1 {
			t.Errorf("the change patched the rule %d times, want 1", n)
		}
	})

	t.Run("a second rule at its own priority", func(t *testing.T) {
		next := newRule("rest-goes-on", 2000, "goto_next")
		next.Spec.Direction = "EGRESS"
		next.Spec.Description = nil
		next.Spec.Match = computev1beta1.NetworkFirewallPolicyPacketMirroringRuleMatch{
			DestIPRanges:  []string{"0.0.0.0/0"},
			Layer4Configs: []computev1beta1.NetworkFirewallPolicyPacketMirroringRuleLayer4Config{{IPProtocol: "all"}},
		}
		if err := env.Kube.Create(env.Ctx, next); err != nil {
			t.Fatal(err)
		}
		env.WaitReady(gvk, namespace, "rest-goes-on")
		got, err := cloudRule(2000)
		if err != nil || got.GetAction() != "goto_next" || got.GetDirection() != "EGRESS" {
			t.Fatalf("cloud rule at 2000 = %v, %v", got, err)
		}
		if _, err := cloudRule(1000); err != nil {
			t.Errorf("the rule at 1000 after adding 2000: %v", err)
		}
	})

	t.Run("a mirror rule without a security profile group is refused", func(t *testing.T) {
		err := env.Kube.Create(env.Ctx, newRule("mirror-without-group", 3000, "mirror"))
		if err == nil || !strings.Contains(err.Error(), "a mirror rule needs securityProfileGroup") {
			t.Errorf("creating a mirror rule without a group: got %v, want the API server to refuse it", err)
		}
	})

	t.Run("the priority and the policy are immutable", func(t *testing.T) {
		o := getObj(t, web)
		o.Spec.Priority = 1001
		if err := env.Kube.Update(env.Ctx, o); err == nil || !strings.Contains(err.Error(), "priority is immutable") {
			t.Errorf("changing the priority: got %v, want the API server to refuse it", err)
		}
		o = getObj(t, web)
		o.Spec.FirewallPolicyRef = kcccomputev1beta1.ComputeFirewallPolicyRef{External: policyPath + "0"}
		if err := env.Kube.Update(env.Ctx, o); err == nil || !strings.Contains(err.Error(), "firewallPolicyRef is immutable") {
			t.Errorf("changing the policy: got %v, want the API server to refuse it", err)
		}
	})

	t.Run("delete removes the rule", func(t *testing.T) {
		if err := env.Kube.Delete(env.Ctx, getObj(t, web)); err != nil {
			t.Fatal(err)
		}
		env.WaitGone(&computev1alpha1.ComputeFirewallPolicyPacketMirroringRule{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: web}})
		if _, err := cloudRule(1000); !missing(err) {
			t.Errorf("after delete, reading the rule at 1000: %v, want it missing", err)
		}
	})

	t.Run("abandon keeps the rule", func(t *testing.T) {
		update(t, "rest-goes-on", func(o *computev1alpha1.ComputeFirewallPolicyPacketMirroringRule) {
			o.Annotations = map[string]string{"cnrm.cloud.google.com/deletion-policy": "abandon"}
		})
		if err := env.Kube.Delete(env.Ctx, getObj(t, "rest-goes-on")); err != nil {
			t.Fatal(err)
		}
		env.WaitGone(&computev1alpha1.ComputeFirewallPolicyPacketMirroringRule{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: "rest-goes-on"}})
		if _, err := cloudRule(2000); err != nil {
			t.Errorf("after abandon, reading the rule at 2000: %v, want it kept", err)
		}
	})
}
