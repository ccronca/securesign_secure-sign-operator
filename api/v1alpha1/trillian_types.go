/*
Copyright 2023.

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

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// TrillianSpec defines the desired state of Trillian
type TrillianSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Foo is an example field of Trillian. Edit trillian_types.go to remove/update
	LogSignerImage string `json:"logSignerImage,omitempty"`
	ServerImage    string `json:"serverImage,omitempty"`
	DbImage        string `json:"dbImage,omitempty"`
	PvcName        string `json:"pvcName,omitempty"`
}

// TrillianStatus defines the observed state of Trillian
type TrillianStatus struct {
	Phase Phase `json:"Phase"`
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// Trillian is the Schema for the trillians API
type Trillian struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   TrillianSpec   `json:"spec,omitempty"`
	Status TrillianStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// TrillianList contains a list of Trillian
type TrillianList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Trillian `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Trillian{}, &TrillianList{})
}