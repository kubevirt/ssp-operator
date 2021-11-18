package common_templates

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"strings"

	templatev1 "github.com/openshift/api/template/v1"
	"github.com/prometheus/client_golang/prometheus"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"kubevirt.io/ssp-operator/internal/common"
	"kubevirt.io/ssp-operator/internal/operands"
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
// +kubebuilder:rbac:groups=template.openshift.io,resources=templates,verbs=get;list;watch;create;update;patch;delete

func init() {
	utilruntime.Must(templatev1.Install(common.Scheme))
}

func WatchClusterTypes() []client.Object {
	return []client.Object{
		&templatev1.Template{},
	}
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
	return WatchClusterTypes()
}

func (c *commonTemplates) WatchTypes() []client.Object {
	return nil
}

func (c *commonTemplates) Reconcile(request *common.Request) ([]common.ReconcileResult, error) {
	oldTemplateFuncs, err := reconcileOlderTemplates(request)
	if err != nil {
		return nil, err
	}

	results, err := common.CollectResourceStatus(request, oldTemplateFuncs...)
	if err != nil {
		return nil, err
	}

	reconcileTemplatesResults, err := common.CollectResourceStatus(request, reconcileTemplatesFuncs(c.templatesBundle)...)
	if err != nil {
		return nil, err
	}

	upgradingNow := isUpgradingNow(request)
	for _, r := range reconcileTemplatesResults {
		if !upgradingNow && (r.OperationResult == controllerutil.OperationResultUpdated) {
			CommonTemplatesRestored.Inc()
		}
	}
	return append(results, reconcileTemplatesResults...), nil
}

func isUpgradingNow(request *common.Request) bool {
	return request.Instance.Status.ObservedVersion != common.GetOperatorVersion()
}

func (c *commonTemplates) Cleanup(request *common.Request) ([]common.CleanupResult, error) {
	objects := []client.Object{}
	namespace := request.Instance.Spec.CommonTemplates.Namespace
	for index := range c.templatesBundle {
		c.templatesBundle[index].ObjectMeta.Namespace = namespace
		objects = append(objects, &c.templatesBundle[index])
	}

	return common.DeleteAll(request, objects...)
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
