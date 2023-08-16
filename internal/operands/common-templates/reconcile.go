package common_templates

import (
	"fmt"
	"regexp"
	"strings"

	templatev1 "github.com/openshift/api/template/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/blang/semver/v4"
	"github.com/go-logr/logr"
	"github.com/prometheus/client_golang/prometheus"

	"kubevirt.io/ssp-operator/internal/common"
	"kubevirt.io/ssp-operator/internal/operands"
)

var (
	CommonTemplatesRestored = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "kubevirt_ssp_common_templates_restored_total",
		Help: "The total number of common templates restored by the operator back to their original state",
	})
)

// Define RBAC rules needed by this operand:
// +kubebuilder:rbac:groups=template.openshift.io,resources=templates,verbs=get;list;watch;create;update;patch;delete

var templateKubevirtIoPattern = regexp.MustCompile(`^(.*\.)?template\.kubevirt\.io/`)

func init() {
	utilruntime.Must(templatev1.Install(common.Scheme))
}

func WatchClusterTypes() []operands.WatchType {
	return []operands.WatchType{
		{Object: &templatev1.Template{}},
	}
}

type commonTemplates struct {
	templatesBundle   []templatev1.Template
	deployedTemplates map[string]bool
}

var _ operands.Operand = &commonTemplates{}

func New(templates []templatev1.Template) operands.Operand {
	deployedTemplates := make(map[string]bool)
	for _, t := range templates {
		deployedTemplates[t.Name] = true
	}
	return &commonTemplates{templatesBundle: templates, deployedTemplates: deployedTemplates}
}

func (c *commonTemplates) Name() string {
	return operandName
}

const (
	operandName      = "common-templates"
	operandComponent = common.AppComponentTemplating
)

func (c *commonTemplates) WatchClusterTypes() []operands.WatchType {
	return WatchClusterTypes()
}

func (c *commonTemplates) WatchTypes() []operands.WatchType {
	return nil
}

func (c *commonTemplates) Reconcile(request *common.Request) ([]common.ReconcileResult, error) {
	reconcileTemplatesResults, err := common.CollectResourceStatus(request, reconcileTemplatesFuncs(c.templatesBundle)...)
	if err != nil {
		return nil, err
	}

	if !isUpgradingNow(request) {
		incrementTemplatesRestoredMetric(reconcileTemplatesResults, request.Logger)
	}

	oldTemplateFuncs, err := c.reconcileOlderTemplates(request)
	if err != nil {
		return nil, err
	}

	oldTemplatesResults, err := common.CollectResourceStatus(request, oldTemplateFuncs...)
	if err != nil {
		return nil, err
	}

	return append(reconcileTemplatesResults, oldTemplatesResults...), nil
}

func isUpgradingNow(request *common.Request) bool {
	return request.Instance.Status.ObservedVersion != common.GetOperatorVersion()
}

func incrementTemplatesRestoredMetric(reconcileResults []common.ReconcileResult, logger logr.Logger) {
	for _, reconcileResult := range reconcileResults {
		if reconcileResult.InitialResource != nil {
			oldVersion := reconcileResult.InitialResource.GetLabels()[TemplateVersionLabel]
			newVersion := reconcileResult.Resource.GetLabels()[TemplateVersionLabel]

			if reconcileResult.OperationResult == common.OperationResultUpdated && oldVersion == newVersion {
				logger.Info(fmt.Sprintf("Changes reverted in common template: %s", reconcileResult.Resource.GetName()))
				CommonTemplatesRestored.Inc()
			}
		}
	}
}

func (c *commonTemplates) Cleanup(request *common.Request) ([]common.CleanupResult, error) {
	var objects []client.Object
	namespace := request.Instance.Spec.CommonTemplates.Namespace

	deprecatedTemplates, err := getDeprecatedTemplates(request)
	if err != nil {
		return nil, err
	}

	for _, obj := range deprecatedTemplates.Items {
		obj.ObjectMeta.Namespace = namespace
		objects = append(objects, &obj)
	}

	for index := range c.templatesBundle {
		c.templatesBundle[index].ObjectMeta.Namespace = namespace
		objects = append(objects, &c.templatesBundle[index])
	}

	return common.DeleteAll(request, objects...)
}

func getDeprecatedTemplates(request *common.Request) (*templatev1.TemplateList, error) {
	deprecatedTemplates := &templatev1.TemplateList{}
	err := request.Client.List(request.Context, deprecatedTemplates, &client.ListOptions{
		LabelSelector: getDeprecatedTemplatesLabelSelector(),
		Namespace:     request.Instance.Spec.CommonTemplates.Namespace,
	})

	// There might not be any templates (in case of a fresh deployment), so a NotFound error is accepted
	if err != nil && !errors.IsNotFound(err) {
		return nil, err
	}
	return deprecatedTemplates, nil
}

func getDeprecatedTemplatesLabelSelector() labels.Selector {
	deprecatedRequirement, err := labels.NewRequirement(TemplateDeprecatedAnnotation, selection.Equals, []string{"true"})
	if err != nil {
		panic(fmt.Sprintf("Failed creating label selector for '%s=%s'", TemplateDeprecatedAnnotation, "true"))
	}
	return labels.NewSelector().Add(*deprecatedRequirement)
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

func (c *commonTemplates) reconcileOlderTemplates(request *common.Request) ([]common.ReconcileFunc, error) {
	existingTemplates := &templatev1.TemplateList{}
	err := request.Client.List(request.Context, existingTemplates, &client.ListOptions{
		LabelSelector: getOldTemplatesLabelSelector(),
		Namespace:     request.Instance.Spec.CommonTemplates.Namespace,
	})

	// There might not be any templates (in case of a fresh deployment), so a NotFound error is accepted
	if err != nil && !errors.IsNotFound(err) {
		return nil, err
	}

	templatesVersion, err := semver.ParseTolerant(Version)
	if err != nil {
		return nil, err
	}

	funcs := make([]common.ReconcileFunc, 0, len(existingTemplates.Items))
	for i := range existingTemplates.Items {
		template := &existingTemplates.Items[i]

		if _, ok := c.deployedTemplates[template.Name]; ok {
			continue
		}

		// if template has higher version than is defined in ssp operator, keep it as it is. If parsing
		// of template version fails, continue with adding deprecated label
		if template.Labels[TemplateVersionLabel] != "" {
			v, err := semver.ParseTolerant(template.Labels[TemplateVersionLabel])
			if err == nil && templatesVersion.Compare(v) == -1 {
				continue
			}
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
					foundTemplate.Labels[TemplateDeprecatedAnnotation] = "true"
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

					// Remove old annotations and labels, if they are not present in the new template.
					// This is useful when new a common-templates version removed some annotations or labels.
					syncPredefinedAnnotationsAndLabels(foundTemplate, newTemplate)

					foundTemplate.Objects = newTemplate.Objects
					foundTemplate.Parameters = newTemplate.Parameters
				}).
				Reconcile()
		})
	}
	return funcs
}

func syncPredefinedAnnotationsAndLabels(foundTemplate, newTemplate *templatev1.Template) {
	for annotation := range foundTemplate.Annotations {
		if isPredefinedKey(annotation) {
			if _, exists := newTemplate.Annotations[annotation]; !exists {
				delete(foundTemplate.Annotations, annotation)
			}
		}
	}

	for label := range foundTemplate.Labels {
		if isPredefinedKey(label) {
			if _, exists := newTemplate.Labels[label]; !exists {
				delete(foundTemplate.Labels, label)
			}
		}
	}
}

func isPredefinedKey(key string) bool {
	return key == "description" ||
		key == "tags" ||
		key == "iconClass" ||
		strings.HasPrefix(key, "openshift.io/") ||
		templateKubevirtIoPattern.MatchString(key)
}
