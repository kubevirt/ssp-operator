package common

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ssp "kubevirt.io/ssp-operator/api/v1beta2"
)

const (
	AppKubernetesNameLabel      = "app.kubernetes.io/name"
	AppKubernetesPartOfLabel    = "app.kubernetes.io/part-of"
	AppKubernetesVersionLabel   = "app.kubernetes.io/version"
	AppKubernetesManagedByLabel = "app.kubernetes.io/managed-by"
	AppKubernetesComponentLabel = "app.kubernetes.io/component"

	AppKubernetesManagedByValue string = "ssp-operator"
)

type AppComponent string

func (a AppComponent) String() string {
	return string(a)
}

const (
	AppComponentMonitoring           AppComponent = "monitoring"
	AppComponentSchedule             AppComponent = "schedule"
	AppComponentTemplating           AppComponent = "templating"
	AppComponentTektonPipelines      AppComponent = "tektonPipelines"
	AppComponentTektonTasks          AppComponent = "tektonTasks"
	AppComponentVMDeletionProtection AppComponent = "vmDeleteProtection"
)

// AddAppLabels to the provided obj
// Name will translate into the AppKubernetesNameLabel
// Component will translate into the AppKubernetesComponentLabel
// Instance wide labels will be taken from the request if available
func AddAppLabels(requestInstance *ssp.SSP, name string, component AppComponent, obj metav1.Object) metav1.Object {
	labels := getOrCreateLabels(obj)
	addCommonLabels(labels, requestInstance, name, component)
	return obj
}

func getOrCreateLabels(obj metav1.Object) map[string]string {
	labels := obj.GetLabels()
	if labels == nil {
		labels = map[string]string{}
		obj.SetLabels(labels)
	}
	return labels
}

func addCommonLabels(labels map[string]string, requestInstance *ssp.SSP, name string, component AppComponent) {
	addInstanceLabels(requestInstance, labels)

	labels[AppKubernetesNameLabel] = name
	labels[AppKubernetesComponentLabel] = component.String()
	labels[AppKubernetesManagedByLabel] = AppKubernetesManagedByValue
}

func addInstanceLabels(requestInstance *ssp.SSP, to map[string]string) {
	if requestInstance.Labels == nil {
		return
	}

	copyLabel(requestInstance.Labels, to, AppKubernetesPartOfLabel)
	copyLabel(requestInstance.Labels, to, AppKubernetesVersionLabel)
}

func copyLabel(from, to map[string]string, key string) {
	to[key] = from[key]
}
