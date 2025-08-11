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

	// The resulting architecture order is ControlPlaneArchitectures
	// followed by WorkloadArchitectures, without duplicates.
	archStrs := concatWithoutRepetitions(
		sspSpec.Cluster.ControlPlaneArchitectures,
		sspSpec.Cluster.WorkloadArchitectures,
	)

	if len(archStrs) == 0 {
		return nil, fmt.Errorf("no architectures are defined in .spec.cluster")
	}

	if ptr.Deref(sspSpec.EnableMultipleArchitectures, false) {
		result := make([]Arch, 0, len(archStrs))
		for _, archStr := range archStrs {
			arch, err := ToArch(archStr)
			if err != nil {
				return nil, err
			}
			result = append(result, arch)
		}
		return result, nil
	}

	arch, err := ToArch(archStrs[0])
	if err != nil {
		return nil, err
	}

	return []Arch{arch}, nil
}

func concatWithoutRepetitions[T comparable](data ...[]T) []T {
	var result []T
	set := map[T]struct{}{}
	for _, slice := range data {
		for _, val := range slice {
			if _, ok := set[val]; !ok {
				set[val] = struct{}{}
				result = append(result, val)
			}
		}
	}
	return result
}
