package webhooks

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	cdiv1beta1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"

	sspv1beta2 "kubevirt.io/ssp-operator/api/v1beta2"
)

var _ = Describe("DataImportCronTemplate validation", func() {
	var dict *sspv1beta2.DataImportCronTemplate

	BeforeEach(func() {
		dict = validDataImportCronTemplate()
	})

	It("should pass when correct DataImportCronTemplate is passed", func() {
		Expect(ValidateDataImportCronTemplate(dict)).To(Succeed())
	})

	It("should fail if name is invalid", func() {
		dict.Name = "definitely invalid name <>[]{}()--++!@#$%^&*|';:"

		err := ValidateDataImportCronTemplate(dict)
		Expect(err).To(MatchError(ContainSubstring("invalid name")))
	})

	It("should fail if registry source is not defined", func() {
		dict.Spec.Template.Spec.Source.Registry = nil

		err := ValidateDataImportCronTemplate(dict)
		Expect(err).To(MatchError(ContainSubstring("missing registry source")))
	})

	It("should fail if SourceRef is defined", func() {
		dict.Spec.Template.Spec.SourceRef = &cdiv1beta1.DataVolumeSourceRef{
			Kind: "DataSource",
			Name: "test-data-source",
		}

		err := ValidateDataImportCronTemplate(dict)
		Expect(err).To(MatchError(ContainSubstring("unsettable fields:")))
	})

	It("should fail if ContentType is defined", func() {
		dict.Spec.Template.Spec.ContentType = cdiv1beta1.DataVolumeKubeVirt

		err := ValidateDataImportCronTemplate(dict)
		Expect(err).To(MatchError(ContainSubstring("unsettable fields:")))
	})

	It("should fail if Checkpoints is defined", func() {
		dict.Spec.Template.Spec.Checkpoints = []cdiv1beta1.DataVolumeCheckpoint{{
			Previous: "fake-1",
			Current:  "fake-2",
		}}

		err := ValidateDataImportCronTemplate(dict)
		Expect(err).To(MatchError(ContainSubstring("unsettable fields:")))
	})

	It("should fail if FinalCheckpoint is defined", func() {
		dict.Spec.Template.Spec.FinalCheckpoint = true

		err := ValidateDataImportCronTemplate(dict)
		Expect(err).To(MatchError(ContainSubstring("unsettable fields:")))
	})

	Context("PVC or Storage validation", func() {
		It("should fail if both PVC and Storage are not defined", func() {
			dict.Spec.Template.Spec.Storage = nil

			err := ValidateDataImportCronTemplate(dict)
			Expect(err).To(MatchError(ContainSubstring("missing DataVolume PVC or Storage fields")))
		})

		It("should fail if both PVC and Storage are defined", func() {
			dict.Spec.Template.Spec.PVC = pvcSpecFromStorageSpec(dict.Spec.Template.Spec.Storage)

			err := ValidateDataImportCronTemplate(dict)
			Expect(err).To(MatchError(ContainSubstring("duplicate storage definition, both target storage and target pvc defined")))
		})

		It("should fail if storage size is missing in Storage", func() {
			delete(dict.Spec.Template.Spec.Storage.Resources.Requests, v1.ResourceStorage)

			err := ValidateDataImportCronTemplate(dict)
			Expect(err).To(MatchError(ContainSubstring("Storage size is missing")))
		})

		It("should fail if storage size is missing in PVC", func() {
			dict.Spec.Template.Spec.PVC = pvcSpecFromStorageSpec(dict.Spec.Template.Spec.Storage)
			dict.Spec.Template.Spec.Storage = nil

			delete(dict.Spec.Template.Spec.PVC.Resources.Requests, v1.ResourceStorage)

			err := ValidateDataImportCronTemplate(dict)
			Expect(err).To(MatchError(ContainSubstring("PVC size is missing")))
		})

		It("should fail if storage size is negative in Storage", func() {
			dict.Spec.Template.Spec.Storage.Resources.Requests[v1.ResourceStorage] = resource.MustParse("-20G")

			err := ValidateDataImportCronTemplate(dict)
			Expect(err).To(MatchError(ContainSubstring("Storage size can't be equal or less than zero")))
		})

		It("should fail if storage size is negative in PVC", func() {
			dict.Spec.Template.Spec.PVC = pvcSpecFromStorageSpec(dict.Spec.Template.Spec.Storage)
			dict.Spec.Template.Spec.Storage = nil

			dict.Spec.Template.Spec.PVC.Resources.Requests[v1.ResourceStorage] = resource.MustParse("-20G")

			err := ValidateDataImportCronTemplate(dict)
			Expect(err).To(MatchError(ContainSubstring("PVC size can't be equal or less than zero")))
		})

		It("should fail if storage class is invalid in Storage", func() {
			dict.Spec.Template.Spec.Storage.StorageClassName = ptr.To("invalid storage class <>[]{}()--++!@#$%^&*|';:")

			err := ValidateDataImportCronTemplate(dict)
			Expect(err).To(MatchError(ContainSubstring("invalid storage class name:")))
		})

		It("should fail if storage class is invalid in PVC", func() {
			dict.Spec.Template.Spec.PVC = pvcSpecFromStorageSpec(dict.Spec.Template.Spec.Storage)
			dict.Spec.Template.Spec.Storage = nil

			dict.Spec.Template.Spec.PVC.StorageClassName = ptr.To("invalid storage class <>[]{}()--++!@#$%^&*|';:")

			err := ValidateDataImportCronTemplate(dict)
			Expect(err).To(MatchError(ContainSubstring("invalid storage class name:")))
		})

		It("should fail if access mode is invalid in Storage", func() {
			dict.Spec.Template.Spec.Storage.AccessModes = []v1.PersistentVolumeAccessMode{
				"unknown-access-mode",
			}

			err := ValidateDataImportCronTemplate(dict)
			Expect(err).To(MatchError(ContainSubstring("unsupported value:")))
		})

		It("should fail if access mode is invalid in PVC", func() {
			dict.Spec.Template.Spec.PVC = pvcSpecFromStorageSpec(dict.Spec.Template.Spec.Storage)
			dict.Spec.Template.Spec.Storage = nil

			dict.Spec.Template.Spec.PVC.AccessModes = []v1.PersistentVolumeAccessMode{
				"unknown-access-mode",
			}

			err := ValidateDataImportCronTemplate(dict)
			Expect(err).To(MatchError(ContainSubstring("unsupported value:")))
		})

		It("should fail if there are multiple access modes in PVC", func() {
			dict.Spec.Template.Spec.PVC = pvcSpecFromStorageSpec(dict.Spec.Template.Spec.Storage)
			dict.Spec.Template.Spec.Storage = nil

			dict.Spec.Template.Spec.PVC.AccessModes = []v1.PersistentVolumeAccessMode{
				v1.ReadOnlyMany, v1.ReadWriteMany,
			}

			err := ValidateDataImportCronTemplate(dict)
			Expect(err).To(MatchError(ContainSubstring("required value: exactly 1 access mode is required")))
		})

		It("should fail if external population is enabled in Storage", func() {
			dict.Spec.Template.Spec.Storage.DataSource = &v1.TypedLocalObjectReference{
				Kind: "PVC",
				Name: "test-pvc",
			}

			err := ValidateDataImportCronTemplate(dict)
			Expect(err).To(MatchError(ContainSubstring("external population is incompatible with DataImportCrons")))

			dict.Spec.Template.Spec.Storage.DataSource = nil
			dict.Spec.Template.Spec.Storage.DataSourceRef = &v1.TypedObjectReference{
				Kind: "PVC",
				Name: "test-pvc",
			}

			err = ValidateDataImportCronTemplate(dict)
			Expect(err).To(MatchError(ContainSubstring("external population is incompatible with DataImportCrons")))
		})

		It("should fail if external population is enabled in PVC", func() {
			dict.Spec.Template.Spec.PVC = pvcSpecFromStorageSpec(dict.Spec.Template.Spec.Storage)
			dict.Spec.Template.Spec.Storage = nil

			dict.Spec.Template.Spec.PVC.DataSource = &v1.TypedLocalObjectReference{
				Kind: "PVC",
				Name: "test-pvc",
			}

			err := ValidateDataImportCronTemplate(dict)
			Expect(err).To(MatchError(ContainSubstring("external population is incompatible with DataImportCrons")))

			dict.Spec.Template.Spec.PVC.DataSource = nil
			dict.Spec.Template.Spec.PVC.DataSourceRef = &v1.TypedObjectReference{
				Kind: "PVC",
				Name: "test-pvc",
			}

			err = ValidateDataImportCronTemplate(dict)
			Expect(err).To(MatchError(ContainSubstring("external population is incompatible with DataImportCrons")))
		})
	})

	Context("registry source validation", func() {
		It("should fail if multiple sources are defined", func() {
			dict.Spec.Template.Spec.Source.Blank = &cdiv1beta1.DataVolumeBlankImage{}

			err := ValidateDataImportCronTemplate(dict)
			Expect(err).To(MatchError(ContainSubstring("multiple DataVolume sources")))
		})

		It("should fail if both URL and ImageStream are not defined", func() {
			dict.Spec.Template.Spec.Source.Registry.URL = nil
			dict.Spec.Template.Spec.Source.Registry.ImageStream = nil

			err := ValidateDataImportCronTemplate(dict)
			Expect(err).To(MatchError(ContainSubstring("source registry should have either URL or ImageStream")))
		})

		It("should fail if both URL and ImageStream are defined", func() {
			dict.Spec.Template.Spec.Source.Registry.URL = ptr.To("docker://example.org/image")
			dict.Spec.Template.Spec.Source.Registry.ImageStream = ptr.To("test-image")

			err := ValidateDataImportCronTemplate(dict)
			Expect(err).To(MatchError(ContainSubstring("source registry should have either URL or ImageStream")))
		})

		It("should fail if URL is invalid", func() {
			dict.Spec.Template.Spec.Source.Registry.URL = ptr.To("invalid url <>[]{}()--++!@#$%^&*|';:")
			dict.Spec.Template.Spec.Source.Registry.ImageStream = nil

			err := ValidateDataImportCronTemplate(dict)
			Expect(err).To(MatchError(ContainSubstring("illegal registry source URL")))
		})

		It("should fail if URL scheme is not 'docker' or 'oci-archive'", func() {
			dict.Spec.Template.Spec.Source.Registry.URL = ptr.To("https://example.org/image")
			dict.Spec.Template.Spec.Source.Registry.ImageStream = nil

			err := ValidateDataImportCronTemplate(dict)
			Expect(err).To(MatchError(ContainSubstring("illegal registry source URL scheme")))
		})

		It("should pass if URL scheme is 'docker'", func() {
			dict.Spec.Template.Spec.Source.Registry.URL = ptr.To("docker://example.org/image")
			dict.Spec.Template.Spec.Source.Registry.ImageStream = nil

			Expect(ValidateDataImportCronTemplate(dict)).To(Succeed())
		})

		It("should pass if URL scheme is 'oci-archive'", func() {
			dict.Spec.Template.Spec.Source.Registry.URL = ptr.To("oci-archive://example.org/image")
			dict.Spec.Template.Spec.Source.Registry.ImageStream = nil

			Expect(ValidateDataImportCronTemplate(dict)).To(Succeed())
		})

		It("should fail if pull method is not 'pod' or 'node'", func() {
			dict.Spec.Template.Spec.Source.Registry.PullMethod = ptr.To[cdiv1beta1.RegistryPullMethod]("unknown-method")

			err := ValidateDataImportCronTemplate(dict)
			Expect(err).To(MatchError(ContainSubstring("importMethod unknown-method is neither %s, %s or \"\"", cdiv1beta1.RegistryPullPod, cdiv1beta1.RegistryPullNode)))
		})

		It("should fail if ImageStream is empty", func() {
			dict.Spec.Template.Spec.Source.Registry.ImageStream = ptr.To("")

			err := ValidateDataImportCronTemplate(dict)
			Expect(err).To(MatchError(ContainSubstring("source registry ImageStream is not valid")))
		})

		It("should fail if 'node' pull method is not set when using ImageStream", func() {
			dict.Spec.Template.Spec.Source.Registry.PullMethod = ptr.To(cdiv1beta1.RegistryPullPod)

			err := ValidateDataImportCronTemplate(dict)
			Expect(err).To(MatchError(ContainSubstring("source registry ImageStream is supported only with node pull import method")))
		})
	})

	It("should fail if cron schedule is invalid", func() {
		dict.Spec.Schedule = "invalid-cron-schedule"

		err := ValidateDataImportCronTemplate(dict)
		Expect(err).To(MatchError(ContainSubstring("illegal cron schedule")))
	})

	It("should fail if ImportsToKeep is negative", func() {
		dict.Spec.ImportsToKeep = ptr.To[int32](-1)

		err := ValidateDataImportCronTemplate(dict)
		Expect(err).To(MatchError(ContainSubstring("illegal ImportsToKeep value")))
	})

	It("should fail if GarbageCollect value is invalid", func() {
		dict.Spec.GarbageCollect = ptr.To[cdiv1beta1.DataImportCronGarbageCollect]("invalid")

		err := ValidateDataImportCronTemplate(dict)
		Expect(err).To(MatchError(ContainSubstring("illegal GarbageCollect value")))
	})

	It("should fail if ManagedDataSource name is invalid", func() {
		dict.Spec.ManagedDataSource = "invalid name []<>{}()!@#%^&*:;'"

		err := ValidateDataImportCronTemplate(dict)
		Expect(err).To(MatchError(ContainSubstring("invalid managedDataSource:")))
	})
})

func validDataImportCronTemplate() *sspv1beta2.DataImportCronTemplate {
	return &sspv1beta2.DataImportCronTemplate{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-data-import-cron-template",
		},
		Spec: cdiv1beta1.DataImportCronSpec{
			Template: cdiv1beta1.DataVolume{
				TypeMeta:   metav1.TypeMeta{},
				ObjectMeta: metav1.ObjectMeta{},
				Spec: cdiv1beta1.DataVolumeSpec{
					Source: &cdiv1beta1.DataVolumeSource{
						Registry: &cdiv1beta1.DataVolumeSourceRegistry{
							ImageStream: ptr.To("image-stream"),
							PullMethod:  ptr.To(cdiv1beta1.RegistryPullNode),
						},
					},
					Storage: &cdiv1beta1.StorageSpec{
						AccessModes: []v1.PersistentVolumeAccessMode{
							v1.ReadWriteMany,
						},
						Resources: v1.VolumeResourceRequirements{
							Requests: map[v1.ResourceName]resource.Quantity{
								v1.ResourceStorage: resource.MustParse("20G"),
							},
						},
						StorageClassName: ptr.To("test-storage-class"),
					},
				},
			},
			Schedule:          "* * * * *",
			GarbageCollect:    ptr.To(cdiv1beta1.DataImportCronGarbageCollectOutdated),
			ImportsToKeep:     ptr.To[int32](3),
			ManagedDataSource: "test-data-source",
		},
	}
}

func pvcSpecFromStorageSpec(storageSpec *cdiv1beta1.StorageSpec) *v1.PersistentVolumeClaimSpec {
	return &v1.PersistentVolumeClaimSpec{
		AccessModes: storageSpec.AccessModes,
		Selector:    storageSpec.Selector,
		Resources: v1.VolumeResourceRequirements{
			Limits:   storageSpec.Resources.Limits,
			Requests: storageSpec.Resources.Requests,
		},
		VolumeName:       storageSpec.VolumeName,
		StorageClassName: storageSpec.StorageClassName,
		VolumeMode:       storageSpec.VolumeMode,
		DataSource:       storageSpec.DataSource,
		DataSourceRef:    storageSpec.DataSourceRef,
	}
}
