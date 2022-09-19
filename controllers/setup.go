package controllers

import (
	"context"
	"fmt"
	"path/filepath"

	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"

	"kubevirt.io/ssp-operator/internal/common"
	"kubevirt.io/ssp-operator/internal/operands"
	common_templates "kubevirt.io/ssp-operator/internal/operands/common-templates"
	data_sources "kubevirt.io/ssp-operator/internal/operands/data-sources"
	"kubevirt.io/ssp-operator/internal/operands/metrics"
	node_labeller "kubevirt.io/ssp-operator/internal/operands/node-labeller"
	template_validator "kubevirt.io/ssp-operator/internal/operands/template-validator"
	template_bundle "kubevirt.io/ssp-operator/internal/template-bundle"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

func CreateAndStartReconciler(ctx context.Context, mgr controllerruntime.Manager) error {
	templatesFile := filepath.Join(templateBundleDir, "common-templates-"+common_templates.Version+".yaml")
	templatesBundle, err := template_bundle.ReadBundle(templatesFile)
	if err != nil {
		return fmt.Errorf("failed to read template bundle: %w", err)
	}

	sspOperands := []operands.Operand{
		metrics.New(),
		template_validator.New(),
		common_templates.New(templatesBundle.Templates),
		data_sources.New(templatesBundle.DataSources),
		node_labeller.New(),
	}

	var requiredCrds []string
	for i := range sspOperands {
		requiredCrds = append(requiredCrds, sspOperands[i].RequiredCrds()...)
	}

	// Check if all needed CRDs exist
	crdList := &extv1.CustomResourceDefinitionList{}
	err = mgr.GetAPIReader().List(ctx, crdList)
	if err != nil {
		return fmt.Errorf("failed to list CRDs: %w", err)
	}

	infrastructureTopology, err := common.GetInfrastructureTopology(ctx, mgr.GetAPIReader())
	if err != nil {
		return fmt.Errorf("failed to get infrastructure topology: %w", err)
	}

	serviceController, err := CreateServiceController(ctx, mgr)
	if err != nil {
		return fmt.Errorf("failed to create service controller: %w", err)
	}

	err = mgr.Add(manager.RunnableFunc(func(ctx context.Context) error {
		err := serviceController.Start(ctx, mgr)
		if err != nil {
			return fmt.Errorf("error starting serviceController: %w", err)
		}

		mgr.GetLogger().Info("Services Controller started")

		return nil
	}))
	if err != nil {
		return fmt.Errorf("error adding service controller: %w", err)
	}

	reconciler := NewSspReconciler(mgr.GetClient(), mgr.GetAPIReader(), infrastructureTopology, sspOperands)

	if requiredCrdsExist(requiredCrds, crdList.Items) {
		// No need to start CRD controller
		err := reconciler.setupController(mgr)
		if err != nil {
			return fmt.Errorf("error setting up SSP controller: %w", err)
		}

	} else {
		mgr.GetLogger().Info("Required CRDs do not exist. Waiting until they are installed.",
			"required_crds", requiredCrds,
		)

		crdController, err := CreateCrdController(mgr, requiredCrds)
		if err != nil {
			return fmt.Errorf("failed to create crd controller: %w", err)
		}

		err = mgr.Add(manager.RunnableFunc(func(ctx context.Context) error {
			// First start the CRD controller
			err := crdController.Start(ctx)
			if err != nil {
				return fmt.Errorf("error from crd controller: %w", err)
			}

			mgr.GetLogger().Info("Required CRDs were installed, starting SSP operator.")

			// Clear variable, so it can be garbage collected
			crdController = nil

			// After it is finished, add the SSP controller to the manager
			return reconciler.setupController(mgr)
		}))
		if err != nil {
			return err
		}
	}

	mgr.GetLogger().Info("starting manager")
	if err := mgr.Start(ctx); err != nil {
		mgr.GetLogger().Error(err, "problem running manager")
		return err
	}
	return nil
}

func requiredCrdsExist(required []string, foundCrds []extv1.CustomResourceDefinition) bool {
OuterLoop:
	for i := range required {
		for j := range foundCrds {
			if required[i] == foundCrds[j].Name {
				continue OuterLoop
			}
		}
		return false
	}
	return true
}
