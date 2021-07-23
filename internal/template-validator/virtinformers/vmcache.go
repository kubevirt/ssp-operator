package virtinformers

import (
	"reflect"
	"sync"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/cache"

	"kubevirt.io/ssp-operator/internal/template-validator/labels"
)

type VmCache interface {
	cache.Store

	HasSynced() bool

	GetVmsForTemplate(template string) []string
}

type VmCacheValue struct {
	Vm       string
	Template string
}

func newVmCacheValue(obj metav1.Object) *VmCacheValue {
	templateKeys := labels.GetTemplateKeys(obj)
	return &VmCacheValue{
		Vm:       vmCacheKey(obj),
		Template: templateKeys.Get().String(),
	}
}

func vmCacheKey(obj metav1.Object) string {
	return types.NamespacedName{
		Name:      obj.GetName(),
		Namespace: obj.GetNamespace(),
	}.String()
}

type NameSet = map[string]struct{}

type templateMap map[string]NameSet

func (t templateMap) Add(template, vm string) {
	vmNameSet, exists := t[template]
	if !exists {
		vmNameSet = NameSet{}
		t[template] = vmNameSet
	}

	vmNameSet[vm] = struct{}{}
}

func (t templateMap) Delete(template, vm string) {
	vmNameSet, exists := t[template]
	if !exists {
		return
	}

	delete(vmNameSet, vm)
	if len(vmNameSet) == 0 {
		delete(t, template)
	}
}

func (t templateMap) List(template string) []string {
	names, exists := t[template]
	if !exists {
		return nil
	}
	result := make([]string, 0, len(names))
	for name := range names {
		result = append(result, name)
	}
	return result
}

type Predicate func(vm metav1.Object) bool

type vmCache struct {
	lock sync.RWMutex

	store          map[string]VmCacheValue
	vmsForTemplate templateMap
	hasSynced      bool

	filter Predicate
}

var _ VmCache = &vmCache{}

func NewVmCache(filter Predicate) VmCache {
	return &vmCache{
		store:          map[string]VmCacheValue{},
		vmsForTemplate: templateMap{},
		filter:         filter,
	}
}

func (v *vmCache) Add(obj interface{}) error {
	metaObj, err := meta.Accessor(obj)
	if err != nil {
		return err
	}

	if !v.filter(metaObj) {
		return nil
	}

	key := vmCacheKey(metaObj)
	val := newVmCacheValue(metaObj)

	v.lock.Lock()
	defer v.lock.Unlock()

	v.store[key] = *val
	v.vmsForTemplate.Add(val.Template, val.Vm)
	return nil
}

func (v *vmCache) Update(obj interface{}) error {
	newObjMeta, err := meta.Accessor(obj)
	if err != nil {
		return err
	}

	if !v.filter(newObjMeta) {
		return v.Delete(obj)
	}

	key := vmCacheKey(newObjMeta)
	newVal := newVmCacheValue(newObjMeta)

	v.lock.Lock()
	defer v.lock.Unlock()

	oldVal, exists := v.store[key]
	if !exists {
		v.store[key] = *newVal
		v.vmsForTemplate.Add(newVal.Template, newVal.Vm)
		return nil
	}

	if reflect.DeepEqual(oldVal, newVal) {
		return nil
	}

	v.store[key] = *newVal
	v.vmsForTemplate.Delete(oldVal.Template, oldVal.Vm)
	v.vmsForTemplate.Add(newVal.Template, newVal.Vm)

	return nil
}

func (v *vmCache) Delete(obj interface{}) error {
	objMeta, err := meta.Accessor(obj)
	if err != nil {
		return err
	}

	v.lock.Lock()
	defer v.lock.Unlock()

	key := vmCacheKey(objMeta)
	val, exists := v.store[key]
	if !exists {
		return nil
	}

	delete(v.store, key)
	v.vmsForTemplate.Delete(val.Template, val.Vm)
	return nil
}

func (v *vmCache) List() []interface{} {
	v.lock.RLock()
	defer v.lock.RUnlock()

	list := make([]interface{}, 0, len(v.store))
	for _, value := range v.store {
		list = append(list, value)
	}
	return list
}

func (v *vmCache) ListKeys() []string {
	v.lock.RLock()
	defer v.lock.RUnlock()

	list := make([]string, 0, len(v.store))
	for key := range v.store {
		list = append(list, key)
	}
	return list
}

func (v *vmCache) Get(obj interface{}) (item interface{}, exists bool, err error) {
	metaObj, err := meta.Accessor(obj)
	if err != nil {
		return nil, false, err
	}
	key := vmCacheKey(metaObj)
	return v.GetByKey(key)
}

func (v *vmCache) GetByKey(key string) (item interface{}, exists bool, err error) {
	v.lock.RLock()
	defer v.lock.RUnlock()

	res, exists := v.store[key]
	return res, exists, nil
}

func (v *vmCache) Replace(list []interface{}, resourceVersion string) error {
	newStore := make(map[string]VmCacheValue, len(list))
	newVmsForTemplate := templateMap{}
	for _, obj := range list {
		metaObj, err := meta.Accessor(obj)
		if err != nil {
			return err
		}

		key := vmCacheKey(metaObj)
		val := newVmCacheValue(metaObj)

		newStore[key] = *val
		newVmsForTemplate.Add(val.Template, val.Vm)
	}

	v.lock.Lock()
	v.lock.Unlock()

	v.store = newStore
	v.vmsForTemplate = newVmsForTemplate
	v.hasSynced = true
	return nil
}

func (v *vmCache) Resync() error {
	// No-op
	return nil
}

func (v *vmCache) HasSynced() bool {
	return v.hasSynced
}

func (v *vmCache) GetVmsForTemplate(template string) []string {
	v.lock.RLock()
	defer v.lock.RUnlock()
	return v.vmsForTemplate.List(template)
}
