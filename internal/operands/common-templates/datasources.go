package common_templates

import (
	"fmt"
	"strings"

	templatev1 "github.com/openshift/api/template/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	kubevirtv1 "kubevirt.io/client-go/api/v1"
	cdiv1beta1 "kubevirt.io/containerized-data-importer/pkg/apis/core/v1beta1"
	ssp "kubevirt.io/ssp-operator/api/v1beta1"
	"kubevirt.io/ssp-operator/internal/common"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// +kubebuilder:rbac:groups=cdi.kubevirt.io,resources=datasources,verbs=get;list;watch;create;update;patch;delete

func registerDataSourceSchemes(s *runtime.Scheme) error {
	if err := cdiv1beta1.AddToScheme(s); err != nil {
		return err
	}
	return nil
}

func extractDataSourceReferencesFromTemplates(request *common.Request) (map[string]struct{}, error) {

	scheme := runtime.NewScheme()
	if err := kubevirtv1.AddToScheme(scheme); err != nil {
		return nil, err
	}
	deserializer := serializer.NewCodecFactory(scheme).UniversalDeserializer()

	datasources := make(map[string]struct{})
	for _, template := range templatesBundle {
		if err := extractDataSourcesFromTemplate(request, datasources, template, deserializer); err != nil {
			return nil, err
		}
	}

	return datasources, nil
}

func extractDataSourcesFromTemplate(
	request *common.Request,
	datasources map[string]struct{},
	template templatev1.Template,
	deserializer runtime.Decoder,
) error {
	parameterDefaults := make(map[string]string)
	for _, param := range template.Parameters {
		parameterDefaults[param.Name] = param.Value
	}
	for _, obj := range template.Objects {
		var vm kubevirtv1.VirtualMachine
		if err := runtime.DecodeInto(deserializer, obj.Raw, &vm); err != nil {
			return err
		}
		if vm.Kind != "VirtualMachine" {
			request.Logger.Info("skipping unexpected resource kind", "template", template.Name, "namespace", template.Namespace, "kind", vm.Kind)
			continue
		}
		for _, dv := range vm.Spec.DataVolumeTemplates {
			sourceRef := dv.Spec.SourceRef

			if !isDataVolumeSourceRef(sourceRef) {
				continue
			}

			sourceRefName := resolveParameter(parameterDefaults, sourceRef.Name)
			sourceRefNamespace := ssp.GoldenImagesNSname

			if sourceRef.Namespace != nil {
				sourceRefNamespace = resolveParameter(parameterDefaults, *sourceRef.Namespace)
				if sourceRefNamespace != ssp.GoldenImagesNSname {
					return fmt.Errorf("invalid namespace: %s", *sourceRef.Namespace)
				}
			}

			datasources[sourceRefName] = struct{}{}
		}
	}
	return nil
}

func isDataVolumeSourceRef(sourceRef *cdiv1beta1.DataVolumeSourceRef) bool {
	if sourceRef == nil {
		return false
	}
	if sourceRef.Kind != cdiv1beta1.DataVolumeDataSource {
		return false
	}

	return true
}

func resolveParameter(params map[string]string, param string) string {
	key := strings.TrimPrefix(param, "${")
	key = strings.TrimSuffix(key, "}")
	if value, found := params[key]; found {
		return value
	}
	return param
}

func reconcileDataSources(request *common.Request) ([]common.ReconcileFunc, error) {
	datasourceNames, err := extractDataSourceReferencesFromTemplates(request)
	if err != nil {
		return nil, err
	}
	datasources := buildDataSources(datasourceNames)

	funcs := make([]common.ReconcileFunc, 0, len(datasources))
	for i := range datasources {
		datasource := &datasources[i]
		funcs = append(funcs, func(request *common.Request) (common.ResourceStatus, error) {
			return common.CreateOrUpdate(request).
				ClusterResource(datasource).
				WithAppLabels(operandName, operandComponent).
				UpdateFunc(func(newRes, foundRes client.Object) {
					// we do not want to update the datasource as that should be handled by DataImportCrons or the admin
				}).
				Reconcile()
		})
	}
	return funcs, nil
}

func buildDataSources(names map[string]struct{}) []cdiv1beta1.DataSource {
	var datasources []cdiv1beta1.DataSource
	for name := range names {
		datasources = append(datasources, buildDataSource(name))
	}
	return datasources
}

func buildDataSource(name string) cdiv1beta1.DataSource {
	return cdiv1beta1.DataSource{
		TypeMeta: v1.TypeMeta{
			APIVersion: cdiv1beta1.SchemeGroupVersion.String(),
			Kind:       cdiv1beta1.DataVolumeDataSource,
		},
		ObjectMeta: v1.ObjectMeta{
			Name:      name,
			Namespace: ssp.GoldenImagesNSname,
		},
		Spec: cdiv1beta1.DataSourceSpec{
			Source: cdiv1beta1.DataSourceSource{
				PVC: &cdiv1beta1.DataVolumeSourcePVC{
					Name:      name,
					Namespace: ssp.GoldenImagesNSname,
				},
			},
		},
	}
}
