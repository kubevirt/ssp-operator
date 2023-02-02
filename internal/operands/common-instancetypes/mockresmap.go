package common_instancetypes

import (
	"sigs.k8s.io/kustomize/api/resmap"
	"sigs.k8s.io/kustomize/api/resource"
	"sigs.k8s.io/kustomize/api/types"
	"sigs.k8s.io/kustomize/kyaml/kio"
	"sigs.k8s.io/kustomize/kyaml/resid"
	"sigs.k8s.io/kustomize/kyaml/yaml"
)

var _ resmap.ResMap = &MockResMap{}

type MockResMap struct {
	resources []*resource.Resource
}

func (m *MockResMap) Size() int {
	return len(m.resources)
}

func (m *MockResMap) Resources() []*resource.Resource {
	return m.resources
}

func (m *MockResMap) Append(*resource.Resource) error {
	panic("Append not implemented by MockResMap")
}

func (m *MockResMap) AppendAll(resmap.ResMap) error {
	panic("AppendAll not implemented by MockResMap")
}

func (m *MockResMap) AbsorbAll(resmap.ResMap) error {
	panic("AbsorbAll not implemented by MockResMap")
}

func (m *MockResMap) AddOriginAnnotation(origin *resource.Origin) error {
	panic("AddOriginAnnotation not implemented by MockResMap")
}

func (m *MockResMap) RemoveOriginAnnotations() error {
	panic("RemoveOriginAnnotations not implemented by MockResMap")
}

func (m *MockResMap) AddTransformerAnnotation(origin *resource.Origin) error {
	panic("AddTransformerAnnotation not implemented by MockResMap")
}

func (m *MockResMap) RemoveTransformerAnnotations() error {
	panic("RemoveTransformerAnnotations not implemented by MockResMap")
}

func (m *MockResMap) AnnotateAll(key string, value string) error {
	panic("AnnotateAll not implemented by MockResMap")
}

func (m *MockResMap) AsYaml() ([]byte, error) {
	panic("AsYaml not implemented by MockResMap")
}

func (m *MockResMap) GetByIndex(int) *resource.Resource {
	panic("GetByIndex not implemented by MockResMap")
}

func (m *MockResMap) GetIndexOfCurrentId(id resid.ResId) (int, error) {
	panic("GetIndexOfCurrentId not implemented by MockResMap")
}

func (m *MockResMap) GetMatchingResourcesByCurrentId(matches resmap.IdMatcher) []*resource.Resource {
	panic("GetMatchingResourcesByCurrentId not implemented by MockResMap")
}

func (m *MockResMap) GetMatchingResourcesByAnyId(matches resmap.IdMatcher) []*resource.Resource {
	panic("GetMatchingResourcesByAnyId not implemented by MockResMap")
}

func (m *MockResMap) GetByCurrentId(resid.ResId) (*resource.Resource, error) {
	panic("GetByCurrentId not implemented by MockResMap")
}

func (m *MockResMap) GetById(resid.ResId) (*resource.Resource, error) {
	panic("GetById not implemented by MockResMap")
}

func (m *MockResMap) GroupedByCurrentNamespace() map[string][]*resource.Resource {
	panic("GroupedByCurrentNamespace not implemented by MockResMap")
}

func (m *MockResMap) GroupedByOriginalNamespace() map[string][]*resource.Resource {
	panic("GroupedByOriginalNamespace not implemented by MockResMap")
}

func (m *MockResMap) ClusterScoped() []*resource.Resource {
	panic("ClusterScoped not implemented by MockResMap")
}

func (m *MockResMap) AllIds() []resid.ResId {
	panic("AllIds not implemented by MockResMap")
}

func (m *MockResMap) Replace(*resource.Resource) (int, error) {
	panic("Replace not implemented by MockResMap")
}

func (m *MockResMap) Remove(resid.ResId) error {
	panic("Remove not implemented by MockResMap")
}

func (m *MockResMap) Clear() {
	panic("Clear not implemented by MockResMap")
}

func (m *MockResMap) DropEmpties() {
	panic("DropEmpties not implemented by MockResMap")
}

func (m *MockResMap) SubsetThatCouldBeReferencedByResource(*resource.Resource) (resmap.ResMap, error) {
	panic("SubsetThatCouldBeReferencedByResource not implemented by MockResMap")
}

func (m *MockResMap) DeAnchor() error {
	panic("DeAnchor not implemented by MockResMap")
}

func (m *MockResMap) DeepCopy() resmap.ResMap {
	panic("DeepCopy not implemented by MockResMap")
}

func (m *MockResMap) ShallowCopy() resmap.ResMap {
	panic("ShallowCopy not implemented by MockResMap")
}

func (m *MockResMap) ErrorIfNotEqualSets(resmap.ResMap) error {
	panic("ErrorIfNotEqualSets not implemented by MockResMap")
}

func (m *MockResMap) ErrorIfNotEqualLists(resmap.ResMap) error {
	panic("ErrorIfNotEqualLists not implemented by MockResMap")
}

func (m *MockResMap) Debug(title string) {
	panic("Debug not implemented by MockResMap")
}

func (m *MockResMap) Select(types.Selector) ([]*resource.Resource, error) {
	panic("Select not implemented by MockResMap")
}

func (m *MockResMap) ToRNodeSlice() []*yaml.RNode {
	panic("ToRNodeSlice not implemented by MockResMap")
}

func (m *MockResMap) ApplySmPatch(selectedSet *resource.IdSet, patch *resource.Resource) error {
	panic("ApplySmPatch not implemented by MockResMap")
}

func (m *MockResMap) RemoveBuildAnnotations() {
	panic("RemoveBuildAnnotations not implemented by MockResMap")
}

func (m *MockResMap) ApplyFilter(f kio.Filter) error {
	panic("ApplyFilter not implemented by MockResMap")
}
