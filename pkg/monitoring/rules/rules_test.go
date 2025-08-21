package rules

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/rhobs/operator-observability-toolkit/pkg/testutil"
)

func TestRules(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Rules Suite")
}

var _ = Describe("Rules Validation", func() {
	var linter *testutil.Linter

	BeforeEach(func() {
		Expect(SetupRules()).To(Succeed())
		linter = testutil.New()
	})

	It("Should validate alerts", func() {
		linter.AddCustomAlertValidations(
			testutil.ValidateAlertNameLength,
			testutil.ValidateAlertRunbookURLAnnotation,
			testutil.ValidateAlertHealthImpactLabel,
			testutil.ValidateAlertPartOfAndComponentLabels)

		problems := linter.LintAlerts(ListAlerts())
		Expect(problems).To(BeEmpty())
	})

	It("Should validate recording rules", func() {
		problems := linter.LintRecordingRules(ListRecordingRules())
		Expect(problems).To(BeEmpty())
	})
})
