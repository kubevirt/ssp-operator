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
	"fmt"
	"io"
	"os"

	"github.com/blang/semver/v4"
	"github.com/operator-framework/api/pkg/lib/version"
	csvv1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	"github.com/spf13/cobra"
	v1 "k8s.io/api/core/v1"
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/yaml"
	"kubevirt.io/ssp-operator/internal/common"
	sigsyaml "sigs.k8s.io/yaml"
)

type generatorFlags struct {
	file                string
	dumpCRDs            bool
	removeCerts         bool
	webhookPort         int32
	csvVersion          string
	namespace           string
	operatorImage       string
	operatorVersion     string
	validatorImage      string
	vmConsoleProxyImage string
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
	if err := generateCsv(); err != nil {
		return err
	}

	if f.dumpCRDs {
		if err := dumpCrds(); err != nil {
			return err
		}
	}

	return nil
}

func generateCsv() error {
	csv := &csvv1.ClusterServiceVersion{}
	if err := readFileToObject(f.file, csv); err != nil {
		return err
	}

	if err = replaceVariables(f, csv); err != nil {
		return err
	}

	if f.removeCerts {
		removeCerts(csv)
	}

	cleanupCsv(csv)

	csv.Spec.RelatedImages = getRelatedImages(f)

	return writeObjectYaml(csv, os.Stdout)
}

func dumpCrds() error {
	files, err := os.ReadDir("data/crd")
	if err != nil {
		return err
	}
	for _, file := range files {
		crd := &extv1.CustomResourceDefinition{}

		fsInfo, err := file.Info()
		if err != nil {
			return err
		}

		if err := readFileToObject(fsInfo.Name(), crd); err != nil {
			return err
		}

		cleanupCrd(crd)
		if err := writeObjectYaml(crd, os.Stdout); err != nil {
			return err
		}
	}
	return nil
}

func getRelatedImages(flags generatorFlags) []csvv1.RelatedImage {
	var relatedImages []csvv1.RelatedImage

	relatedImages = appendRelatedImage(relatedImages, flags.validatorImage, "template-validator")
	relatedImages = appendRelatedImage(relatedImages, flags.vmConsoleProxyImage, "vm-console-proxy")

	return relatedImages
}

func appendRelatedImage(slice []csvv1.RelatedImage, image string, name string) []csvv1.RelatedImage {
	if image != "" {
		slice = append(slice, csvv1.RelatedImage{
			Name:  name,
			Image: image,
		})
	}
	return slice
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

func cleanupCsv(csv *csvv1.ClusterServiceVersion) {
	// remove status and metadata.creationTimestamp
	csv.Status = csvv1.ClusterServiceVersionStatus{}
	csv.ObjectMeta.CreationTimestamp = metav1.Time{}

	deployments := csv.Spec.InstallStrategy.StrategySpec.DeploymentSpecs
	for i := range deployments {
		deployment := &deployments[i]
		deployment.Spec.Template.ObjectMeta.CreationTimestamp = metav1.Time{}
	}
}

func cleanupCrd(crd *extv1.CustomResourceDefinition) {
	// remove status and metadata.creationTimestamp
	crd.Status = extv1.CustomResourceDefinitionStatus{}
	crd.ObjectMeta.CreationTimestamp = metav1.Time{}
}

func readFileToObject(filename string, obj runtime.Object) error {
	fileBytes, err := os.ReadFile(filename)
	if err != nil {
		return err
	}
	return yaml.NewYAMLOrJSONDecoder(bytes.NewReader(fileBytes), 1024).Decode(obj)
}

func writeObjectYaml(obj runtime.Object, writer io.Writer) error {
	yamlBytes, err := sigsyaml.Marshal(obj)
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
