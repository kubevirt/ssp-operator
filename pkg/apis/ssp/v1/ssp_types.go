package v1

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
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	// Add custom validation using kubebuilder tags: https://book-v1.book.kubebuilder.io/beyond_basics/generating_crd.html

	// TemplateValidator is configuration of the template validator operand
	TemplateValidator TemplateValidator `json:"templateValidator"`
}

// SSPStatus defines the observed state of SSP
type SSPStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	// Add custom validation using kubebuilder tags: https://book-v1.book.kubebuilder.io/beyond_basics/generating_crd.html
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// SSP is the Schema for the ssps API
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=ssps,scope=Namespaced
type SSP struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SSPSpec   `json:"spec,omitempty"`
	Status SSPStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// SSPList contains a list of SSP
type SSPList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []SSP `json:"items"`
}

func init() {
	SchemeBuilder.Register(&SSP{}, &SSPList{})
}
