/*
Copyright 2022 VerwaerdeWim.

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

// SynkSourceSpec defines the desired state of SynkSource
type SynkSourceSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	Resources []Resource `json:"resources"`
}

// SynkSourceStatus defines the observed state of SynkSource
type SynkSourceStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

type Resource struct {
	Group        string `json:"group"`
	Version      string `json:"version"`
	ResourceType string `json:"resource"`
	Namespace    string `json:"namespace"`
	//+optional
	Names []string `json:"names"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// SynkSource is the Schema for the synksources API
type SynkSource struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SynkSourceSpec   `json:"spec,omitempty"`
	Status SynkSourceStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// SynkSourceList contains a list of SynkSource
type SynkSourceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []SynkSource `json:"items"`
}

func init() {
	SchemeBuilder.Register(&SynkSource{}, &SynkSourceList{})
}
