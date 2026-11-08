// Copyright 2026 High Five Labs
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

// Package fakecloud puts Config Connector's mockgcp behind a network
// endpoint, so a manager running in a cluster can use it. mockgcp is an
// in-process http.RoundTripper; Handler serves it over plain HTTP, and
// Redirect is the manager's side: every Google API request goes to the
// fake cloud's URL, with the host it was meant for in a header.
//
// For tests only. A manager pointed at a fake cloud loads no Google
// credentials (its HTTP client is set, so the Google client libraries add
// no authentication), so nothing real can leak to it.
package fakecloud

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
)

// HostHeader carries the Google API host a request was meant for.
const HostHeader = "X-Fake-Cloud-Host"

// Redirect returns a RoundTripper that sends every request to base,
// keeping its path and query, with the original host in HostHeader.
func Redirect(base *url.URL) http.RoundTripper {
	return &redirect{base: base, inner: http.DefaultTransport}
}

type redirect struct {
	base  *url.URL
	inner http.RoundTripper
}

func (r *redirect) RoundTrip(req *http.Request) (*http.Response, error) {
	out := req.Clone(req.Context())
	out.Header.Set(HostHeader, req.URL.Host)
	u := *req.URL
	u.Scheme = r.base.Scheme
	u.Host = r.base.Host
	out.URL = &u
	out.Host = r.base.Host
	return r.inner.RoundTrip(out)
}

// Handler serves a fake cloud's RoundTripper: it rebuilds each request for
// the host in HostHeader, over https as the client meant it, and returns
// the fake cloud's answer.
func Handler(cloud http.RoundTripper) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		host := req.Header.Get(HostHeader)
		if host == "" {
			http.Error(w, fmt.Sprintf("missing %s header", HostHeader), http.StatusBadRequest)
			return
		}
		u := *req.URL
		u.Scheme = "https"
		u.Host = host
		in := req.Clone(req.Context())
		in.RequestURI = ""
		in.URL = &u
		in.Host = host
		in.Header.Del(HostHeader)
		resp, err := cloud.RoundTrip(in)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		defer resp.Body.Close()
		for k, vs := range resp.Header {
			for _, v := range vs {
				w.Header().Add(k, v)
			}
		}
		w.WriteHeader(resp.StatusCode)
		_, _ = io.Copy(w, resp.Body)
	})
}
