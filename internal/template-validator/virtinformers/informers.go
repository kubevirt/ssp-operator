package virtinformers

import (
	"math/rand"
	"time"

	templatev1 "github.com/openshift/api/template/v1"
	k8sv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"kubevirt.io/client-go/log"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
)

type Informers struct {
	templateInformer cache.SharedIndexInformer
	stopCh           chan struct{}
}

func (inf *Informers) Start() {
	go inf.templateInformer.Run(inf.stopCh)
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

func NewInformers() (*Informers, error) {
	config, err := ctrl.GetConfig()
	if err != nil {
		log.Log.Errorf("unable to get kubeconfig: %v", err)
		return nil, err
	}

	informer, err := createTemplateInformer(config)
	if err != nil {
		return nil, err
	}

	return &Informers{
		templateInformer: informer,
		stopCh:           make(chan struct{}, 1),
	}, nil
}

func createTemplateInformer(restConfig *rest.Config) (cache.SharedIndexInformer, error) {
	gvk, err := apiutil.GVKForObject(&templatev1.Template{}, scheme.Scheme)
	if err != nil {
		log.Log.Errorf("error getting GVK for Template: %v", err)
		return nil, err
	}

	restClient, err := apiutil.RESTClientForGVK(gvk, false, restConfig, scheme.Codecs)
	if err != nil {
		log.Log.Errorf("error creating client: %v", err)
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

// resyncPeriod computes the time interval a shared informer waits before resyncing with the api server
func resyncPeriod(minResyncPeriod time.Duration) time.Duration {
	factor := rand.Float64() + 1
	return time.Duration(float64(minResyncPeriod.Nanoseconds()) * factor)
}
