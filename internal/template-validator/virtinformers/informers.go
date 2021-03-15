package virtinformers

import (
	"context"
	"math/rand"
	"sync"
	"time"

	templatev1 "github.com/openshift/api/template/v1"
	templatev1client "github.com/openshift/client-go/template/clientset/versioned/typed/template/v1"
	k8sv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"kubevirt.io/client-go/kubecli"
	"kubevirt.io/client-go/log"
)

var once sync.Once
var pkgInformers *Informers

type Informers struct {
	TemplateInformer cache.SharedIndexInformer
}

func (inf *Informers) Available() bool {
	return inf != nil && inf.TemplateInformer != nil
}

func GetInformers() *Informers {
	once.Do(func() {
		pkgInformers = newInformers()
	})
	return pkgInformers
}

// SetInformers created for unittest usage only
func SetInformers(informers *Informers) {
	once.Do(func() {
		pkgInformers = informers
	})
}

func newInformers() *Informers {
	config, err := kubecli.GetConfig()
	if err != nil {
		panic(err)
	}

	kubeInformerFactory := NewKubeInformerFactory(config)
	return &Informers{
		TemplateInformer: kubeInformerFactory.Template(),
	}
}

type newSharedInformer func() cache.SharedIndexInformer

type KubeInformerFactory interface {
	// Starts any informers that have not been started yet
	// This function is thread safe and idempotent
	Start(stopCh <-chan struct{})

	Template() cache.SharedIndexInformer
}

type kubeInformerFactory struct {
	restConfig    *rest.Config
	lock          sync.Mutex
	defaultResync time.Duration

	informers        map[string]cache.SharedIndexInformer
	startedInformers map[string]bool
}

func NewKubeInformerFactory(restConfig *rest.Config) KubeInformerFactory {
	return &kubeInformerFactory{
		restConfig: restConfig,
		// Resulting resync period will be between 12 and 24 hours, like the default for k8s
		defaultResync:    resyncPeriod(12 * time.Hour),
		informers:        make(map[string]cache.SharedIndexInformer),
		startedInformers: make(map[string]bool),
	}
}

func (f *kubeInformerFactory) Start(stopCh <-chan struct{}) {
	f.lock.Lock()
	defer f.lock.Unlock()

	for name, informer := range f.informers {
		if f.startedInformers[name] {
			// skip informers that have already started.
			log.Log.Infof("SKIPPING informer %s", name)
			continue
		}
		log.Log.Infof("STARTING informer %s", name)
		go informer.Run(stopCh)
		f.startedInformers[name] = true
	}
}

func (f *kubeInformerFactory) getInformer(key string, newFunc newSharedInformer) cache.SharedIndexInformer {
	f.lock.Lock()
	defer f.lock.Unlock()

	informer, exists := f.informers[key]
	if exists {
		return informer
	}
	informer = newFunc()
	f.informers[key] = informer

	return informer
}

func (f *kubeInformerFactory) Template() cache.SharedIndexInformer {
	return f.getInformer("templateInformer", func() cache.SharedIndexInformer {
		tmplclient, err := templatev1client.NewForConfig(f.restConfig)
		if err != nil {
			log.Log.Errorf("error creating the template client: %v", err)
			return nil
		}

		_, err = tmplclient.Templates(k8sv1.NamespaceAll).List(context.TODO(), metav1.ListOptions{Limit: 1})
		if err != nil {
			log.Log.Errorf("error probing the template resource: %v", err)
			return nil
		}

		lw := cache.NewListWatchFromClient(tmplclient.RESTClient(), "templates", k8sv1.NamespaceAll, fields.Everything())
		return cache.NewSharedIndexInformer(lw, &templatev1.Template{}, f.defaultResync, cache.Indexers{})
	})
}

// resyncPeriod computes the time interval a shared informer waits before resyncing with the api server
func resyncPeriod(minResyncPeriod time.Duration) time.Duration {
	factor := rand.Float64() + 1
	return time.Duration(float64(minResyncPeriod.Nanoseconds()) * factor)
}
