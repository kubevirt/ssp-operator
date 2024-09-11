package tests

import (
	. "github.com/onsi/ginkgo/v2"
)

var _ = Describe("Common Instance Types", func() {
	BeforeEach(func() {
		strategy.SkipSspUpdateTestsIfNeeded()
		waitUntilDeployed()
	})

	AfterEach(func() {
		strategy.RevertToOriginalSspCr()
	})

	Context("operand", func() {

	})
})
