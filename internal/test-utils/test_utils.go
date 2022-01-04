package test_utils

import (
	. "github.com/onsi/gomega"
	"kubevirt.io/ssp-operator/internal/common"

	"k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func ExpectResourceExists(resource client.Object, request common.Request) {
	key := client.ObjectKeyFromObject(resource)
	ExpectWithOffset(1, request.Client.Get(request.Context, key, resource)).ToNot(HaveOccurred())
}

func ExpectResourceNotExists(resource client.Object, request common.Request) {
	key := client.ObjectKeyFromObject(resource)
	err := request.Client.Get(request.Context, key, resource)
	ExpectWithOffset(1, err).To(HaveOccurred())
	ExpectWithOffset(1, errors.IsNotFound(err)).To(BeTrue())
}
