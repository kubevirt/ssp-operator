package controllers

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/go-logr/logr"
	v1 "github.com/openshift/api/config/v1"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	crd_watch "kubevirt.io/ssp-operator/internal/crd-watch"
	"kubevirt.io/ssp-operator/internal/env"
	"kubevirt.io/ssp-operator/internal/operands"
	common_templates "kubevirt.io/ssp-operator/internal/operands/common-templates"
	data_sources "kubevirt.io/ssp-operator/internal/operands/data-sources"
	"kubevirt.io/ssp-operator/internal/operands/metrics"
	template_validator "kubevirt.io/ssp-operator/internal/operands/template-validator"
	vm_console_proxy "kubevirt.io/ssp-operator/internal/operands/vm-console-proxy"
	vm_delete_protection "kubevirt.io/ssp-operator/internal/operands/vm-delete-protection"
	template_bundle "kubevirt.io/ssp-operator/internal/template-bundle"
	vm_console_proxy_bundle "kubevirt.io/ssp-operator/internal/vm-console-proxy-bundle"
)

// Need to watch CRDs
// +kubebuilder:rbac:groups=apiextensions.k8s.io,resources=customresourcedefinitions,verbs=list;watch

func StartControllers(ctx context.Context, mgr controllerruntime.Manager, controllers []Controller) error {
	mgrCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	mgrCtx = logr.NewContext(mgrCtx, mgr.GetLogger())

	if err := setupManager(mgrCtx, cancel, mgr, controllers); err != nil {
		return fmt.Errorf("failed to setup manager: %w", err)
	}

	mgr.GetLogger().Info("starting manager")
	if err := mgr.Start(mgrCtx); err != nil {
		mgr.GetLogger().Error(err, "problem running manager")
		return fmt.Errorf("failed to start manager: %w", err)
	}
	return nil
}

func CreateControllers(ctx context.Context, apiReader client.Reader, olmDeployment bool, sspServiceHostname string) ([]Controller, error) {
	runningOnOpenShift, err := env.RunningOnOpenshift(ctx, apiReader)
	if err != nil {
		return nil, fmt.Errorf("failed to check if running on openshift: %w", err)
	}

	templatesBundleFile := filepath.Join(templateBundleDir, fmt.Sprintf("common-templates-%s.yaml", common_templates.Version))
	templates, err := template_bundle.ReadTemplates(templatesBundleFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read template bundle: %w", err)
	}

	dataSourceCollection, err := template_bundle.CollectDataSources(templates)
	if err != nil {
		return nil, fmt.Errorf("failed to collect DataSource names from templates: %w", err)
	}

	vmConsoleProxyBundlePath := vm_console_proxy_bundle.GetBundlePath()
	vmConsoleProxyBundle, err := vm_console_proxy_bundle.ReadBundle(vmConsoleProxyBundlePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read vm-console-proxy bundle: %w", err)
	}

	sspOperands := []operands.Operand{
		data_sources.New(dataSourceCollection, runningOnOpenShift),
		vm_delete_protection.New(),
	}

	if runningOnOpenShift {
		templatesOperand, err := common_templates.New(templates)
		if err != nil {
			return nil, fmt.Errorf("failed to create common templates operand: %w", err)
		}

		sspOperands = append(sspOperands,
			metrics.New(),
			template_validator.New(),
			templatesOperand,
			vm_console_proxy.New(vmConsoleProxyBundle),
		)
	}

	infrastructureTopology := v1.HighlyAvailableTopologyMode
	if runningOnOpenShift {
		infrastructureTopology, err = env.GetInfrastructureTopology(ctx, apiReader)
		if err != nil {
			return nil, fmt.Errorf("failed to get infrastructure topology: %w", err)
		}
	}

	serviceController, err := CreateServiceController()
	if err != nil {
		return nil, fmt.Errorf("failed to create service controller: %w", err)
	}

	return []Controller{
		serviceController,
		NewWebhookConfigurationController(),
		NewVmController(),
		NewSspController(infrastructureTopology, sspOperands, olmDeployment, sspServiceHostname),
	}, nil
}

func setupManager(ctx context.Context, cancel context.CancelFunc, mgr controllerruntime.Manager, controllers []Controller) error {
	var requiredCrds []string
	for _, controller := range controllers {
		requiredCrds = append(requiredCrds, controller.RequiredCrds()...)
	}

	crdWatch := crd_watch.New(mgr.GetCache(), requiredCrds...)
	// Cleanly stops the manager and exit. The pod will be restarted.
	crdWatch.AllCrdsAddedHandler = cancel
	crdWatch.SomeCrdRemovedHandler = cancel

	if err := crdWatch.Init(ctx, mgr.GetAPIReader()); err != nil {
		return fmt.Errorf("failed to initialize CRD watch: %w", err)
	}

	if missingCrds := crdWatch.MissingCrds(); len(missingCrds) > 0 {
		mgr.GetLogger().Error(nil, "Some required crds are missing. The operator will not create any new resources.",
			"missingCrds", missingCrds,
		)
	}

	if err := mgr.Add(crdWatch); err != nil {
		return fmt.Errorf("failed to add CRD watch to manager: %w", err)
	}

	for _, controller := range controllers {
		if err := controller.AddToManager(mgr, crdWatch); err != nil {
			return fmt.Errorf("error adding %s: %w", controller.Name(), err)
		}
	}
	return nil
}
