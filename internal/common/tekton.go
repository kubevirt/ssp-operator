package common

import (
	"fmt"

	pipeline "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	v1 "k8s.io/api/core/v1"
	rbac "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func RemoveTektonResource(objects []client.Object, request *Request) error {
	for _, resource := range objects {
		err := request.Client.Delete(request.Context, resource)
		if errors.IsNotFound(err) {
			continue
		}
		if err != nil {
			request.Logger.Error(err, fmt.Sprintf("Error deleting tekton resource \"%s\": %s", resource.GetName(), err))
			return err
		}
	}
	return nil
}

func GetPipelinesManagedByTTO(request *Request) ([]pipeline.Pipeline, error) {
	pipelines := &pipeline.PipelineList{}
	err := request.Client.List(request.Context, pipelines, &client.MatchingLabels{
		AppKubernetesManagedByLabel: TektonAppKubernetesManagedByValue,
	})
	if err != nil {
		return nil, err
	}
	return pipelines.Items, nil
}

func GetConfigMapsManagedByTTO(request *Request) ([]v1.ConfigMap, error) {
	configMaps := &v1.ConfigMapList{}
	err := request.Client.List(request.Context, configMaps, &client.MatchingLabels{
		AppKubernetesManagedByLabel: TektonAppKubernetesManagedByValue,
	})
	if err != nil {
		return nil, err
	}
	return configMaps.Items, nil
}

func GetClusterRolesManagedByTTO(request *Request) ([]rbac.ClusterRole, error) {
	clusterRoles := &rbac.ClusterRoleList{}
	err := request.Client.List(request.Context, clusterRoles, &client.MatchingLabels{
		AppKubernetesManagedByLabel: TektonAppKubernetesManagedByValue,
	})
	if err != nil {
		return nil, err
	}
	return clusterRoles.Items, nil
}

func GetServiceAccountsManagedByTTO(request *Request) ([]v1.ServiceAccount, error) {
	serviceAccounts := &v1.ServiceAccountList{}
	err := request.Client.List(request.Context, serviceAccounts, &client.MatchingLabels{
		AppKubernetesManagedByLabel: TektonAppKubernetesManagedByValue,
	})
	if err != nil {
		return nil, err
	}
	return serviceAccounts.Items, nil
}

func GetRoleBindingsManagedByTTO(request *Request) ([]rbac.RoleBinding, error) {
	roleBindings := &rbac.RoleBindingList{}
	err := request.Client.List(request.Context, roleBindings, &client.MatchingLabels{
		AppKubernetesManagedByLabel: TektonAppKubernetesManagedByValue,
	})
	if err != nil {
		return nil, err
	}
	return roleBindings.Items, nil
}
