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

// CockroachDBBackupJobSpec defines the desired state of CockroachDBBackupJob
type CockroachDBBackupJobSpec struct {
	BackupTarget string `json:"backup_target"`
}

// CockroachDBBackupJobStatus defines the observed state of CockroachDBBackupJob
type CockroachDBBackupJobStatus struct {
	JobId       string      `json:"job_id"`
	Status      string      `json:"status"`
	Description string      `json:"description"`
	Created     metav1.Time `json:"created"`
	Started     metav1.Time `json:"started"`
	Finished    metav1.Time `json:"finished"`
	Error       string      `json:"error"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// CockroachDBBackupJob is the Schema for the cockroachdbbackupjobs API
type CockroachDBBackupJob struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   CockroachDBBackupJobSpec   `json:"spec,omitempty"`
	Status CockroachDBBackupJobStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// CockroachDBBackupJobList contains a list of CockroachDBBackupJob
type CockroachDBBackupJobList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []CockroachDBBackupJob `json:"items"`
}

func init() {
	SchemeBuilder.Register(&CockroachDBBackupJob{}, &CockroachDBBackupJobList{})
}
