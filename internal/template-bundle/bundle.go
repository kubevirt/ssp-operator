package template_bundle

import (
	"bytes"
	"fmt"
	osconfv1 "github.com/openshift/api/config/v1"
	"io"
	"io/ioutil"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	kubevirtv1 "kubevirt.io/api/core/v1"

	templatev1 "github.com/openshift/api/template/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/yaml"
	cdiv1beta1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
)

type Bundle struct {
	Templates   []templatev1.Template
	DataSources []cdiv1beta1.DataSource
}

func ReadBundle(filename string, topologyMode osconfv1.TopologyMode, scheme *runtime.Scheme) (Bundle, error) {
	templates, err := readTemplates(filename)
	if err != nil {
		return Bundle{}, err
	}

	sources, err := extractDataSources(templates)
	if err != nil {
		return Bundle{}, err
	}

	codec := serializer.NewCodecFactory(scheme).LegacyCodec(kubevirtv1.GroupVersion)
	err = manageSingleReplicaInfrastructure(templates, &topologyMode, codec)
	if err != nil {
		return Bundle{}, err
	}

	return Bundle{
		Templates:   templates,
		DataSources: sources,
	}, nil
}

func readTemplates(filename string) ([]templatev1.Template, error) {
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

func extractDataSources(templates []templatev1.Template) ([]cdiv1beta1.DataSource, error) {
	uniqueNames := map[string]struct{}{}

	var dataSources []cdiv1beta1.DataSource
	for i := range templates {
		template := &templates[i]

		usesDataSources, err := vmTemplateUsesSourceRef(template)
		if err != nil {
			return nil, err
		}
		if !usesDataSources {
			continue
		}

		name, exists := findDataSourceName(template)
		if !exists {
			continue
		}

		namespace, exists := findDataSourceNamespace(template)
		if !exists {
			continue
		}

		namespacedName := namespace + "/" + name
		if _, duplicateName := uniqueNames[namespacedName]; !duplicateName {
			dataSources = append(dataSources, createDataSource(name, namespace))
			uniqueNames[namespacedName] = struct{}{}
		}
	}

	return dataSources, nil
}

func vmTemplateUsesSourceRef(template *templatev1.Template) (bool, error) {
	vmUnstructured, isVm, err := decodeToVMUnstructured(&template.Objects[0])
	if err != nil {
		return false, err
	}

	if !isVm {
		return false, fmt.Errorf("template %s contains unexpected object: %s, %s", template.Name, vmUnstructured.GetAPIVersion(), vmUnstructured.GetKind())
	}

	dataVolumes, found, err := unstructured.NestedSlice(vmUnstructured.UnstructuredContent(),
		"spec", "dataVolumeTemplates")
	if err != nil {
		return false, err
	}
	if !found {
		return false, nil
	}

	for _, volumeTemplate := range dataVolumes {
		volumeObj := volumeTemplate.(map[string]interface{})
		val, exists, err := unstructured.NestedFieldNoCopy(volumeObj,
			"spec", "sourceRef")
		if err != nil {
			return false, err
		}
		if exists && val != nil {
			return true, nil
		}
	}

	return false, nil
}

func decodeToVMUnstructured(obj *runtime.RawExtension) (*unstructured.Unstructured, bool, error) {
	vmUnstructured := &unstructured.Unstructured{}
	err := yaml.NewYAMLOrJSONDecoder(bytes.NewReader(obj.Raw), 1024).Decode(vmUnstructured)
	if err != nil {
		return nil, false, err
	}

	isVm := vmUnstructured.GetAPIVersion() == "kubevirt.io/v1" &&
		vmUnstructured.GetKind() == "VirtualMachine"
	return vmUnstructured, isVm, nil
}

func findDataSourceName(template *templatev1.Template) (string, bool) {
	const dataSourceNameOld = "SRC_PVC_NAME"
	const dataSourceName = "DATA_SOURCE_NAME"

	name, exists := findParameterValue(dataSourceName, template)
	if exists {
		return name, true
	}
	return findParameterValue(dataSourceNameOld, template)
}

func findDataSourceNamespace(template *templatev1.Template) (string, bool) {
	const dataSourceNamespaceOld = "SRC_PVC_NAMESPACE"
	const dataSourceNamespace = "DATA_SOURCE_NAMESPACE"

	name, exists := findParameterValue(dataSourceNamespace, template)
	if exists {
		return name, true
	}
	return findParameterValue(dataSourceNamespaceOld, template)
}

func findParameterValue(name string, template *templatev1.Template) (string, bool) {
	for i := range template.Parameters {
		if template.Parameters[i].Name == name {
			return template.Parameters[i].Value, true
		}
	}
	return "", false
}

func createDataSource(name, namespace string) cdiv1beta1.DataSource {
	return cdiv1beta1.DataSource{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: cdiv1beta1.DataSourceSpec{
			Source: cdiv1beta1.DataSourceSource{
				PVC: &cdiv1beta1.DataVolumeSourcePVC{
					Name:      name,
					Namespace: namespace,
				},
			},
		},
	}
}

func manageSingleReplicaInfrastructure(templates []templatev1.Template, topologyMode *osconfv1.TopologyMode, codec runtime.Encoder) error {
	if *topologyMode != osconfv1.SingleReplicaTopologyMode {
		return nil
	}

	for templateIdx := range templates {
		objects := templates[templateIdx].Objects
		for objIdx := range objects {
			rawObj := &objects[objIdx]
			vmUnstructured, isVm, err := decodeToVMUnstructured(rawObj)
			if err != nil {
				return err
			}

			if !isVm {
				continue
			}

			val, found, err := unstructured.NestedFieldNoCopy(vmUnstructured.Object, "spec", "template", "spec", "evictionStrategy")
			if err != nil {
				return err
			}

			if !found || val != string(kubevirtv1.EvictionStrategyLiveMigrate) {
				continue
			}

			unstructured.RemoveNestedField(vmUnstructured.Object, "spec", "template", "spec", "evictionStrategy")
			encoded, err := runtime.Encode(codec, vmUnstructured)
			if err != nil {
				return err
			}
			templates[templateIdx].Objects[objIdx].Raw = encoded
		}
	}
	return nil
}
