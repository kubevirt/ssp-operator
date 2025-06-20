package common_templates

import (
	"fmt"
	"regexp"
	"runtime"
	"slices"
	"strings"

	"github.com/blang/semver/v4"
	"github.com/go-logr/logr"
	templatev1 "github.com/openshift/api/template/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

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
	templatesByArch map[string][]templatev1.Template
}

var _ operands.Operand = &commonTemplates{}

func New(templatesByArch map[string][]templatev1.Template) operands.Operand {
	// TODO -- maybe split here based on arch
	return &commonTemplates{templatesByArch: templatesByArch}
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
	var clusterArchs []string
	if request.Instance.Spec.EnableMultipleArchitectures != nil && *request.Instance.Spec.EnableMultipleArchitectures {
		if request.Instance.Spec.Cluster == nil {
			return nil, fmt.Errorf("SSP .spec.cluster needs to be non-nil, if multi-architecture is enabled")
		}
		clusterArchs = request.Instance.Spec.Cluster.WorkloadArchitectures
	} else {
		if request.Instance.Spec.Cluster != nil && len(request.Instance.Spec.Cluster.ControlPlaneArchitectures) > 0 {
			// Take the first architecture of the control plane
			clusterArchs = []string{request.Instance.Spec.Cluster.ControlPlaneArchitectures[0]}
		} else {
			// TODO -- is this correct? Yes, at least for backward compat.
			clusterArchs = []string{runtime.GOARCH}
		}
	}

	// TODO -- this is memory inefficient, improve
	var templates []templatev1.Template
	templatesSet := map[string]struct{}{}
	for _, arch := range clusterArchs {
		for i := range c.templatesByArch[arch] {
			template := &c.templatesByArch[arch][i]
			templates = append(templates, *template)
			templatesSet[template.Name] = struct{}{}
		}
	}

	reconcileTemplatesResults, err := common.CollectResourceStatus(request, reconcileTemplatesFuncs(templates)...)
	if err != nil {
		return nil, err
	}

	// TODO -- how will this metric handle cluster change?
	if !operatorIsUpgrading(request) && !request.InstanceChanged {
		incrementTemplatesRestoredMetric(reconcileTemplatesResults, request.Logger)
	}

	oldTemplateFuncs, err := c.deprecateOrDeleteOldTemplates(request, templatesSet, clusterArchs)
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
	ownedTemplates, err := listAllOwnedTemplates(request)
	if err != nil && !errors.IsNotFound(err) {
		return nil, fmt.Errorf("error listing owned templates: %w", err)
	}

	var results []common.CleanupResult
	for _, obj := range ownedTemplates {
		result, err := common.Cleanup(request, &obj)
		if err != nil {
			return nil, fmt.Errorf("error cleaning up template %s/%s: %w", obj.Namespace, obj.Name, err)
		}
		results = append(results, result)
	}

	return results, nil
}

func (c *commonTemplates) deprecateOrDeleteOldTemplates(request *common.Request, deployedTemplateNames map[string]struct{}, archs []string) ([]common.ReconcileFunc, error) {
	existingTemplates, err := listAllOwnedTemplates(request)
	// There might not be any templates (in case of a fresh deployment), so a NotFound error is accepted
	if err != nil && !errors.IsNotFound(err) {
		return nil, err
	}

	latestTemplateVersion, err := semver.ParseTolerant(Version)
	if err != nil {
		return nil, err
	}

	// TODO -- do not preallocate (move to earlier commit) - a lot of existing templates
	funcs := make([]common.ReconcileFunc, 0, len(existingTemplates))
	for i := range existingTemplates {
		template := &existingTemplates[i]

		if _, ok := deployedTemplateNames[template.Name]; ok {
			continue
		}

		if !template.DeletionTimestamp.IsZero() {
			continue
		}

		templateArch := template.Labels[TemplateArchitectureLabel]
		if templateArch == "" {
			templateArch = TemplateDefaultArchitecture
		}

		// Delete the template, if it is not one of the cluster architectures
		if !slices.Contains(archs, templateArch) {
			// TODO -- investigate object lifetimes (may not be gc collected)
			funcs = append(funcs, reconcileDeleteTemplate(template))
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

		// TODO -- investigate object lifetimes (may not be gc collected)
		funcs = append(funcs, reconcileDeprecateTemplate(template))
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

func listAllOwnedTemplates(request *common.Request) ([]templatev1.Template, error) {
	return common.ListOwnedResources[templatev1.TemplateList, templatev1.Template](request)
}
