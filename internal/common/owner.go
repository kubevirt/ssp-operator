package common

import (
	"reflect"

	"github.com/operator-framework/operator-lib/handler"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func CheckOwnerAnnotation(obj client.Object, owner client.Object) bool {
	annotations := obj.GetAnnotations()
	if annotations == nil {
		return false
	}

	ownerGroupKind := owner.GetObjectKind().GroupVersionKind().GroupKind()
	typeVal, ok := annotations[handler.TypeAnnotation]
	if !ok || typeVal != ownerGroupKind.String() {
		return false
	}

	sspNamespacedName := types.NamespacedName{
		Namespace: owner.GetNamespace(),
		Name:      owner.GetName(),
	}

	namespacedNameVal, ok := annotations[handler.NamespacedNameAnnotation]
	return ok && namespacedNameVal == sspNamespacedName.String()
}

func ListOwnedResources[L any, T any, PtrL interface {
	*L
	client.ObjectList
}, PtrT interface {
	*T
	client.Object
}](request *Request, listOpts ...client.ListOption) ([]T, error) {
	listObj := PtrL(new(L))
	if err := request.Client.List(request.Context, listObj, listOpts...); err != nil {
		return nil, err
	}

	items := extractItems[PtrL, T](listObj)
	var filteredItems []T
	for _, item := range items {
		if !CheckOwnerAnnotation(PtrT(&item), request.Instance) {
			continue
		}
		filteredItems = append(filteredItems, item)
	}
	return filteredItems, nil
}

func extractItems[L client.ObjectList, T any](list L) []T {
	listValue := reflect.ValueOf(list)
	if !listValue.IsValid() {
		return nil
	}

	itemsValue := listValue.Elem().FieldByName("Items")
	return itemsValue.Interface().([]T)
}
