package crd_watch_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestCrdWatch(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "CrdWatch Suite")
}
