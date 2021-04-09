/*
Copyright 2021.

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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type S3StorageSpec struct {
	Foo string `json:"foo,omitempty"`
}

// S3StorageStatus defines the observed state of S3Storage
type S3StorageStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// S3Storage is the Schema for the s3storages API
type S3Storage struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   S3StorageSpec   `json:"spec,omitempty"`
	Status S3StorageStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// S3StorageList contains a list of S3Storage
type S3StorageList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []S3Storage `json:"items"`
}

func init() {
	SchemeBuilder.Register(&S3Storage{}, &S3StorageList{})
}
