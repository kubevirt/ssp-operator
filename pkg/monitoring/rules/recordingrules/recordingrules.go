package recordingrules

import "github.com/rhobs/operator-observability-toolkit/pkg/operatorrules"

func Register(registry *operatorrules.Registry) error {
	return registry.RegisterRecordingRules(
		operatorRecordingRules(),
		vmiRecordingRules,
	)
}
