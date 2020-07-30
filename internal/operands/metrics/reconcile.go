package metrics

import (
	"fmt"
	"reflect"

	promv1 "github.com/coreos/prometheus-operator/pkg/apis/monitoring/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"

	"kubevirt.io/ssp-operator/internal/common"
)

func AddWatchTypesToScheme(s *runtime.Scheme) error {
	return promv1.AddToScheme(s)
}

func WatchTypes() []runtime.Object {
	return []runtime.Object{&promv1.PrometheusRule{}}
}

func Reconcile(request *common.Request) error {
	rule := newPrometheusRule(request.Namespace)

	err := request.SetControllerReferenceFor(rule)
	if err != nil {
		return err
	}

	var found promv1.PrometheusRule
	err = request.Client.Get(request.Context,
		types.NamespacedName{Name: rule.Name, Namespace: rule.Namespace},
		&found)
	if errors.IsNotFound(err) {
		request.Logger.Info(fmt.Sprintf("Creating PrometheusRule: %s", rule.Name))
		return request.Client.Create(request.Context, rule)
	}
	if err != nil {
		return err
	}

	if reflect.DeepEqual(rule.Spec, found.Spec) {
		return nil
	}

	found.Spec = rule.Spec
	request.Logger.Info(fmt.Sprintf("Updating PrometheusRule: %s", found.Name))
	return request.Client.Update(request.Context, &found)
}
