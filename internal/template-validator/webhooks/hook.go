/*
 * This file is part of the KubeVirt project
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 * Copyright 2019 Red Hat, Inc.
 */

package validating

import (
	"encoding/json"
	"net/http"

	"github.com/davecgh/go-spew/spew"
	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"kubevirt.io/client-go/log"

	"kubevirt.io/ssp-operator/internal/template-validator/virtinformers"
)

const (
	VmValidatePath string = "/virtualmachine-validate"
)

type admitFunc func(*admissionv1.AdmissionReview) *admissionv1.AdmissionResponse

type Webhooks interface {
	Register()
}

type webhooks struct {
	informers *virtinformers.Informers
}

func NewWebhooks(informers *virtinformers.Informers) Webhooks {
	return &webhooks{informers: informers}
}

func (w *webhooks) Register() {
	http.HandleFunc(VmValidatePath, func(resp http.ResponseWriter, req *http.Request) {
		serve(resp, req, w.admitVm)
	})
}

func (w *webhooks) admitVm(ar *admissionv1.AdmissionReview) *admissionv1.AdmissionResponse {
	vm, err := GetAdmissionReviewVM(ar)
	if err != nil {
		return ToAdmissionResponseError(err)
	}

	if vm.DeletionTimestamp != nil {
		return ToAdmissionResponseOK()
	}

	rules, err := getValidationRulesForVM(vm, w.informers.TemplateStore())
	if err != nil {
		return ToAdmissionResponseError(err)
	}

	log.Log.V(8).Infof("admission vm:\n%s", spew.Sdump(vm))
	log.Log.V(8).Infof("admission rules:\n%s", spew.Sdump(rules))

	causes := ValidateVm(rules, vm)
	if len(causes) > 0 {
		return ToAdmissionResponse(causes)
	}

	return ToAdmissionResponseOK()
}

func serve(resp http.ResponseWriter, req *http.Request, admit admitFunc) {
	review, err := GetAdmissionReview(req)

	log.Log.V(8).Infof("evaluating admission")
	defer log.Log.V(8).Infof("evaluated admission")

	if err != nil {
		resp.WriteHeader(http.StatusBadRequest)
		return
	}

	log.Log.V(8).Infof("admission review:\n%s", spew.Sdump(review))

	reviewResponse := admit(review)

	log.Log.V(8).Infof("admission review response:\n%s", spew.Sdump(reviewResponse))

	response := admissionv1.AdmissionReview{
		TypeMeta: metav1.TypeMeta{
			APIVersion: admissionv1.SchemeGroupVersion.String(),
			Kind:       "AdmissionReview",
		},
	}

	if reviewResponse != nil {
		response.Response = reviewResponse
		response.Response.UID = review.Request.UID
	}

	responseBytes, err := json.Marshal(response)
	if err != nil {
		log.Log.Errorf("failed json encode webhook response: %v", err)
		resp.WriteHeader(http.StatusBadRequest)
		return
	}
	if _, err := resp.Write(responseBytes); err != nil {
		log.Log.Errorf("failed to write webhook response: %v", err)
		resp.WriteHeader(http.StatusBadRequest)
		return
	}
}
