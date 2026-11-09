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

package resourcemanager_test

// ProjectDefaultServiceAccounts end to end on the fake cloud
// (pkg/test/mockenv, patch 0024) or a real project (docs/live-runs.md). The
// case: a project module takes away the default service accounts' power
// by disabling, deleting or deprivileging them, someone turns one back on
// by hand, and removing the module gives them back. It changes a whole
// project's accounts, so a live run uses only a test project under the
// test folder (mockenv.FolderProject; skipped without one), whose Compute
// Engine API is on (so its default account exists). The fake project has
// no default accounts: the test makes them, as turning on Compute Engine
// and App Engine does, and one account that is not a default.

import (
	"sort"
	"strings"
	"testing"
	"time"

	"cloud.google.com/go/iam/apiv1/iampb"
	refsv1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/apis/refs/v1beta1"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/k8s"
	iamapi "google.golang.org/api/iam/v1"
	"google.golang.org/genproto/googleapis/type/expr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"

	rmv1alpha1 "github.com/highfivelabs/config-connector-extensions/apis/resourcemanager/v1alpha1"
	"github.com/highfivelabs/config-connector-extensions/pkg/controller/direct/resourcemanager"
	"github.com/highfivelabs/config-connector-extensions/pkg/iampolicy"
	"github.com/highfivelabs/config-connector-extensions/pkg/test/mockenv"
)

func TestProjectDefaultServiceAccounts(t *testing.T) {
	const namespace = "example"
	gvk := rmv1alpha1.ProjectDefaultServiceAccountsGVK
	env := mockenv.New(t, namespace)
	project := mockenv.FolderProject(t)
	iam, err := iamapi.NewService(env.Ctx, env.GCPOptions()...)
	if err != nil {
		t.Fatal(err)
	}
	// The project's IAM policy through cloudresourcemanager v3, as the
	// controller reads it (the fake keeps v1's apart).
	policies := &iampolicy.Client{HTTP: env.HTTP}
	projectType, _ := iampolicy.Lookup("Project")
	accountName := func(id string) string { return "projects/" + project + "/serviceAccounts/" + id }

	// list is the project's accounts, deleted ones left out.
	list := func(t *testing.T) []*iamapi.ServiceAccount {
		t.Helper()
		var all []*iamapi.ServiceAccount
		if err := iam.Projects.ServiceAccounts.List("projects/"+project).Pages(env.Ctx, func(r *iamapi.ListServiceAccountsResponse) error {
			all = append(all, r.Accounts...)
			return nil
		}); err != nil {
			t.Fatalf("listing service accounts: %v", err)
		}
		return all
	}
	// state is "enabled", "disabled" or "deleted" for each account by
	// unique ID.
	state := func(t *testing.T, ids []string) map[string]string {
		t.Helper()
		got := map[string]string{}
		for _, id := range ids {
			got[id] = "deleted"
		}
		for _, sa := range list(t) {
			if _, ok := got[sa.UniqueId]; ok {
				got[sa.UniqueId] = "enabled"
				if sa.Disabled {
					got[sa.UniqueId] = "disabled"
				}
			}
		}
		return got
	}
	waitState := func(t *testing.T, ids []string, want string) {
		t.Helper()
		deadline := time.Now().Add(mockenv.Timeout())
		for {
			got := state(t, ids)
			ok := true
			for _, s := range got {
				ok = ok && s == want
			}
			if ok {
				return
			}
			if time.Now().After(deadline) {
				t.Fatalf("timed out waiting for the default accounts to be %s: %v", want, got)
			}
			time.Sleep(500 * time.Millisecond)
		}
	}
	enable := func(t *testing.T, id string) {
		t.Helper()
		if _, err := iam.Projects.ServiceAccounts.Enable(accountName(id), &iamapi.EnableServiceAccountRequest{}).Context(env.Ctx).Do(); err != nil {
			t.Fatalf("enabling %s: %v", id, err)
		}
	}
	create := func(t *testing.T, accountID, displayName string) *iamapi.ServiceAccount {
		t.Helper()
		sa, err := iam.Projects.ServiceAccounts.Create("projects/"+project, &iamapi.CreateServiceAccountRequest{
			AccountId:      accountID,
			ServiceAccount: &iamapi.ServiceAccount{DisplayName: displayName},
		}).Context(env.Ctx).Do()
		if err != nil {
			t.Fatalf("creating service account %s: %v", accountID, err)
		}
		return sa
	}
	newObj := func(name, action string, restore *string) *rmv1alpha1.ProjectDefaultServiceAccounts {
		return &rmv1alpha1.ProjectDefaultServiceAccounts{
			ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: name, Labels: mockenv.Labels()},
			Spec: rmv1alpha1.ProjectDefaultServiceAccountsSpec{
				ProjectRef:    &refsv1beta1.ProjectRef{External: project},
				Action:        action,
				RestorePolicy: restore,
			},
		}
	}
	getObj := func(t *testing.T, name string) *rmv1alpha1.ProjectDefaultServiceAccounts {
		t.Helper()
		obj := &rmv1alpha1.ProjectDefaultServiceAccounts{}
		if err := env.Kube.Get(env.Ctx, types.NamespacedName{Namespace: namespace, Name: name}, obj); err != nil {
			t.Fatal(err)
		}
		return obj
	}
	remove := func(t *testing.T, name string) {
		t.Helper()
		if err := env.Kube.Delete(env.Ctx, getObj(t, name)); err != nil {
			t.Fatal(err)
		}
		env.WaitGone(&rmv1alpha1.ProjectDefaultServiceAccounts{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: name}})
	}
	resync := func(t *testing.T, name, n string) {
		t.Helper()
		o := getObj(t, name)
		o.Annotations = map[string]string{k8s.InternalForceReconcileAnnotation: n}
		if err := env.Kube.Update(env.Ctx, o); err != nil {
			t.Fatal(err)
		}
	}
	calls := func(verb string) int {
		n := 0
		for _, r := range env.Requests() {
			if strings.HasPrefix(r, "POST ") && strings.Contains(r, "/projects/"+project+"/serviceAccounts/") && strings.HasSuffix(r, ":"+verb) {
				n++
			}
		}
		return n
	}

	if !mockenv.Live() {
		create(t, "project-compute-default", "Compute Engine default service account")
		create(t, "project-appengine-default", "App Engine default service account")
		create(t, "app-runtime", "not a default")
	}
	var ids []string
	emails := map[string]string{}
	others := map[string]bool{}
	for _, sa := range list(t) {
		if resourcemanager.IsDefaultServiceAccount(sa.DisplayName) {
			ids = append(ids, sa.UniqueId)
			emails[sa.UniqueId] = sa.Email
		} else {
			others[sa.UniqueId] = sa.Disabled
		}
	}
	sort.Strings(ids)
	if len(ids) == 0 {
		t.Fatalf("project %s has no default service accounts (is the Compute Engine API on?)", project)
	}
	// Leave the defaults enabled and present however the test ends.
	t.Cleanup(func() {
		for id, s := range state(t, ids) {
			switch s {
			case "deleted":
				if _, err := iam.Projects.ServiceAccounts.Undelete(accountName(id), &iamapi.UndeleteServiceAccountRequest{}).Context(env.Ctx).Do(); err != nil {
					t.Errorf("undeleting %s: %v", emails[id], err)
				}
			case "disabled":
				enable(t, id)
			}
		}
	})
	othersUntouched := func(t *testing.T) {
		t.Helper()
		for _, sa := range list(t) {
			if was, ok := others[sa.UniqueId]; ok && sa.Disabled != was {
				t.Errorf("account %s, not a default, changed: disabled %v, was %v", sa.Email, sa.Disabled, was)
			}
		}
	}
	statusAccounts := func(t *testing.T, u *unstructured.Unstructured) {
		t.Helper()
		got, _, _ := unstructured.NestedStringMap(u.Object, "status", "serviceAccounts")
		for _, id := range ids {
			if got[id] != emails[id] {
				t.Errorf("status.serviceAccounts[%s] = %q, want %q (all: %v)", id, got[id], emails[id], got)
			}
		}
	}

	t.Run("DISABLE disables the defaults only", func(t *testing.T) {
		if err := env.Kube.Create(env.Ctx, newObj("disable", "DISABLE", nil)); err != nil {
			t.Fatal(err)
		}
		u := env.WaitReady(gvk, namespace, "disable")
		waitState(t, ids, "disabled")
		othersUntouched(t)
		statusAccounts(t, u)
		if ref, _, _ := unstructured.NestedString(u.Object, "status", "externalRef"); ref != "projects/"+project+"/defaultServiceAccounts" {
			t.Errorf("status.externalRef = %q", ref)
		}
		if p := getObj(t, "disable").Spec.RestorePolicy; p == nil || *p != "REVERT" {
			t.Errorf("spec.restorePolicy = %v, want the default REVERT", p)
		}
	})

	t.Run("a resync disables nothing more", func(t *testing.T) {
		n := calls("disable")
		resync(t, "disable", "1")
		time.Sleep(3 * time.Second)
		env.WaitReady(gvk, namespace, "disable")
		if d := calls("disable") - n; d != 0 {
			t.Errorf("a resync called disable %d times", d)
		}
	})

	t.Run("a default enabled by hand is disabled again", func(t *testing.T) {
		enable(t, ids[0])
		resync(t, "disable", "2")
		waitState(t, ids, "disabled")
	})

	t.Run("action is immutable", func(t *testing.T) {
		o := getObj(t, "disable")
		o.Spec.Action = "DELETE"
		if err := env.Kube.Update(env.Ctx, o); err == nil || !strings.Contains(err.Error(), "action is immutable") {
			t.Errorf("changing spec.action: got %v, want the API server to refuse it", err)
		}
	})

	t.Run("only DELETE, DISABLE or DEPRIVILEGE", func(t *testing.T) {
		if err := env.Kube.Create(env.Ctx, newObj("bad", "REMOVE", nil)); err == nil || !strings.Contains(err.Error(), "spec.action") {
			t.Errorf("creating with REMOVE: got %v, want the API server to refuse it", err)
		}
	})

	t.Run("delete with REVERT enables them", func(t *testing.T) {
		remove(t, "disable")
		waitState(t, ids, "enabled")
	})

	t.Run("delete with NONE leaves them disabled", func(t *testing.T) {
		if err := env.Kube.Create(env.Ctx, newObj("none", "DISABLE", mockenv.Ptr("NONE"))); err != nil {
			t.Fatal(err)
		}
		env.WaitReady(gvk, namespace, "none")
		waitState(t, ids, "disabled")
		n := calls("enable")
		remove(t, "none")
		if got := state(t, ids); got[ids[0]] != "disabled" {
			t.Errorf("after delete with NONE: %v, want them disabled", got)
		}
		if d := calls("enable") - n; d != 0 {
			t.Errorf("delete with NONE called enable %d times", d)
		}
		for _, id := range ids {
			enable(t, id)
		}
	})

	t.Run("DELETE deletes them and REVERT undeletes them", func(t *testing.T) {
		if err := env.Kube.Create(env.Ctx, newObj("delete", "DELETE", nil)); err != nil {
			t.Fatal(err)
		}
		u := env.WaitReady(gvk, namespace, "delete")
		waitState(t, ids, "deleted")
		othersUntouched(t)
		statusAccounts(t, u)
		// The deleted accounts are no longer listed; status keeps them.
		resync(t, "delete", "1")
		time.Sleep(3 * time.Second)
		statusAccounts(t, env.WaitReady(gvk, namespace, "delete"))
		remove(t, "delete")
		waitState(t, ids, "enabled")
	})

	t.Run("DEPRIVILEGE removes them from every binding", func(t *testing.T) {
		member := func(id string) string { return "serviceAccount:" + emails[id] }
		policy := func(t *testing.T) *iampb.Policy {
			t.Helper()
			p, err := policies.Get(env.Ctx, projectType, "projects/"+project)
			if err != nil {
				t.Fatalf("reading the project's IAM policy: %v", err)
			}
			return p
		}
		if !mockenv.Live() {
			// The roles the defaults get, and one binding with a condition.
			p := policy(t)
			p.Version = 3
			p.Bindings = append(p.Bindings,
				&iampb.Binding{Role: "roles/editor", Members: []string{member(ids[0]), "user:someone@example.com"}},
				&iampb.Binding{Role: "roles/viewer", Members: []string{member(ids[len(ids)-1])}},
				&iampb.Binding{Role: "roles/browser", Members: []string{member(ids[0]), "group:admins@example.com"},
					Condition: &expr.Expr{Title: "weekdays", Expression: "request.time.getDayOfWeek() < 5"}},
			)
			if _, err := policies.Set(env.Ctx, projectType, "projects/"+project, p); err != nil {
				t.Fatalf("granting the defaults roles: %v", err)
			}
		}
		if err := env.Kube.Create(env.Ctx, newObj("deprivilege", "DEPRIVILEGE", nil)); err != nil {
			t.Fatal(err)
		}
		statusAccounts(t, env.WaitReady(gvk, namespace, "deprivilege"))
		p := policy(t)
		for _, b := range p.Bindings {
			for _, m := range b.Members {
				for _, id := range ids {
					if strings.EqualFold(m, member(id)) {
						t.Errorf("%s still holds %s", m, b.Role)
					}
				}
			}
		}
		waitState(t, ids, "enabled")
		if !mockenv.Live() {
			kept := map[string]string{}
			for _, b := range p.Bindings {
				cond := ""
				if b.Condition != nil {
					cond = b.Condition.Title
				}
				kept[b.Role] = strings.Join(b.Members, ",") + "|" + cond
			}
			if kept["roles/editor"] != "user:someone@example.com|" {
				t.Errorf("roles/editor = %q, want the other member kept", kept["roles/editor"])
			}
			if _, ok := kept["roles/viewer"]; ok {
				t.Errorf("roles/viewer, left with no members, is still in the policy")
			}
			if kept["roles/browser"] != "group:admins@example.com|weekdays" {
				t.Errorf("roles/browser = %q, want the other member and the condition kept", kept["roles/browser"])
			}
		}
		etag := string(p.Etag)
		remove(t, "deprivilege")
		if after := policy(t); string(after.Etag) != etag {
			t.Errorf("delete changed the IAM policy; DEPRIVILEGE is never reverted")
		}
	})

	t.Run("REVERT_AND_IGNORE_FAILURE deletes the object when undelete fails", func(t *testing.T) {
		if mockenv.Live() {
			t.Skip("needs a default account's email taken again, which a real project cannot do")
		}
		if err := env.Kube.Create(env.Ctx, newObj("ignore", "DELETE", mockenv.Ptr("REVERT_AND_IGNORE_FAILURE"))); err != nil {
			t.Fatal(err)
		}
		env.WaitReady(gvk, namespace, "ignore")
		waitState(t, ids, "deleted")
		// The compute default's email in use again: it cannot be undeleted.
		taken := create(t, "project-compute-default", "taken")
		remove(t, "ignore")
		got := state(t, ids)
		for _, id := range ids {
			want := "enabled"
			if emails[id] == taken.Email {
				want = "deleted"
			}
			if got[id] != want {
				t.Errorf("%s is %s, want %s", emails[id], got[id], want)
			}
		}
		if _, err := iam.Projects.ServiceAccounts.Delete(accountName(taken.UniqueId)).Context(env.Ctx).Do(); err != nil {
			t.Fatal(err)
		}
	})
}
