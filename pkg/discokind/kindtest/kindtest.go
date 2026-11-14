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

// Package kindtest is the end-to-end test every discokind kind runs, on
// the fake cloud (task check) or against a real project (task test-live),
// from fixtures beside the test:
//
//	testdata/discokind/<Kind>/create.yaml   the object to create
//	testdata/discokind/<Kind>/update.yaml   optional: the same object with a changed spec
//	testdata/discokind/<Kind>/live-skip     optional: why it cannot run live (one line)
//	testdata/discokind/<Kind>/<case>/...    optional: another set, for RunCase
//
// The fixtures are YAML with placeholders: ${PROJECT} (mockenv.Project),
// ${NAME} (mockenv.Name of the kind's lower-case name: the run ID prefixes
// it live), ${ID} (${NAME} without dashes, for IDs that take none),
// ${RUN_ID}, ${ORGANIZATION} (E2E_ORGANIZATION live, 123456789 in the
// fake), and ${KIND_NAME} (${NAME} of the first object, also in the
// abandoned object's fixture, whose ${NAME} ends in -kept: for a parent a
// Setup makes once for both).
//
// The test: the object becomes Ready and the cloud holds what its spec
// says; a forced resync writes nothing; the update's spec is patched in
// (or, for a kind the API cannot patch, the API server refuses it); delete
// removes it from the cloud; and an object with deletion-policy abandon
// leaves its resource behind. Everything it makes in the cloud it deletes.
package kindtest

import (
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/controller/direct"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/k8s"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"

	"github.com/highfivelabs/config-connector-extensions/pkg/discokind"
	"github.com/highfivelabs/config-connector-extensions/pkg/test/mockenv"
)

const namespace = "discokind"

// Setup is what a kind needs in the cloud before its test that its
// fixtures cannot make (a fleet for a rollout sequence). It runs after the
// environment is up and registers its own cleanup with t.Cleanup.
type Setup func(t *testing.T, env *mockenv.Env)

var setups = map[string]Setup{}

// RegisterSetup gives a kind a Setup; a hand-written test file beside the
// generated one calls it from init.
func RegisterSetup(kind string, s Setup) { setups[kind] = s }

// Run is the test of one kind.
func Run(t *testing.T, kindName string) { run(t, kindName, "") }

// RunCase is the same test of a kind with a second set of fixtures, in
// testdata/discokind/<Kind>/<caseName>/, for a kind whose resource has
// shapes one set cannot cover (a gateway's two types). Its objects are
// named <kind>-<caseName>, and its Setup is the one registered as
// "<Kind>/<caseName>", if any.
func RunCase(t *testing.T, kindName, caseName string) {
	if caseName == "" {
		t.Fatal("RunCase needs a case name")
	}
	run(t, kindName, caseName)
}

func run(t *testing.T, kindName, caseName string) {
	k := discokind.ByKind(kindName)
	if k == nil {
		t.Fatalf("no discokind kind %s (hack/discokind/kinds.yaml)", kindName)
	}
	dir := filepath.Join("testdata", "discokind", kindName, caseName)
	if mockenv.Live() {
		if b, err := os.ReadFile(filepath.Join(dir, "live-skip")); err == nil {
			t.Skip(strings.TrimSpace(string(b)))
		}
	}
	base := strings.ToLower(kindName)
	setupKey := kindName
	if caseName != "" {
		base += "-" + caseName
		setupKey += "/" + caseName
	}
	create := load(t, filepath.Join(dir, "create.yaml"), base, base)
	var update *unstructured.Unstructured
	if _, err := os.Stat(filepath.Join(dir, "update.yaml")); err == nil {
		update = load(t, filepath.Join(dir, "update.yaml"), base, base)
	}

	env := mockenv.New(t, namespace)
	if s := setups[setupKey]; s != nil {
		s(t, env)
	}
	api := &discokind.API{HTTP: env.HTTP, BaseURL: k.BaseURL, LocationHost: k.LocationHost, GetQuery: k.GetQuery}
	if !mockenv.Live() {
		api.PollInterval = 200 * time.Millisecond
	}
	gvk := k.GVK
	writes := func(name string) int {
		n := 0
		for _, r := range env.Requests() {
			method, url, _ := strings.Cut(r, " ")
			if (method == http.MethodPatch || method == http.MethodPost || method == http.MethodPut) && strings.Contains(url, name) {
				n++
			}
		}
		return n
	}
	gone := func(name string) bool {
		var got map[string]any
		err := RetryQuota(func() (err error) {
			got, err = api.Get(env.Ctx, name)
			return err
		})
		if direct.IsNotFound(err) {
			return true
		}
		if k.MissingIs403 && direct.HasHTTPCode(err, http.StatusForbidden) {
			// The API's answer for a name it does not hold
			// (Kind.MissingIs403); the test's account can read the
			// resource while it exists.
			return true
		}
		if err != nil {
			t.Fatalf("reading %s: %v", name, err)
		}
		return k.SoftDelete != nil && got[k.SoftDelete.Field] == k.SoftDelete.Value
	}
	// cleanup deletes the cloud resource if it is still there.
	cleanup := func(name string) {
		t.Cleanup(func() {
			if name == "" || gone(name) {
				return
			}
			err := RetryQuota(func() error {
				op, err := api.Do(env.Ctx, http.MethodDelete, name, nil, nil)
				if err == nil && k.DeleteLRO {
					_, err = api.Wait(env.Ctx, op)
				}
				return err
			})
			if err != nil && !direct.IsNotFound(err) {
				t.Errorf("cleanup: deleting %s: %v", name, err)
			}
		})
	}
	get := func(name string) *unstructured.Unstructured {
		u := &unstructured.Unstructured{}
		u.SetGroupVersionKind(gvk)
		if err := env.Kube.Get(env.Ctx, client.ObjectKey{Namespace: namespace, Name: name}, u); err != nil {
			t.Fatal(err)
		}
		return u
	}
	// waitCloud waits for the cloud's name to hold want, the spec of the
	// object objName; on timeout it says why the object is not done (its
	// Ready condition carries the API's refusal, which no log shows).
	waitCloud := func(objName, name string, want map[string]any) map[string]any {
		deadline := time.Now().Add(mockenv.Timeout())
		for {
			got, err := api.Get(env.Ctx, name)
			if err == nil && len(discokind.Diff(want, got)) == 0 {
				return got
			}
			if time.Now().After(deadline) {
				ready, msg := mockenv.Ready(get(objName))
				t.Fatalf("the cloud's %s does not hold the spec: differs in %v (err %v)\nwant %v\ngot  %v\nthe object's Ready: %v %q", name, discokind.Diff(want, got), err, want, got, ready, msg)
			}
			time.Sleep(pollEvery())
		}
	}
	spec := func(u *unstructured.Unstructured) map[string]any {
		s, _, _ := unstructured.NestedMap(u.Object, "spec")
		return k.Body(s)
	}

	obj := create.DeepCopy()
	if err := env.Kube.Create(env.Ctx, obj); err != nil {
		t.Fatal(err)
	}
	var cloudName string

	t.Run("create makes the resource the spec describes", func(t *testing.T) {
		u := env.WaitReady(gvk, namespace, obj.GetName())
		cloudName, _, _ = unstructured.NestedString(u.Object, "status", "externalRef")
		if cloudName == "" {
			t.Fatal("status.externalRef is not set")
		}
		cleanup(cloudName)
		if !k.NameRegexp().MatchString(cloudName) {
			t.Errorf("status.externalRef %q is not a %s name (%s)", cloudName, kindName, k.Name)
		}
		waitCloud(obj.GetName(), cloudName, spec(create))
	})
	if cloudName == "" {
		t.FailNow()
	}

	t.Run("a resync writes nothing", func(t *testing.T) {
		before := writes(cloudName)
		u := get(obj.GetName())
		ann := u.GetAnnotations()
		if ann == nil {
			ann = map[string]string{}
		}
		ann[k8s.InternalForceReconcileAnnotation] = time.Now().Format(time.RFC3339Nano)
		u.SetAnnotations(ann)
		if err := env.Kube.Update(env.Ctx, u); err != nil {
			t.Fatal(err)
		}
		time.Sleep(3 * time.Second)
		env.WaitReady(gvk, namespace, obj.GetName())
		if n := writes(cloudName) - before; n != 0 {
			t.Errorf("a resync wrote %d times", n)
		}
	})

	if update != nil {
		t.Run("a changed spec reaches the cloud", func(t *testing.T) {
			u := get(obj.GetName())
			newSpec, _, _ := unstructured.NestedMap(update.Object, "spec")
			if err := unstructured.SetNestedMap(u.Object, newSpec, "spec"); err != nil {
				t.Fatal(err)
			}
			err := env.Kube.Update(env.Ctx, u)
			if !k.Updatable {
				if err == nil || !strings.Contains(err.Error(), "immutable") {
					t.Fatalf("changing a kind the API cannot patch: err = %v, want immutable", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			waitCloud(obj.GetName(), cloudName, spec(update))
			env.WaitReady(gvk, namespace, obj.GetName())
		})
	}

	t.Run("delete removes the resource", func(t *testing.T) {
		if err := env.Kube.Delete(env.Ctx, get(obj.GetName())); err != nil {
			t.Fatal(err)
		}
		env.WaitGone(get(obj.GetName()))
		if !gone(cloudName) {
			t.Errorf("%s is still in the cloud", cloudName)
		}
	})

	t.Run("abandon keeps the resource", func(t *testing.T) {
		kept := load(t, filepath.Join(dir, "create.yaml"), base+"-kept", base)
		kept.SetAnnotations(map[string]string{k8s.DeletionPolicyAnnotation: k8s.DeletionPolicyAbandon})
		if err := env.Kube.Create(env.Ctx, kept); err != nil {
			t.Fatal(err)
		}
		u := env.WaitReady(gvk, namespace, kept.GetName())
		name, _, _ := unstructured.NestedString(u.Object, "status", "externalRef")
		cleanup(name)
		if err := env.Kube.Delete(env.Ctx, u); err != nil {
			t.Fatal(err)
		}
		env.WaitGone(u)
		if gone(name) {
			t.Errorf("the abandoned %s is gone from the cloud", name)
		}
	})
}

// load reads a fixture with its placeholders filled for base, the object's
// name, and kindBase, the kind's.
// pollEvery is how often the test reads the cloud while it waits: live,
// slowly, since some APIs allow only a few reads a minute (Dataplex: 10 per
// user and region), and the controller reads with the same account.
func pollEvery() time.Duration {
	if mockenv.Live() {
		return 10 * time.Second
	}
	return 500 * time.Millisecond
}

// RetryQuota runs f until it returns anything but a refusal for quota
// (429), waiting longer each time, for at most mockenv.Timeout().
func RetryQuota(f func() error) error {
	wait := 15 * time.Second
	if !mockenv.Live() {
		wait = 100 * time.Millisecond
	}
	deadline := time.Now().Add(mockenv.Timeout())
	for {
		err := f()
		if !direct.HasHTTPCode(err, http.StatusTooManyRequests) || time.Now().After(deadline) {
			return err
		}
		time.Sleep(wait)
		if wait < 2*time.Minute {
			wait *= 2
		}
	}
}

func load(t *testing.T, path, base, kindBase string) *unstructured.Unstructured {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	name := mockenv.Name(base)
	org := "123456789"
	if mockenv.Live() {
		org = os.Getenv("E2E_ORGANIZATION")
	}
	s := strings.NewReplacer(
		"${PROJECT}", mockenv.Project,
		"${NAME}", name,
		"${ID}", strings.ReplaceAll(name, "-", ""),
		"${RUN_ID}", mockenv.RunID,
		"${ORGANIZATION}", org,
		"${KIND_NAME}", mockenv.Name(kindBase),
	).Replace(string(b))
	u := &unstructured.Unstructured{}
	if err := yaml.Unmarshal([]byte(s), &u.Object); err != nil {
		t.Fatalf("%s: %v", path, err)
	}
	if u.GetName() == "" {
		t.Fatal(errors.New(path + ": no metadata.name"))
	}
	u.SetNamespace(namespace)
	return u
}
