package common

import (
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
