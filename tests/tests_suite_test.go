package tests

import (
	"context"
	templatev1 "github.com/openshift/api/template/v1"
	"testing"
	"time"

	promv1 "github.com/coreos/prometheus-operator/pkg/apis/monitoring/v1"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	sspv1alpha1 "kubevirt.io/ssp-operator/api/v1alpha1"
	"kubevirt.io/ssp-operator/internal/operands/metrics"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
)

const (
	// TODO - maybe randomize namespace
	testNamespace             = "ssp-operator-functests"
	commonTemplatesTestNS     = "ssp-operator-functests-templates"
	timeout                   = 180 * time.Second
	templateValidatorReplicas = 1
)

var (
	apiClient client.Client
	ctx       context.Context
	ssp       *sspv1alpha1.SSP
)

var _ = BeforeSuite(func() {
	setupApiClient()

	namespaceObj := &v1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: testNamespace}}
	Expect(apiClient.Create(ctx, namespaceObj)).ToNot(HaveOccurred())

	namespaceObj = &v1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: commonTemplatesTestNS}}
	Expect(apiClient.Create(ctx, namespaceObj)).ToNot(HaveOccurred())

	ssp = &sspv1alpha1.SSP{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-ssp",
			Namespace: testNamespace,
		},
		Spec: sspv1alpha1.SSPSpec{
			TemplateValidator: sspv1alpha1.TemplateValidator{
				Replicas: templateValidatorReplicas,
			},
			CommonTemplates: sspv1alpha1.CommonTemplates{
				Namespace: commonTemplatesTestNS,
			},
		},
	}

	Expect(apiClient.Create(ctx, ssp)).ToNot(HaveOccurred())

	// Wait for resources creation
	// TODO - use 'ready' condition on SSP resource, when it is implemented
	Eventually(func() error {
		return apiClient.Get(ctx, client.ObjectKey{
			Name:      metrics.PrometheusRuleName,
			Namespace: testNamespace,
		}, &promv1.PrometheusRule{})
	}, timeout, time.Second).ShouldNot(HaveOccurred())
	Eventually(func() error {
		return apiClient.Get(ctx, client.ObjectKey{
			Name:      "windows10-desktop-medium-v0.11.3",
			Namespace: commonTemplatesTestNS,
		}, &templatev1.Template{})
	}, timeout, time.Second).ShouldNot(HaveOccurred())
})

var _ = AfterSuite(func() {
	if ssp != nil {
		Expect(apiClient.Delete(ctx, ssp)).ToNot(HaveOccurred())
		waitForDeletion(client.ObjectKey{
			Name:      ssp.Name,
			Namespace: ssp.Namespace,
		}, &sspv1alpha1.SSP{})
	}

	namespaceObj := &v1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: testNamespace}}
	err := apiClient.Delete(ctx, namespaceObj)
	Expect(err == nil || errors.IsNotFound(err)).To(BeTrue())

	namespaceObj = &v1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: commonTemplatesTestNS}}
	err = apiClient.Delete(ctx, namespaceObj)
	Expect(err == nil || errors.IsNotFound(err)).To(BeTrue())

	waitForDeletion(client.ObjectKey{Name: testNamespace}, &v1.Namespace{})
	waitForDeletion(client.ObjectKey{Name: commonTemplatesTestNS}, &v1.Namespace{})
})

func setupApiClient() {
	Expect(sspv1alpha1.AddToScheme(scheme.Scheme)).ToNot(HaveOccurred())
	Expect(promv1.AddToScheme(scheme.Scheme)).ToNot(HaveOccurred())
	Expect(templatev1.Install(scheme.Scheme)).ToNot(HaveOccurred())

	cfg, err := config.GetConfig()
	Expect(err).ToNot(HaveOccurred())
	apiClient, err = client.New(cfg, client.Options{})
	Expect(err).ToNot(HaveOccurred())

	ctx = context.Background()
}

func waitForDeletion(key client.ObjectKey, obj runtime.Object) {
	Eventually(func() bool {
		err := apiClient.Get(ctx, key, obj)
		return errors.IsNotFound(err)
	}, timeout, time.Second).Should(BeTrue())
}

func TestFunctional(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Functional test suite")
}
