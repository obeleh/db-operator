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

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// DbCopyCronJobSpec defines the desired state of DbCopyCronJob
type DbCopyCronJobSpec struct {
	FromDbName string `json:"from_db_name"`
	ToDbName   string `json:"to_db_name"`
	Interval   string `json:"interval"`
}

// DbCopyCronJobStatus defines the observed state of DbCopyCronJob
type DbCopyCronJobStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// DbCopyCronJob is the Schema for the dbcopycronjobs API
type DbCopyCronJob struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   DbCopyCronJobSpec   `json:"spec,omitempty"`
	Status DbCopyCronJobStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// DbCopyCronJobList contains a list of DbCopyCronJob
type DbCopyCronJobList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []DbCopyCronJob `json:"items"`
}

func init() {
	SchemeBuilder.Register(&DbCopyCronJob{}, &DbCopyCronJobList{})
}
