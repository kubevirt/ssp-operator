package common

import "sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

type cacheKey struct {
	Kind      string
	Name      string
	Namespace string
}

type cacheValue struct {
	resourceVersion string
	generation      int64
}

type VersionCache map[cacheKey]cacheValue

func (v VersionCache) Contains(obj controllerutil.Object) bool {
	cached, ok := v[cacheKeyFromObj(obj)]
	if !ok {
		return false
	}
	if obj.GetGeneration() == 0 {
		if obj.GetResourceVersion() == "" {
			return false
		}
		return cached.resourceVersion == obj.GetResourceVersion()
	}

	return cached.generation == obj.GetGeneration()
}

func (v VersionCache) Add(obj controllerutil.Object) {
	kind := obj.GetObjectKind().GroupVersionKind().Kind
	if kind == "" {
		// Do not cache objects without kind
		return
	}
	v[cacheKeyFromObj(obj)] = cacheValue{
		resourceVersion: obj.GetResourceVersion(),
		generation:      obj.GetGeneration(),
	}
}

func (v VersionCache) RemoveObj(obj controllerutil.Object) {
	delete(v, cacheKeyFromObj(obj))
}

func cacheKeyFromObj(obj controllerutil.Object) cacheKey {
	return cacheKey{
		Kind:      obj.GetObjectKind().GroupVersionKind().Kind,
		Name:      obj.GetName(),
		Namespace: obj.GetNamespace(),
	}
}
