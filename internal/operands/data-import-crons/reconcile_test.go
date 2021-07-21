package dataimportcrons

import (
	"context"
	"fmt"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	. "kubevirt.io/ssp-operator/internal/test-utils"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	cdiv1beta1 "kubevirt.io/containerized-data-importer/pkg/apis/core/v1beta1"
	ssp "kubevirt.io/ssp-operator/api/v1beta1"
	"kubevirt.io/ssp-operator/internal/common"
)

var (
	log     = logf.Log.WithName("data-import-cron-operand")
	operand = GetOperand()
)

const (
	namespace = "kubevirt"
	name      = "test-ssp"
)

func TestDataImportCron(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Data Import Cron Suite")
}

var _ = Describe("DataImportCrons operand", func() {
	const dataImportCronName = "dataimportcron-"
	var (
		request     common.Request
		importCrons = []ssp.DataImportCronTemplate{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: dataImportCronName + "1",
				},
				Spec: cdiv1beta1.DataImportCronSpec{
					Source: cdiv1beta1.DataImportCronSource{
						Registry: &cdiv1beta1.DataVolumeSourceRegistry{
							URL: dataImportCronName + "1",
						},
					},
					ManagedDataSource: dataImportCronName + "1",
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: dataImportCronName + "2",
				},
				Spec: cdiv1beta1.DataImportCronSpec{
					Source: cdiv1beta1.DataImportCronSource{
						Registry: &cdiv1beta1.DataVolumeSourceRegistry{
							URL: dataImportCronName + "2",
						},
					},
					ManagedDataSource: dataImportCronName + "2",
				},
			},
		}
	)

	BeforeEach(func() {
		s := scheme.Scheme
		Expect(ssp.AddToScheme(s)).ToNot(HaveOccurred())
		Expect(operand.AddWatchTypesToScheme(s)).ToNot(HaveOccurred())

		client := fake.NewClientBuilder().WithScheme(s).Build()
		request = common.Request{
			Request: reconcile.Request{
				NamespacedName: types.NamespacedName{
					Namespace: namespace,
					Name:      name,
				},
			},
			Client:  client,
			Context: context.Background(),
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
					DataImportCronTemplates: importCrons,
				},
			},
			Logger:       log,
			VersionCache: common.VersionCache{},
		}
	})

	It("should create DataImportCron resources", func() {
		_, err := operand.Reconcile(&request)
		Expect(err).ToNot(HaveOccurred())
		Expect(importCrons).ToNot(BeNil())
		for _, cronTemplate := range importCrons {
			cron := cronTemplate.AsDataImportCron()
			cron.Namespace = ssp.GoldenImagesNSname

			By(fmt.Sprintf("expecting %s/%s to be created", cron.Namespace, cron.Name))
			ExpectResourceExists(&cron, request)
		}
	})

	It("should delete DataImportCron resources previously created by the operator and no longer managed", func() {
		_, err := operand.Reconcile(&request)
		Expect(err).ToNot(HaveOccurred())
		Expect(importCrons).ToNot(BeNil())
		for _, cronTemplate := range importCrons {
			cron := cronTemplate.AsDataImportCron()
			cron.Namespace = ssp.GoldenImagesNSname

			By(fmt.Sprintf("expecting %s/%s to be created", cron.Namespace, cron.Name))
			ExpectResourceExists(&cron, request)
		}
		noLongerManaged := request.Instance.Spec.DataImportCronTemplates[1:]
		request.Instance.Spec.DataImportCronTemplates = request.Instance.Spec.DataImportCronTemplates[:1]
		_, err = operand.Reconcile(&request)
		Expect(err).ToNot(HaveOccurred())
		Expect(importCrons).ToNot(BeNil())
		for _, cronTemplate := range noLongerManaged {
			cron := cronTemplate.AsDataImportCron()
			cron.Namespace = ssp.GoldenImagesNSname

			By(fmt.Sprintf("expecting %s/%s to be deleted", cron.Namespace, cron.Name))
			ExpectResourceNotExists(&cron, request)
		}
	})

	It("should delete created resources on cleanup", func() {
		_, err := operand.Reconcile(&request)
		Expect(err).NotTo(HaveOccurred())
		err = operand.Cleanup(&request)
		Expect(err).NotTo(HaveOccurred())
		for _, cronTemplate := range importCrons {
			cron := cronTemplate.AsDataImportCron()
			cron.Namespace = ssp.GoldenImagesNSname

			By(fmt.Sprintf("expecting %s/%s to be deleted", cron.Namespace, cron.Name))
			ExpectResourceNotExists(&cron, request)
		}
	})
})
