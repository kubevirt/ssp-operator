package controllers

import (
	"context"
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"path/filepath"

	"sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"kubevirt.io/ssp-operator/internal/operands"
	"kubevirt.io/ssp-operator/internal/operands/common-templates"
	"kubevirt.io/ssp-operator/internal/operands/data-sources"
	"kubevirt.io/ssp-operator/internal/operands/metrics"
	"kubevirt.io/ssp-operator/internal/operands/node-labeller"
	"kubevirt.io/ssp-operator/internal/operands/template-validator"
	template_bundle "kubevirt.io/ssp-operator/internal/template-bundle"
)

func CreateAndSetupReconciler(mgr controllerruntime.Manager) error {
	templatesFile := filepath.Join(templateBundleDir, "common-templates-"+common_templates.Version+".yaml")
	templatesBundle, err := template_bundle.ReadBundle(templatesFile)
	if err != nil {
		return err
	}

	sspOperands := []operands.Operand{
		metrics.New(),
		template_validator.New(),
		common_templates.New(templatesBundle.Templates),
		data_sources.New(),
		node_labeller.New(),
	}

	var requiredCrds []string
	for i := range sspOperands {
		requiredCrds = append(requiredCrds, sspOperands[i].RequiredCrds()...)
	}

	// Check if all needed CRDs exist
	crdList := &extv1.CustomResourceDefinitionList{}
	err = mgr.GetAPIReader().List(context.TODO(), crdList)
	if err != nil {
		return err
	}

	reconciler := NewSspReconciler(mgr.GetClient(), sspOperands)

	if requiredCrdsExist(requiredCrds, crdList.Items) {
		// No need to start CRD controller
		return reconciler.setupController(mgr)
	}

	mgr.GetLogger().Info("Required CRDs do not exist. Waiting until they are installed.",
		"required_crds", requiredCrds,
	)

	crdController, err := CreateCrdController(mgr, requiredCrds)
	if err != nil {
		return err
	}

	return mgr.Add(manager.RunnableFunc(func(ctx context.Context) error {
		// First start the CRD controller
		err := crdController.Start(ctx)
		if err != nil {
			return err
		}

		mgr.GetLogger().Info("Required CRDs were installed, starting SSP operator.")

		// Clear variable, so it can be garbage collected
		crdController = nil

		// After it is finished, add the SSP controller to the manager
		return reconciler.setupController(mgr)
	}))
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
