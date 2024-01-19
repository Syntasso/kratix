/*
Copyright 2021 Syntasso.

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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	BasicAuthMethod = "basicAuth"
	SSHAuthMethod   = "ssh"
)

// GitStateStoreSpec defines the desired state of GitStateStore
type GitStateStoreSpec struct {
	URL string `json:"url,omitempty"`

	StateStoreCoreFields `json:",inline"`

	//+kubebuilder:validation:Optional
	//+kubebuilder:default=main
	Branch string `json:"branch,omitempty"`

	// AuthMethod used to access the StateStore
	//+kubebuilder:validation:Enum=basicAuth;ssh
	//+kubebuilder:default:=basicAuth
	AuthMethod string `json:"authMethod,omitempty"`
}

// GitStateStoreStatus defines the observed state of GitStateStore
type GitStateStoreStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of GitStateStore
	// Important: Run "make" to regenerate code after modifying this file
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:scope=Cluster,path=gitstatestores

// GitStateStore is the Schema for the gitstatestores API
type GitStateStore struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   GitStateStoreSpec   `json:"spec,omitempty"`
	Status GitStateStoreStatus `json:"status,omitempty"`
}

func (g *GitStateStore) GetSecretRef() *corev1.SecretReference {
	return g.Spec.SecretRef
}

//+kubebuilder:object:root=true

// GitStateStoreList contains a list of GitStateStore
type GitStateStoreList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []GitStateStore `json:"items"`
}

func init() {
	SchemeBuilder.Register(&GitStateStore{}, &GitStateStoreList{})
}
