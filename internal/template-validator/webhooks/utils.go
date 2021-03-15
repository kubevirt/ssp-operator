package validating

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubevirt "kubevirt.io/client-go/api/v1"
)

// GetAdmissionReview
func GetAdmissionReview(r *http.Request) (*admissionv1.AdmissionReview, error) {
	var body []byte
	if r.Body != nil {
		if data, err := ioutil.ReadAll(r.Body); err == nil {
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

// ToAdmissionResponseError
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

func GetAdmissionReviewVM(ar *admissionv1.AdmissionReview) (*kubevirt.VirtualMachine, *kubevirt.VirtualMachine, error) {
	if ar.Request.Resource.Resource != "virtualmachines" {
		return nil, nil, fmt.Errorf("expect resource %v to be '%s'", ar.Request.Resource, "virtualmachines")
	}

	var err error
	raw := ar.Request.Object.Raw
	newVM := kubevirt.VirtualMachine{}

	err = json.Unmarshal(raw, &newVM)
	if err != nil {
		return nil, nil, err
	}

	if ar.Request.Operation == admissionv1.Update {
		raw := ar.Request.OldObject.Raw
		oldVM := kubevirt.VirtualMachine{}
		err = json.Unmarshal(raw, &oldVM)
		if err != nil {
			return nil, nil, err
		}
		return &newVM, &oldVM, nil
	}

	return &newVM, nil, nil
}
