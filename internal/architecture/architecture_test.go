package architecture_test

import (
	"runtime"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"k8s.io/utils/ptr"

	ssp "kubevirt.io/ssp-operator/api/v1beta3"
	"kubevirt.io/ssp-operator/internal/architecture"
)

var _ = Describe("GetSSPArchs", func() {
	It("should return defaultArchitecture if .spec.cluster is nil", func() {
		var defaultArchitecture = architecture.ToArchOrPanic(runtime.GOARCH)

		archs, err := architecture.GetSSPArchs(&ssp.SSPSpec{})
		Expect(err).ToNot(HaveOccurred())
		Expect(archs).To(ConsistOf(defaultArchitecture))
	})

	It("should fail if .spec.cluster is nil and multi-architecture is enabled", func() {
		_, err := architecture.GetSSPArchs(&ssp.SSPSpec{
			EnableMultipleArchitectures: ptr.To(true),
		})
		Expect(err).To(MatchError(".spec.cluster cannot be nil, if .spec.enableMultipleArchitectures is true"))
	})

	It("should fail if no architectures are defined", func() {
		_, err := architecture.GetSSPArchs(&ssp.SSPSpec{
			Cluster: &ssp.Cluster{},
		})
		Expect(err).To(MatchError("no architectures are defined in .spec.cluster"))
	})

	Context("with multi-arch disabled", func() {
		It("should return the first control plane architecture", func() {
			archs, err := architecture.GetSSPArchs(&ssp.SSPSpec{
				Cluster: &ssp.Cluster{
					WorkloadArchitectures:     []string{string(architecture.AMD64), string(architecture.ARM64)},
					ControlPlaneArchitectures: []string{string(architecture.S390X)},
				},
			})
			Expect(err).ToNot(HaveOccurred())
			Expect(archs).To(ConsistOf(architecture.S390X))
		})

		It("should return the first workload architecture if control plane architectures are empty", func() {
			archs, err := architecture.GetSSPArchs(&ssp.SSPSpec{
				Cluster: &ssp.Cluster{
					WorkloadArchitectures: []string{string(architecture.AMD64), string(architecture.ARM64)},
				},
			})
			Expect(err).ToNot(HaveOccurred())
			Expect(archs).To(ConsistOf(architecture.AMD64))
		})

		It("should fail if the selected architecture is unknown", func() {
			_, err := architecture.GetSSPArchs(&ssp.SSPSpec{
				Cluster: &ssp.Cluster{
					ControlPlaneArchitectures: []string{"invalid-arch"},
				},
			})
			Expect(err).To(MatchError("unknown architecture: invalid-arch"))
		})
	})

	Context("with multi-arch enabled", func() {
		It("should return workload architectures", func() {
			archs, err := architecture.GetSSPArchs(&ssp.SSPSpec{
				EnableMultipleArchitectures: ptr.To(true),
				Cluster: &ssp.Cluster{
					WorkloadArchitectures:     []string{string(architecture.AMD64), string(architecture.ARM64)},
					ControlPlaneArchitectures: []string{string(architecture.S390X)},
				},
			})
			Expect(err).ToNot(HaveOccurred())
			Expect(archs).To(ConsistOf(architecture.AMD64, architecture.ARM64))
		})

		It("should return control plane architectures if workload architectures are empty", func() {
			archs, err := architecture.GetSSPArchs(&ssp.SSPSpec{
				EnableMultipleArchitectures: ptr.To(true),
				Cluster: &ssp.Cluster{
					ControlPlaneArchitectures: []string{string(architecture.S390X)},
				},
			})
			Expect(err).ToNot(HaveOccurred())
			Expect(archs).To(ConsistOf(architecture.S390X))
		})

		It("should fail if workload architectures contain an unknown architecture", func() {
			_, err := architecture.GetSSPArchs(&ssp.SSPSpec{
				EnableMultipleArchitectures: ptr.To(true),
				Cluster: &ssp.Cluster{
					WorkloadArchitectures: []string{string(architecture.AMD64), "unknown-arch"},
				},
			})
			Expect(err).To(MatchError("unknown architecture: unknown-arch"))
		})

		It("should fail if control plane architectures contain an unknown architecture", func() {
			_, err := architecture.GetSSPArchs(&ssp.SSPSpec{
				EnableMultipleArchitectures: ptr.To(true),
				Cluster: &ssp.Cluster{
					ControlPlaneArchitectures: []string{string(architecture.S390X), "invalid-arch"},
				},
			})
			Expect(err).To(MatchError("unknown architecture: invalid-arch"))
		})
	})
})

func TestArchitecture(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Architecture Suite")
}
