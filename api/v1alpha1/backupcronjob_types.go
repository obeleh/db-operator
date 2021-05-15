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

type BackupCronJobSpec struct {
	BackupTarget  string  `json:"backup_target"`
	FixedFileName *string `json:"fixed_file_name,omitempty"`
	Interval      string  `json:"interval"`
}

// BackupCronJobStatus defines the observed state of BackupCronJob
type BackupCronJobStatus struct {
	Exists      bool   `json:"exists"`
	CronJobName string `json:"cronjob_name"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// BackupCronJob is the Schema for the backupcronjobs API
type BackupCronJob struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   BackupCronJobSpec   `json:"spec,omitempty"`
	Status BackupCronJobStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// BackupCronJobList contains a list of BackupCronJob
type BackupCronJobList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []BackupCronJob `json:"items"`
}

func init() {
	SchemeBuilder.Register(&BackupCronJob{}, &BackupCronJobList{})
}
