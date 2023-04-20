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

	It("should return correct value for CLEANUP_VM_IMG when variable is set", func() {
		os.Setenv(CleanupVMImageKey, testURL)
		res := GetCleanupVMImage()
		Expect(res).To(Equal(testURL), "CLEANUP_VM_IMG should equal")
		os.Unsetenv(CleanupVMImageKey)
	})

	It("should return correct value for CLEANUP_VM_IMG when variable is not set", func() {
		res := GetCleanupVMImage()
		Expect(res).To(Equal(DeafultCleanupVMIMG), "CLEANUP_VM_IMG should equal")
	})

	It("should return correct value for COPY_TEMPLATE_IMG when variable is set", func() {
		os.Setenv(CopyTemplateImageKey, testURL)
		res := GetCopyTemplateImage()
		Expect(res).To(Equal(testURL), "COPY_TEMPLATE_IMG should equal")
		os.Unsetenv(CopyTemplateImageKey)
	})

	It("should return correct value for COPY_TEMPLATE_IMG when variable is not set", func() {
		res := GetCopyTemplateImage()
		Expect(res).To(Equal(DeafultCopyTemplateIMG), "COPY_TEMPLATE_IMG should equal")
	})

	It("should return correct value for MODIFY_DATA_OBJECT_IMG when variable is set", func() {
		os.Setenv(ModifyDataObjectImageKey, testURL)
		res := GetModifyDataObjectImage()
		Expect(res).To(Equal(testURL), "MODIFY_DATA_OBJECT_IMG should equal")
		os.Unsetenv(ModifyDataObjectImageKey)
	})

	It("should return correct value for MODIFY_DATA_OBJECT_IMG when variable is not set", func() {
		res := GetModifyDataObjectImage()
		Expect(res).To(Equal(DeafultModifyDataObjectIMG), "MODIFY_DATA_OBJECT_IMG should equal")
	})

	It("should return correct value for CREATE_VM_IMG when variable is set", func() {
		os.Setenv(CreateVMImageKey, testURL)
		res := GetCreateVMImage()
		Expect(res).To(Equal(testURL), "CREATE_VM_IMG should equal")
		os.Unsetenv(CreateVMImageKey)
	})

	It("should return correct value for CREATE_VM_IMG when variable is not set", func() {
		res := GetCreateVMImage()
		Expect(res).To(Equal(DeafultCreateVMIMG), "CREATE_VM_IMG should equal")
	})

	It("should return correct value for DISK_VIRT_CUSTOMIZE_IMG when variable is set", func() {
		os.Setenv(DiskVirtCustomizeImageKey, testURL)
		res := GetDiskVirtCustomizeImage()
		Expect(res).To(Equal(testURL), "DISK_VIRT_CUSTOMIZE_IMG should equal")
		os.Unsetenv(DiskVirtCustomizeImageKey)
	})

	It("should return correct value for DISK_VIRT_CUSTOMIZE_IMG when variable is not set", func() {
		res := GetDiskVirtCustomizeImage()
		Expect(res).To(Equal(DeafultDiskVirtCustomizeIMG), "DISK_VIRT_CUSTOMIZE_IMG should equal")
	})

	It("should return correct value for DISK_VIRT_SYSPREP_IMG when variable is set", func() {
		os.Setenv(DiskVirtSysprepImageKey, testURL)
		res := GetDiskVirtSysprepImage()
		Expect(res).To(Equal(testURL), "DISK_VIRT_SYSPREP_IMG should equal")
		os.Unsetenv(DiskVirtSysprepImageKey)
	})

	It("should return correct value for DISK_VIRT_SYSPREP_IMG when variable is not set", func() {
		res := GetDiskVirtSysprepImage()
		Expect(res).To(Equal(DeafultDiskVirtSysprepIMG), "DISK_VIRT_SYSPREP_IMG should equal")
	})

	It("should return correct value for MODIFY_VM_TEMPLATE_IMG when variable is set", func() {
		os.Setenv(ModifyVMTemplateImageKey, testURL)
		res := GetModifyVMTemplateImage()
		Expect(res).To(Equal(testURL), "MODIFY_VM_TEMPLATE_IMG should equal")
		os.Unsetenv(ModifyVMTemplateImageKey)
	})

	It("should return correct value for MODIFY_VM_TEMPLATE_IMG when variable is not set", func() {
		res := GetModifyVMTemplateImage()
		Expect(res).To(Equal(DeafultModifyVMTemplateIMG), "MODIFY_VM_TEMPLATE_IMG should equal")
	})

	It("should return correct value for WAIT_FOR_VMI_STATUS_IMG when variable is set", func() {
		os.Setenv(WaitForVMISTatusImageKey, testURL)
		res := GetWaitForVMIStatusImage()
		Expect(res).To(Equal(testURL), "WAIT_FOR_VMI_STATUS_IMG should equal")
		os.Unsetenv(WaitForVMISTatusImageKey)
	})

	It("should return correct value for WAIT_FOR_VMI_STATUS_IMG when variable is not set", func() {
		res := GetWaitForVMIStatusImage()
		Expect(res).To(Equal(DefaultWaitForVMIStatusIMG), "WAIT_FOR_VMI_STATUS_IMG should equal")
	})

	It("should return correct value for VIRTIO_IMG when variable is set", func() {
		os.Setenv(VirtioImageKey, testURL)
		res := GetVirtioImage()
		Expect(res).To(Equal(testURL), "VIRTIO_IMG should equal")
		os.Unsetenv(VirtioImageKey)
	})

	It("should return correct value for VIRTIO_IMG when variable is not set", func() {
		res := GetVirtioImage()
		Expect(res).To(Equal(DefaultVirtioIMG), "VIRTIO_IMG should equal")
	})

	It("should return correct value for GENERATE_SSH_KEYS_IMG when variable is set", func() {
		os.Setenv(GenerateSSHKeysImageKey, testURL)
		res := GetSSHKeysStatusImage()
		Expect(res).To(Equal(testURL), "GENERATE_SSH_KEYS_IMG should equal")
		os.Unsetenv(GenerateSSHKeysImageKey)
	})

	It("should return correct value for GENERATE_SSH_KEYS_IMG when variable is not set", func() {
		res := GetSSHKeysStatusImage()
		Expect(res).To(Equal(GenerateSSHKeysIMG), "GENERATE_SSH_KEYS_IMG should equal")
	})
})
