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

// CockroachDBBackupCronJobSpec defines the desired state of CockroachDBBackupCronJob
type CockroachDBBackupCronJobSpec struct {
	Interval     string `json:"interval"`
	Suspend      bool   `json:"suspend"`
	Creator      string `json:"creator"`
	BackupTarget string `json:"backup_target"`
}

// CockroachDBBackupCronJobStatus defines the observed state of CockroachDBBackupCronJob
type CockroachDBBackupCronJobStatus struct {
	ScheduleId     int64  `json:"schedule_id"`
	ScheduleStatus string `json:"schedule_status"`
	State          string `json:"state"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// CockroachDBBackupCronJob is the Schema for the cockroachdbbackupcronjobs API
type CockroachDBBackupCronJob struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   CockroachDBBackupCronJobSpec   `json:"spec,omitempty"`
	Status CockroachDBBackupCronJobStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// CockroachDBBackupCronJobList contains a list of CockroachDBBackupCronJob
type CockroachDBBackupCronJobList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []CockroachDBBackupCronJob `json:"items"`
}

func init() {
	SchemeBuilder.Register(&CockroachDBBackupCronJob{}, &CockroachDBBackupCronJobList{})
}
