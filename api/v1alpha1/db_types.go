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

// DbSpec defines the desired state of Db
type DbSpec struct {
	Server         string `json:"server"`
	DbName         string `json:"db_name"`
	DropOnDeletion bool   `json:"drop_on_deletion"`
	CascadeOnDrop  bool   `json:"cascade_on_drop,omitempty"`
	AfterCreateSQL string `json:"after_create_sql,omitempty"`
}

// DbStatus defines the observed state of Db
type DbStatus struct {
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// Db is the Schema for the dbs API
type Db struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   DbSpec   `json:"spec,omitempty"`
	Status DbStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// DbList contains a list of Db
type DbList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Db `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Db{}, &DbList{})
}
