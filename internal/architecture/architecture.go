package architecture

import (
	"fmt"
	"runtime"

	"k8s.io/utils/ptr"
	ssp "kubevirt.io/ssp-operator/api/v1beta3"
)

type Arch string

const (
	AMD64 Arch = "amd64"
	ARM64 Arch = "arm64"
	S390X Arch = "s390x"
)

func ToArch(arch string) (Arch, error) {
	switch arch {
	case string(AMD64):
		return AMD64, nil
	case string(ARM64):
		return ARM64, nil
	case string(S390X):
		return S390X, nil
	default:
		return "", fmt.Errorf("unknown architecture: %s", arch)
	}
}

func ToArchOrPanic(arch string) Arch {
	result, err := ToArch(arch)
	if err != nil {
		panic(err)
	}
	return result
}

func GetSSPArchs(sspSpec *ssp.SSPSpec) ([]Arch, error) {
	if sspSpec.Cluster == nil {
		if ptr.Deref(sspSpec.EnableMultipleArchitectures, false) {
			return nil, fmt.Errorf(".spec.cluster cannot be nil, if .spec.enableMultipleArchitectures is true")
		}
		defaultArchitecture, err := ToArch(runtime.GOARCH)
		if err != nil {
			return nil, fmt.Errorf("default architecture is not supported: %w", err)
		}
		return []Arch{defaultArchitecture}, nil
	}

	archs := sspSpec.Cluster.WorkloadArchitectures
	if len(archs) == 0 {
		archs = sspSpec.Cluster.ControlPlaneArchitectures
	}

	if len(archs) == 0 {
		return nil, fmt.Errorf("no architectures are defined in .spec.cluster")
	}

	if ptr.Deref(sspSpec.EnableMultipleArchitectures, false) {
		result := make([]Arch, 0, len(archs))
		for _, archStr := range archs {
			arch, err := ToArch(archStr)
			if err != nil {
				return nil, err
			}
			result = append(result, arch)
		}
		return result, nil
	}

	// For single architecture case, we prefer the first of ControlPlaneArchitectures.
	// If there are none, we use the first of WorkloadArchitectures.
	archStr := archs[0]
	if len(sspSpec.Cluster.ControlPlaneArchitectures) > 0 {
		archStr = sspSpec.Cluster.ControlPlaneArchitectures[0]
	}

	arch, err := ToArch(archStr)
	if err != nil {
		return nil, err
	}

	return []Arch{arch}, nil
}
