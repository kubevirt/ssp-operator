package webhooks

import (
	"fmt"
	neturl "net/url"
	"reflect"

	cronexpr "github.com/robfig/cron/v3"
	core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/validation"
	cdiv1beta1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"

	sspv1beta2 "kubevirt.io/ssp-operator/api/v1beta2"
)

// These validations were mostly copied from CDI code, so that SSP catches some validation
// errors in the webhook, before trying to create an invalid DataImportCron.
// https://github.com/kubevirt/containerized-data-importer/blob/main/pkg/apiserver/webhooks/dataimportcron-validate.go

func validateDataImportCronTemplate(cron *sspv1beta2.DataImportCronTemplate) error {
	if nameErrors := validation.IsDNS1035Label(cron.Name); len(nameErrors) > 0 {
		return fmt.Errorf("invalid name: %q", nameErrors)
	}

	if cron.Spec.Template.Spec.Source == nil || cron.Spec.Template.Spec.Source.Registry == nil {
		return fmt.Errorf("missing registry source")
	}

	if cron.Spec.Template.Spec.SourceRef != nil ||
		cron.Spec.Template.Spec.ContentType != "" ||
		len(cron.Spec.Template.Spec.Checkpoints) > 0 ||
		cron.Spec.Template.Spec.FinalCheckpoint {
		return fmt.Errorf("unsettable fields: SourceRef, ContentType, Checkpoints, FinalCheckpoint")
	}

	if err := validateDataVolumeSpec(&cron.Spec.Template.Spec); err != nil {
		return fmt.Errorf("invalid template.spec: %w", err)
	}

	if cron.Spec.Schedule != "" {
		if _, err := cronexpr.ParseStandard(cron.Spec.Schedule); err != nil {
			return fmt.Errorf("illegal cron schedule")
		}
	}

	if cron.Spec.ImportsToKeep != nil && *cron.Spec.ImportsToKeep < 0 {
		return fmt.Errorf("illegal ImportsToKeep value")
	}

	if cron.Spec.GarbageCollect != nil &&
		*cron.Spec.GarbageCollect != cdiv1beta1.DataImportCronGarbageCollectNever &&
		*cron.Spec.GarbageCollect != cdiv1beta1.DataImportCronGarbageCollectOutdated {
		return fmt.Errorf("illegal GarbageCollect value")
	}

	if dataSourceNameErrors := validation.IsDNS1035Label(cron.Spec.ManagedDataSource); len(dataSourceNameErrors) > 0 {
		return fmt.Errorf("invalid managedDataSource: %q", dataSourceNameErrors)
	}

	return nil
}

func validateDataVolumeSpec(spec *cdiv1beta1.DataVolumeSpec) error {
	if spec.PVC == nil && spec.Storage == nil {
		return fmt.Errorf("missing DataVolume PVC or Storage fields")
	}

	if spec.PVC != nil && spec.Storage != nil {
		return fmt.Errorf("duplicate storage definition, both target storage and target pvc defined")
	}

	if err := validateStorageSize(spec); err != nil {
		return err
	}

	if err := validateStorageClassName(spec); err != nil {
		return err
	}

	if spec.PVC != nil {
		accessModes := spec.PVC.AccessModes
		if len(accessModes) != 1 {
			return fmt.Errorf("required value: exactly 1 access mode is required")
		}
		if err := validateAccessMode(accessModes[0]); err != nil {
			return err
		}
		if spec.PVC.DataSource != nil || spec.PVC.DataSourceRef != nil {
			return fmt.Errorf("external population is incompatible with DataImportCrons")
		}
	} else if spec.Storage != nil {
		for _, accessMode := range spec.Storage.AccessModes {
			if err := validateAccessMode(accessMode); err != nil {
				return err
			}
		}
		if spec.Storage.DataSource != nil || spec.Storage.DataSourceRef != nil {
			return fmt.Errorf("external population is incompatible with DataImportCrons")
		}
	}

	if err := validateNumberOfSources(spec.Source); err != nil {
		return err
	}

	if registry := spec.Source.Registry; registry != nil {
		if err := validateDataVolumeSourceRegistry(registry); err != nil {
			return err
		}
	}
	return nil
}

func validateStorageSize(spec *cdiv1beta1.DataVolumeSpec) error {
	var name string
	var resourceRequests core.ResourceList

	if spec.PVC != nil {
		resourceRequests = spec.PVC.Resources.Requests
		name = "PVC"
	} else if spec.Storage != nil {
		resourceRequests = spec.Storage.Resources.Requests
		name = "Storage"
	}

	storageSize, ok := resourceRequests["storage"]
	if !ok {
		return fmt.Errorf("%s size is missing", name)
	}
	if storageSize.Value() <= 0 {
		return fmt.Errorf("%s size can't be equal or less than zero", name)
	}
	return nil
}

func validateStorageClassName(spec *cdiv1beta1.DataVolumeSpec) error {
	var sc *string

	if spec.PVC != nil {
		sc = spec.PVC.StorageClassName
	} else if spec.Storage != nil {
		sc = spec.Storage.StorageClassName
	}

	if sc == nil || *sc == "" {
		return nil
	}

	if scErrors := validation.IsDNS1123Subdomain(*sc); len(scErrors) > 0 {
		return fmt.Errorf("invalid storage class name: %q", scErrors)
	}
	return nil
}

func validateAccessMode(accessMode core.PersistentVolumeAccessMode) error {
	switch accessMode {
	case core.ReadWriteOnce:
	case core.ReadOnlyMany:
	case core.ReadWriteMany:
	case core.ReadWriteOncePod:
	default:
		return fmt.Errorf(`unsupported value: "%s": supported values: "ReadOnlyMany", "ReadWriteMany", "ReadWriteOnce", "ReadWriteOncePod"`, string(accessMode))
	}
	return nil
}

func validateNumberOfSources(source *cdiv1beta1.DataVolumeSource) error {
	numberOfSources := 0
	s := reflect.ValueOf(source).Elem()
	for i := 0; i < s.NumField(); i++ {
		if !reflect.ValueOf(s.Field(i).Interface()).IsNil() {
			numberOfSources++
		}
	}
	if numberOfSources == 0 {
		return fmt.Errorf("missing DataVolume source")
	}
	if numberOfSources > 1 {
		return fmt.Errorf("multiple DataVolume sources")
	}
	return nil
}

func validateDataVolumeSourceRegistry(sourceRegistry *cdiv1beta1.DataVolumeSourceRegistry) error {
	sourceURL := sourceRegistry.URL
	sourceIS := sourceRegistry.ImageStream
	if (sourceURL == nil && sourceIS == nil) || (sourceURL != nil && sourceIS != nil) {
		return fmt.Errorf("source registry should have either URL or ImageStream")
	}

	if sourceURL != nil {
		url, err := neturl.Parse(*sourceURL)
		if err != nil {
			return fmt.Errorf("illegal registry source URL %s: %w", *sourceURL, err)
		}

		if url.Scheme != cdiv1beta1.RegistrySchemeDocker && url.Scheme != cdiv1beta1.RegistrySchemeOci {
			return fmt.Errorf("illegal registry source URL scheme %s", url)
		}
	}

	importMethod := sourceRegistry.PullMethod
	if importMethod != nil && *importMethod != cdiv1beta1.RegistryPullPod && *importMethod != cdiv1beta1.RegistryPullNode {
		return fmt.Errorf("importMethod %s is neither %s, %s or \"\"", *importMethod, cdiv1beta1.RegistryPullPod, cdiv1beta1.RegistryPullNode)
	}

	if sourceIS != nil && *sourceIS == "" {
		return fmt.Errorf("source registry ImageStream is not valid")
	}

	if sourceIS != nil && (importMethod == nil || *importMethod != cdiv1beta1.RegistryPullNode) {
		return fmt.Errorf("source registry ImageStream is supported only with node pull import method")
	}

	return nil
}
