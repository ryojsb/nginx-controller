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

package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// NginxSpec defines the desired state of Nginx
type NginxSpec struct {
	// +kubebuiler:validation:Required
	// +kubebuiler:validation:Format := string
	DeploymentName string `json:"deploymentName"`

	// +kubebuiler:validation:Required
	// +kubebuilder:validation:Minimum=0
	Replicas *int32 `json:"replicas"`
}

// NginxStatus defines the observed state of Nginx
type NginxStatus struct {
	//+option
	AvailableReplicas int32 `json:"availableReplicas"`

	// Deployment Status（独自追加）
	//+option
	DeploymentStatus string `json:"deploymentStatus"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:scope=Namespaced
//+kubebuilder:resource:shortName="nx"
//+kubebuilder:printcolumn:name="Deployment",type="string",JSONPath=".spec.deploymentName"
//+kubebuilder:printcolumn:name="AvailableReplicas",type="integer",JSONPath=".status.availableReplicas"
//+kubebuilder:printcolumn:name="DeploymentStatus",type="string",JSONPath=".status.deploymentStatus"

// Nginx is the Schema for the nginxes API
type Nginx struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   NginxSpec   `json:"spec,omitempty"`
	Status NginxStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// NginxList contains a list of Nginx
type NginxList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Nginx `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Nginx{}, &NginxList{})
}
