package controller

import (
	"kubevirt.io/ssp-operator/pkg/controller/ssp"
)

func init() {
	// AddToManagerFuncs is a list of functions to create controllers and add them to a manager.
	AddToManagerFuncs = append(AddToManagerFuncs, ssp.Add)
}
