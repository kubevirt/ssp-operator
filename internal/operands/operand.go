package operands

import (
	"sigs.k8s.io/controller-runtime/pkg/client"

	"kubevirt.io/ssp-operator/internal/common"
)

type Operand interface {
	// WatchTypes returns a slice of namespaced resources, that the operator should watch.
	WatchTypes() []WatchType

	// WatchClusterTypes returns a slice of cluster resources, that the operator should watch.
	WatchClusterTypes() []WatchType

	// RequiredCrds returns names of CRDs, that need to be installed for the operand to work.
	RequiredCrds() []string

	// Reconcile creates and updates resources.
	Reconcile(*common.Request) ([]common.ReconcileResult, error)

	// Cleanup removes any created cluster resources.
	// They don't use owner references, so the garbage collector will not remove them.
	Cleanup(*common.Request) ([]common.CleanupResult, error)

	// Name returns the name of the operand
	Name() string
}

type WatchType struct {
	Object client.Object

	// WatchFullObject specifies if the operator should watch for any changes in the full object.
	// Otherwise, only these changes in spec, labels, and annotations.
	// If an object does not have spec field, the full object is watched by default.
	WatchFullObject bool
}
