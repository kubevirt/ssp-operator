/*


Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/blang/semver/v4"
	"github.com/operator-framework/api/pkg/lib/version"
	csvv1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	"github.com/spf13/cobra"
	v1 "k8s.io/api/core/v1"
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/yaml"
	"kubevirt.io/ssp-operator/internal/common"
	sigsyaml "sigs.k8s.io/yaml"
)

type generatorFlags struct {
	file                     string
	dumpCRDs                 bool
	removeCerts              bool
	webhookPort              int32
	csvVersion               string
	namespace                string
	operatorImage            string
	operatorVersion          string
	validatorImage           string
	tektonTasksImage         string
	tektonTasksDiskVirtImage string
	virtioImage              string
	vmConsoleProxyImage      string
}

var (
	f       generatorFlags
	rootCmd = &cobra.Command{
		Use:   "csv-generator",
		Short: "csv-generator for ssp operator",
		Long:  `csv-generator generates deploy manifest for ssp operator`,
		Run: func(cmd *cobra.Command, args []string) {
			err := runGenerator()
			if err != nil {
				fmt.Fprintf(os.Stderr, "%v\n", err)
				os.Exit(1)
			}
		},
	}
)

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.Flags().StringVar(&f.file, "file", "data/olm-catalog/ssp-operator.clusterserviceversion.yaml", "Location of the CSV yaml to modify")
	rootCmd.Flags().StringVar(&f.csvVersion, "csv-version", "", "Version of csv manifest (required)")
	rootCmd.Flags().StringVar(&f.namespace, "namespace", "", "Namespace in which ssp operator will be deployed (required)")
	rootCmd.Flags().StringVar(&f.operatorImage, "operator-image", "", "Link to operator image (required)")
	rootCmd.Flags().StringVar(&f.operatorVersion, "operator-version", "", "Operator version (required)")
	rootCmd.Flags().StringVar(&f.validatorImage, "validator-image", "", "Link to template-validator image")
	rootCmd.Flags().StringVar(&f.tektonTasksImage, "tekton-tasks-image", "", "Link to tekton tasks image")
	rootCmd.Flags().StringVar(&f.tektonTasksDiskVirtImage, "tekton-tasks-disk-virt-image", "", "Link to tekton tasks disk virt image")
	rootCmd.Flags().StringVar(&f.virtioImage, "virtio-image", "", "Link to virtio image")
	rootCmd.Flags().StringVar(&f.vmConsoleProxyImage, "vm-console-proxy-image", "", "Link to VM console proxy image")
	rootCmd.Flags().Int32Var(&f.webhookPort, "webhook-port", 0, "Container port for the admission webhook")
	rootCmd.Flags().BoolVar(&f.removeCerts, "webhook-remove-certs", false, "Remove the webhook certificate volume and mount")
	rootCmd.Flags().BoolVar(&f.dumpCRDs, "dump-crds", false, "Dump crds to stdout")

	if err := rootCmd.MarkFlagRequired("csv-version"); err != nil {
		panic(fmt.Sprintf("%v", err))
	}
	if err := rootCmd.MarkFlagRequired("namespace"); err != nil {
		panic(fmt.Sprintf("%v", err))
	}
	if err := rootCmd.MarkFlagRequired("operator-image"); err != nil {
		panic(fmt.Sprintf("%v", err))
	}
	if err := rootCmd.MarkFlagRequired("operator-version"); err != nil {
		panic(fmt.Sprintf("%v", err))
	}
}

func runGenerator() error {
	csvFile, err := os.ReadFile(f.file)
	if err != nil {
		return err
	}

	decoder := yaml.NewYAMLOrJSONDecoder(bytes.NewReader(csvFile), 1024)
	csv := csvv1.ClusterServiceVersion{}
	err = decoder.Decode(&csv)
	if err != nil {
		return err
	}

	err = replaceVariables(f, &csv)
	if err != nil {
		return err
	}

	if f.removeCerts {
		removeCerts(&csv)
	}

	relatedImages, err := buildRelatedImages(f)
	if err != nil {
		return err
	}

	err = marshallObject(csv, relatedImages, os.Stdout)
	if err != nil {
		return err
	}
	if !f.dumpCRDs {
		return nil
	}
	files, err := os.ReadDir("data/crd")
	if err != nil {
		return err
	}
	for _, file := range files {
		crd := extv1.CustomResourceDefinition{}

		fsInfo, err := file.Info()
		if err != nil {
			return err
		}

		err = readAndDecodeToCRD(fsInfo, &crd)
		if err != nil {
			return err
		}

		err = marshallObject(crd, nil, os.Stdout)
		if err != nil {
			return err
		}
	}
	return nil
}

func readAndDecodeToCRD(file os.FileInfo, crd *extv1.CustomResourceDefinition) error {
	crdFile, err := os.ReadFile(fmt.Sprintf("data/crd/%s", file.Name()))
	if err != nil {
		return err
	}
	decoder := yaml.NewYAMLOrJSONDecoder(bytes.NewReader(crdFile), 1024)
	err = decoder.Decode(&crd)
	if err != nil {
		return err
	}
	return nil
}

func buildRelatedImage(imageDesc string, imageName string) (map[string]interface{}, error) {
	ri := make(map[string]interface{})
	ri["name"] = imageName
	ri["image"] = imageDesc

	return ri, nil
}

func buildRelatedImages(flags generatorFlags) ([]interface{}, error) {
	var relatedImages = make([]interface{}, 0)

	if flags.validatorImage != "" {
		relatedImage, err := buildRelatedImage(flags.validatorImage, "template-validator")
		if err != nil {
			return nil, err
		}
		relatedImages = append(relatedImages, relatedImage)
	}

	if flags.tektonTasksImage != "" {
		relatedImage, err := buildRelatedImage(flags.tektonTasksImage, "tekton-tasks")
		if err != nil {
			return nil, err
		}
		relatedImages = append(relatedImages, relatedImage)
	}

	if flags.tektonTasksDiskVirtImage != "" {
		relatedImage, err := buildRelatedImage(flags.tektonTasksDiskVirtImage, "tekton-tasks-disk-virt")
		if err != nil {
			return nil, err
		}
		relatedImages = append(relatedImages, relatedImage)
	}

	if flags.virtioImage != "" {
		relatedImage, err := buildRelatedImage(flags.virtioImage, "virtio-container")
		if err != nil {
			return nil, err
		}
		relatedImages = append(relatedImages, relatedImage)
	}

	if flags.vmConsoleProxyImage != "" {
		relatedImage, err := buildRelatedImage(flags.vmConsoleProxyImage, "vm-console-proxy")
		if err != nil {
			return nil, err
		}
		relatedImages = append(relatedImages, relatedImage)
	}

	return relatedImages, nil
}

func replaceVariables(flags generatorFlags, csv *csvv1.ClusterServiceVersion) error {
	csv.Name = "ssp-operator.v" + flags.csvVersion
	v, err := semver.New(flags.csvVersion)
	if err != nil {
		return err
	}
	csv.Spec.Version = version.OperatorVersion{
		Version: *v,
	}

	templateSpec := &csv.Spec.InstallStrategy.StrategySpec.DeploymentSpecs[0].Spec.Template.Spec
	for i, container := range templateSpec.Containers {
		updatedContainer := container
		if container.Name == "manager" {
			updatedContainer.Image = flags.operatorImage
			updatedContainer.Env = updateContainerEnvVars(flags, container)
			templateSpec.Containers[i] = updatedContainer
			break
		}
	}

	if flags.webhookPort > 0 {
		csv.Spec.WebhookDefinitions[0].ContainerPort = flags.webhookPort
	}

	return nil
}

func updateContainerEnvVars(flags generatorFlags, container v1.Container) []v1.EnvVar {
	updatedVariables := make([]v1.EnvVar, 0)
	for _, envVariable := range container.Env {
		switch envVariable.Name {
		case common.TemplateValidatorImageKey:
			if flags.validatorImage != "" {
				envVariable.Value = flags.validatorImage
			}
		case common.OperatorVersionKey:
			if flags.operatorVersion != "" {
				envVariable.Value = flags.operatorVersion
			}
		case common.TektonTasksImageKey:
			if flags.tektonTasksImage != "" {
				envVariable.Value = flags.tektonTasksImage
			}
		case common.TektonTasksDiskVirtImageKey:
			if flags.tektonTasksDiskVirtImage != "" {
				envVariable.Value = flags.tektonTasksDiskVirtImage
			}
		case common.VirtioImageKey:
			if flags.virtioImage != "" {
				envVariable.Value = flags.virtioImage
			}
		case common.VmConsoleProxyImageKey:
			if flags.vmConsoleProxyImage != "" {
				envVariable.Value = flags.vmConsoleProxyImage
			}
		}

		updatedVariables = append(updatedVariables, envVariable)
	}
	return updatedVariables
}

func removeCerts(csv *csvv1.ClusterServiceVersion) {
	// Remove the certs mount from the manager container
	templateSpec := &csv.Spec.InstallStrategy.StrategySpec.DeploymentSpecs[0].Spec.Template.Spec
	for i, container := range templateSpec.Containers {
		if container.Name == "manager" {
			updatedVolumeMounts := templateSpec.Containers[i].VolumeMounts
			for j, volumeMount := range templateSpec.Containers[i].VolumeMounts {
				if volumeMount.Name == "cert" {
					updatedVolumeMounts = append(templateSpec.Containers[i].VolumeMounts[:j], templateSpec.Containers[i].VolumeMounts[j+1:]...)
					break
				}
			}
			templateSpec.Containers[i].VolumeMounts = updatedVolumeMounts
			break
		}
	}

	// Remove the cert volume definition
	updatedVolumes := templateSpec.Volumes
	for i, volume := range templateSpec.Volumes {
		if volume.Name == "cert" {
			updatedVolumes = append(templateSpec.Volumes[:i], templateSpec.Volumes[i+1:]...)
		}
	}
	templateSpec.Volumes = updatedVolumes
}

func marshallObject(obj interface{}, relatedImages []interface{}, writer io.Writer) error {
	jsonBytes, err := json.Marshal(obj)
	if err != nil {
		return err
	}

	var r unstructured.Unstructured
	if err := json.Unmarshal(jsonBytes, &r.Object); err != nil {
		return err
	}

	// remove status and metadata.creationTimestamp
	unstructured.RemoveNestedField(r.Object, "metadata", "creationTimestamp")
	unstructured.RemoveNestedField(r.Object, "template", "metadata", "creationTimestamp")
	unstructured.RemoveNestedField(r.Object, "spec", "template", "metadata", "creationTimestamp")
	unstructured.RemoveNestedField(r.Object, "status")

	deployments, exists, err := unstructured.NestedSlice(r.Object, "spec", "install", "spec", "deployments")
	if err != nil {
		return err
	}

	if exists {
		for _, obj := range deployments {
			deployment := obj.(map[string]interface{})
			unstructured.RemoveNestedField(deployment, "metadata", "creationTimestamp")
			unstructured.RemoveNestedField(deployment, "spec", "template", "metadata", "creationTimestamp")
			unstructured.RemoveNestedField(deployment, "status")
		}
		if err = unstructured.SetNestedSlice(r.Object, deployments, "spec", "install", "spec", "deployments"); err != nil {
			return err
		}
	}

	if len(relatedImages) > 0 {
		if err = unstructured.SetNestedSlice(r.Object, relatedImages, "spec", "relatedImages"); err != nil {
			return err
		}
	}

	yamlBytes, err := sigsyaml.Marshal(r.Object)
	if err != nil {
		return err
	}

	_, err = writer.Write([]byte("---\n"))
	if err != nil {
		return err
	}

	_, err = writer.Write(yamlBytes)
	if err != nil {
		return err
	}

	return nil
}
