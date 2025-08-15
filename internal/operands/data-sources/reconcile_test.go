package data_sources

import (
	"context"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	cdiv1beta1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	ssp "kubevirt.io/ssp-operator/api/v1beta3"
	"kubevirt.io/ssp-operator/internal"
	"kubevirt.io/ssp-operator/internal/architecture"
	"kubevirt.io/ssp-operator/internal/common"
	"kubevirt.io/ssp-operator/internal/operands"
	common_templates "kubevirt.io/ssp-operator/internal/operands/common-templates"
	template_bundle "kubevirt.io/ssp-operator/internal/template-bundle"
	. "kubevirt.io/ssp-operator/internal/test-utils"
)

var log = logf.Log.WithName("data-sources operand")

const (
	namespace = "kubevirt"
	name      = "test-ssp"
)

var _ = Describe("Data-Sources operand", func() {
	const (
		centos8 = "centos8"
		win10   = "win10"
	)

	var (
		dataSourceCollection template_bundle.DataSourceCollection

		operand operands.Operand
		request common.Request
	)

	BeforeEach(func() {
		dataSourceCollection = template_bundle.DataSourceCollection{}

		dataSourceCollection.AddNameAndArch(centos8, architecture.AMD64)
		dataSourceCollection.AddNameAndArch(centos8, architecture.ARM64)
		dataSourceCollection.AddNameAndArch(centos8, architecture.S390X)

		dataSourceCollection.AddNameAndArch(win10, architecture.AMD64)
		dataSourceCollection.AddNameAndArch(win10, architecture.ARM64)

		operand = New(dataSourceCollection, false)

		client := fake.NewClientBuilder().WithScheme(common.Scheme).Build()
		request = common.Request{
			Request: reconcile.Request{
				NamespacedName: types.NamespacedName{
					Namespace: namespace,
					Name:      name,
				},
			},
			Client:         client,
			UncachedReader: client,
			Context:        context.Background(),
			Instance: &ssp.SSP{
				TypeMeta: metav1.TypeMeta{
					Kind:       "SSP",
					APIVersion: ssp.GroupVersion.String(),
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: namespace,
				},
				Spec: ssp.SSPSpec{
					CommonTemplates: ssp.CommonTemplates{
						Namespace: namespace,
					},
				},
			},
			Logger:       log,
			VersionCache: common.VersionCache{},
		}
	})

	It("should create golden-images namespace", func() {
		_, err := operand.Reconcile(&request)
		Expect(err).ToNot(HaveOccurred())
		ExpectResourceExists(newGoldenImagesNS(internal.GoldenImagesNamespace), request)
	})

	It("should create view role", func() {
		_, err := operand.Reconcile(&request)
		Expect(err).ToNot(HaveOccurred())
		ExpectResourceExists(newViewRole(internal.GoldenImagesNamespace), request)
	})

	It("should create view role binding", func() {
		_, err := operand.Reconcile(&request)
		Expect(err).ToNot(HaveOccurred())
		ExpectResourceExists(newViewRoleBinding(internal.GoldenImagesNamespace), request)
	})

	It("should create edit role", func() {
		_, err := operand.Reconcile(&request)
		Expect(err).ToNot(HaveOccurred())
		ExpectResourceExists(newEditRole(), request)
	})

	It("should create DataSources", func() {
		_, err := operand.Reconcile(&request)
		Expect(err).ToNot(HaveOccurred())

		for name := range dataSourceCollection.Names() {
			ExpectResourceExists(testDataSource(name), request)
		}
	})

	DescribeTable("should create NetworkPolicies", func(runningOnOpenShift bool) {
		operand = New(dataSourceCollection, runningOnOpenShift)

		_, err := operand.Reconcile(&request)
		Expect(err).ToNot(HaveOccurred())

		for _, policy := range newNetworkPolicies(internal.GoldenImagesNamespace, runningOnOpenShift) {
			ExpectResourceExists(policy, request)
		}
	},
		Entry("on Kubernetes", false),
		Entry("on OpenShift", true),
	)

	Context("with DataImportCron template", func() {
		var (
			cronTemplate ssp.DataImportCronTemplate
		)

		BeforeEach(func() {
			dataSourceName := centos8
			cronTemplate = ssp.DataImportCronTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Name: dataSourceName,
				},
				Spec: cdiv1beta1.DataImportCronSpec{
					ManagedDataSource: dataSourceName,
				},
			}

			request.Instance.Spec.CommonTemplates.DataImportCronTemplates = []ssp.DataImportCronTemplate{cronTemplate}
		})

		Context("without existing PVC", func() {
			It("should create DataImportCron in golden images namespace", func() {
				_, err := operand.Reconcile(&request)
				Expect(err).ToNot(HaveOccurred())

				createdDataImportCron := cdiv1beta1.DataImportCron{}
				err = request.Client.Get(request.Context, client.ObjectKey{
					Name:      cronTemplate.GetName(),
					Namespace: internal.GoldenImagesNamespace,
				}, &createdDataImportCron)
				Expect(err).ToNot(HaveOccurred())
				Expect(createdDataImportCron.Spec).To(Equal(cronTemplate.Spec))
			})

			It("should remove DataImportCron if template removed from SSP CR in golden images namespace", func() {
				_, err := operand.Reconcile(&request)
				Expect(err).ToNot(HaveOccurred())

				cron := cronTemplate.AsDataImportCron()
				cron.Namespace = internal.GoldenImagesNamespace
				ExpectResourceExists(&cron, request)

				request.Instance.Spec.CommonTemplates.DataImportCronTemplates = nil

				_, err = operand.Reconcile(&request)
				Expect(err).ToNot(HaveOccurred())

				ExpectResourceNotExists(&cron, request)
			})

			It("should create DataImportCron in other namespace", func() {
				cronTemplate.Namespace = "other-namespace"
				request.Instance.Spec.CommonTemplates.DataImportCronTemplates = []ssp.DataImportCronTemplate{cronTemplate}

				_, err := operand.Reconcile(&request)
				Expect(err).ToNot(HaveOccurred())

				createdDataImportCron := cdiv1beta1.DataImportCron{}
				err = request.Client.Get(request.Context, client.ObjectKey{
					Name:      cronTemplate.GetName(),
					Namespace: cronTemplate.GetNamespace(),
				}, &createdDataImportCron)
				Expect(err).ToNot(HaveOccurred())
				Expect(createdDataImportCron.Spec).To(Equal(cronTemplate.Spec))
			})

			It("should remove DataImportCron if template removed from SSP CR in other namespace", func() {
				cronTemplate.Namespace = "other-namespace"
				request.Instance.Spec.CommonTemplates.DataImportCronTemplates = []ssp.DataImportCronTemplate{cronTemplate}

				_, err := operand.Reconcile(&request)
				Expect(err).ToNot(HaveOccurred())

				cron := cronTemplate.AsDataImportCron()
				ExpectResourceExists(&cron, request)

				request.Instance.Spec.CommonTemplates.DataImportCronTemplates = nil

				_, err = operand.Reconcile(&request)
				Expect(err).ToNot(HaveOccurred())

				ExpectResourceNotExists(&cron, request)
			})

			It("should restore DataSource if DataImportCron template is removed", func() {
				originalDataSource := testDataSource(centos8)

				_, err := operand.Reconcile(&request)
				Expect(err).ToNot(HaveOccurred())

				cron := cronTemplate.AsDataImportCron()
				cron.Namespace = internal.GoldenImagesNamespace
				ExpectResourceExists(&cron, request)

				// Update DataSource to simulate CDI
				ds := &cdiv1beta1.DataSource{}
				dsKey := client.ObjectKeyFromObject(originalDataSource)
				Expect(request.Client.Get(request.Context, dsKey, ds)).To(Succeed())

				ds.Spec.Source.PVC = &cdiv1beta1.DataVolumeSourcePVC{
					Name:      "test",
					Namespace: internal.GoldenImagesNamespace,
				}
				Expect(request.Client.Update(request.Context, ds)).To(Succeed())

				_, err = operand.Reconcile(&request)
				Expect(err).ToNot(HaveOccurred())

				request.Instance.Spec.CommonTemplates.DataImportCronTemplates = nil
				_, err = operand.Reconcile(&request)
				Expect(err).ToNot(HaveOccurred())
				ExpectResourceNotExists(&cron, request)

				// Test that DataSource was restored
				ds = &cdiv1beta1.DataSource{}
				Expect(request.Client.Get(request.Context, dsKey, ds)).To(Succeed())
				Expect(ds.Spec).To(Equal(originalDataSource.Spec))
			})

			DescribeTable("should not restore DataSource if DataImportCron is present", func(source cdiv1beta1.DataSourceSource) {
				_, err := operand.Reconcile(&request)
				Expect(err).ToNot(HaveOccurred())

				cron := cronTemplate.AsDataImportCron()
				cron.Namespace = internal.GoldenImagesNamespace
				ExpectResourceExists(&cron, request)

				// Update DataSource to simulate CDI
				ds := &cdiv1beta1.DataSource{}
				dsKey := client.ObjectKeyFromObject(testDataSource(centos8))
				Expect(request.Client.Get(request.Context, dsKey, ds)).To(Succeed())
				ds.Spec.Source = source
				Expect(request.Client.Update(request.Context, ds)).To(Succeed())

				_, err = operand.Reconcile(&request)
				Expect(err).ToNot(HaveOccurred())

				// Test that DataSource was not changed
				Expect(request.Client.Get(request.Context, dsKey, ds)).To(Succeed())
				Expect(ds.Spec.Source).To(Equal(source))
			},
				Entry("and prefers PVCs", cdiv1beta1.DataSourceSource{
					PVC: &cdiv1beta1.DataVolumeSourcePVC{
						Namespace: "test",
						Name:      "test",
					},
					Snapshot: nil,
				}),
				Entry("and prefers Snapshots", cdiv1beta1.DataSourceSource{
					PVC: nil,
					Snapshot: &cdiv1beta1.DataVolumeSourceSnapshot{
						Namespace: "test",
						Name:      "test",
					},
				}),
			)
		})

		Context("with existing PVC", func() {
			var (
				pvc *v1.PersistentVolumeClaim
			)

			BeforeEach(func() {
				pvc = &v1.PersistentVolumeClaim{
					ObjectMeta: metav1.ObjectMeta{
						Name:      centos8,
						Namespace: internal.GoldenImagesNamespace,
					},
					Spec: v1.PersistentVolumeClaimSpec{},
				}

				Expect(request.Client.Create(request.Context, pvc)).To(Succeed())
			})

			AfterEach(func() {
				err := request.Client.Delete(request.Context, pvc)
				if err != nil && !errors.IsNotFound(err) {
					Expect(err).ToNot(HaveOccurred())
				}
			})

			It("should not create DataImportCron", func() {
				_, err := operand.Reconcile(&request)
				Expect(err).ToNot(HaveOccurred())

				cron := cronTemplate.AsDataImportCron()
				cron.Namespace = internal.GoldenImagesNamespace
				ExpectResourceNotExists(&cron, request)
			})

			It("should create DataImportCron if specific label is added to DataSource", func() {
				_, err := operand.Reconcile(&request)
				Expect(err).ToNot(HaveOccurred())

				cron := cronTemplate.AsDataImportCron()
				cron.Namespace = internal.GoldenImagesNamespace
				ExpectResourceNotExists(&cron, request)

				foundDs := &cdiv1beta1.DataSource{}
				dsKey := client.ObjectKeyFromObject(testDataSource(centos8))
				Expect(request.Client.Get(request.Context, dsKey, foundDs)).To(Succeed())

				if foundDs.GetLabels() == nil {
					foundDs.SetLabels(map[string]string{})
				}
				const label = "cdi.kubevirt.io/dataImportCron"
				foundDs.GetLabels()[label] = ""

				Expect(request.Client.Update(request.Context, foundDs)).To(Succeed())

				_, err = operand.Reconcile(&request)
				Expect(err).ToNot(HaveOccurred())

				ExpectResourceExists(&cron, request)
			})
		})

		It("should keep DataImportCron, if not owned by SSP CR", func() {
			cron := &cdiv1beta1.DataImportCron{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cron",
					Namespace: internal.GoldenImagesNamespace,
				},
				Spec: cdiv1beta1.DataImportCronSpec{},
			}

			err := request.Client.Create(request.Context, cron)
			Expect(err).ToNot(HaveOccurred())

			ExpectResourceExists(cron, request)

			_, err = operand.Reconcile(&request)
			Expect(err).ToNot(HaveOccurred())

			ExpectResourceExists(cron, request)
		})
	})

	Context("with multi-arch enabled", func() {
		BeforeEach(func() {
			request.Instance.Spec.EnableMultipleArchitectures = ptr.To(true)
			request.Instance.Spec.Cluster = &ssp.Cluster{
				WorkloadArchitectures:     []string{string(architecture.AMD64), string(architecture.ARM64), string(architecture.S390X)},
				ControlPlaneArchitectures: []string{string(architecture.AMD64)},
			}
		})

		It("should create arch-specific DataSources", func() {
			_, err := operand.Reconcile(&request)
			Expect(err).ToNot(HaveOccurred())

			for dsName, archs := range dataSourceCollection {
				dataSource := testDataSource(dsName)
				for _, arch := range archs {
					key := client.ObjectKey{
						Name:      dataSource.Name + "-" + string(arch),
						Namespace: dataSource.Namespace,
					}

					found := &cdiv1beta1.DataSource{}
					Expect(request.Client.Get(request.Context, key, found)).To(Succeed())
					Expect(found.Spec.Source.PVC.Name).To(Equal(dataSource.Spec.Source.PVC.Name + "-" + string(arch)))
					Expect(found.Labels).To(HaveKeyWithValue(common_templates.TemplateArchitectureLabel, string(arch)))
				}
			}
		})

		It("should not create DataSources for architectures not supported in cluster", func() {
			request.Instance.Spec.Cluster = &ssp.Cluster{
				WorkloadArchitectures:     []string{string(architecture.S390X)},
				ControlPlaneArchitectures: []string{string(architecture.S390X)},
			}

			_, err := operand.Reconcile(&request)
			Expect(err).ToNot(HaveOccurred())

			for dsName, archs := range dataSourceCollection {
				dataSource := testDataSource(dsName)
				for _, arch := range archs {
					if arch == architecture.S390X {
						continue
					}

					Expect(request.Client.Get(request.Context, client.ObjectKey{
						Name:      dataSource.Name + "-" + string(arch),
						Namespace: dataSource.Namespace,
					}, &cdiv1beta1.DataSource{})).
						To(MatchError(errors.IsNotFound, "errors.IsNotFound"))
				}
			}
		})

		It("should remove DataSources for architecture that is no longer in cluster", func() {
			_, err := operand.Reconcile(&request)
			Expect(err).ToNot(HaveOccurred())

			key := client.ObjectKey{
				Name:      centos8 + "-" + string(architecture.S390X),
				Namespace: internal.GoldenImagesNamespace,
			}

			Expect(request.Client.Get(request.Context, key, &cdiv1beta1.DataSource{})).To(Succeed())

			request.Instance.Spec.Cluster = &ssp.Cluster{
				WorkloadArchitectures:     []string{string(architecture.AMD64), string(architecture.ARM64)},
				ControlPlaneArchitectures: []string{string(architecture.AMD64)},
			}

			_, err = operand.Reconcile(&request)
			Expect(err).ToNot(HaveOccurred())

			Expect(request.Client.Get(request.Context, key, &cdiv1beta1.DataSource{})).
				To(MatchError(errors.IsNotFound, "errors.IsNotFound"))
		})

		It("should create DataSource reference pointing to default arch DataSource", func() {
			_, err := operand.Reconcile(&request)
			Expect(err).ToNot(HaveOccurred())

			for dsName := range dataSourceCollection.Names() {
				dataSource := testDataSource(dsName)
				key := client.ObjectKey{
					Name:      dataSource.Name,
					Namespace: dataSource.Namespace,
				}

				found := &cdiv1beta1.DataSource{}
				Expect(request.Client.Get(request.Context, key, found)).To(Succeed())

				defaultDsName := dsName + "-" + string(architecture.AMD64)

				Expect(found.Spec.Source.DataSource).ToNot(BeNil())
				Expect(found.Spec.Source.DataSource.Name).To(Equal(defaultDsName))
				Expect(found.Spec.Source.DataSource.Namespace).To(Equal(dataSource.Namespace))

				Expect(found.Spec.Source.PVC).To(BeNil())
				Expect(found.Spec.Source.Snapshot).To(BeNil())
			}
		})

		It("should create DataSource reference pointing to existing DataSource", func() {
			request.Instance.Spec.Cluster = &ssp.Cluster{
				WorkloadArchitectures:     []string{string(architecture.AMD64), string(architecture.ARM64), string(architecture.S390X)},
				ControlPlaneArchitectures: []string{string(architecture.S390X)},
			}

			_, err := operand.Reconcile(&request)
			Expect(err).ToNot(HaveOccurred())

			foundCentosDs := &cdiv1beta1.DataSource{}
			Expect(request.Client.Get(request.Context, client.ObjectKey{
				Name:      centos8,
				Namespace: internal.GoldenImagesNamespace,
			}, foundCentosDs)).To(Succeed())

			// Default DataSource for centos8 can be control plane architecture
			centosDefaultDsName := centos8 + "-" + string(architecture.S390X)
			Expect(foundCentosDs.Spec.Source.DataSource).ToNot(BeNil())
			Expect(foundCentosDs.Spec.Source.DataSource.Name).To(Equal(centosDefaultDsName))
			Expect(foundCentosDs.Spec.Source.DataSource.Namespace).To(Equal(internal.GoldenImagesNamespace))

			foundWinDs := &cdiv1beta1.DataSource{}
			Expect(request.Client.Get(request.Context, client.ObjectKey{
				Name:      win10,
				Namespace: internal.GoldenImagesNamespace,
			}, foundWinDs)).To(Succeed())

			// Default DataSource for windows will be the first compatible workload arch
			winDefaultDsName := win10 + "-" + string(architecture.AMD64)
			Expect(foundWinDs.Spec.Source.DataSource).ToNot(BeNil())
			Expect(foundWinDs.Spec.Source.DataSource.Name).To(Equal(winDefaultDsName))
			Expect(foundWinDs.Spec.Source.DataSource.Namespace).To(Equal(internal.GoldenImagesNamespace))
		})

		It("should remove arch-specific DataSources when multi-arch is disabled", func() {
			_, err := operand.Reconcile(&request)
			Expect(err).ToNot(HaveOccurred())

			request.Instance.Spec.EnableMultipleArchitectures = nil

			_, err = operand.Reconcile(&request)
			Expect(err).ToNot(HaveOccurred())

			for dsName := range dataSourceCollection.Names() {
				dataSource := testDataSource(dsName)
				for _, arch := range []architecture.Arch{architecture.AMD64, architecture.ARM64, architecture.S390X} {
					key := client.ObjectKey{
						Name:      dataSource.Name + "-" + string(arch),
						Namespace: dataSource.Namespace,
					}

					Expect(request.Client.Get(request.Context, key, &cdiv1beta1.DataSource{})).
						To(MatchError(errors.IsNotFound, "errors.IsNotFound"))
				}
			}
		})

		It("should update default DataSource when multi-arch is disabled", func() {
			_, err := operand.Reconcile(&request)
			Expect(err).ToNot(HaveOccurred())

			request.Instance.Spec.EnableMultipleArchitectures = nil

			_, err = operand.Reconcile(&request)
			Expect(err).ToNot(HaveOccurred())

			for dsName := range dataSourceCollection.Names() {
				dataSource := testDataSource(dsName)
				key := client.ObjectKey{
					Name:      dataSource.Name,
					Namespace: dataSource.Namespace,
				}

				found := &cdiv1beta1.DataSource{}
				Expect(request.Client.Get(request.Context, key, found)).To(Succeed())
				Expect(found.Spec.Source.PVC.Name).To(Equal(dataSource.Spec.Source.PVC.Name))
			}
		})

		It("default DataSource should point to PVC without arch specific suffix", func() {
			pvc := &v1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      centos8,
					Namespace: internal.GoldenImagesNamespace,
				},
				Spec: v1.PersistentVolumeClaimSpec{},
			}
			Expect(request.Client.Create(request.Context, pvc)).To(Succeed())

			_, err := operand.Reconcile(&request)
			Expect(err).ToNot(HaveOccurred())

			found := &cdiv1beta1.DataSource{}
			Expect(request.Client.Get(request.Context, client.ObjectKey{
				Name:      centos8 + "-" + string(architecture.AMD64),
				Namespace: internal.GoldenImagesNamespace,
			}, found)).To(Succeed())

			Expect(found.Spec.Source.PVC.Name).To(Equal(pvc.Name))
		})

		Context("with DataImportCron template", func() {
			var (
				cronTemplate ssp.DataImportCronTemplate
				cronArchs    []architecture.Arch
			)

			BeforeEach(func() {
				cronArchs = []architecture.Arch{
					architecture.AMD64,
					architecture.ARM64,
					architecture.S390X,
				}
				cronTemplate = ssp.DataImportCronTemplate{
					ObjectMeta: metav1.ObjectMeta{
						Name: centos8,
						Annotations: map[string]string{
							DataImportCronArchsAnnotation: "amd64,arm64,s390x",
						},
					},
					Spec: cdiv1beta1.DataImportCronSpec{
						ManagedDataSource: centos8,
						Template: cdiv1beta1.DataVolume{
							Spec: cdiv1beta1.DataVolumeSpec{
								Source: &cdiv1beta1.DataVolumeSource{
									Registry: &cdiv1beta1.DataVolumeSourceRegistry{},
								},
							},
						},
					},
				}

				request.Instance.Spec.CommonTemplates.DataImportCronTemplates = []ssp.DataImportCronTemplate{cronTemplate}
			})

			It("should create multiple DataImportCrons", func() {
				_, err := operand.Reconcile(&request)
				Expect(err).ToNot(HaveOccurred())

				for _, arch := range cronArchs {
					cron := cdiv1beta1.DataImportCron{}
					Expect(request.Client.Get(request.Context, client.ObjectKey{
						Name:      cronTemplate.Name + "-" + string(arch),
						Namespace: internal.GoldenImagesNamespace,
					}, &cron)).To(Succeed())

					Expect(cron.Annotations).ToNot(HaveKey(DataImportCronArchsAnnotation))
					Expect(cron.Labels).To(HaveKeyWithValue(DataImportCronDataSourceNameLabel, cronTemplate.Spec.ManagedDataSource))
					Expect(cron.Labels).To(HaveKeyWithValue(common_templates.TemplateArchitectureLabel, string(arch)))
					Expect(cron.Spec.ManagedDataSource).To(HaveSuffix(string(arch)))
					Expect(cron.Spec.Template.Spec.Source.Registry.Platform).ToNot(BeNil())
					Expect(cron.Spec.Template.Spec.Source.Registry.Platform.Architecture).To(Equal(string(arch)))
				}
			})

			It("should not create DataImportCron for unsupported architecture", func() {
				request.Instance.Spec.Cluster.WorkloadArchitectures = []string{string(architecture.AMD64), string(architecture.ARM64)}

				_, err := operand.Reconcile(&request)
				Expect(err).ToNot(HaveOccurred())

				Expect(request.Client.Get(request.Context, client.ObjectKey{
					Name:      cronTemplate.Name + "-" + string(architecture.S390X),
					Namespace: internal.GoldenImagesNamespace,
				}, &cdiv1beta1.DataImportCron{})).
					To(MatchError(errors.IsNotFound, "errors.IsNotFound"))
			})

			It("should remove DataImportCron when multi-arch is disabled", func() {
				_, err := operand.Reconcile(&request)
				Expect(err).ToNot(HaveOccurred())

				dataImportCronList := &cdiv1beta1.DataImportCronList{}
				Expect(request.Client.List(request.Context, dataImportCronList)).To(Succeed())
				Expect(dataImportCronList.Items).To(HaveLen(3))

				request.Instance.Spec.EnableMultipleArchitectures = ptr.To(false)

				_, err = operand.Reconcile(&request)
				Expect(err).ToNot(HaveOccurred())

				dataImportCronList = &cdiv1beta1.DataImportCronList{}
				Expect(request.Client.List(request.Context, dataImportCronList)).To(Succeed())
				Expect(dataImportCronList.Items).To(HaveLen(1))

				dataImportCron := dataImportCronList.Items[0]
				Expect(dataImportCron.Name).To(Equal(cronTemplate.Name))
				Expect(dataImportCron.Annotations).ToNot(HaveKey(DataImportCronArchsAnnotation))
				Expect(dataImportCron.Spec).To(Equal(cronTemplate.Spec))
			})

			Context("without architecture annotation", func() {
				BeforeEach(func() {
					cronTemplate.Annotations = nil
					request.Instance.Spec.CommonTemplates.DataImportCronTemplates = []ssp.DataImportCronTemplate{cronTemplate}
				})

				It("should create DataImportCron for default arch, if it manages a DataSource from common template", func() {
					_, err := operand.Reconcile(&request)
					Expect(err).ToNot(HaveOccurred())

					cron := cdiv1beta1.DataImportCron{}
					Expect(request.Client.Get(request.Context, client.ObjectKey{
						Name:      cronTemplate.Name,
						Namespace: internal.GoldenImagesNamespace,
					}, &cron)).To(Succeed())

					const defaultArch = architecture.AMD64
					Expect(cron.Spec.ManagedDataSource).To(Equal(cronTemplate.Spec.ManagedDataSource + "-" + string(defaultArch)))
				})

				It("should create DataImportCron for default arch that is different from control-plane arch, if it manages a DataSource from common template", func() {
					request.Instance.Spec.Cluster = &ssp.Cluster{
						WorkloadArchitectures:     []string{string(architecture.ARM64), string(architecture.S390X)},
						ControlPlaneArchitectures: []string{string(architecture.S390X)},
					}

					cronTemplate.Name = win10
					cronTemplate.Spec.ManagedDataSource = win10
					request.Instance.Spec.CommonTemplates.DataImportCronTemplates = []ssp.DataImportCronTemplate{cronTemplate}

					_, err := operand.Reconcile(&request)
					Expect(err).ToNot(HaveOccurred())

					cron := cdiv1beta1.DataImportCron{}
					Expect(request.Client.Get(request.Context, client.ObjectKey{
						Name:      cronTemplate.Name,
						Namespace: internal.GoldenImagesNamespace,
					}, &cron)).To(Succeed())

					const defaultArch = architecture.ARM64
					Expect(cron.Spec.ManagedDataSource).To(Equal(cronTemplate.Spec.ManagedDataSource + "-" + string(defaultArch)))
				})

				It("should create an architecture agnostic DataImportCron, if it does not manage DataSource from common template", func() {
					cronTemplate.Name = "test-cron"
					cronTemplate.Spec.ManagedDataSource = "test-ds"
					request.Instance.Spec.CommonTemplates.DataImportCronTemplates = []ssp.DataImportCronTemplate{cronTemplate}

					_, err := operand.Reconcile(&request)
					Expect(err).ToNot(HaveOccurred())

					cron := cdiv1beta1.DataImportCron{}
					Expect(request.Client.Get(request.Context, client.ObjectKey{
						Name:      cronTemplate.Name,
						Namespace: internal.GoldenImagesNamespace,
					}, &cron)).To(Succeed())

					Expect(cron.Spec.ManagedDataSource).To(Equal(cronTemplate.Spec.ManagedDataSource))
				})
			})

			It("should not create DataImportCron, if default DataSource points to an existing PVC without arch specific suffix", func() {
				pvc := &v1.PersistentVolumeClaim{
					ObjectMeta: metav1.ObjectMeta{
						Name:      centos8,
						Namespace: internal.GoldenImagesNamespace,
					},
					Spec: v1.PersistentVolumeClaimSpec{},
				}
				Expect(request.Client.Create(request.Context, pvc)).To(Succeed())

				_, err := operand.Reconcile(&request)
				Expect(err).ToNot(HaveOccurred())

				defaultArch := architecture.AMD64

				for _, arch := range cronArchs {
					err := request.Client.Get(request.Context, client.ObjectKey{
						Name:      cronTemplate.Name + "-" + string(arch),
						Namespace: internal.GoldenImagesNamespace,
					}, &cdiv1beta1.DataImportCron{})

					if arch == defaultArch {
						Expect(err).To(MatchError(errors.IsNotFound, "errors.IsNotFound"))
					} else {
						Expect(err).ToNot(HaveOccurred())
					}
				}
			})

			It("should create DataSource reference for custom DataImportCron", func() {
				const name = "custom-image"
				cronTemplate.Name = name
				cronTemplate.Spec.ManagedDataSource = name

				request.Instance.Spec.CommonTemplates.DataImportCronTemplates = []ssp.DataImportCronTemplate{cronTemplate}

				_, err := operand.Reconcile(&request)
				Expect(err).ToNot(HaveOccurred())

				dataSource := &cdiv1beta1.DataSource{}
				Expect(request.Client.Get(request.Context, client.ObjectKey{
					Name:      cronTemplate.Spec.ManagedDataSource,
					Namespace: internal.GoldenImagesNamespace,
				}, dataSource)).To(Succeed())

				defaultArch := architecture.AMD64

				Expect(dataSource.Spec.Source.DataSource).ToNot(BeNil())
				Expect(dataSource.Spec.Source.DataSource.Name).To(Equal(name + "-" + string(defaultArch)))
				Expect(dataSource.Spec.Source.DataSource.Namespace).To(Equal(internal.GoldenImagesNamespace))
			})
		})
	})

	It("should remove old DataImportCron when multi-arch is enabled", func() {
		cronTemplate := ssp.DataImportCronTemplate{
			ObjectMeta: metav1.ObjectMeta{
				Name: centos8,
			},
			Spec: cdiv1beta1.DataImportCronSpec{
				ManagedDataSource: centos8,
				Template: cdiv1beta1.DataVolume{
					Spec: cdiv1beta1.DataVolumeSpec{
						Source: &cdiv1beta1.DataVolumeSource{
							Registry: &cdiv1beta1.DataVolumeSourceRegistry{},
						},
					},
				},
			},
		}
		request.Instance.Spec.CommonTemplates.DataImportCronTemplates = []ssp.DataImportCronTemplate{cronTemplate}

		_, err := operand.Reconcile(&request)
		Expect(err).ToNot(HaveOccurred())

		Expect(request.Client.Get(request.Context, client.ObjectKey{
			Name:      cronTemplate.GetName(),
			Namespace: internal.GoldenImagesNamespace,
		}, &cdiv1beta1.DataImportCron{})).To(Succeed())

		// Enable multi-arch
		cronTemplate.Annotations = map[string]string{
			DataImportCronArchsAnnotation: "amd64,arm64,s390x",
		}
		request.Instance.Spec.CommonTemplates.DataImportCronTemplates = []ssp.DataImportCronTemplate{cronTemplate}

		request.Instance.Spec.EnableMultipleArchitectures = ptr.To(true)
		request.Instance.Spec.Cluster = &ssp.Cluster{
			WorkloadArchitectures:     []string{string(architecture.AMD64), string(architecture.ARM64), string(architecture.S390X)},
			ControlPlaneArchitectures: []string{string(architecture.AMD64)},
		}

		_, err = operand.Reconcile(&request)
		Expect(err).ToNot(HaveOccurred())

		Expect(request.Client.Get(request.Context, client.ObjectKey{
			Name:      cronTemplate.GetName(),
			Namespace: internal.GoldenImagesNamespace,
		}, &cdiv1beta1.DataImportCron{})).
			To(MatchError(errors.IsNotFound, "errors.IsNotFound"))
	})
})

func testDataSource(name string) *cdiv1beta1.DataSource {
	return &cdiv1beta1.DataSource{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: internal.GoldenImagesNamespace,
		},
		Spec: cdiv1beta1.DataSourceSpec{
			Source: cdiv1beta1.DataSourceSource{
				PVC: &cdiv1beta1.DataVolumeSourcePVC{
					Name:      name,
					Namespace: internal.GoldenImagesNamespace,
				},
			},
		},
	}
}

func TestDataSources(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "DataSources Suite")
}
