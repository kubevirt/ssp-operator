package common

import (
	"kubevirt.io/ssp-operator/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	AppKubernetesNameLabel      = "app.kubernetes.io/name"
	AppKubernetesPartOfLabel    = "app.kubernetes.io/part-of"
	AppKubernetesVersionLabel   = "app.kubernetes.io/version"
	AppKubernetesManagedByLabel = "app.kubernetes.io/managed-by"
	AppKubernetesComponentLabel = "app.kubernetes.io/component"
)

type AppComponent string

func (a AppComponent) String() string {
	return string(a)
}

const (
	AppComponentMonitoring AppComponent = "monitoring"
	AppComponentSchedule   AppComponent = "schedule"
	AppComponentTemplating AppComponent = "templating"

	OperatorName = "ssp-operator"
)

// AddAppLabels to the provided obj
// Name will translate into the AppKubernetesNameLabel
// Component will translate into the AppKubernetesComponentLabel
// Instance wide labels will be taken from the request if available
func AddAppLabels(requestInstance *v1beta1.SSP, name string, component AppComponent, obj client.Object) client.Object {
	labels := getOrCreateLabels(obj)
	addInstanceLabels(requestInstance, labels)

	labels[AppKubernetesNameLabel] = name
	labels[AppKubernetesComponentLabel] = component.String()
	labels[AppKubernetesManagedByLabel] = OperatorName

	return obj
}

func getOrCreateLabels(obj client.Object) map[string]string {
	labels := obj.GetLabels()
	if labels == nil {
		labels = map[string]string{}
		obj.SetLabels(labels)
	}
	return labels
}

func addInstanceLabels(requestInstance *v1beta1.SSP, to map[string]string) {
	if requestInstance.Labels == nil {
		return
	}

	copyLabel(requestInstance.Labels, to, AppKubernetesPartOfLabel)
	copyLabel(requestInstance.Labels, to, AppKubernetesVersionLabel)
}

func copyLabel(from, to map[string]string, key string) {
	to[key] = from[key]
}
