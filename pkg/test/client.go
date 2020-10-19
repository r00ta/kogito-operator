// Copyright 2019 Red Hat, Inc. and/or its affiliates
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

package test

import (
	"github.com/RHsyseng/operator-utils/pkg/resource"
	"github.com/kiegroup/kogito-cloud-operator/pkg/client"
	"github.com/kiegroup/kogito-cloud-operator/pkg/client/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/discovery"
	discfake "k8s.io/client-go/discovery/fake"
	clienttesting "k8s.io/client-go/testing"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	imgfake "github.com/openshift/client-go/image/clientset/versioned/fake"
)

// FakeClientBuilder create client object for tests
type FakeClientBuilder interface {
	AddK8sObjects(objects []runtime.Object) FakeClientBuilder
	AddImageObjects(imageObjs []runtime.Object) FakeClientBuilder
	AddBuildObjects(buildObjs []runtime.Object) FakeClientBuilder
	OnOpenShift() FakeClientBuilder
	SupportPrometheus() FakeClientBuilder
	Build() *client.Client
}

// NewFakeClientBuilder provide new object FakeClientBuilder
func NewFakeClientBuilder() FakeClientBuilder {
	return &fakeClientStruct{}
}

type fakeClientStruct struct {
	objects    []runtime.Object
	imageObjs  []runtime.Object
	buildObjs  []runtime.Object
	openShift  bool
	prometheus bool
}

// AddK8sObjects ...
func (f *fakeClientStruct) AddK8sObjects(objects []runtime.Object) FakeClientBuilder {
	f.objects = objects
	return f
}

// AddImageObjects add image objects
func (f *fakeClientStruct) AddImageObjects(imageObjs []runtime.Object) FakeClientBuilder {
	f.imageObjs = imageObjs
	return f
}

// AddBuildObjects add build object
func (f *fakeClientStruct) AddBuildObjects(buildObjs []runtime.Object) FakeClientBuilder {
	f.buildObjs = buildObjs
	return f
}

func (f *fakeClientStruct) SupportPrometheus() FakeClientBuilder {
	f.prometheus = true
	return f
}

// OnOpenShift ...
func (f *fakeClientStruct) OnOpenShift() FakeClientBuilder {
	f.openShift = true
	return f
}

// Build ...
func (f *fakeClientStruct) Build() *client.Client {
	// Create a fake client to mock API calls.
	cli := fake.NewFakeClientWithScheme(meta.GetRegisteredSchema(), f.objects...)
	// OpenShift Image Client Fake with image tag defined and image built
	imgCli := imgfake.NewSimpleClientset(f.imageObjs...).ImageV1()
	// OpenShift Build Client Fake with build for s2i defined, since we'll trigger a build during the reconcile phase
	buildCli := newBuildFake(f.buildObjs...)

	return &client.Client{
		ControlCli: cli,
		BuildCli:   buildCli,
		ImageCli:   imgCli,
		Discovery:  f.createFakeDiscoveryClient(),
	}
}

// CreateFakeClient will create a fake client for mock test on Kubernetes env, use cases that depends on OpenShift should use CreateFakeClientOnOpenShift
func CreateFakeClient(objects []runtime.Object, imageObjs []runtime.Object, buildObjs []runtime.Object) *client.Client {
	return NewFakeClientBuilder().AddK8sObjects(objects).AddImageObjects(imageObjs).AddBuildObjects(buildObjs).Build()
}

// CreateFakeClientOnOpenShift same as CreateFakeClientWithDisco setting openshift flag to true
func CreateFakeClientOnOpenShift(objects []runtime.Object, imageObjs []runtime.Object, buildObjs []runtime.Object) *client.Client {
	return NewFakeClientBuilder().AddK8sObjects(objects).AddImageObjects(imageObjs).AddBuildObjects(buildObjs).OnOpenShift().Build()
}

// CreateFakeDiscoveryClient creates a fake discovery client that supports prometheus, infinispan, strimzi api
func (f *fakeClientStruct) createFakeDiscoveryClient() discovery.DiscoveryInterface {
	disco := &discfake.FakeDiscovery{
		Fake: &clienttesting.Fake{
			Resources: []*metav1.APIResourceList{
				{GroupVersion: "infinispan.org/v1"},
				{GroupVersion: "kafka.strimzi.io/v1beta1"},
				{GroupVersion: "keycloak.org/v1alpha1"},
				{GroupVersion: "intergr8ly.org/v1alpha1"},
			},
		},
	}

	if f.prometheus {
		disco.Fake.Resources = append(disco.Fake.Resources,
			&metav1.APIResourceList{GroupVersion: "monitoring.coreos.com/v1alpha1"})
	}

	if f.openShift {
		disco.Fake.Resources = append(disco.Fake.Resources,
			&metav1.APIResourceList{GroupVersion: "openshift.io/v1"},
			&metav1.APIResourceList{GroupVersion: "build.openshift.io/v1"})
	}
	return disco
}

// ToRuntimeObjects converts RHSysUtils array KubernetesResource into k8s runtime.Object array
func ToRuntimeObjects(resources ...resource.KubernetesResource) []runtime.Object {
	var k8sObject []runtime.Object
	for _, resource := range resources {
		k8sObject = append(k8sObject, resource)
	}
	return k8sObject
}
