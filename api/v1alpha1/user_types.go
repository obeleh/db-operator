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

type DbPriv struct {
	Scope        string `json:"scope"`
	Privs        string `json:"privs"`
	DefaultPrivs string `json:"default_privs,omitempty"`
	Grantor      string `json:"grantor_user_name,omitempty"`
}

// UserSpec defines the desired state of User
type UserSpec struct {
	UserName     string   `json:"user_name"`
	SecretName   string   `json:"secret_name"`
	PasswordKey  string   `json:"password_key,omitempty"` // defaults to password
	CaCertKey    string   `json:"ca_cert_key,omitempty"`
	TlsCrtKey    string   `json:"tls_cert_key,omitempty"`
	TlsKeyKey    string   `json:"tls_key_key,omitempty"`
	DbServerName string   `json:"db_server_name"`
	DbPrivs      []DbPriv `json:"db_privs"`
	ServerPrivs  string   `json:"server_privs"`
}

// UserStatus defines the observed state of User
type UserStatus struct {
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// User is the Schema for the users API
type User struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   UserSpec   `json:"spec,omitempty"`
	Status UserStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// UserList contains a list of User
type UserList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []User `json:"items"`
}

func init() {
	SchemeBuilder.Register(&User{}, &UserList{})
}
