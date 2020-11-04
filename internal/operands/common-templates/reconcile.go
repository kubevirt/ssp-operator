package common_templates

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"strings"

	templatev1 "github.com/openshift/api/template/v1"
	"github.com/prometheus/client_golang/prometheus"
	core "k8s.io/api/core/v1"
	rbac "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/controller-runtime/pkg/client"

	ssp "kubevirt.io/ssp-operator/api/v1beta1"
	"kubevirt.io/ssp-operator/internal/common"
	"kubevirt.io/ssp-operator/internal/operands"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

var (
	deployedTemplates = make(map[string]bool)
)

var (
	CommonTemplatesRestored = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "total_restored_common_templates",
		Help: "The total number of common templates restored by the operator back to their original state",
	})
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

func init() {
	utilruntime.Must(templatev1.Install(common.Scheme))
}

type commonTemplates struct {
	templatesBundle []templatev1.Template
}

var _ operands.Operand = &commonTemplates{}

func New(templates []templatev1.Template) operands.Operand {
	return &commonTemplates{templatesBundle: templates}
}

func (c *commonTemplates) Name() string {
	return operandName
}

const (
	operandName      = "common-templates"
	operandComponent = common.AppComponentTemplating
)

func (c *commonTemplates) WatchClusterTypes() []client.Object {
	return []client.Object{
		&rbac.ClusterRole{},
		&rbac.Role{},
		&rbac.RoleBinding{},
		&core.Namespace{},
		&templatev1.Template{},
	}
}

func (c *commonTemplates) WatchTypes() []client.Object {
	return nil
}

func (c *commonTemplates) Reconcile(request *common.Request) ([]common.ReconcileResult, error) {
	funcs := []common.ReconcileFunc{
		reconcileGoldenImagesNS,
		reconcileViewRole,
		reconcileViewRoleBinding,
		reconcileEditRole,
	}

	oldTemplateFuncs, err := reconcileOlderTemplates(request)
	if err != nil {
		return nil, err
	}

	funcs = append(funcs, oldTemplateFuncs...)
	results, err := common.CollectResourceStatus(request, funcs...)
	if err != nil {
		return nil, err
	}

	reconcileTemplatesResults, err := common.CollectResourceStatus(request, reconcileTemplatesFuncs(c.templatesBundle)...)
	if err != nil {
		return nil, err
	}
	for _, r := range reconcileTemplatesResults {
		if r.OperationResult == controllerutil.OperationResultUpdated {
			CommonTemplatesRestored.Inc()
		}
	}
	return append(results, reconcileTemplatesResults...), nil
}

func (c *commonTemplates) Cleanup(request *common.Request) error {
	objects := []client.Object{
		newGoldenImagesNS(ssp.GoldenImagesNSname),
		newViewRole(ssp.GoldenImagesNSname),
		newViewRoleBinding(ssp.GoldenImagesNSname),
		newEditRole(),
	}
	namespace := request.Instance.Spec.CommonTemplates.Namespace
	for index := range c.templatesBundle {
		c.templatesBundle[index].ObjectMeta.Namespace = namespace
		objects = append(objects, &c.templatesBundle[index])
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

func reconcileGoldenImagesNS(request *common.Request) (common.ReconcileResult, error) {
	return common.CreateOrUpdate(request).
		ClusterResource(newGoldenImagesNS(ssp.GoldenImagesNSname)).
		WithAppLabels(operandName, operandComponent).
		Reconcile()
}

func reconcileViewRole(request *common.Request) (common.ReconcileResult, error) {
	return common.CreateOrUpdate(request).
		ClusterResource(newViewRole(ssp.GoldenImagesNSname)).
		WithAppLabels(operandName, operandComponent).
		UpdateFunc(func(newRes, foundRes client.Object) {
			foundRole := foundRes.(*rbac.Role)
			newRole := newRes.(*rbac.Role)
			foundRole.Rules = newRole.Rules
		}).
		Reconcile()
}

func reconcileViewRoleBinding(request *common.Request) (common.ReconcileResult, error) {
	return common.CreateOrUpdate(request).
		ClusterResource(newViewRoleBinding(ssp.GoldenImagesNSname)).
		WithAppLabels(operandName, operandComponent).
		UpdateFunc(func(newRes, foundRes client.Object) {
			newBinding := newRes.(*rbac.RoleBinding)
			foundBinding := foundRes.(*rbac.RoleBinding)
			foundBinding.Subjects = newBinding.Subjects
			foundBinding.RoleRef = newBinding.RoleRef
		}).
		Reconcile()
}

func reconcileEditRole(request *common.Request) (common.ReconcileResult, error) {
	return common.CreateOrUpdate(request).
		ClusterResource(newEditRole()).
		WithAppLabels(operandName, operandComponent).
		UpdateFunc(func(newRes, foundRes client.Object) {
			newRole := newRes.(*rbac.ClusterRole)
			foundRole := foundRes.(*rbac.ClusterRole)
			foundRole.Rules = newRole.Rules
		}).
		Reconcile()
}
func getOldTemplatesLabelSelector() labels.Selector {
	baseRequirement, err := labels.NewRequirement(TemplateTypeLabel, selection.Equals, []string{TemplateTypeLabelBaseValue})
	if err != nil {
		panic(fmt.Sprintf("Failed creating label selector for '%s=%s'", TemplateTypeLabel, TemplateTypeLabelBaseValue))
	}

	// Only fetching older templates  to prevent duplication of API calls
	versionRequirement, err := labels.NewRequirement(TemplateVersionLabel, selection.NotEquals, []string{Version})
	if err != nil {
		panic(fmt.Sprintf("Failed creating label selector for '%s!=%s'", TemplateVersionLabel, Version))
	}

	return labels.NewSelector().Add(*baseRequirement, *versionRequirement)
}

func reconcileOlderTemplates(request *common.Request) ([]common.ReconcileFunc, error) {
	existingTemplates := &templatev1.TemplateList{}
	err := request.Client.List(request.Context, existingTemplates, &client.ListOptions{
		LabelSelector: getOldTemplatesLabelSelector(),
		Namespace:     request.Instance.Spec.CommonTemplates.Namespace,
	})

	// There might not be any templates (in case of a fresh deployment), so a NotFound error is accepted
	if err != nil && !errors.IsNotFound(err) {
		return nil, err
	}

	funcs := make([]common.ReconcileFunc, 0, len(existingTemplates.Items))
	for i := range existingTemplates.Items {
		template := &existingTemplates.Items[i]

		if _, ok := deployedTemplates[template.Name]; ok {
			continue
		}

		funcs = append(funcs, func(*common.Request) (common.ReconcileResult, error) {
			return common.CreateOrUpdate(request).
				ClusterResource(template).
				WithAppLabels(operandName, operandComponent).
				UpdateFunc(func(_, foundRes client.Object) {
					foundTemplate := foundRes.(*templatev1.Template)
					foundTemplate.Annotations[TemplateDeprecatedAnnotation] = "true"
					for key := range foundTemplate.Labels {
						if strings.HasPrefix(key, TemplateOsLabelPrefix) ||
							strings.HasPrefix(key, TemplateFlavorLabelPrefix) ||
							strings.HasPrefix(key, TemplateWorkloadLabelPrefix) {
							delete(foundTemplate.Labels, key)
						}
					}
				}).
				Reconcile()
		})
	}

	return funcs, nil
}

func reconcileTemplatesFuncs(templatesBundle []templatev1.Template) []common.ReconcileFunc {
	funcs := make([]common.ReconcileFunc, 0, len(templatesBundle))
	for i := range templatesBundle {
		template := &templatesBundle[i]
		funcs = append(funcs, func(request *common.Request) (common.ReconcileResult, error) {
			namespace := request.Instance.Spec.CommonTemplates.Namespace
			template.ObjectMeta.Namespace = namespace
			return common.CreateOrUpdate(request).
				ClusterResource(template).
				WithAppLabels(operandName, operandComponent).
				UpdateFunc(func(newRes, foundRes client.Object) {
					newTemplate := newRes.(*templatev1.Template)
					foundTemplate := foundRes.(*templatev1.Template)
					foundTemplate.Objects = newTemplate.Objects
					foundTemplate.Parameters = newTemplate.Parameters
				}).
				Reconcile()
		})
	}
	return funcs
}

func ReadTemplates(filename string) ([]templatev1.Template, error) {
	var bundle []templatev1.Template
	file, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	decoder := yaml.NewYAMLOrJSONDecoder(bytes.NewReader(file), 1024)
	for {
		template := templatev1.Template{}
		err = decoder.Decode(&template)
		if err == io.EOF {
			return bundle, nil
		}
		if err != nil {
			return nil, err
		}
		if template.Name != "" {
			bundle = append(bundle, template)
		}
	}
}
