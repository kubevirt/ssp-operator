package metrics

import (
	"reflect"

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
		&promv1.PrometheusRule{},
		func(newRes controllerutil.Object, foundRes controllerutil.Object) bool {
			newRule := newRes.(*promv1.PrometheusRule)
			foundRule := foundRes.(*promv1.PrometheusRule)
			if !reflect.DeepEqual(newRule.Spec, foundRule.Spec) {
				foundRule.Spec = newRule.Spec
				return true
			}
			return false
		})
}
