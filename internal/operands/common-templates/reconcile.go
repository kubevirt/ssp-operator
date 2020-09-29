package common_templates

import (
	"fmt"
	templatev1 "github.com/openshift/api/template/v1"
	core "k8s.io/api/core/v1"
	rbac "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"kubevirt.io/ssp-operator/internal/common"
	"kubevirt.io/ssp-operator/internal/operands"
	"path/filepath"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sync"
)

var (
	loadTemplatesOnce sync.Once
	templatesBundle   []templatev1.Template
)

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

func (c *commonTemplates) Reconcile(request *common.Request) error {
	for _, f := range []func(*common.Request) error{
		reconcileGoldenImagesNS,
		reconcileViewRole,
		reconcileViewRoleBinding,
		reconcileEditRole,
		reconcileTemplates,
	} {
		if err := f(request); err != nil {
			return err
		}
	}
	return nil
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

func reconcileGoldenImagesNS(request *common.Request) error {
	return common.CreateOrUpdateClusterResource(request, newGoldenImagesNS(GoldenImagesNSname),
		func(newRes, foundRes controllerutil.Object) {
			newNS := newRes.(*core.Namespace)
			foundNS := foundRes.(*core.Namespace)
			foundNS.Spec = newNS.Spec
		})
}
func reconcileViewRole(request *common.Request) error {
	return common.CreateOrUpdateClusterResource(request, newViewRole(GoldenImagesNSname),
		func(newRes, foundRes controllerutil.Object) {
			foundRole := foundRes.(*rbac.Role)
			newRole := newRes.(*rbac.Role)
			foundRole.Rules = newRole.Rules
		})
}

func reconcileViewRoleBinding(request *common.Request) error {
	return common.CreateOrUpdateClusterResource(request, newViewRoleBinding(GoldenImagesNSname),
		func(newRes, foundRes controllerutil.Object) {
			newBinding := newRes.(*rbac.RoleBinding)
			foundBinding := foundRes.(*rbac.RoleBinding)
			foundBinding.Subjects = newBinding.Subjects
			foundBinding.RoleRef = newBinding.RoleRef
		})
}

func reconcileEditRole(request *common.Request) error {
	return common.CreateOrUpdateClusterResource(request, newEditRole(),
		func(newRes, foundRes controllerutil.Object) {
			newRole := newRes.(*rbac.ClusterRole)
			foundRole := foundRes.(*rbac.ClusterRole)
			foundRole.Rules = newRole.Rules
		})
}

func reconcileTemplates(request *common.Request) error {
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
	for _, template := range templatesBundle {
		template.ObjectMeta.Namespace = namespace
		err := common.CreateOrUpdateClusterResource(request, &template,
			func(newRes, foundRes controllerutil.Object) {
				newTemplate := newRes.(*templatev1.Template)
				foundTemplate := foundRes.(*templatev1.Template)
				foundTemplate.Objects = newTemplate.Objects
				foundTemplate.Parameters = newTemplate.Parameters
			})
		if err != nil {
			return err
		}
	}
	return nil
}
