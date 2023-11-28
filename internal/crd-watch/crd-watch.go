package crd_watch

import (
	"context"
	"fmt"
	"sync"

	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
	ctrlcache "sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

type CrdList interface {
	CrdExists(crdName string) bool
	MissingCrds() []string
}

type CrdWatch struct {
	AllCrdsAddedHandler   func()
	SomeCrdRemovedHandler func()

	lock         sync.Mutex
	requiredCrds map[string]struct{}
	existingCrds map[string]struct{}
	missingCrds  map[string]struct{}

	initialized bool
	cache       ctrlcache.Cache
}

func New(requiredCrds ...string) *CrdWatch {
	requiredCrdsMap := make(map[string]struct{}, len(requiredCrds))
	missingCrds := make(map[string]struct{}, len(requiredCrds))
	for _, crdName := range requiredCrds {
		requiredCrdsMap[crdName] = struct{}{}
		missingCrds[crdName] = struct{}{}
	}

	return &CrdWatch{
		requiredCrds: requiredCrdsMap,
		existingCrds: map[string]struct{}{},
		missingCrds:  missingCrds,
	}
}

func (c *CrdWatch) Init(ctx context.Context, reader client.Reader) error {
	if err := c.sync(ctx, reader); err != nil {
		return err
	}

	c.initialized = true
	return nil
}

func (c *CrdWatch) CrdExists(crdName string) bool {
	c.lock.Lock()
	defer c.lock.Unlock()

	if !c.initialized {
		panic("crd watch not initialized")
	}

	_, exists := c.existingCrds[crdName]
	return exists
}

func (c *CrdWatch) MissingCrds() []string {
	c.lock.Lock()
	defer c.lock.Unlock()

	if !c.initialized {
		panic("crd watch not initialized")
	}

	names := make([]string, 0, len(c.missingCrds))
	for crdName := range c.missingCrds {
		names = append(names, crdName)
	}
	return names
}

var _ manager.Runnable = &CrdWatch{}

func (c *CrdWatch) Start(ctx context.Context) error {
	if !c.initialized {
		err := c.Init(ctx, c.cache)
		if err != nil {
			return fmt.Errorf("failed to initialize crd watch: %w", err)
		}
	}

	informer, err := c.cache.GetInformer(ctx, &metav1.PartialObjectMetadata{
		TypeMeta: metav1.TypeMeta{
			APIVersion: apiextensions.GroupName + "/v1",
			Kind:       "CustomResourceDefinition",
		},
	})
	if err != nil {
		return fmt.Errorf("failed to get informer: %w", err)
	}

	_, err = informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			c.lock.Lock()
			defer c.lock.Unlock()
			c.crdAdded(obj.(*metav1.PartialObjectMetadata).GetName())
		},
		DeleteFunc: func(obj interface{}) {
			c.lock.Lock()
			defer c.lock.Unlock()
			c.crdDeleted(obj.(*metav1.PartialObjectMetadata).GetName())
		},
	})
	if err != nil {
		return err
	}

	if err := c.sync(ctx, c.cache); err != nil {
		return err
	}

	// This function has to block, because that is what manager.Runnable expects.
	<-ctx.Done()
	return nil
}

func (c *CrdWatch) sync(ctx context.Context, reader client.Reader) error {
	crdMetaList := &metav1.PartialObjectMetadataList{
		TypeMeta: metav1.TypeMeta{
			APIVersion: apiextensions.SchemeGroupVersion.String(),
			Kind:       "CustomResourceDefinitionList",
		},
	}

	err := reader.List(ctx, crdMetaList)
	if err != nil {
		return fmt.Errorf("failed to list CRDs: %w", err)
	}

	newCrds := make(map[string]struct{}, len(crdMetaList.Items))
	for i := range crdMetaList.Items {
		name := crdMetaList.Items[i].Name
		newCrds[name] = struct{}{}
	}

	c.lock.Lock()
	defer c.lock.Unlock()

	// Collecting added and deleted CRDs to slices,
	// because the c.crdAdded() and c.crdDeleted()
	// modify the c.existingCrds map
	addedCrds := make([]string, 0, len(crdMetaList.Items))
	deletedCrds := make([]string, 0, len(c.existingCrds))

	for name := range newCrds {
		if _, exists := c.existingCrds[name]; !exists {
			addedCrds = append(addedCrds, name)
		}
	}
	for name := range c.existingCrds {
		if _, exists := newCrds[name]; !exists {
			deletedCrds = append(deletedCrds, name)
		}
	}

	for _, name := range addedCrds {
		c.crdAdded(name)
	}
	for _, name := range deletedCrds {
		c.crdDeleted(name)
	}

	return nil
}

func (c *CrdWatch) crdAdded(crdName string) {
	c.existingCrds[crdName] = struct{}{}
	missingCountOld := len(c.missingCrds)
	delete(c.missingCrds, crdName)

	if !c.initialized || c.AllCrdsAddedHandler == nil {
		return
	}

	// Trigger the handler when the last required CRD is added
	if missingCountOld == 1 && len(c.missingCrds) == 0 {
		c.AllCrdsAddedHandler()
	}
}

func (c *CrdWatch) crdDeleted(crdName string) {
	delete(c.existingCrds, crdName)
	if _, isRequired := c.requiredCrds[crdName]; !isRequired {
		return
	}

	missingCountOld := len(c.missingCrds)
	c.missingCrds[crdName] = struct{}{}

	if !c.initialized || c.SomeCrdRemovedHandler == nil {
		return
	}

	// Trigger the handler when all crds exist and then one is removed.
	if missingCountOld == 0 {
		c.SomeCrdRemovedHandler()
	}
}

// TODO -- handle injection

//var _ inject.Cache = &CrdWatch{}
//
//func (c *CrdWatch) InjectCache(cache ctrlcache.Cache) error {
//	c.cache = cache
//	return nil
//}
