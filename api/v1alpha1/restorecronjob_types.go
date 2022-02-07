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

type RestoreCronJobSpec struct {
	RestoreTarget string  `json:"restore_target"`
	Interval      string  `json:"interval"`
	FixedFileName *string `json:"fixed_file_name,omitempty"`
	Suspend       bool    `json:"suspend"`
}

// RestoreCronJobStatus defines the observed state of RestoreCronJob
type RestoreCronJobStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// RestoreCronJob is the Schema for the restorecronjobs API
type RestoreCronJob struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RestoreCronJobSpec   `json:"spec,omitempty"`
	Status RestoreCronJobStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// RestoreCronJobList contains a list of RestoreCronJob
type RestoreCronJobList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []RestoreCronJob `json:"items"`
}

func init() {
	SchemeBuilder.Register(&RestoreCronJob{}, &RestoreCronJobList{})
}
