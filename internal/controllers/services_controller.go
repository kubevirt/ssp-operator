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
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"kubevirt.io/ssp-operator/internal/common"
	crd_watch "kubevirt.io/ssp-operator/internal/crd-watch"
	"kubevirt.io/ssp-operator/internal/env"
	"kubevirt.io/ssp-operator/internal/operands/metrics"
)

const (
	ServiceManagedByLabelValue = "ssp-operator-services"
	MetricsServiceName         = "ssp-operator-metrics"
	OperatorName               = "ssp-operator"
	ServiceControllerName      = "service-controller"
)

func ServiceObject(namespace string, appKubernetesPartOfValue string) *v1.Service {
	policyCluster := v1.ServiceInternalTrafficPolicyCluster
	labels := map[string]string{
		common.AppKubernetesManagedByLabel: ServiceManagedByLabelValue,
		common.AppKubernetesVersionLabel:   env.GetOperatorVersion(),
		common.AppKubernetesComponentLabel: ServiceControllerName,
		metrics.PrometheusLabelKey:         metrics.PrometheusLabelValue,
	}
	if appKubernetesPartOfValue != "" {
		labels[common.AppKubernetesPartOfLabel] = appKubernetesPartOfValue
	}
	return &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      MetricsServiceName,
			Namespace: namespace,
			Labels:    labels,
		},
		Spec: v1.ServiceSpec{
			InternalTrafficPolicy: &policyCluster,
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

func CreateServiceController() (Controller, error) {
	logger := ctrl.Log.WithName("controllers").WithName("Resources")
	namespace, err := env.GetOperatorNamespace()
	if err != nil {
		return nil, fmt.Errorf("error getting operator namespace: %w", err)
	}

	reconciler := &serviceReconciler{
		log:               logger,
		operatorNamespace: namespace,
	}

	return reconciler, nil
}

// serviceReconciler reconciles the required services in the operator's namespace
type serviceReconciler struct {
	log               logr.Logger
	operatorNamespace string

	client     client.Client
	deployment *apps.Deployment
}

var _ Controller = &serviceReconciler{}

var _ reconcile.Reconciler = &serviceReconciler{}

func (s *serviceReconciler) Name() string {
	return ServiceControllerName
}

func (s *serviceReconciler) AddToManager(mgr ctrl.Manager, _ crd_watch.CrdList) error {
	return mgr.Add(manager.RunnableFunc(func(ctx context.Context) error {
		// Using API reader to not create an informer in the cache
		deployment, err := getOperatorDeployment(ctx, s.operatorNamespace, mgr.GetAPIReader())
		if err != nil {
			return fmt.Errorf("error getting operator deployment: %w", err)
		}

		err = createMetricsService(ctx, deployment, mgr.GetClient())
		if err != nil && !errors.IsAlreadyExists(err) {
			return fmt.Errorf("error creating service: %w", err)
		}

		s.client = mgr.GetClient()
		s.deployment = deployment

		return s.setupController(mgr)
	}))
}

func (s *serviceReconciler) RequiredCrds() []string {
	return nil
}

func (s *serviceReconciler) setServiceOwnerReference(service *v1.Service) error {
	return controllerutil.SetOwnerReference(s.deployment, service, s.client.Scheme())
}

func (s *serviceReconciler) setupController(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		Named("service-controller").
		For(&v1.Service{}, builder.WithPredicates(predicate.NewPredicateFuncs(
			func(object client.Object) bool {
				return object.GetName() == MetricsServiceName && object.GetNamespace() == s.operatorNamespace
			}))).
		Complete(s)
}

func createMetricsService(ctx context.Context, deployment *apps.Deployment, apiClient client.Client) error {
	appKubernetesPartOfValue := deployment.GetLabels()[common.AppKubernetesPartOfLabel]
	service := ServiceObject(deployment.Namespace, appKubernetesPartOfValue)
	err := controllerutil.SetOwnerReference(deployment, service, apiClient.Scheme())
	if err != nil {
		return fmt.Errorf("error setting owner reference: %w", err)
	}
	return apiClient.Create(ctx, service)
}

func getOperatorDeployment(ctx context.Context, namespace string, apiReader client.Reader) (*apps.Deployment, error) {
	objKey := client.ObjectKey{Namespace: namespace, Name: OperatorName}
	var deployment apps.Deployment
	err := apiReader.Get(ctx, objKey, &deployment)
	if err != nil {
		return nil, fmt.Errorf("getOperatorDeployment, get deployment: %w", err)
	}
	return &deployment, nil
}

func (s *serviceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (res ctrl.Result, err error) {
	s.log.Info("Starting service reconciliation...", "request", req.String())
	appKubernetesPartOfValue := s.deployment.GetLabels()[common.AppKubernetesPartOfLabel]
	service := ServiceObject(req.Namespace, appKubernetesPartOfValue)
	var foundService v1.Service
	foundService.Name = service.Name
	foundService.Namespace = service.Namespace

	_, err = controllerutil.CreateOrUpdate(ctx, s.client, &foundService, func() error {
		if !foundService.GetDeletionTimestamp().IsZero() {
			// Skip update, because the resource is being deleted
			return nil
		}

		clusterIP := foundService.Spec.ClusterIP
		foundService.Spec = service.Spec
		foundService.Spec.ClusterIP = clusterIP

		common.UpdateLabels(service, &foundService)

		err = s.setServiceOwnerReference(&foundService)
		if err != nil {
			return fmt.Errorf("error at setServiceOwnerReference: %w", err)
		}
		return nil
	})

	return ctrl.Result{}, err
}
