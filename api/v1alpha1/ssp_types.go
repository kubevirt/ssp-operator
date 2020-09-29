/*


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
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type TemplateValidator struct {
	// Replicas is the number of replicas of the template validator pod
	Replicas int32 `json:"replicas"`

	// Node Affinity affinity for TemplateValidator pods
	Affinity *v1.Affinity `json:"affinity,omitempty"`

	// NodeSelector labels for TemplateValidator
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`

	// Tolerations for TemplateValidator
	Tolerations []v1.Toleration `json:"tolerations,omitempty"`
}

// SSPSpec defines the desired state of SSP
type SSPSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// TemplateValidator is configuration of the template validator operand
	TemplateValidator TemplateValidator `json:"templateValidator"`
}

// SSPStatus defines the observed state of SSP
type SSPStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// SSP is the Schema for the ssps API
type SSP struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SSPSpec   `json:"spec,omitempty"`
	Status SSPStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// SSPList contains a list of SSP
type SSPList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []SSP `json:"items"`
}

func init() {
	SchemeBuilder.Register(&SSP{}, &SSPList{})
}
