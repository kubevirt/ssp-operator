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
	"fmt"
	"net/http"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"kubevirt.io/client-go/log"
	"sigs.k8s.io/controller-runtime/pkg/client"

	common_templates "kubevirt.io/ssp-operator/internal/operands/common-templates"
	"kubevirt.io/ssp-operator/internal/template-validator/labels"
	"kubevirt.io/ssp-operator/internal/template-validator/virtinformers"
)

var (
	vmsRejected = promauto.NewCounter(prometheus.CounterOpts{
		Name: "total_rejected_vms",
		Help: "The total number of rejected vms",
	})
)

const (
	VmValidatePath       string = "/virtualmachine-validate"
	TemplateValidatePath string = "/template-validate"
)

type admitFunc func(*admissionv1.AdmissionReview) *admissionv1.AdmissionResponse

type Webhooks interface {
	Register()
}

type webhooks struct {
	informers *virtinformers.Informers
}

func NewWebhooks(informers *virtinformers.Informers) Webhooks {
	return &webhooks{
		informers: informers,
	}
}

func (w *webhooks) Register() {
	http.HandleFunc(VmValidatePath, func(resp http.ResponseWriter, req *http.Request) {
		serve(resp, req, w.admitVm)
	})
	http.HandleFunc(TemplateValidatePath, func(resp http.ResponseWriter, req *http.Request) {
		serve(resp, req, w.admitTemplate)
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

	if vmJson, err := json.Marshal(vm); err == nil {
		log.Log.V(8).Infof("admission vm:\n%s", string(vmJson))
	} else {
		log.Log.V(8).Infof("admission vm:\nCould not marshal VM to json: %s", err.Error())
	}

	if rulesJson, err := json.Marshal(rules); err == nil {
		log.Log.V(8).Infof("admission rules:\n%s", string(rulesJson))
	} else {
		log.Log.V(8).Infof("admission vm:\nCould not marshal rules to json: %s", err.Error())
	}

	causes := ValidateVm(rules, vm)
	if len(causes) > 0 {
		vmsRejected.Inc()
		return ToAdmissionResponse(causes)
	}

	return ToAdmissionResponseOK()
}

func (w *webhooks) admitTemplate(ar *admissionv1.AdmissionReview) *admissionv1.AdmissionResponse {
	if ar.Request.Operation != admissionv1.Delete {
		return ToAdmissionResponseOK()
	}

	template, err := GetAdmissionReviewTemplate(ar)
	if err != nil {
		return ToAdmissionResponseError(err)
	}

	// Check if template is a common template
	_, ok := template.Labels[common_templates.TemplateTypeLabel]
	if !ok {
		return ToAdmissionResponseOK()
	}

	// Old versions of common templates had annotation validation
	_, ok = template.Annotations[labels.AnnotationValidationKey]
	if !ok {
		return ToAdmissionResponseOK()
	}

	// Old template cannot be removed if a VM uses it.
	templateKey := client.ObjectKeyFromObject(template).String()
	vms := w.informers.VmCache().GetVmsForTemplate(templateKey)
	if len(vms) == 0 {
		return ToAdmissionResponseOK()
	}

	return &admissionv1.AdmissionResponse{
		Allowed: false,
		Result: &metav1.Status{
			Message: fmt.Sprintf(
				"Template cannot be deleted, because the following VMs are referencing it for validation: %s",
				strings.Join(vms, ", "),
			),
			Reason: metav1.StatusReasonForbidden,
			Code:   http.StatusForbidden,
		},
	}
}

func serve(resp http.ResponseWriter, req *http.Request, admit admitFunc) {
	review, err := GetAdmissionReview(req)

	log.Log.V(8).Infof("evaluating admission")
	defer log.Log.V(8).Infof("evaluated admission")

	if err != nil {
		resp.WriteHeader(http.StatusBadRequest)
		return
	}

	if reviewJson, err := json.Marshal(review); err == nil {
		log.Log.V(8).Infof("admission review:\n%s", string(reviewJson))
	} else {
		log.Log.V(8).Infof("admission review:\nCould not marshal review to json: %s", err.Error())
	}

	reviewResponse := admit(review)

	if reviewResponseJson, err := json.Marshal(reviewResponse); err == nil {
		log.Log.V(8).Infof("admission review response:\n%s", string(reviewResponseJson))
	} else {
		log.Log.V(8).Infof("admission review response:\nCould not marshal reviewResponse to json: %s", err.Error())
	}

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
