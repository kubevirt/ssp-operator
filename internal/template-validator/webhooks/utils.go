package validating

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	templatev1 "github.com/openshift/api/template/v1"
	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubevirt "kubevirt.io/api/core/v1"
)

func GetAdmissionReview(r *http.Request) (*admissionv1.AdmissionReview, error) {
	var body []byte
	if r.Body != nil {
		if data, err := io.ReadAll(r.Body); err == nil {
			body = data
		}
	}

	// verify the content type is accurate
	contentType := r.Header.Get("Content-Type")
	if contentType != "application/json" {
		return nil, fmt.Errorf("contentType=%s, expect application/json", contentType)
	}

	ar := &admissionv1.AdmissionReview{}
	err := json.Unmarshal(body, ar)
	return ar, err
}

func ToAdmissionResponseOK() *admissionv1.AdmissionResponse {
	reviewResponse := admissionv1.AdmissionResponse{}
	reviewResponse.Allowed = true
	return &reviewResponse
}

func ToAdmissionResponseError(err error) *admissionv1.AdmissionResponse {
	return &admissionv1.AdmissionResponse{
		Result: &metav1.Status{
			Message: err.Error(),
			Code:    http.StatusBadRequest,
		},
	}
}

func ToAdmissionResponse(causes []metav1.StatusCause) *admissionv1.AdmissionResponse {
	globalMessage := ""
	for _, cause := range causes {
		if globalMessage == "" {
			globalMessage = cause.Message
		} else {
			globalMessage = fmt.Sprintf("%s, %s", globalMessage, cause.Message)
		}
	}

	return &admissionv1.AdmissionResponse{
		Result: &metav1.Status{
			Message: globalMessage,
			Reason:  metav1.StatusReasonInvalid,
			Code:    http.StatusUnprocessableEntity,
			Details: &metav1.StatusDetails{
				Causes: causes,
			},
		},
	}
}

func GetAdmissionReviewVM(ar *admissionv1.AdmissionReview) (*kubevirt.VirtualMachine, error) {
	if ar.Request.Resource.Resource != "virtualmachines" {
		return nil, fmt.Errorf("expect resource %v to be '%s'", ar.Request.Resource, "virtualmachines")
	}

	newVM := &kubevirt.VirtualMachine{}
	err := json.Unmarshal(ar.Request.Object.Raw, newVM)
	return newVM, err
}

func GetAdmissionReviewTemplate(ar *admissionv1.AdmissionReview) (*templatev1.Template, error) {
	const resourceName = "templates"
	if ar.Request.Resource.Resource != resourceName {
		return nil, fmt.Errorf("expected resource %v to be '%s'", ar.Request.Resource, resourceName)
	}

	obj := &ar.Request.Object
	if ar.Request.Operation == admissionv1.Delete {
		obj = &ar.Request.OldObject
	}

	template := &templatev1.Template{}
	err := json.Unmarshal(obj.Raw, template)
	return template, err
}
