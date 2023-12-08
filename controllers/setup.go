package controllers

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/go-logr/logr"
	v1 "github.com/openshift/api/config/v1"
	kubevirtv1 "kubevirt.io/api/core/v1"
	"kubevirt.io/ssp-operator/internal/common"
	crd_watch "kubevirt.io/ssp-operator/internal/crd-watch"
	"kubevirt.io/ssp-operator/internal/operands"
	common_instancetypes "kubevirt.io/ssp-operator/internal/operands/common-instancetypes"
	common_templates "kubevirt.io/ssp-operator/internal/operands/common-templates"
	data_sources "kubevirt.io/ssp-operator/internal/operands/data-sources"
	"kubevirt.io/ssp-operator/internal/operands/metrics"
	tekton_pipelines "kubevirt.io/ssp-operator/internal/operands/tekton-pipelines"
	tekton_tasks "kubevirt.io/ssp-operator/internal/operands/tekton-tasks"
	template_validator "kubevirt.io/ssp-operator/internal/operands/template-validator"
	vm_console_proxy "kubevirt.io/ssp-operator/internal/operands/vm-console-proxy"
	tekton_bundle "kubevirt.io/ssp-operator/internal/tekton-bundle"
	template_bundle "kubevirt.io/ssp-operator/internal/template-bundle"
	vm_console_proxy_bundle "kubevirt.io/ssp-operator/internal/vm-console-proxy-bundle"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

// Need to watch CRDs
// +kubebuilder:rbac:groups=apiextensions.k8s.io,resources=customresourcedefinitions,verbs=list;watch

func CreateAndStartReconciler(ctx context.Context, mgr controllerruntime.Manager) error {
	mgrCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	mgrCtx = logr.NewContext(mgrCtx, mgr.GetLogger())

	if err := setupManager(mgrCtx, cancel, mgr); err != nil {
		return fmt.Errorf("failed to setup manager: %w", err)
	}

	mgr.GetLogger().Info("starting manager")
	if err := mgr.Start(mgrCtx); err != nil {
		mgr.GetLogger().Error(err, "problem running manager")
		return fmt.Errorf("failed to start manager: %w", err)
	}
	return nil
}

func setupManager(ctx context.Context, cancel context.CancelFunc, mgr controllerruntime.Manager) error {
	runningOnOpenShift, err := common.RunningOnOpenshift(ctx, mgr.GetAPIReader())
	if err != nil {
		return fmt.Errorf("failed to check if running on openshift: %w", err)
	}

	templatesFile := filepath.Join(templateBundleDir, "common-templates-"+common_templates.Version+".yaml")
	templatesBundle, err := template_bundle.ReadBundle(templatesFile)
	if err != nil {
		return fmt.Errorf("failed to read template bundle: %w", err)
	}

	vmConsoleProxyBundlePath := vm_console_proxy_bundle.GetBundlePath()
	vmConsoleProxyBundle, err := vm_console_proxy_bundle.ReadBundle(vmConsoleProxyBundlePath)
	if err != nil {
		return fmt.Errorf("failed to read vm-console-proxy bundle: %w", err)
	}

	tektonPipelinesBundlePaths := tekton_bundle.GetTektonPipelineBundlePaths()
	tektonPipelinesBundle, err := tekton_bundle.ReadBundle(tektonPipelinesBundlePaths)
	if err != nil {
		return fmt.Errorf("failed to read tekton pipelines bundle: %w", err)
	}

	tektonTasksBundlePath := tekton_bundle.GetTektonTasksBundlePath(runningOnOpenShift)
	tektonTasksBundle, err := tekton_bundle.ReadBundle([]string{tektonTasksBundlePath})
	if err != nil {
		return fmt.Errorf("failed to read tekton tasks bundle: %w", err)
	}

	tektonPipelinesOperand := tekton_pipelines.New(tektonPipelinesBundle)
	tektonTasksOperand := tekton_tasks.New(tektonTasksBundle)

	sspOperands := []operands.Operand{
		// The bundle paths are not hardcoded within New to allow tests to use a different path
		common_instancetypes.New(
			common_instancetypes.BundleDir+common_instancetypes.ClusterInstancetypesBundle,
			common_instancetypes.BundleDir+common_instancetypes.ClusterPreferencesBundle,
		),
		data_sources.New(templatesBundle.DataSources),
		// Tekton Tasks Operand should be before Pipelines to avoid errors
		tektonTasksOperand,
		tektonPipelinesOperand,
	}

	if runningOnOpenShift {
		sspOperands = append(sspOperands,
			metrics.New(),
			template_validator.New(),
			common_templates.New(templatesBundle.Templates),
			vm_console_proxy.New(vmConsoleProxyBundle),
		)
	}

	var requiredCrds []string

	for i := range sspOperands {
		requiredCrds = append(requiredCrds, getRequiredCrds(sspOperands[i])...)
	}

	// Add VMController necessary VirtualMachine CRD
	vmKind := strings.ToLower(kubevirtv1.VirtualMachineGroupVersionKind.Kind) + "s"
	vmCRD := vmKind + "." + kubevirtv1.VirtualMachineGroupVersionKind.Group
	requiredCrds = append(requiredCrds, vmCRD)

	crdWatch := crd_watch.New(mgr.GetCache(), requiredCrds...)
	// Cleanly stops the manager and exit. The pod will be restarted.
	crdWatch.AllCrdsAddedHandler = cancel
	crdWatch.SomeCrdRemovedHandler = cancel

	if err = crdWatch.Init(ctx, mgr.GetAPIReader()); err != nil {
		return fmt.Errorf("failed to initialize CRD watch: %w", err)
	}

	if missingCrds := crdWatch.MissingCrds(); len(missingCrds) > 0 {
		mgr.GetLogger().Error(nil, "Some required crds are missing. The operator will not create any new resources.",
			"missingCrds", missingCrds,
		)
	}

	infrastructureTopology := v1.HighlyAvailableTopologyMode
	if runningOnOpenShift {
		infrastructureTopology, err = common.GetInfrastructureTopology(ctx, mgr.GetAPIReader())
		if err != nil {
			return fmt.Errorf("failed to get infrastructure topology: %w", err)
		}
	}

	if err = mgr.Add(crdWatch); err != nil {
		return fmt.Errorf("failed to add CRD watch to manager: %w", err)
	}

	serviceController, err := CreateServiceController(ctx, mgr)
	if err != nil {
		return fmt.Errorf("failed to create service controller: %w", err)
	}

	if err = mgr.Add(getRunnable(mgr, serviceController)); err != nil {
		return fmt.Errorf("error adding service controller: %w", err)
	}

	if crdWatch.CrdExists(vmCRD) {
		vmController, cErr := CreateVmController(mgr)
		if cErr != nil {
			return fmt.Errorf("[vm controller] failed to create vm controller: %w", cErr)
		}

		if cErr = mgr.Add(getRunnable(mgr, vmController)); cErr != nil {
			return fmt.Errorf("[vm controller] error adding: %w", cErr)
		}

		mgr.GetLogger().Info("[vm controller] added")
	}

	reconciler := NewSspReconciler(mgr.GetClient(), mgr.GetAPIReader(), infrastructureTopology, sspOperands, crdWatch)

	return reconciler.setupController(mgr)
}

func getRequiredCrds(operand operands.Operand) []string {
	var result []string
	for _, watchType := range operand.WatchTypes() {
		if watchType.Crd != "" {
			result = append(result, watchType.Crd)
		}
	}
	for _, watchType := range operand.WatchClusterTypes() {
		if watchType.Crd != "" {
			result = append(result, watchType.Crd)
		}
	}
	return result
}

func getRunnable(mgr controllerruntime.Manager, ctrl ControllerReconciler) manager.Runnable {
	return manager.RunnableFunc(func(ctx context.Context) error {
		mgr.GetLogger().Info(fmt.Sprintf("Starting %s", ctrl.Name()))
		err := ctrl.Start(ctx, mgr)
		if err != nil {
			return fmt.Errorf("error starting %s: %w", ctrl.Name(), err)
		}

		mgr.GetLogger().Info(fmt.Sprintf("%s started", ctrl.Name()))

		return nil
	})
}
