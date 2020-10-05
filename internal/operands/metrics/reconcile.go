package metrics

import (
	promv1 "github.com/coreos/prometheus-operator/pkg/apis/monitoring/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"kubevirt.io/ssp-operator/internal/common"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func AddWatchTypesToScheme(s *runtime.Scheme) error {
	return promv1.AddToScheme(s)
}

func WatchTypes() []runtime.Object {
	return []runtime.Object{&promv1.PrometheusRule{}}
}

func Reconcile(request *common.Request) error {
	return common.CreateOrUpdateResource(request,
		newPrometheusRule(request.Namespace),
		func(newRes, foundRes controllerutil.Object) {
			foundRes.(*promv1.PrometheusRule).Spec = newRes.(*promv1.PrometheusRule).Spec
		})
}
