package common

import "sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

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
)

// AddAppLabels to the provided obj
// Name will translate into the AppKubernetesNameLabel
// Component will translate into the AppKubernetesComponentLabel
// Instance wide labels will be taken from the request if available
func AddAppLabels(request *Request, name string, component AppComponent, obj controllerutil.Object) controllerutil.Object {
	labels := getOrCreateLabels(obj)
	addInstanceLabels(request, labels)

	labels[AppKubernetesNameLabel] = name
	labels[AppKubernetesComponentLabel] = component.String()
	labels[AppKubernetesManagedByLabel] = "ssp-operator"

	return obj
}

func getOrCreateLabels(obj controllerutil.Object) map[string]string {
	labels := obj.GetLabels()
	if labels == nil {
		labels = map[string]string{}
		obj.SetLabels(labels)
	}
	return labels
}

func addInstanceLabels(request *Request, to map[string]string) {
	if request.Instance.Labels == nil {
		return
	}

	copyLabel(request.Instance.Labels, to, AppKubernetesPartOfLabel)
	copyLabel(request.Instance.Labels, to, AppKubernetesVersionLabel)
}

func copyLabel(from, to map[string]string, key string) {
	to[key] = from[key]
}
