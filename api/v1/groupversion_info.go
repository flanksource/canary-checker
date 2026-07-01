/*
Copyright 2020 The Kubernetes authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package v1 contains API Schema definitions for the canaries v1 API group
// +kubebuilder:object:generate:=true
// +groupName=canaries.flanksource.com
package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var (
	// GroupVersion is group version used to register these objects
	GroupVersion = schema.GroupVersion{Group: "canaries.flanksource.com", Version: "v1"}

	// SchemeBuilder is used to add go types to the GroupVersionKind scheme
	SchemeBuilder = &schemeBuilder{GroupVersion: GroupVersion}

	// AddToScheme adds the types in this group-version to the given scheme.
	AddToScheme = SchemeBuilder.AddToScheme
)

type schemeBuilder struct {
	GroupVersion schema.GroupVersion
	objects      []runtime.Object
}

func (b *schemeBuilder) Register(objects ...runtime.Object) {
	b.objects = append(b.objects, objects...)
}

func (b *schemeBuilder) AddToScheme(s *runtime.Scheme) error {
	s.AddKnownTypes(b.GroupVersion, b.objects...)
	metav1.AddToGroupVersion(s, b.GroupVersion)
	return nil
}
