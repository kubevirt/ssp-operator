package tests

import (
	"context"
	"os"
	"strconv"
	"testing"
	"time"

	promv1 "github.com/coreos/prometheus-operator/pkg/apis/monitoring/v1"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	secv1 "github.com/openshift/api/security/v1"
	templatev1 "github.com/openshift/api/template/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	lifecycleapi "kubevirt.io/controller-lifecycle-operator-sdk/pkg/sdk/api"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	"sigs.k8s.io/controller-runtime/pkg/client/config"

	sspv1alpha1 "kubevirt.io/ssp-operator/api/v1alpha1"
)

const (
	// TODO - maybe randomize namespace
	testNamespace             = "ssp-operator-functests"
	commonTemplatesTestNS     = "ssp-operator-functests-templates"
	timeout                   = 8 * time.Minute
	templateValidatorReplicas = 1
)

var (
	apiClient client.Client
	ctx       context.Context
	ssp       *sspv1alpha1.SSP

	sspListerWatcher cache.ListerWatcher

	deploymentTimedOut bool
)

var _ = BeforeSuite(func() {
	setupApiClient()

	Eventually(func() error {
		namespaceObj := &v1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: testNamespace}}
		return apiClient.Create(ctx, namespaceObj)
	}, timeout, time.Second).ShouldNot(HaveOccurred())

	Eventually(func() error {
		namespaceObj := &v1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: commonTemplatesTestNS}}
		return apiClient.Create(ctx, namespaceObj)
	}, timeout, time.Second).ShouldNot(HaveOccurred())

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
			NodeLabeller: sspv1alpha1.NodeLabeller{},
		},
	}

	Eventually(func() error {
		return apiClient.Create(ctx, ssp)
	}, timeout, time.Second).ShouldNot(HaveOccurred())

	// Wait to finish deployment before running any tests
	waitUntilDeployed()
})

var _ = AfterSuite(func() {
	if shouldSkipCleanup() {
		return
	}

	if ssp != nil {
		err := apiClient.Delete(ctx, ssp)
		expectSuccessOrNotFound(err)
		waitForDeletion(client.ObjectKey{
			Name:      ssp.Name,
			Namespace: ssp.Namespace,
		}, &sspv1alpha1.SSP{})
	}

	err1 := apiClient.Delete(ctx, &v1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: testNamespace}})
	err2 := apiClient.Delete(ctx, &v1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: commonTemplatesTestNS}})
	expectSuccessOrNotFound(err1)
	expectSuccessOrNotFound(err2)

	waitForDeletion(client.ObjectKey{Name: testNamespace}, &v1.Namespace{})
	waitForDeletion(client.ObjectKey{Name: commonTemplatesTestNS}, &v1.Namespace{})
})

func expectSuccessOrNotFound(err error) {
	if err != nil && !errors.IsNotFound(err) {
		Expect(err).ToNot(HaveOccurred())
	}
}

func setupApiClient() {
	Expect(sspv1alpha1.AddToScheme(scheme.Scheme)).ToNot(HaveOccurred())
	Expect(promv1.AddToScheme(scheme.Scheme)).ToNot(HaveOccurred())
	Expect(templatev1.Install(scheme.Scheme)).ToNot(HaveOccurred())
	Expect(secv1.Install(scheme.Scheme)).ToNot(HaveOccurred())

	cfg, err := config.GetConfig()
	Expect(err).ToNot(HaveOccurred())
	apiClient, err = client.New(cfg, client.Options{})
	Expect(err).ToNot(HaveOccurred())

	ctx = context.Background()
	sspListerWatcher = createSspListerWatcher(cfg)
}

func createSspListerWatcher(cfg *rest.Config) cache.ListerWatcher {
	sspGvk, err := apiutil.GVKForObject(&sspv1alpha1.SSP{}, scheme.Scheme)
	Expect(err).ToNot(HaveOccurred())

	restClient, err := apiutil.RESTClientForGVK(sspGvk, cfg, serializer.NewCodecFactory(scheme.Scheme))
	Expect(err).ToNot(HaveOccurred())

	return cache.NewListWatchFromClient(restClient, "ssps", testNamespace, fields.Everything())
}

func getSsp() *sspv1alpha1.SSP {
	key := client.ObjectKey{Name: ssp.Name, Namespace: ssp.Namespace}
	foundSsp := &sspv1alpha1.SSP{}
	Expect(apiClient.Get(ctx, key, foundSsp)).ToNot(HaveOccurred())
	return foundSsp
}

func waitUntilDeployed() {
	if deploymentTimedOut {
		Fail("Timed out waiting for SSP to be in phase Deployed.")
	}

	// Set to true before waiting. In case Eventually fails,
	// it will panic and the deploymentTimedOut will be left true
	deploymentTimedOut = true
	EventuallyWithOffset(1, func() bool {
		return getSsp().Status.Phase == lifecycleapi.PhaseDeployed
	}, timeout, time.Second).Should(BeTrue())
	deploymentTimedOut = false
}

func waitForDeletion(key client.ObjectKey, obj runtime.Object) {
	EventuallyWithOffset(1, func() bool {
		err := apiClient.Get(ctx, key, obj)
		return errors.IsNotFound(err)
	}, timeout, time.Second).Should(BeTrue())
}

func shouldSkipCleanup() bool {
	noCleanupEnv := os.Getenv("NO_CLEANUP_AFTER_TESTS")
	if noCleanupEnv == "" {
		return false
	}
	noCleanup, err := strconv.ParseBool(noCleanupEnv)
	if err != nil {
		return false
	}
	return noCleanup
}

func TestFunctional(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Functional test suite")
}
