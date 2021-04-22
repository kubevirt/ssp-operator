package common

import (
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type cacheKey struct {
	Kind      string
	Name      string
	Namespace string
}

type cacheValue struct {
	uid             types.UID
	resourceVersion string
	generation      int64
}

type VersionCache map[cacheKey]cacheValue

func (v VersionCache) Contains(obj client.Object) bool {
	cached, ok := v[cacheKeyFromObj(obj)]
	if !ok {
		return false
	}
	if obj.GetUID() != cached.uid {
		return false
	}
	if obj.GetGeneration() == 0 {
		objResourceVersion := obj.GetResourceVersion()
		return objResourceVersion != "" && objResourceVersion == cached.resourceVersion
	}
	return cached.generation == obj.GetGeneration()
}

func (v VersionCache) Add(obj client.Object) {
	kind := obj.GetObjectKind().GroupVersionKind().Kind
	if kind == "" {
		// Do not cache objects without kind
		return
	}
	v[cacheKeyFromObj(obj)] = cacheValue{
		uid:             obj.GetUID(),
		resourceVersion: obj.GetResourceVersion(),
		generation:      obj.GetGeneration(),
	}
}

func (v VersionCache) RemoveObj(obj client.Object) {
	delete(v, cacheKeyFromObj(obj))
}

func cacheKeyFromObj(obj client.Object) cacheKey {
	return cacheKey{
		Kind:      obj.GetObjectKind().GroupVersionKind().Kind,
		Name:      obj.GetName(),
		Namespace: obj.GetNamespace(),
	}
}
