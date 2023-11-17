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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// PipelineSpec defines the desired state of Pipeline
type PipelineSpec struct {
	Containers []Container `json:"containers,omitempty"`
}

type Container struct {
	Name    string   `json:"name,omitempty"`
	Image   string   `json:"image,omitempty"`
	Args    []string `json:"args,omitempty"`
	Command []string `json:"command,omitempty"`
}

// Pipeline is the Schema for the pipelines API
type Pipeline struct {
	//Note: Removed TypeMeta in order to stop the CRD generation.
	//		This is only for internal Kratix use.
	//metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec PipelineSpec `json:"spec,omitempty"`
}
