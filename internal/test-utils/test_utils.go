package test_utils

import (
	. "github.com/onsi/gomega"
	"kubevirt.io/ssp-operator/internal/common"

	"k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func ExpectResourceExists(resource client.Object, request common.Request) {
	key := client.ObjectKeyFromObject(resource)
	Expect(request.Client.Get(request.Context, key, resource)).ToNot(HaveOccurred())
}

func ExpectResourceNotExists(resource client.Object, request common.Request) {
	key := client.ObjectKeyFromObject(resource)
	err := request.Client.Get(request.Context, key, resource)
	Expect(err).To(HaveOccurred())
	Expect(errors.IsNotFound(err)).To(BeTrue())
}
