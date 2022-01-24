package data_sources

import (
	"context"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	cdiv1beta1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	ssp "kubevirt.io/ssp-operator/api/v1beta1"
	"kubevirt.io/ssp-operator/internal/common"
	"kubevirt.io/ssp-operator/internal/operands"
	. "kubevirt.io/ssp-operator/internal/test-utils"
)

var log = logf.Log.WithName("data-sources operand")

const (
	namespace = "kubevirt"
	name      = "test-ssp"
)

var _ = Describe("Data-Sources operand", func() {
	var (
		testDataSources []cdiv1beta1.DataSource

		operand operands.Operand
		request common.Request
	)

	BeforeEach(func() {
		testDataSources = getDataSources()

		operand = New(testDataSources)

		client := fake.NewFakeClientWithScheme(common.Scheme)
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
		ExpectResourceExists(newGoldenImagesNS(ssp.GoldenImagesNSname), request)
	})

	It("should create view role", func() {
		_, err := operand.Reconcile(&request)
		Expect(err).ToNot(HaveOccurred())
		ExpectResourceExists(newViewRole(ssp.GoldenImagesNSname), request)
	})

	It("should create view role binding", func() {
		_, err := operand.Reconcile(&request)
		Expect(err).ToNot(HaveOccurred())
		ExpectResourceExists(newViewRoleBinding(ssp.GoldenImagesNSname), request)
	})

	It("should create edit role", func() {
		_, err := operand.Reconcile(&request)
		Expect(err).ToNot(HaveOccurred())
		ExpectResourceExists(newEditRole(), request)
	})

	It("should create DataSources", func() {
		_, err := operand.Reconcile(&request)
		Expect(err).ToNot(HaveOccurred())

		for _, ds := range testDataSources {
			ExpectResourceExists(&ds, request)
		}
	})

	Context("with DataImportCron template", func() {
		var (
			cronTemplate ssp.DataImportCronTemplate
		)

		BeforeEach(func() {
			dataSourceName := testDataSources[0].GetName()
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
			It("should create DataImportCron", func() {
				_, err := operand.Reconcile(&request)
				Expect(err).ToNot(HaveOccurred())

				createdDataImportCron := cdiv1beta1.DataImportCron{}
				err = request.Client.Get(request.Context, client.ObjectKey{
					Name:      cronTemplate.GetName(),
					Namespace: ssp.GoldenImagesNSname,
				}, &createdDataImportCron)
				Expect(err).ToNot(HaveOccurred())
				Expect(createdDataImportCron.Spec).To(Equal(cronTemplate.Spec))
			})

			It("should not create DataSource", func() {
				_, err := operand.Reconcile(&request)
				Expect(err).ToNot(HaveOccurred())

				ExpectResourceNotExists(&testDataSources[0], request)
			})

			It("should remove DataImportCron if template removed from SSP CR", func() {
				_, err := operand.Reconcile(&request)
				Expect(err).ToNot(HaveOccurred())

				cron := cronTemplate.AsDataImportCron()
				cron.Namespace = ssp.GoldenImagesNSname
				ExpectResourceExists(&cron, request)

				request.Instance.Spec.CommonTemplates.DataImportCronTemplates = nil

				_, err = operand.Reconcile(&request)
				Expect(err).ToNot(HaveOccurred())

				ExpectResourceNotExists(&cron, request)
			})

			It("should restore DataSource if DataImportCron template is removed", func() {
				_, err := operand.Reconcile(&request)
				Expect(err).ToNot(HaveOccurred())

				cron := cronTemplate.AsDataImportCron()
				cron.Namespace = ssp.GoldenImagesNSname
				ExpectResourceExists(&cron, request)

				ExpectResourceNotExists(&testDataSources[0], request)

				request.Instance.Spec.CommonTemplates.DataImportCronTemplates = nil
				_, err = operand.Reconcile(&request)
				Expect(err).ToNot(HaveOccurred())

				ExpectResourceNotExists(&cron, request)
				ExpectResourceExists(&testDataSources[0], request)
			})
		})

		Context("with existing PVC", func() {
			var (
				pvc *v1.PersistentVolumeClaim
			)

			BeforeEach(func() {
				pvc = &v1.PersistentVolumeClaim{
					ObjectMeta: metav1.ObjectMeta{
						Name:      testDataSources[0].Spec.Source.PVC.Name,
						Namespace: testDataSources[0].Spec.Source.PVC.Namespace,
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
				cron.Namespace = ssp.GoldenImagesNSname
				ExpectResourceNotExists(&cron, request)
			})

			It("should create DataImportCron if specific label is added to DataSource", func() {
				_, err := operand.Reconcile(&request)
				Expect(err).ToNot(HaveOccurred())

				cron := cronTemplate.AsDataImportCron()
				cron.Namespace = ssp.GoldenImagesNSname
				ExpectResourceNotExists(&cron, request)

				foundDs := &cdiv1beta1.DataSource{}
				Expect(request.Client.Get(request.Context, client.ObjectKeyFromObject(&testDataSources[0]), foundDs)).To(Succeed())

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
					Namespace: ssp.GoldenImagesNSname,
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
})

func getDataSources() []cdiv1beta1.DataSource {
	const name1 = "centos8"
	const name2 = "win10"

	return []cdiv1beta1.DataSource{{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name1,
			Namespace: ssp.GoldenImagesNSname,
		},
		Spec: cdiv1beta1.DataSourceSpec{
			Source: cdiv1beta1.DataSourceSource{
				PVC: &cdiv1beta1.DataVolumeSourcePVC{
					Name:      name1,
					Namespace: ssp.GoldenImagesNSname,
				},
			},
		},
	}, {
		ObjectMeta: metav1.ObjectMeta{
			Name:      name2,
			Namespace: ssp.GoldenImagesNSname,
		},
		Spec: cdiv1beta1.DataSourceSpec{
			Source: cdiv1beta1.DataSourceSource{
				PVC: &cdiv1beta1.DataVolumeSourcePVC{
					Name:      name2,
					Namespace: ssp.GoldenImagesNSname,
				},
			},
		},
	}}
}

func TestDataSources(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "DataSources Suite")
}
