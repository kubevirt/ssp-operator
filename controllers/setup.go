package controllers

import (
	"context"
	"fmt"
	"path/filepath"

	"kubevirt.io/ssp-operator/internal/common"
	crd_watch "kubevirt.io/ssp-operator/internal/crd-watch"
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

// Need to watch CRDs
// +kubebuilder:rbac:groups=apiextensions.k8s.io,resources=customresourcedefinitions,verbs=get;list;watch

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

	mgrCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	crdWatch := crd_watch.New(requiredCrds...)
	// Cleanly stops the manager and exit. The pod will be restarted.
	crdWatch.AllCrdsAddedHandler = cancel
	crdWatch.SomeCrdRemovedHandler = cancel

	err = crdWatch.Init(mgrCtx, mgr.GetAPIReader())
	if err != nil {
		return err
	}

	if missingCrds := crdWatch.MissingCrds(); len(missingCrds) > 0 {
		mgr.GetLogger().Error(nil, "Some required crds are missing. The operator will not create any new resources.",
			"missingCrds", missingCrds,
		)
	}

	err = mgr.Add(crdWatch)
	if err != nil {
		return err
	}

	infrastructureTopology, err := common.GetInfrastructureTopology(mgrCtx, mgr.GetAPIReader())
	if err != nil {
		return fmt.Errorf("failed to get infrastructure topology: %w", err)
	}

	serviceController, err := CreateServiceController(mgrCtx, mgr)
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

	reconciler := NewSspReconciler(mgr.GetClient(), mgr.GetAPIReader(), infrastructureTopology, sspOperands, crdWatch)

	err = reconciler.setupController(mgr)
	if err != nil {
		return err
	}

	mgr.GetLogger().Info("starting manager")
	if err := mgr.Start(mgrCtx); err != nil {
		mgr.GetLogger().Error(err, "problem running manager")
		return err
	}
	return nil
}
