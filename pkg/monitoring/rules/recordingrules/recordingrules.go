package recordingrules

import "github.com/machadovilaca/operator-observability/pkg/operatorrules"

func Register(registry *operatorrules.Registry) error {
	return registry.RegisterRecordingRules(
		operatorRecordingRules(),
		vmiRecordingRules,
	)
}
