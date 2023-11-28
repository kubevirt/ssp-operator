package crd_watch

import (
	"context"
	goruntime "runtime"
	"sync/atomic"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	internalmeta "k8s.io/apimachinery/pkg/apis/meta/internalversion"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/conversion"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/cache/informertest"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var _ = Describe("CRD watch", func() {
	const (
		crd1 = "required-crd-1"
		crd2 = "required-crd-2"
		crd3 = "required-crd-3"
	)

	var (
		fakeInformers *informertest.FakeInformers

		crdWatch *CrdWatch
		cancel   context.CancelFunc
	)

	BeforeEach(func() {
		Expect(internalmeta.AddToScheme(scheme.Scheme)).To(Succeed())
		Expect(apiextensions.AddToScheme(scheme.Scheme)).To(Succeed())

		// For unit tests, we to manually add conversion functions from CRD to PartialObjectMetadata
		Expect(addConversionFunctions(scheme.Scheme)).To(Succeed())

		crdObj := &apiextensions.CustomResourceDefinition{
			TypeMeta: metav1.TypeMeta{
				APIVersion: apiextensions.SchemeGroupVersion.String(),
				Kind:       "CustomResourceDefinition",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: crd1,
			},
		}

		fakeClient := fake.NewClientBuilder().Build()
		Expect(fakeClient.Create(context.Background(), crdObj)).To(Succeed())

		fakeInformers = &informertest.FakeInformers{}

		crdWatch = New(crd1, crd2, crd3)
		Expect(crdWatch.Init(context.Background(), fakeClient)).To(Succeed())
		// TODO -- fix injection
		// Expect(crdWatch.InjectCache(fakeInformers)).To(Succeed())

		var ctx context.Context
		ctx, cancel = context.WithCancel(context.Background())
		go func() {
			defer GinkgoRecover()
			Expect(crdWatch.Start(ctx)).To(Succeed())
		}()
		// Yield the current goroutine and allow the above one to start.
		// This is a hack, because there is no way for tests to wait until crdWatch registers informers.
		goruntime.Gosched()
	})

	AfterEach(func() {
		if cancel != nil {
			cancel()
		}
	})

	It("should check for existing CRD", func() {
		const crdName = "test-crd"
		addCrdToFakeInformers(crdName, fakeInformers)
		Expect(crdWatch.CrdExists(crdName)).To(BeTrue(), "Crd should exist")
	})

	It("should check for non-existing CRD", func() {
		addCrdToFakeInformers("test-crd", fakeInformers)
		Expect(crdWatch.CrdExists("nonexistent-crd")).To(BeFalse(), "Crd should not exist")
	})

	It("should return no missing CRD", func() {
		addCrdToFakeInformers(crd2, fakeInformers)
		addCrdToFakeInformers(crd3, fakeInformers)

		Expect(crdWatch.MissingCrds()).To(BeEmpty())
	})

	It("should return CRDs that are missing", func() {
		addCrdToFakeInformers(crd2, fakeInformers)

		missingCrds := crdWatch.MissingCrds()
		Expect(missingCrds).To(HaveLen(1))
		Expect(missingCrds).To(ContainElement(crd3))
	})

	Context("AllCrdsAddedHandler", func() {
		It("should call handler when all requested CRDs are added", func() {
			handlerCalled := make(chan struct{})
			crdWatch.AllCrdsAddedHandler = func() {
				close(handlerCalled)
			}

			addCrdToFakeInformers(crd2, fakeInformers)
			addCrdToFakeInformers(crd3, fakeInformers)

			Eventually(handlerCalled, 50*time.Millisecond).Should(BeClosed())
		})

		It("should not call the handler multiple times", func() {
			var callCount int32
			crdWatch.AllCrdsAddedHandler = func() {
				atomic.AddInt32(&callCount, 1)
			}

			// Adding all needed CRDs to call handler for the first time
			addCrdToFakeInformers(crd2, fakeInformers)
			addCrdToFakeInformers(crd3, fakeInformers)

			Eventually(func() int32 {
				return atomic.LoadInt32(&callCount)
			}, 50*time.Millisecond).Should(BeNumerically(">", 0))

			// Add Another CRD and verify that the handler will not be called again
			addCrdToFakeInformers("not-required-crd", fakeInformers)

			Consistently(func() int32 {
				return atomic.LoadInt32(&callCount)
			}, 100*time.Millisecond).Should(Equal(1))
		})

		It("should not call the handler when some CRDs are missing", func() {

		})
	})

	Context("SomeCrdRemovedHandler", func() {

	})

	It("should execute SomeCrdRemovedHandler when some required CRDs are removed", func() {
		addCrdToFakeInformers(crd2, fakeInformers)
		addCrdToFakeInformers(crd3, fakeInformers)

		handlerCalled := make(chan struct{})
		crdWatch.SomeCrdRemovedHandler = func() {
			close(handlerCalled)
		}

		removeCrdFromFakeInformers(crd1, fakeInformers)

		Eventually(handlerCalled, 50*time.Millisecond).Should(BeClosed())
	})
})

func addConversionFunctions(s *runtime.Scheme) error {
	err := s.AddConversionFunc((*apiextensions.CustomResourceDefinition)(nil), (*metav1.PartialObjectMetadata)(nil), func(a, b interface{}, scope conversion.Scope) error {
		crd := a.(*apiextensions.CustomResourceDefinition)
		partialMeta := b.(*metav1.PartialObjectMetadata)

		partialMeta.TypeMeta = crd.TypeMeta
		partialMeta.ObjectMeta = crd.ObjectMeta
		return nil
	})
	if err != nil {
		return err
	}

	return s.AddConversionFunc((*apiextensions.CustomResourceDefinitionList)(nil), (*metav1.PartialObjectMetadataList)(nil), func(a, b interface{}, scope conversion.Scope) error {
		crdList := a.(*apiextensions.CustomResourceDefinitionList)
		partialMetaList := b.(*metav1.PartialObjectMetadataList)

		partialMetaList.TypeMeta = crdList.TypeMeta
		partialMetaList.ListMeta = crdList.ListMeta

		partialMetaList.Items = make([]metav1.PartialObjectMetadata, len(crdList.Items))
		for i := range crdList.Items {
			if err := scope.Convert(&crdList.Items[i], &partialMetaList.Items[i]); err != nil {
				return err
			}
		}
		return nil
	})
}

func addCrdToFakeInformers(crdName string, fakeInformers *informertest.FakeInformers) {
	fakeInformer, err := fakeInformers.FakeInformerFor(&metav1.PartialObjectMetadata{})
	ExpectWithOffset(1, err).ToNot(HaveOccurred())
	fakeInformer.Add(crdPartialMetadata(crdName))
}

func removeCrdFromFakeInformers(crdName string, fakeInformers *informertest.FakeInformers) {
	fakeInformer, err := fakeInformers.FakeInformerFor(&metav1.PartialObjectMetadata{})
	ExpectWithOffset(1, err).ToNot(HaveOccurred())
	fakeInformer.Delete(crdPartialMetadata(crdName))
}

func crdPartialMetadata(crdName string) *metav1.PartialObjectMetadata {
	return &metav1.PartialObjectMetadata{
		TypeMeta: metav1.TypeMeta{
			APIVersion: apiextensions.SchemeGroupVersion.String(),
			Kind:       "CustomResourceDefinition",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: crdName,
		},
	}
}
