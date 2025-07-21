package common_templates

import (
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/blang/semver/v4"
	"github.com/go-logr/logr"
	templatev1 "github.com/openshift/api/template/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"kubevirt.io/ssp-operator/internal/architecture"
	"kubevirt.io/ssp-operator/internal/common"
	"kubevirt.io/ssp-operator/internal/env"
	"kubevirt.io/ssp-operator/internal/operands"
	metrics "kubevirt.io/ssp-operator/pkg/monitoring/metrics/ssp-operator"
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
	templatesByArch map[architecture.Arch][]templatev1.Template
}

var _ operands.Operand = &commonTemplates{}

func New(templates []templatev1.Template) (operands.Operand, error) {
	templatesByArch := map[architecture.Arch][]templatev1.Template{}
	for _, template := range templates {
		arch, err := GetTemplateArch(&template)
		if err != nil {
			return nil, fmt.Errorf("failed to get architecture of template %s: %w", template.Name, err)
		}
		templatesByArch[arch] = append(templatesByArch[arch], template)
	}

	return &commonTemplates{templatesByArch: templatesByArch}, nil
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
	clusterArchs, err := architecture.GetSSPArchs(&request.Instance.Spec)
	if err != nil {
		return nil, err
	}

	var templates []templatev1.Template
	for _, arch := range clusterArchs {
		templates = append(templates, c.templatesByArch[arch]...)
	}

	reconcileTemplatesResults, err := common.CollectResourceStatus(request, reconcileTemplatesFuncs(templates)...)
	if err != nil {
		return nil, err
	}

	if !operatorIsUpgrading(request) && !request.InstanceChanged {
		incrementTemplatesRestoredMetric(reconcileTemplatesResults, request.Logger)
	}

	oldTemplateFuncs, err := c.deprecateOrDeleteOldTemplates(request, templates, clusterArchs)
	if err != nil {
		return nil, err
	}

	oldTemplatesResults, err := common.CollectResourceStatus(request, oldTemplateFuncs...)
	if err != nil {
		return nil, err
	}

	return append(reconcileTemplatesResults, oldTemplatesResults...), nil
}

func operatorIsUpgrading(request *common.Request) bool {
	return request.Instance.Status.ObservedVersion != env.GetOperatorVersion()
}

func incrementTemplatesRestoredMetric(reconcileResults []common.ReconcileResult, logger logr.Logger) {
	for _, reconcileResult := range reconcileResults {
		if reconcileResult.InitialResource != nil {
			oldVersion := reconcileResult.InitialResource.GetLabels()[TemplateVersionLabel]
			newVersion := reconcileResult.Resource.GetLabels()[TemplateVersionLabel]

			if reconcileResult.OperationResult == common.OperationResultUpdated && oldVersion == newVersion {
				logger.Info(fmt.Sprintf("Changes reverted in common template: %s", reconcileResult.Resource.GetName()))
				metrics.IncCommonTemplatesRestored()
			}
		}
	}
}

func (c *commonTemplates) Cleanup(request *common.Request) ([]common.CleanupResult, error) {
	var objects []client.Object
	deprecatedTemplates, err := getDeprecatedTemplates(request)
	if err != nil {
		return nil, err
	}

	for _, obj := range deprecatedTemplates.Items {
		objects = append(objects, &obj)
	}

	ownedTemplates, err := common.ListOwnedResources[templatev1.TemplateList, templatev1.Template](request)
	if err != nil {
		return nil, fmt.Errorf("failed to list owned templates: %w", err)
	}

	for _, template := range ownedTemplates {
		objects = append(objects, &template)
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

func (c *commonTemplates) deprecateOrDeleteOldTemplates(request *common.Request, deployedTemplates []templatev1.Template, archs []architecture.Arch) ([]common.ReconcileFunc, error) {
	oldTemplates := &templatev1.TemplateList{}
	err := request.Client.List(request.Context, oldTemplates, &client.ListOptions{
		LabelSelector: getOldTemplatesLabelSelector(),
		Namespace:     request.Instance.Spec.CommonTemplates.Namespace,
	})
	// There might not be any templates (in case of a fresh deployment), so a NotFound error is accepted
	if err != nil && !errors.IsNotFound(err) {
		return nil, err
	}

	latestTemplateVersion, err := semver.ParseTolerant(Version)
	if err != nil {
		return nil, err
	}

	ownedTemplates, err := common.ListOwnedResources[templatev1.TemplateList, templatev1.Template](request,
		client.InNamespace(request.Instance.Spec.CommonTemplates.Namespace))
	if err != nil {
		return nil, fmt.Errorf("failed to list owned templates: %w", err)
	}

	nonDeployedTemplates := map[string]templatev1.Template{}
	for _, template := range oldTemplates.Items {
		nonDeployedTemplates[template.Name] = template
	}
	for _, template := range ownedTemplates {
		nonDeployedTemplates[template.Name] = template
	}
	for i := range deployedTemplates {
		delete(nonDeployedTemplates, deployedTemplates[i].Name)
	}

	var funcs []common.ReconcileFunc
	for _, template := range nonDeployedTemplates {
		if !template.DeletionTimestamp.IsZero() {
			continue
		}

		// Delete the template if it is not one of the cluster architectures
		templateArch, err := GetTemplateArch(&template)
		if err != nil {
			// On error, we assume that the template has unknown architecture,
			// and it will be deleted.
			templateArch = ""
		}

		if !slices.Contains(archs, templateArch) {
			funcs = append(funcs, reconcileDeleteTemplate(&template))
			continue
		}

		// If template has lower version, than what is defined in ssp operator, deprecate it.
		// Deprecate also, if version label cannot be parsed.
		if template.Labels[TemplateVersionLabel] != "" {
			version, err := semver.ParseTolerant(template.Labels[TemplateVersionLabel])
			if err == nil && latestTemplateVersion.Compare(version) != 1 {
				continue
			}
		}

		funcs = append(funcs, reconcileDeprecateTemplate(&template))
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

func reconcileDeprecateTemplate(template *templatev1.Template) common.ReconcileFunc {
	return func(request *common.Request) (common.ReconcileResult, error) {
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
	}
}

func reconcileDeleteTemplate(template *templatev1.Template) common.ReconcileFunc {
	return func(request *common.Request) (common.ReconcileResult, error) {
		err := request.Client.Delete(request.Context, template)
		if errors.IsNotFound(err) {
			return common.ReconcileResult{
				Resource: template,
			}, nil
		}
		if err != nil {
			return common.ReconcileResult{}, fmt.Errorf(
				"error deleting template with non-cluster architecture %s/%s: %w",
				template.Namespace, template.Name, err)
		}
		return common.ResourceDeletedResult(template, common.OperationResultDeleted), nil
	}
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
