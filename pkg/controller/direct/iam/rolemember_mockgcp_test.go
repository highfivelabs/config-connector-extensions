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

package iam_test

// IAMRoleMember end to end on the fake cloud (pkg/test/mockenv; patches 0018
// and 0019 serve Cloud Storage's and Compute Engine's policy paths) or a
// real project (docs/live-runs.md). The case: a module that gives one
// member a role on a resource Config Connector's IAMPolicyMember cannot
// reference, and expects every other member of the role, and every other
// role, to stay as it is. Three such resources, one for each way a policy
// is reached: a regional secret (the google.iam.v1 verbs on a regional
// host), a managed folder (Cloud Storage's own paths) and a backend service
// (Compute Engine's). Each is the run's own (mockenv.Name) and is deleted at
// the end.

import (
	"slices"
	"strings"
	"testing"
	"time"

	gcp "cloud.google.com/go/compute/apiv1"
	pb "cloud.google.com/go/compute/apiv1/computepb"
	"cloud.google.com/go/iam/apiv1/iampb"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/k8s"
	"google.golang.org/api/option"
	secretmanager "google.golang.org/api/secretmanager/v1"
	storageapi "google.golang.org/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"

	iamv1alpha1 "github.com/highfivelabs/config-connector-extensions/apis/iam/v1alpha1"
	"github.com/highfivelabs/config-connector-extensions/pkg/iampolicy"
	"github.com/highfivelabs/config-connector-extensions/pkg/test/mockenv"
)

var memberGVK = iamv1alpha1.IAMRoleMemberGVK

// target is a resource under test: its kind and full name in the cloud, a
// role it accepts, and how to read its policy.
type target struct {
	kind, name, role string
}

func TestIAMRoleMember(t *testing.T) {
	env := mockenv.New(t, namespace)
	policies := &iampolicy.Client{HTTP: env.HTTP}

	// The members are service accounts of the project, which a real policy
	// accepts (the project's organization refuses the projectOwner and
	// allUsers kinds on a secret). The run's own and two others that exist
	// in the project; the fake cloud accepts any.
	account := func(id string) string {
		return "serviceAccount:" + id + "@" + mockenv.Project + ".iam.gserviceaccount.com"
	}
	mine := "serviceAccount:" + mockenv.ServiceAccount(t)
	theirs := account("offload-runner")
	bystander := account("catalog-live-tests")
	sorted := func(s ...string) []string { slices.Sort(s); return s }

	policyOf := func(t *testing.T, tg target) *iampb.Policy {
		t.Helper()
		rt, _ := iampolicy.Lookup(tg.kind)
		p, err := policies.Get(env.Ctx, rt, tg.name)
		if err != nil {
			t.Fatalf("reading the policy of %s: %v", tg.name, err)
		}
		return p
	}
	members := func(t *testing.T, tg target) []string {
		t.Helper()
		m, _ := iampolicy.Members(policyOf(t, tg), tg.role, nil)
		return m
	}
	outOfBand := func(t *testing.T, tg target, change func(*iampb.Policy) bool) {
		t.Helper()
		rt, _ := iampolicy.Lookup(tg.kind)
		if _, err := policies.Modify(env.Ctx, rt, tg.name, change); err != nil {
			t.Fatalf("changing the policy of %s out of band: %v", tg.name, err)
		}
	}
	waitFor := func(t *testing.T, what string, cond func() bool) {
		t.Helper()
		deadline := time.Now().Add(mockenv.Timeout())
		for !cond() {
			if time.Now().After(deadline) {
				t.Fatalf("timed out waiting for %s", what)
			}
			time.Sleep(500 * time.Millisecond)
		}
	}
	resync := func(t *testing.T, name string) {
		t.Helper()
		o := &iamv1alpha1.IAMRoleMember{}
		if err := env.Kube.Get(env.Ctx, types.NamespacedName{Namespace: namespace, Name: name}, o); err != nil {
			t.Fatal(err)
		}
		o.Annotations = map[string]string{k8s.InternalForceReconcileAnnotation: time.Now().Format(time.RFC3339Nano)}
		if err := env.Kube.Update(env.Ctx, o); err != nil {
			t.Fatal(err)
		}
	}
	newMember := func(name string, tg target, member string) *iamv1alpha1.IAMRoleMember {
		return &iamv1alpha1.IAMRoleMember{
			ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: name},
			Spec: iamv1alpha1.IAMRoleMemberSpec{
				ResourceRef: iamv1alpha1.IAMResourceReference{Kind: tg.kind, External: tg.name},
				Role:        tg.role,
				Member:      mockenv.Ptr(member),
			},
		}
	}
	del := func(t *testing.T, o *iamv1alpha1.IAMRoleMember) {
		t.Helper()
		if err := env.Kube.Delete(env.Ctx, o); err != nil {
			t.Fatal(err)
		}
		env.WaitGone(o)
	}

	// exercise is the case, for one resource.
	exercise := func(t *testing.T, tg target, prefix string) {
		// A member somebody else gave the role: it must survive everything.
		outOfBand(t, tg, func(p *iampb.Policy) bool {
			return iampolicy.AddMembers(p, tg.role, nil, []string{bystander})
		})

		first := newMember(prefix+"-mine", tg, mine)
		second := newMember(prefix+"-theirs", tg, theirs)
		for _, o := range []*iamv1alpha1.IAMRoleMember{first, second} {
			if err := env.Kube.Create(env.Ctx, o); err != nil {
				t.Fatal(err)
			}
		}

		t.Run("create adds the member and leaves the others", func(t *testing.T) {
			u := env.WaitReady(memberGVK, namespace, first.Name)
			env.WaitReady(memberGVK, namespace, second.Name)
			want := tg.name + "#" + tg.role + "#" + mine
			if ref, _, _ := unstructured.NestedString(u.Object, "status", "externalRef"); ref != want {
				t.Errorf("status.externalRef = %q, want %q", ref, want)
			}
			if got, want := members(t, tg), sorted(mine, theirs, bystander); !slices.Equal(got, want) {
				t.Errorf("members of %s = %v, want %v", tg.role, got, want)
			}
		})

		t.Run("a member added out of band stays", func(t *testing.T) {
			resync(t, first.Name)
			env.WaitReady(memberGVK, namespace, first.Name)
			time.Sleep(time.Second)
			if got := members(t, tg); !slices.Contains(got, bystander) {
				t.Errorf("the member added out of band was removed: %v", got)
			}
		})

		t.Run("a member removed out of band is put back", func(t *testing.T) {
			outOfBand(t, tg, func(p *iampb.Policy) bool {
				return iampolicy.RemoveMembers(p, tg.role, nil, []string{mine})
			})
			if slices.Contains(members(t, tg), mine) {
				t.Fatal("the member was not removed")
			}
			resync(t, first.Name)
			waitFor(t, "the member to be put back", func() bool { return slices.Contains(members(t, tg), mine) })
			env.WaitReady(memberGVK, namespace, first.Name)
			if got, want := members(t, tg), sorted(mine, theirs, bystander); !slices.Equal(got, want) {
				t.Errorf("members of %s = %v, want %v", tg.role, got, want)
			}
		})

		t.Run("a resync of a matching policy writes nothing", func(t *testing.T) {
			count := func() int {
				n := 0
				for _, r := range env.Requests() {
					post := strings.HasPrefix(r, "POST ") && strings.Contains(r, "setIamPolicy")
					put := strings.HasPrefix(r, "PUT ") && strings.Contains(r, "/iam")
					if post || put {
						n++
					}
				}
				return n
			}
			before := count()
			resync(t, first.Name)
			time.Sleep(2 * time.Second)
			env.WaitReady(memberGVK, namespace, first.Name)
			time.Sleep(time.Second)
			if got := count(); got != before {
				t.Errorf("policy writes after a resync with nothing to change: %d", got-before)
			}
		})

		t.Run("delete removes this member and no other", func(t *testing.T) {
			del(t, second)
			if got, want := members(t, tg), sorted(mine, bystander); !slices.Equal(got, want) {
				t.Errorf("members of %s after the first delete = %v, want %v", tg.role, got, want)
			}
			del(t, first)
			if got := members(t, tg); !slices.Equal(got, []string{bystander}) {
				t.Errorf("members of %s after the second delete = %v, want [%s]", tg.role, got, bystander)
			}
		})

		t.Run("abandon leaves the member in the cloud", func(t *testing.T) {
			kept := newMember(prefix+"-kept", tg, theirs)
			kept.Annotations = map[string]string{"cnrm.cloud.google.com/deletion-policy": "abandon"}
			if err := env.Kube.Create(env.Ctx, kept); err != nil {
				t.Fatal(err)
			}
			env.WaitReady(memberGVK, namespace, kept.Name)
			del(t, kept)
			if got, want := members(t, tg), sorted(theirs, bystander); !slices.Equal(got, want) {
				t.Errorf("members of %s after an abandoning delete = %v, want %v", tg.role, got, want)
			}
		})
	}

	t.Run("a regional secret", func(t *testing.T) {
		const region = "us-central1"
		sm, err := secretmanager.NewService(env.Ctx, append(env.GCPOptions(),
			option.WithEndpoint("https://secretmanager."+region+".rep.googleapis.com/"))...)
		if err != nil {
			t.Fatal(err)
		}
		id := mockenv.Name("role-member-secret")
		parent := "projects/" + mockenv.Project + "/locations/" + region
		if _, err := sm.Projects.Locations.Secrets.Create(parent, &secretmanager.Secret{Labels: mockenv.Labels()}).SecretId(id).Context(env.Ctx).Do(); err != nil {
			t.Fatalf("creating the secret: %v", err)
		}
		tg := target{kind: "SecretManagerSecret", name: parent + "/secrets/" + id, role: "roles/secretmanager.secretAccessor"}
		t.Cleanup(func() {
			if _, err := sm.Projects.Locations.Secrets.Delete(tg.name).Context(env.Ctx).Do(); err != nil && !iampolicy.IsNotFound(err) {
				t.Errorf("deleting the secret: %v", err)
			}
		})
		exercise(t, tg, "secret")

		t.Run("the same member under a condition is its own binding", func(t *testing.T) {
			o := newMember("secret-timed", tg, mine)
			o.Spec.Condition = &iamv1alpha1.IAMCondition{
				Title:      "until 2099",
				Expression: `request.time < timestamp("2099-01-01T00:00:00Z")`,
			}
			plain := newMember("secret-plain", tg, mine)
			for _, m := range []*iamv1alpha1.IAMRoleMember{plain, o} {
				if err := env.Kube.Create(env.Ctx, m); err != nil {
					t.Fatal(err)
				}
			}
			env.WaitReady(memberGVK, namespace, plain.Name)
			u := env.WaitReady(memberGVK, namespace, o.Name)
			if ref, _, _ := unstructured.NestedString(u.Object, "status", "externalRef"); ref != tg.name+"#"+tg.role+"#"+mine+"#until 2099" {
				t.Errorf("status.externalRef = %q", ref)
			}
			cond := &iampolicy.Condition{Title: o.Spec.Condition.Title, Expression: o.Spec.Condition.Expression}
			if got, _ := iampolicy.Members(policyOf(t, tg), tg.role, cond); !slices.Equal(got, []string{mine}) {
				t.Errorf("conditional members = %v, want [%s]", got, mine)
			}
			del(t, o)
			if _, found := iampolicy.Members(policyOf(t, tg), tg.role, cond); found {
				t.Error("the conditional binding is still in the policy after its object was deleted")
			}
			if got := members(t, tg); !slices.Contains(got, mine) {
				t.Errorf("the unconditional member went with the conditional one: %v", got)
			}
			del(t, plain)
		})

		t.Run("a member from a service account, named by its email", func(t *testing.T) {
			o := newMember("secret-from", tg, "")
			o.Spec.Member = nil
			o.Spec.MemberFrom = &iamv1alpha1.IAMRoleBindingMemberFrom{}
			o.Spec.MemberFrom.ServiceAccountRef.External = strings.TrimPrefix(theirs, "serviceAccount:")
			if err := env.Kube.Create(env.Ctx, o); err != nil {
				t.Fatal(err)
			}
			env.WaitReady(memberGVK, namespace, o.Name)
			if got := members(t, tg); !slices.Contains(got, theirs) {
				t.Errorf("members = %v, want %s among them", got, theirs)
			}
			del(t, o)
		})

		t.Run("a member and a memberFrom, or neither, are refused", func(t *testing.T) {
			both := newMember("both", tg, mine)
			both.Spec.MemberFrom = &iamv1alpha1.IAMRoleBindingMemberFrom{}
			both.Spec.MemberFrom.ServiceAccountRef.External = strings.TrimPrefix(theirs, "serviceAccount:")
			if err := env.Kube.Create(env.Ctx, both); err == nil || !strings.Contains(err.Error(), "exactly one of member and memberFrom") {
				t.Errorf("both: err = %v", err)
			}
			neither := newMember("neither", tg, "")
			neither.Spec.Member = nil
			if err := env.Kube.Create(env.Ctx, neither); err == nil || !strings.Contains(err.Error(), "member") {
				// The CRD's oneOf (from Config Connector's validation step)
				// answers before the CEL rule does.
				t.Errorf("neither: err = %v", err)
			}
		})

		t.Run("a member waits for the object it names", func(t *testing.T) {
			o := newMember("late", tg, mine)
			o.Spec.ResourceRef = iamv1alpha1.IAMResourceReference{Kind: "SecretManagerSecret", APIVersion: "secretmanager.cnrm.cloud.google.com/v1beta1", Name: "not-yet"}
			if err := env.Kube.Create(env.Ctx, o); err != nil {
				t.Fatal(err)
			}
			if msg := env.WaitNotReady(memberGVK, namespace, "late"); !strings.Contains(msg, "not-yet") {
				t.Errorf("the message does not name the missing object: %q", msg)
			}
			// Nothing was made for it, so the delete has nothing to remove.
			del(t, o)
		})

		t.Run("a kind it does not know is refused", func(t *testing.T) {
			o := newMember("unknown", tg, mine)
			o.Spec.ResourceRef.Kind = "Spaceship"
			if err := env.Kube.Create(env.Ctx, o); err != nil {
				t.Fatal(err)
			}
			msg := env.WaitNotReady(memberGVK, namespace, "unknown")
			if !strings.Contains(msg, "Spaceship") || !strings.Contains(msg, "StorageManagedFolder") {
				t.Errorf("the message should name the kind and list the supported ones: %q", msg)
			}
			del(t, o)
		})
	})

	t.Run("a managed folder", func(t *testing.T) {
		gcs, err := storageapi.NewService(env.Ctx, env.GCPOptions()...)
		if err != nil {
			t.Fatal(err)
		}
		bucket := mockenv.Name("role-member-bucket")
		if _, err := gcs.Buckets.Insert(mockenv.Project, &storageapi.Bucket{
			Name: bucket, Location: "US", Labels: mockenv.Labels(),
			IamConfiguration: &storageapi.BucketIamConfiguration{UniformBucketLevelAccess: &storageapi.BucketIamConfigurationUniformBucketLevelAccess{Enabled: true}},
		}).Context(env.Ctx).Do(); err != nil {
			t.Fatalf("creating the bucket: %v", err)
		}
		// A nested folder, whose name the API takes escaped whole.
		const folder = "team/app/"
		if _, err := gcs.ManagedFolders.Insert(bucket, &storageapi.ManagedFolder{Name: folder}).Context(env.Ctx).Do(); err != nil {
			t.Fatalf("creating the managed folder: %v", err)
		}
		t.Cleanup(func() {
			if err := gcs.ManagedFolders.Delete(bucket, folder).Context(env.Ctx).Do(); err != nil && !iampolicy.IsNotFound(err) {
				t.Errorf("deleting the managed folder: %v", err)
			}
			if err := gcs.Buckets.Delete(bucket).Context(env.Ctx).Do(); err != nil && !iampolicy.IsNotFound(err) {
				t.Errorf("deleting the bucket: %v", err)
			}
		})
		tg := target{kind: "StorageManagedFolder", name: "projects/_/buckets/" + bucket + "/managedFolders/" + folder, role: "roles/storage.objectViewer"}
		exercise(t, tg, "folder")

		t.Run("the folder's policy is the folder's own", func(t *testing.T) {
			// The bucket's policy is not the folder's: nothing above touched it.
			rt, _ := iampolicy.Lookup("StorageBucket")
			p, err := policies.Get(env.Ctx, rt, "projects/_/buckets/"+bucket)
			if err != nil {
				t.Fatal(err)
			}
			if m, _ := iampolicy.Members(p, tg.role, nil); slices.Contains(m, mine) {
				t.Errorf("the bucket got the folder's member: %v", m)
			}
		})
	})

	t.Run("a backend service", func(t *testing.T) {
		health, err := gcp.NewHealthChecksRESTClient(env.Ctx, env.GCPOptions()...)
		if err != nil {
			t.Fatal(err)
		}
		backends, err := gcp.NewBackendServicesRESTClient(env.Ctx, env.GCPOptions()...)
		if err != nil {
			t.Fatal(err)
		}
		hcName, bsName := mockenv.Name("role-member-hc"), mockenv.Name("role-member-bs")
		op, err := health.Insert(env.Ctx, &pb.InsertHealthCheckRequest{Project: mockenv.Project, HealthCheckResource: &pb.HealthCheck{
			Name: mockenv.Ptr(hcName), Type: mockenv.Ptr("HTTP"),
			HttpHealthCheck: &pb.HTTPHealthCheck{Port: mockenv.Ptr(int32(80))},
		}})
		if err == nil {
			err = op.Wait(env.Ctx)
		}
		if err != nil {
			t.Fatalf("making the health check: %v", err)
		}
		t.Cleanup(func() {
			defer health.Close()
			op, err := health.Delete(env.Ctx, &pb.DeleteHealthCheckRequest{Project: mockenv.Project, HealthCheck: hcName})
			if err == nil {
				err = op.Wait(env.Ctx)
			}
			if err != nil && !direct.IsNotFound(err) {
				t.Errorf("deleting the health check: %v", err)
			}
		})
		op, err = backends.Insert(env.Ctx, &pb.InsertBackendServiceRequest{Project: mockenv.Project, BackendServiceResource: &pb.BackendService{
			Name: mockenv.Ptr(bsName), Protocol: mockenv.Ptr("HTTP"), LoadBalancingScheme: mockenv.Ptr("EXTERNAL_MANAGED"),
			HealthChecks: []string{"projects/" + mockenv.Project + "/global/healthChecks/" + hcName},
		}})
		if err == nil {
			err = op.Wait(env.Ctx)
		}
		if err != nil {
			t.Fatalf("making the backend service: %v", err)
		}
		t.Cleanup(func() {
			defer backends.Close()
			op, err := backends.Delete(env.Ctx, &pb.DeleteBackendServiceRequest{Project: mockenv.Project, BackendService: bsName})
			if err == nil {
				err = op.Wait(env.Ctx)
			}
			if err != nil && !direct.IsNotFound(err) {
				t.Errorf("deleting the backend service: %v", err)
			}
		})
		tg := target{kind: "ComputeBackendService", name: "projects/" + mockenv.Project + "/global/backendServices/" + bsName, role: "roles/compute.loadBalancerServiceUser"}
		exercise(t, tg, "backend")
	})
}
