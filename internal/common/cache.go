package common

import "sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

type versionCacheKey struct {
	Kind      string
	Name      string
	Namespace string
}

type VersionCache map[versionCacheKey]string

func (v VersionCache) Contains(obj controllerutil.Object) bool {
	resVersion := obj.GetResourceVersion()
	if resVersion == "" {
		return false
	}
	cachedVersion, ok := v[cacheKeyFromObj(obj)]
	return ok && resVersion == cachedVersion
}

func (v VersionCache) Add(obj controllerutil.Object) {
	kind := obj.GetObjectKind().GroupVersionKind().Kind
	if kind == "" {
		// Do not cache objects without kind
		return
	}
	v[cacheKeyFromObj(obj)] = obj.GetResourceVersion()
}

func (v VersionCache) RemoveObj(obj controllerutil.Object) {
	delete(v, cacheKeyFromObj(obj))
}

func cacheKeyFromObj(obj controllerutil.Object) versionCacheKey {
	return versionCacheKey{
		Kind:      obj.GetObjectKind().GroupVersionKind().Kind,
		Name:      obj.GetName(),
		Namespace: obj.GetNamespace(),
	}
}
