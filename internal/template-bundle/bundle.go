package template_bundle

import (
	"bytes"
	"fmt"
	"io"
	"os"

	templatev1 "github.com/openshift/api/template/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/yaml"

	"kubevirt.io/ssp-operator/internal"
	common_templates "kubevirt.io/ssp-operator/internal/operands/common-templates"
)

// TODO -- check if there is validation that there are no duplicate templates

func ReadTemplates(filename string) (map[string][]templatev1.Template, error) {
	templatesByArch := map[string][]templatev1.Template{}
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	// Ignoring error from close, because we have already read the data.
	defer func() { _ = file.Close() }()

	decoder := yaml.NewYAMLToJSONDecoder(file)
	for {
		template := templatev1.Template{}
		err = decoder.Decode(&template)
		if err == io.EOF {
			return templatesByArch, nil
		}
		if err != nil {
			return nil, err
		}
		if template.Name == "" {
			continue
		}

		// TODO -- possibility of duplicate templates?
		templateArch := template.Labels[common_templates.TemplateArchitectureLabel]
		if templateArch == "" {
			templateArch = common_templates.TemplateDefaultArchitecture
		}
		templatesByArch[templateArch] = append(templatesByArch[templateArch], template)
	}
}

func CollectDataSourceNames(templatesByArch map[string][]templatev1.Template) (map[string][]string, error) {
	// TODO -- simplify logic
	dataSourceArchs := map[string]map[string]struct{}{}
	for arch, templates := range templatesByArch {
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
			// This check is needed, so later code can assume that all DataSources
			// should be created in the internal.GoldenImagesNamespace
			if exists && namespace != internal.GoldenImagesNamespace {
				// If this happens, it is a programmer's error.
				return nil, fmt.Errorf(
					"common template %s has invalid default DATA_SOURCE_NAMESPACE value: %s, expected: %s",
					template.Name, namespace, internal.GoldenImagesNamespace)
			}

			if dataSourceArchs[name] == nil {
				dataSourceArchs[name] = map[string]struct{}{}
			}
			dataSourceArchs[name][arch] = struct{}{}
		}
	}

	result := make(map[string][]string, len(dataSourceArchs))
	for name, archs := range dataSourceArchs {
		result[name] = make([]string, 0, len(archs))
		for arch := range archs {
			result[name] = append(result[name], arch)
		}
	}

	return result, nil
}

func vmTemplateUsesSourceRef(template *templatev1.Template) (bool, error) {
	vmUnstructured := &unstructured.Unstructured{}
	err := yaml.NewYAMLOrJSONDecoder(bytes.NewReader(template.Objects[0].Raw), 1024).Decode(vmUnstructured)
	if err != nil {
		return false, err
	}

	isVm := vmUnstructured.GetAPIVersion() == "kubevirt.io/v1" &&
		vmUnstructured.GetKind() == "VirtualMachine"
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
