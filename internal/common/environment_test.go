package common

import (
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("environments", func() {
	testURL := "test:test"

	It("should return correct value for OPERATOR_VERSION when variable is set", func() {
		os.Setenv(OperatorVersionKey, "v0.0.1")
		res := GetOperatorVersion()
		Expect(res).To(Equal("v0.0.1"), "OPERATOR_VERSION should equal")
		os.Unsetenv(OperatorVersionKey)
	})

	It("should return correct value for OPERATOR_VERSION when variable is not set", func() {
		res := GetOperatorVersion()
		Expect(res).To(Equal(defaultOperatorVersion), "OPERATOR_VERSION should equal")
	})

	It("should return correct value for TEKTON_TASKS_IMG when variable is set", func() {
		os.Setenv(TektonTasksImageKey, testURL)
		res := GetTektonTasksImage()
		Expect(res).To(Equal(testURL), "TEKTON_TASKS_IMG should equal")
		os.Unsetenv(TektonTasksImageKey)
	})

	It("should return correct value for TEKTON_TASKS_IMG when variable is not set", func() {
		res := GetTektonTasksImage()
		Expect(res).To(Equal(DefaultTektonTasksIMG), "TEKTON_TASKS_IMG should equal")
	})

	It("should return correct value for TEKTON_TASKS_DISK_VIRT_IMG when variable is set", func() {
		os.Setenv(TektonTasksDiskVirtImageKey, testURL)
		res := GetTektonTasksDiskVirtImage()
		Expect(res).To(Equal(testURL), "TEKTON_TASKS_DISK_VIRT_IMG should equal")
		os.Unsetenv(TektonTasksDiskVirtImageKey)
	})

	It("should return correct value for TEKTON_TASKS_DISK_VIRT_IMG when variable is not set", func() {
		res := GetTektonTasksDiskVirtImage()
		Expect(res).To(Equal(DeafultTektonTasksDiskVirtIMG), "TEKTON_TASKS_DISK_VIRT_IMG should equal")
	})

})
