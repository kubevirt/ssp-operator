package validation

import (
	"encoding/json"

	k6tv1 "kubevirt.io/client-go/api/v1"

	"kubevirt.io/ssp-operator/internal/template-validator/validation/path"
)

type Rule struct {
	// mandatory keys
	Rule    RuleType  `json:"rule"`
	Name    string    `json:"name"`
	Path    path.Path `json:"path"`
	Message string    `json:"message"`
	// optional keys
	Valid       *path.Path `json:"valid,omitempty"`
	JustWarning bool       `json:"justWarning,omitempty"`
	// arguments (optional keys)
	Values    []path.StringOrPath `json:"values,omitempty"`
	Min       *path.IntOrPath     `json:"min,omitempty"`
	Max       *path.IntOrPath     `json:"max,omitempty"`
	MinLength *path.IntOrPath     `json:"minLength,omitempty"`
	MaxLength *path.IntOrPath     `json:"maxLength,omitempty"`
	Regex     string              `json:"regex,omitempty"`
}

func (r *Rule) findPathOn(vm *k6tv1.VirtualMachine) (bool, error) {
	results, err := r.Valid.Find(vm)
	if err != nil {
		return false, err
	}
	return results.Len() > 0, nil
}

func (r *Rule) IsAppliableOn(vm *k6tv1.VirtualMachine) (bool, error) {
	if r.Valid == nil {
		// nothing to check against, so it is OK
		return true, nil
	}
	ok, err := r.findPathOn(vm)
	if err == path.ErrInvalidJSONPath {
		return false, nil
	}
	return ok, err
}

func ParseRules(data []byte) ([]Rule, error) {
	var rules []Rule
	if len(data) == 0 {
		// nothing to do
		return rules, nil
	}
	err := json.Unmarshal(data, &rules)
	return rules, err
}
