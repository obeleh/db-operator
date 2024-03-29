/*
Copyright 2022.

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

type DbCopyJobSpec struct {
	FromDbName     string `json:"from_db_name"`
	ToDbName       string `json:"to_db_name"`
	ServiceAccount string `json:"service_account,omitempty"`
}

// DbCopyJobStatus defines the observed state of DbCopyJob
type DbCopyJobStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// DbCopyJob is the Schema for the dbcopyjobs API
type DbCopyJob struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   DbCopyJobSpec   `json:"spec,omitempty"`
	Status DbCopyJobStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// DbCopyJobList contains a list of DbCopyJob
type DbCopyJobList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []DbCopyJob `json:"items"`
}

func init() {
	SchemeBuilder.Register(&DbCopyJob{}, &DbCopyJobList{})
}
