package virtinformers

import (
	"math/rand"
	"time"

	templatev1 "github.com/openshift/api/template/v1"
	k8sv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	kubevirtv1 "kubevirt.io/client-go/api/v1"
	"kubevirt.io/client-go/log"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"

	"kubevirt.io/ssp-operator/internal/template-validator/labels"
)

type Informers struct {
	templateInformer cache.SharedIndexInformer
	vmCache          VmCache
	vmCacheReflector *cache.Reflector
	stopCh           chan struct{}
}

func (inf *Informers) Start() {
	go inf.templateInformer.Run(inf.stopCh)
	go inf.vmCacheReflector.Run(inf.stopCh)

	log.Log.Infof("validator app: started informers")
	cache.WaitForCacheSync(
		inf.stopCh,
		inf.templateInformer.HasSynced,
	)
	log.Log.Infof("validator app: synced informers")
}

func (inf *Informers) Stop() {
	close(inf.stopCh)
}

func (inf *Informers) TemplateStore() cache.Store {
	return inf.templateInformer.GetStore()
}

func (inf *Informers) VmCache() VmCache {
	return inf.vmCache
}

func NewInformers(scheme *runtime.Scheme) (*Informers, error) {
	config, err := ctrl.GetConfig()
	if err != nil {
		log.Log.Errorf("unable to get kubeconfig: %v", err)
		return nil, err
	}

	informer, err := createTemplateInformer(config, scheme)
	if err != nil {
		return nil, err
	}

	vms := NewVmCache(vmNeedsTemplate)
	reflector, err := createVmCacheReflector(config, scheme, vms)
	if err != nil {
		return nil, err
	}

	return &Informers{
		templateInformer: informer,
		vmCache:          vms,
		vmCacheReflector: reflector,
		stopCh:           make(chan struct{}, 1),
	}, nil
}

func vmNeedsTemplate(vm metav1.Object) bool {
	if _, ok := vm.GetAnnotations()[labels.VmValidationAnnotationKey]; ok {
		return false
	}

	templateKeys := labels.GetTemplateKeys(vm)
	return templateKeys.IsValid()
}

func createTemplateInformer(restConfig *rest.Config, scheme *runtime.Scheme) (cache.SharedIndexInformer, error) {
	restClient, err := restClientForObject(&templatev1.Template{}, restConfig, scheme)
	if err != nil {
		return nil, err
	}

	lw := cache.NewListWatchFromClient(restClient, "templates", k8sv1.NamespaceAll, fields.Everything())

	_, err = lw.List(metav1.ListOptions{Limit: 1})
	if err != nil {
		log.Log.Errorf("error probing the template resource: %v", err)
		return nil, err
	}

	// Resulting resync period will be between 12 and 24 hours, like the default for k8s
	resync := resyncPeriod(12 * time.Hour)
	return cache.NewSharedIndexInformer(lw, &templatev1.Template{}, resync, cache.Indexers{}), nil
}

func createVmCacheReflector(restConfig *rest.Config, scheme *runtime.Scheme, store cache.Store) (*cache.Reflector, error) {
	restClient, err := restClientForObject(&kubevirtv1.VirtualMachine{}, restConfig, scheme)
	if err != nil {
		return nil, err
	}

	lw := cache.NewListWatchFromClient(restClient, "virtualmachines", k8sv1.NamespaceAll, fields.Everything())

	_, err = lw.List(metav1.ListOptions{Limit: 1})
	if err != nil {
		log.Log.Errorf("error probing the virtual machine resource: %v", err)
		return nil, err
	}

	return cache.NewReflector(
		lw,
		&kubevirtv1.VirtualMachine{},
		store,
		resyncPeriod(12*time.Hour),
	), nil
}

func restClientForObject(obj runtime.Object, restConfig *rest.Config, scheme *runtime.Scheme) (rest.Interface, error) {
	gvk, err := apiutil.GVKForObject(obj, scheme)
	if err != nil {
		return nil, err
	}

	restClient, err := apiutil.RESTClientForGVK(gvk, false, restConfig, serializer.NewCodecFactory(scheme))
	if err != nil {
		log.Log.Errorf("error creating client: %v", err)
		return nil, err
	}
	return restClient, nil
}

// resyncPeriod computes the time interval a shared informer waits before resyncing with the api server
func resyncPeriod(minResyncPeriod time.Duration) time.Duration {
	factor := rand.Float64() + 1
	return time.Duration(float64(minResyncPeriod.Nanoseconds()) * factor)
}
