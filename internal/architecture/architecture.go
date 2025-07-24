package architecture

import "fmt"

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
