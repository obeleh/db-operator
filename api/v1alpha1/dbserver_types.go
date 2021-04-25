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

type DbServerSpec struct {
	// Server address
	Address string `json:"address"`
	// Server port
	// +kubebuilder:validation:Minimum:=1
	Port int `json:"port"`
	// +kubebuilder:validation:MinLength=1
	UserName string `json:"user_name"`
	// +kubebuilder:validation:MinLength=1
	SecretName string `json:"secret_name"`
	SecretKey  string `json:"secret_key,omitempty"`
	Version    string `json:"version,omitempty"`
	ServerType string `json:"server_type"`
}

// DbServerStatus defines the observed state of DbServer
type DbServerStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// DbServer is the Schema for the dbservers API
type DbServer struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   DbServerSpec   `json:"spec,omitempty"`
	Status DbServerStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// DbServerList contains a list of DbServer
type DbServerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []DbServer `json:"items"`
}

func init() {
	SchemeBuilder.Register(&DbServer{}, &DbServerList{})
}
