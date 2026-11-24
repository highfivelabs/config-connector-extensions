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

package artifactregistry_test

// ArtifactRegistryRepository's vulnerability scanning config,
// disallowUnspecifiedMode and Apt and Yum remote upstreams (application
// queue item 4) end to end on the fake cloud (pkg/test/mockenv, patch
// 0068), or a real project (task test-live). The repositories are the
// run's own and empty (free), and are deleted at the end.

import (
	"strings"
	"testing"
	"time"

	pb "cloud.google.com/go/artifactregistry/apiv1/artifactregistrypb"
	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/k8s"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"

	arv1beta1 "github.com/highfivelabs/config-connector-extensions/apis/artifactregistry/v1beta1"
	"github.com/highfivelabs/config-connector-extensions/pkg/test/mockenv"
)

func TestArtifactRegistryRepositoryFields(t *testing.T) {
	env := mockenv.New(t, namespace)
	cloud := newClient(t, env)
	get := func(t *testing.T, name string) *pb.Repository {
		t.Helper()
		got, err := cloud.GetRepository(env.Ctx, &pb.GetRepositoryRequest{Name: cloudName(name)})
		if err != nil {
			t.Fatalf("reading the repository from the cloud: %v", err)
		}
		return got
	}
	getObj := func(t *testing.T, name string) *arv1beta1.ArtifactRegistryRepository {
		t.Helper()
		o := &arv1beta1.ArtifactRegistryRepository{}
		if err := env.Kube.Get(env.Ctx, types.NamespacedName{Namespace: namespace, Name: name}, o); err != nil {
			t.Fatal(err)
		}
		return o
	}
	// update changes the object, again on a conflict with the controller's
	// status writes.
	update := func(t *testing.T, name string, change func(*arv1beta1.ArtifactRegistryRepository)) error {
		t.Helper()
		return retry.RetryOnConflict(retry.DefaultRetry, func() error {
			o := getObj(t, name)
			change(o)
			return env.Kube.Update(env.Ctx, o)
		})
	}
	// resyncPatches forces a reconcile and returns how many updates it sent.
	resyncPatches := func(t *testing.T, name string) int {
		t.Helper()
		patches := count(env, "PATCH", cloudName(name))
		if err := update(t, name, func(o *arv1beta1.ArtifactRegistryRepository) {
			o.Annotations[k8s.InternalForceReconcileAnnotation] = time.Now().String()
		}); err != nil {
			t.Fatal(err)
		}
		time.Sleep(3 * time.Second)
		env.WaitReady(gvk, namespace, name)
		return count(env, "PATCH", cloudName(name)) - patches
	}
	newRepository := func(name string) *arv1beta1.ArtifactRegistryRepository {
		return &arv1beta1.ArtifactRegistryRepository{
			ObjectMeta: metav1.ObjectMeta{
				Namespace:   namespace,
				Name:        name,
				Annotations: map[string]string{"cnrm.cloud.google.com/project-id": mockenv.Project},
				Labels:      mockenv.Labels(),
			},
			Spec: arv1beta1.ArtifactRegistryRepositorySpec{
				Location:    region,
				Format:      mockenv.Ptr("DOCKER"),
				Mode:        mockenv.Ptr("STANDARD_REPOSITORY"),
				Description: mockenv.Ptr("scanning settings"),
			},
		}
	}

	name := mockenv.Name("scanning")
	repo := newRepository(name)
	repo.Spec.VulnerabilityScanningConfig = &arv1beta1.Repository_VulnerabilityScanningConfig{EnablementConfig: mockenv.Ptr("DISABLED")}
	repo.Spec.DisallowUnspecifiedMode = mockenv.Ptr(true)
	if err := env.Kube.Create(env.Ctx, repo); err != nil {
		t.Fatal(err)
	}

	t.Run("create sets scanning off and disallowUnspecifiedMode", func(t *testing.T) {
		env.WaitReady(gvk, namespace, name)
		got := get(t, name)
		if c := got.GetVulnerabilityScanningConfig(); c.GetEnablementConfig() != pb.Repository_VulnerabilityScanningConfig_DISABLED || c.GetEnablementState() != pb.Repository_VulnerabilityScanningConfig_SCANNING_DISABLED {
			t.Errorf("cloud vulnerabilityScanningConfig = %v, want DISABLED", c)
		}
		if !got.GetDisallowUnspecifiedMode() {
			t.Error("cloud disallowUnspecifiedMode is false")
		}
	})

	t.Run("a resync sends no update", func(t *testing.T) {
		if n := resyncPatches(t, name); n != 0 {
			t.Errorf("a resync sent %d updates", n)
		}
	})

	t.Run("a changed scanning config is sent alone", func(t *testing.T) {
		if err := update(t, name, func(o *arv1beta1.ArtifactRegistryRepository) {
			o.Spec.VulnerabilityScanningConfig.EnablementConfig = mockenv.Ptr("INHERITED")
		}); err != nil {
			t.Fatal(err)
		}
		deadline := time.Now().Add(mockenv.Timeout())
		for get(t, name).GetVulnerabilityScanningConfig().GetEnablementConfig() != pb.Repository_VulnerabilityScanningConfig_INHERITED {
			if time.Now().After(deadline) {
				t.Fatalf("cloud vulnerabilityScanningConfig = %v, want INHERITED", get(t, name).GetVulnerabilityScanningConfig())
			}
			time.Sleep(time.Second)
		}
		env.WaitReady(gvk, namespace, name)
		masks := updateMasks(t, env, name)
		if len(masks) == 0 || masks[len(masks)-1] != "vulnerabilityScanningConfig" {
			t.Errorf("update masks = %v, want the last to be vulnerabilityScanningConfig alone", masks)
		}
		if n := resyncPatches(t, name); n != 0 {
			t.Errorf("a resync after the change sent %d updates", n)
		}
	})

	t.Run("a spec without the scanning config leaves the repository's", func(t *testing.T) {
		if err := update(t, name, func(o *arv1beta1.ArtifactRegistryRepository) {
			o.Spec.VulnerabilityScanningConfig = nil
		}); err != nil {
			t.Fatal(err)
		}
		if n := resyncPatches(t, name); n != 0 {
			t.Errorf("taking the scanning config off the spec sent %d updates", n)
		}
		if got := get(t, name).GetVulnerabilityScanningConfig().GetEnablementConfig(); got != pb.Repository_VulnerabilityScanningConfig_INHERITED {
			t.Errorf("cloud enablementConfig = %v, want INHERITED kept", got)
		}
	})

	t.Run("disallowUnspecifiedMode is immutable", func(t *testing.T) {
		err := update(t, name, func(o *arv1beta1.ArtifactRegistryRepository) {
			o.Spec.DisallowUnspecifiedMode = mockenv.Ptr(false)
		})
		if err == nil || !strings.Contains(err.Error(), "disallowUnspecifiedMode is immutable") {
			t.Errorf("changing disallowUnspecifiedMode: %v, want refused", err)
		}
	})

	t.Run("a repository without the fields sends no update", func(t *testing.T) {
		plain := mockenv.Name("no-scanning")
		if err := env.Kube.Create(env.Ctx, newRepository(plain)); err != nil {
			t.Fatal(err)
		}
		env.WaitReady(gvk, namespace, plain)
		if n := resyncPatches(t, plain); n != 0 {
			t.Errorf("a resync of a repository without the fields sent %d updates", n)
		}
		o := getObj(t, plain)
		if err := env.Kube.Delete(env.Ctx, o); err != nil {
			t.Fatal(err)
		}
		env.WaitGone(o)
	})

	t.Run("delete removes the repository", func(t *testing.T) {
		o := getObj(t, name)
		if err := env.Kube.Delete(env.Ctx, o); err != nil {
			t.Fatal(err)
		}
		env.WaitGone(o)
	})
}

func TestArtifactRegistryAptYumRemoteRepositories(t *testing.T) {
	env := mockenv.New(t, namespace)
	cloud := newClient(t, env)

	cases := []struct {
		name, format string
		remote       *arv1beta1.ArtifactRegistryRepositoryRemoteRepositoryConfig
		check        func(*pb.RemoteRepositoryConfig) bool
	}{
		{
			name: "apt-public", format: "APT",
			remote: &arv1beta1.ArtifactRegistryRepositoryRemoteRepositoryConfig{
				Description: mockenv.Ptr("Debian bookworm"),
				AptRepository: &arv1beta1.ArtifactRegistryRepositoryAptRepository{PublicRepository: &arv1beta1.ArtifactRegistryRepositoryAptRepositoryPublicRepository{
					RepositoryBase: mockenv.Ptr("DEBIAN"), RepositoryPath: mockenv.Ptr("debian/dists/bookworm"),
				}},
			},
			check: func(c *pb.RemoteRepositoryConfig) bool {
				p := c.GetAptRepository().GetPublicRepository()
				return p.GetRepositoryBase() == pb.RemoteRepositoryConfig_AptRepository_PublicRepository_DEBIAN && p.GetRepositoryPath() == "debian/dists/bookworm"
			},
		},
		{
			name: "yum-public", format: "YUM",
			remote: &arv1beta1.ArtifactRegistryRepositoryRemoteRepositoryConfig{
				Description: mockenv.Ptr("Rocky Linux 9"),
				YumRepository: &arv1beta1.ArtifactRegistryRepositoryYumRepository{PublicRepository: &arv1beta1.ArtifactRegistryRepositoryYumRepositoryPublicRepository{
					RepositoryBase: mockenv.Ptr("ROCKY"), RepositoryPath: mockenv.Ptr("pub/rocky/9/BaseOS/x86_64/os"),
				}},
			},
			check: func(c *pb.RemoteRepositoryConfig) bool {
				p := c.GetYumRepository().GetPublicRepository()
				return p.GetRepositoryBase() == pb.RemoteRepositoryConfig_YumRepository_PublicRepository_ROCKY && p.GetRepositoryPath() == "pub/rocky/9/BaseOS/x86_64/os"
			},
		},
		{
			// example.com is no Yum repository: validation is off (see
			// dockerUpstream).
			name: "yum-custom", format: "YUM",
			remote: &arv1beta1.ArtifactRegistryRepositoryRemoteRepositoryConfig{
				Description:               mockenv.Ptr("private Yum mirror"),
				DisableUpstreamValidation: mockenv.Ptr(true),
				YumRepository: &arv1beta1.ArtifactRegistryRepositoryYumRepository{CustomRepository: &arv1beta1.ArtifactRegistryRepositoryYumRepositoryCustomRepository{
					URI: mockenv.Ptr("https://yum.example.com/"),
				}},
			},
			check: func(c *pb.RemoteRepositoryConfig) bool {
				return c.GetYumRepository().GetCustomRepository().GetUri() == "https://yum.example.com/"
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			name := mockenv.Name(tc.name)
			repo := newRemoteRepository(name, tc.format, tc.remote)
			if err := env.Kube.Create(env.Ctx, repo); err != nil {
				t.Fatal(err)
			}
			env.WaitReady(gvk, namespace, name)
			got, err := cloud.GetRepository(env.Ctx, &pb.GetRepositoryRequest{Name: cloudName(name)})
			if err != nil {
				t.Fatal(err)
			}
			if !tc.check(got.GetRemoteRepositoryConfig()) {
				t.Errorf("cloud remoteRepositoryConfig = %v", got.GetRemoteRepositoryConfig())
			}
			patches := count(env, "PATCH", cloudName(name))
			o := &arv1beta1.ArtifactRegistryRepository{}
			err = retry.RetryOnConflict(retry.DefaultRetry, func() error {
				if err := env.Kube.Get(env.Ctx, types.NamespacedName{Namespace: namespace, Name: name}, o); err != nil {
					return err
				}
				o.Annotations[k8s.InternalForceReconcileAnnotation] = "1"
				return env.Kube.Update(env.Ctx, o)
			})
			if err != nil {
				t.Fatal(err)
			}
			time.Sleep(3 * time.Second)
			env.WaitReady(gvk, namespace, name)
			if n := count(env, "PATCH", cloudName(name)) - patches; n != 0 {
				t.Errorf("a resync sent %d updates", n)
			}
			if err := env.Kube.Delete(env.Ctx, o); err != nil {
				t.Fatal(err)
			}
			env.WaitGone(o)
		})
	}

	t.Run("the CRD refuses a public and a custom upstream together", func(t *testing.T) {
		bad := newRemoteRepository(mockenv.Name("apt-both"), "APT", &arv1beta1.ArtifactRegistryRepositoryRemoteRepositoryConfig{
			AptRepository: &arv1beta1.ArtifactRegistryRepositoryAptRepository{
				PublicRepository: &arv1beta1.ArtifactRegistryRepositoryAptRepositoryPublicRepository{RepositoryBase: mockenv.Ptr("UBUNTU"), RepositoryPath: mockenv.Ptr("ubuntu")},
				CustomRepository: &arv1beta1.ArtifactRegistryRepositoryAptRepositoryCustomRepository{URI: mockenv.Ptr("https://apt.example.com/")},
			},
		})
		if err := env.Kube.Create(env.Ctx, bad); err == nil || !strings.Contains(err.Error(), "set one of publicRepository and customRepository") {
			t.Errorf("creating both: %v, want refused", err)
		}
	})
}
