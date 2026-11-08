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

//go:build fakecloud

package main

// The fake-cloud switch, for tests only. It is compiled in only with
// -tags fakecloud (tests/operator/run.sh builds the test images that way);
// the default build has neither the --fake-cloud-url flag nor pkg/fakecloud.

import (
	"flag"
	"net/http"
	"net/url"

	"github.com/GoogleCloudPlatform/k8s-config-connector/pkg/config"
	"k8s.io/klog/v2"

	"github.com/highfivelabs/gaps-gcp/pkg/fakecloud"
)

func init() {
	fakeCloudURL := flag.String("fake-cloud-url", "", "for tests only: send every Google API call to this fake cloud (cmd/fakegcp) instead of Google; no Google credentials are loaded")
	useFakeCloud = func(cfg *config.ControllerConfig) {
		if *fakeCloudURL == "" {
			return
		}
		u, err := url.Parse(*fakeCloudURL)
		if err != nil || u.Scheme == "" || u.Host == "" {
			klog.Fatalf("--fake-cloud-url %q is not a URL", *fakeCloudURL)
		}
		// With HTTPClient set, the Google client libraries add no
		// credentials: none are loaded, so none can reach the fake.
		cfg.HTTPClient = &http.Client{Transport: fakecloud.Redirect(u)}
		klog.Warningf("using the fake cloud at %s: no Google credentials loaded (tests only)", u)
	}
}
