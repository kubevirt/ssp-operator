package common_templates

import (
	"context"
	"fmt"

	kvsspv1 "github.com/kubevirt/kubevirt-ssp-operator/pkg/apis/kubevirt/v1"
	templatev1 "github.com/openshift/api/template/v1"
	core "k8s.io/api/core/v1"
	rbac "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"kubevirt.io/ssp-operator/internal/common"
	"kubevirt.io/ssp-operator/internal/operands"
	"path/filepath"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sync"
)

var (
	loadTemplatesOnce sync.Once
	templatesBundle   []templatev1.Template
)

// Define RBAC rules needed by this operand:
// +kubebuilder:rbac:groups=core,resources=namespaces,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=template.openshift.io,resources=templates,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=clusterroles;roles;rolebindings,verbs=get;list;watch;create;update;patch;delete

// RBAC for created roles
// +kubebuilder:rbac:groups=core,resources=persistentvolumeclaims,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=persistentvolumeclaims/status,verbs=get;list;watch
// +kubebuilder:rbac:groups=cdi.kubevirt.io,resources=datavolumes,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=cdi.kubevirt.io,resources=datavolumes/source,verbs=create
// +kubebuilder:rbac:groups=ssp.kubevirt.io,resources=kubevirtcommontemplatesbundles,verbs=get;list;watch;create;update;patch;delete

type commonTemplates struct{}

func GetOperand() operands.Operand {
	return &commonTemplates{}
}

func (c *commonTemplates) AddWatchTypesToScheme(s *runtime.Scheme) error {
	return templatev1.Install(s)
}

func (c *commonTemplates) WatchClusterTypes() []runtime.Object {
	return []runtime.Object{
		&rbac.ClusterRole{},
		&rbac.Role{},
		&rbac.RoleBinding{},
		&core.Namespace{},
		&templatev1.Template{},
	}
}

func (c *commonTemplates) WatchTypes() []runtime.Object {
	return nil
}

func (c *commonTemplates) Reconcile(request *common.Request) ([]common.ResourceStatus, error) {
	pauseCRs(request)
	funcs := []common.ReconcileFunc{
		reconcileGoldenImagesNS,
		reconcileViewRole,
		reconcileViewRoleBinding,
		reconcileEditRole,
	}
	funcs = append(funcs, reconcileTemplatesFuncs(request)...)
	return common.CollectResourceStatus(request, funcs...)
}

func (c *commonTemplates) Cleanup(request *common.Request) error {
	objects := []controllerutil.Object{
		newGoldenImagesNS(GoldenImagesNSname),
		newViewRole(GoldenImagesNSname),
		newViewRoleBinding(GoldenImagesNSname),
		newEditRole(),
	}
	namespace := request.Instance.Spec.CommonTemplates.Namespace
	for index := range templatesBundle {
		templatesBundle[index].ObjectMeta.Namespace = namespace
		objects = append(objects, &templatesBundle[index])
	}
	for _, obj := range objects {
		err := request.Client.Delete(request.Context, obj)
		if err != nil && !errors.IsNotFound(err) {
			request.Logger.Error(err, fmt.Sprintf("Error deleting \"%s\": %s", obj.GetName(), err))
			return err
		}
	}
	return nil
}

func pauseCRs(request *common.Request) {
	patch := []byte(`{"metadata":{"annotations":{"kubevirt.io/operator.paused": "true"}}}`)
	var kubevirtCommonTemplatesBundles kvsspv1.KubevirtCommonTemplatesBundleList
	err := request.Client.List(context.TODO(), &kubevirtCommonTemplatesBundles, &client.ListOptions{})
	if err != nil {
		request.Logger.Error(err, fmt.Sprintf("Error listing common template bundles: %s", err))
		return
	}
	if err == nil && len(kubevirtCommonTemplatesBundles.Items) > 0 {
		for _, kubevirtCommonTemplatesBundle := range kubevirtCommonTemplatesBundles.Items {
			err = request.Client.Patch(context.TODO(), &kvsspv1.KubevirtCommonTemplatesBundle{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: kubevirtCommonTemplatesBundle.ObjectMeta.Namespace,
					Name:      kubevirtCommonTemplatesBundle.ObjectMeta.Name,
				},
			}, client.RawPatch(types.MergePatchType, patch))
			if err != nil {
				request.Logger.Error(err, fmt.Sprintf("Error pausing %s from namespace %s: %s",
					kubevirtCommonTemplatesBundle.ObjectMeta.Name,
					kubevirtCommonTemplatesBundle.ObjectMeta.Namespace,
					err))
			}
		}
	}
}

func reconcileGoldenImagesNS(request *common.Request) (common.ResourceStatus, error) {
	return common.CreateOrUpdateClusterResource(request, newGoldenImagesNS(GoldenImagesNSname),
		func(newRes, foundRes controllerutil.Object) {
			newNS := newRes.(*core.Namespace)
			foundNS := foundRes.(*core.Namespace)
			foundNS.Spec = newNS.Spec
		})
}
func reconcileViewRole(request *common.Request) (common.ResourceStatus, error) {
	return common.CreateOrUpdateClusterResource(request, newViewRole(GoldenImagesNSname),
		func(newRes, foundRes controllerutil.Object) {
			foundRole := foundRes.(*rbac.Role)
			newRole := newRes.(*rbac.Role)
			foundRole.Rules = newRole.Rules
		})
}

func reconcileViewRoleBinding(request *common.Request) (common.ResourceStatus, error) {
	return common.CreateOrUpdateClusterResource(request, newViewRoleBinding(GoldenImagesNSname),
		func(newRes, foundRes controllerutil.Object) {
			newBinding := newRes.(*rbac.RoleBinding)
			foundBinding := foundRes.(*rbac.RoleBinding)
			foundBinding.Subjects = newBinding.Subjects
			foundBinding.RoleRef = newBinding.RoleRef
		})
}

func reconcileEditRole(request *common.Request) (common.ResourceStatus, error) {
	return common.CreateOrUpdateClusterResource(request, newEditRole(),
		func(newRes, foundRes controllerutil.Object) {
			newRole := newRes.(*rbac.ClusterRole)
			foundRole := foundRes.(*rbac.ClusterRole)
			foundRole.Rules = newRole.Rules
		})
}

func reconcileTemplatesFuncs(request *common.Request) []common.ReconcileFunc {
	loadTemplates := func() {
		var err error
		filename := filepath.Join(bundleDir, "common-templates-"+Version+".yaml")
		templatesBundle, err = readTemplates(filename)
		if err != nil {
			request.Logger.Error(err, fmt.Sprintf("Error reading from template bundle, %v", err))
			panic(err)
		}
		if len(templatesBundle) == 0 {
			panic("No templates could be found in the installed bundle")
		}
	}
	// Only load templates Once
	loadTemplatesOnce.Do(loadTemplates)

	namespace := request.Instance.Spec.CommonTemplates.Namespace
	funcs := make([]common.ReconcileFunc, 0, len(templatesBundle))
	for i := range templatesBundle {
		template := &templatesBundle[i]
		template.ObjectMeta.Namespace = namespace
		funcs = append(funcs, func(request *common.Request) (common.ResourceStatus, error) {
			return common.CreateOrUpdateClusterResource(request, template,
				func(newRes, foundRes controllerutil.Object) {
					newTemplate := newRes.(*templatev1.Template)
					foundTemplate := foundRes.(*templatev1.Template)
					foundTemplate.Objects = newTemplate.Objects
					foundTemplate.Parameters = newTemplate.Parameters
				})
		})
	}
	return funcs
}
