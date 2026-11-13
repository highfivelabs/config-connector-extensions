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

package logging_test

// LoggingBillingAccountSink's destination in each form a bucket is written
// in, on the fake cloud: a Cloud Storage bucket as projects/_/buckets/name,
// its bare name, storage.googleapis.com/name, gs://name and a StorageBucket
// object by name, and a log bucket with and without its
// logging.googleapis.com/ prefix. The catalog's billing-account chart sends
// projects/_/buckets/name; fabric and Terraform write
// storage.googleapis.com/name. Fake only: the forms are the controller's,
// and live waits on a billing account (TestLoggingBillingAccountSink).

import (
	"testing"

	gcp "cloud.google.com/go/logging/apiv2"
	pb "cloud.google.com/go/logging/apiv2/loggingpb"
	billingv1alpha1 "github.com/GoogleCloudPlatform/k8s-config-connector/apis/billing/v1alpha1"
	loggingv1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/apis/logging/v1beta1"
	storagev1beta1 "github.com/GoogleCloudPlatform/k8s-config-connector/apis/storage/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	loggingv1alpha1 "github.com/highfivelabs/config-connector-extensions/apis/logging/v1alpha1"
	"github.com/highfivelabs/config-connector-extensions/pkg/test/mockenv"
)

func TestLoggingBillingAccountSinkDestinations(t *testing.T) {
	if mockenv.Live() {
		t.Skip("fake only: the destination forms are the controller's; TestLoggingBillingAccountSink runs live with a billing account")
	}
	const account = "billingAccounts/012345-6789AB-CDEF01"
	env := mockenv.New(t, namespace)
	cloud, err := gcp.NewConfigRESTClient(env.Ctx, env.GCPOptions()...)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cloud.Close() })

	const bucket = "cat-billing-logs"
	logBucket := "projects/" + mockenv.Project + "/locations/global/buckets/billing"

	// A StorageBucket object, as Config Connector's would be (its
	// controller does not run here): the reference resolves it from its
	// name and project.
	sb := &unstructured.Unstructured{}
	sb.SetGroupVersionKind(storagev1beta1.StorageBucketGVK)
	sb.SetNamespace(namespace)
	sb.SetName(bucket)
	sb.SetAnnotations(map[string]string{"cnrm.cloud.google.com/project-id": mockenv.Project})
	sb.Object["spec"] = map[string]any{"location": "US"}
	if err := env.Kube.Create(env.Ctx, sb); err != nil {
		t.Fatal(err)
	}
	if err := unstructured.SetNestedField(sb.Object, "projects/"+mockenv.Project+"/buckets/"+bucket, "status", "externalRef"); err != nil {
		t.Fatal(err)
	}
	if err := env.Kube.Status().Update(env.Ctx, sb); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name string
		dest loggingv1beta1.LogSinkDestination
		want string
	}{
		{"underscore-project", loggingv1beta1.LogSinkDestination{StorageBucketRef: &storagev1beta1.StorageBucketRef{External: "projects/_/buckets/" + bucket}}, "storage.googleapis.com/" + bucket},
		{"bare-name", loggingv1beta1.LogSinkDestination{StorageBucketRef: &storagev1beta1.StorageBucketRef{External: bucket}}, "storage.googleapis.com/" + bucket},
		{"service-prefix", loggingv1beta1.LogSinkDestination{StorageBucketRef: &storagev1beta1.StorageBucketRef{External: "storage.googleapis.com/" + bucket}}, "storage.googleapis.com/" + bucket},
		{"gs-url", loggingv1beta1.LogSinkDestination{StorageBucketRef: &storagev1beta1.StorageBucketRef{External: "gs://" + bucket}}, "storage.googleapis.com/" + bucket},
		{"bucket-object", loggingv1beta1.LogSinkDestination{StorageBucketRef: &storagev1beta1.StorageBucketRef{Name: bucket}}, "storage.googleapis.com/" + bucket},
		{"log-bucket", loggingv1beta1.LogSinkDestination{LoggingLogBucketRef: &loggingv1beta1.LoggingLogBucketRef{External: logBucket}}, "logging.googleapis.com/" + logBucket},
		{"log-bucket-prefix", loggingv1beta1.LogSinkDestination{LoggingLogBucketRef: &loggingv1beta1.LoggingLogBucketRef{External: "logging.googleapis.com/" + logBucket}}, "logging.googleapis.com/" + logBucket},
	}
	var objs []*loggingv1alpha1.LoggingBillingAccountSink
	for _, c := range cases {
		o := &loggingv1alpha1.LoggingBillingAccountSink{
			ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: c.name},
			Spec: loggingv1alpha1.LoggingBillingAccountSinkSpec{
				BillingAccountRef: billingv1alpha1.BillingAccountRef{External: account},
				Destination:       c.dest,
				Filter:            mockenv.Ptr(`logName:"cloudaudit.googleapis.com"`),
			},
		}
		if err := env.Kube.Create(env.Ctx, o); err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		objs = append(objs, o)
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			env.WaitReady(loggingv1alpha1.LoggingBillingAccountSinkGVK, namespace, c.name)
			got, err := cloud.GetSink(env.Ctx, &pb.GetSinkRequest{SinkName: account + "/sinks/" + c.name})
			if err != nil {
				t.Fatalf("reading the sink: %v", err)
			}
			if got.GetDestination() != c.want {
				t.Errorf("destination = %q, want %q", got.GetDestination(), c.want)
			}
		})
	}
	for _, o := range objs {
		if err := env.Kube.Delete(env.Ctx, o); err != nil {
			t.Fatal(err)
		}
		env.WaitGone(o)
	}
	if err := env.Kube.Delete(env.Ctx, sb); err != nil {
		t.Fatal(err)
	}
}
