package controllers

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	apps "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"kubevirt.io/ssp-operator/internal/common"
	"kubevirt.io/ssp-operator/internal/operands/metrics"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	MetricsServiceName = "ssp-operator-metrics"
	OperatorName       = "ssp-operator"
)

func ServiceObject(namespace string) *v1.Service {
	policyCluster := v1.ServiceInternalTrafficPolicyCluster
	familyPolicy := v1.IPFamilyPolicySingleStack
	return &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      MetricsServiceName,
			Namespace: namespace,
			Labels: map[string]string{
				metrics.PrometheusLabelKey: metrics.PrometheusLabelValue,
			},
		},
		Spec: v1.ServiceSpec{
			InternalTrafficPolicy: &policyCluster,
			IPFamilies:            []v1.IPFamily{v1.IPv4Protocol},
			IPFamilyPolicy:        &familyPolicy,
			Ports: []v1.ServicePort{
				{
					Name:       metrics.MetricsPortName,
					Port:       443,
					Protocol:   v1.ProtocolTCP,
					TargetPort: intstr.FromString(metrics.MetricsPortName),
				},
			},
			Selector: map[string]string{
				metrics.PrometheusLabelKey: metrics.PrometheusLabelValue,
				"name":                     OperatorName,
			},
			SessionAffinity: v1.ServiceAffinityNone,
			Type:            v1.ServiceTypeClusterIP,
		},
	}
}

// Annotation to generate RBAC roles to read and modify services
// +kubebuilder:rbac:groups="",resources=services,verbs=get;watch;list;create;update;delete

func CreateServiceController(mgr ctrl.Manager) (*serviceReconciler, error) {
	return newServiceReconciler(mgr)
}

func (r *serviceReconciler) Start(ctx context.Context, mgr ctrl.Manager) error {
	err := r.createMetricsService(ctx)
	if err != nil && !errors.IsAlreadyExists(err) {
		return fmt.Errorf("error start serviceReconciler: %w", err)
	}

	return r.setupController(mgr)
}

func (r *serviceReconciler) setServiceOwnerReference(service *v1.Service) error {
	return controllerutil.SetOwnerReference(r.deployment, service, r.client.Scheme())
}

func (r *serviceReconciler) createMetricsService(ctx context.Context) error {
	service := ServiceObject(r.serviceNamespace)
	err := r.setServiceOwnerReference(service)
	if err != nil {
		return fmt.Errorf("error setting owner reference: %w", err)
	}
	return r.client.Create(ctx, service)
}

func (r *serviceReconciler) setupController(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		Named("service-controller").
		For(&v1.Service{}, builder.WithPredicates(predicate.NewPredicateFuncs(
			func(object client.Object) bool {
				return object.GetName() == MetricsServiceName && object.GetNamespace() == r.serviceNamespace
			}))).
		Complete(r)
}

// serviceReconciler reconciles the required services in the operator's namespace
type serviceReconciler struct {
	client           client.Client
	log              logr.Logger
	serviceNamespace string
	deployment       *apps.Deployment
}

func getOperatorDeployment(namespace string, apiReader client.Reader) (*apps.Deployment, error) {
	objKey := client.ObjectKey{Namespace: namespace, Name: OperatorName}
	var deployment apps.Deployment
	err := apiReader.Get(context.TODO(), objKey, &deployment)
	if err != nil {
		return nil, fmt.Errorf("getOperatorDeployment, get deployment: %w", err)
	}
	return &deployment, nil
}

func newServiceReconciler(mgr ctrl.Manager) (*serviceReconciler, error) {
	logger := ctrl.Log.WithName("controllers").WithName("Resources")
	namespace, err := common.GetOperatorNamespace(logger)
	if err != nil {
		return nil, fmt.Errorf("in newServiceReconciler: %w", err)
	}

	deployment, err := getOperatorDeployment(namespace, mgr.GetAPIReader())
	if err != nil {
		return nil, fmt.Errorf("in newServiceReconciler: %w", err)
	}

	reconciler := &serviceReconciler{
		client:           mgr.GetClient(),
		log:              logger,
		serviceNamespace: namespace,
		deployment:       deployment,
	}

	return reconciler, nil
}

func (r *serviceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (res ctrl.Result, err error) {
	r.log.Info("Starting service reconciliation...", "request", req.String())
	service := ServiceObject(req.Namespace)
	var foundService v1.Service
	foundService.Name = service.Name
	foundService.Namespace = service.Namespace

	_, err = controllerutil.CreateOrUpdate(ctx, r.client, &foundService, func() error {
		if !foundService.GetDeletionTimestamp().IsZero() {
			// Skip update, because the resource is being deleted
			return nil
		}

		clusterIP := foundService.Spec.ClusterIP
		foundService.Spec = service.Spec
		foundService.Spec.ClusterIP = clusterIP

		common.UpdateLabels(service, &foundService)

		err = r.setServiceOwnerReference(&foundService)
		if err != nil {
			return fmt.Errorf("error at setServiceOwnerReference: %w", err)
		}
		return nil
	})

	return ctrl.Result{}, err
}

var _ reconcile.Reconciler = &serviceReconciler{}
