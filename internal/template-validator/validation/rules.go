package validation

import (
	"encoding/json"

	k6tv1 "kubevirt.io/client-go/api/v1"

	"kubevirt.io/ssp-operator/internal/template-validator/validation/path"
)

// the flow:
// 1. first you do ParseRule and get []Rule. This is little more than raw data rearranged in Go structs.
//    You can work with that programmatically, but the first thing you may want to do is
// 2. ...

type Rule struct {
	// mandatory keys
	Rule    RuleType `json:"rule"`
	Name    string   `json:"name"`
	Path    string   `json:"path"`
	Message string   `json:"message"`
	// optional keys
	Valid       string `json:"valid,omitempty"`
	JustWarning bool   `json:"justWarning,omitempty"`
	// arguments (optional keys)
	Values    []string    `json:"values,omitempty"`
	Min       interface{} `json:"min,omitempty"`
	Max       interface{} `json:"max,omitempty"`
	MinLength interface{} `json:"minLength,omitempty"`
	MaxLength interface{} `json:"maxLength,omitempty"`
	Regex     string      `json:"regex,omitempty"`
}

func (r *Rule) findPathOn(vm *k6tv1.VirtualMachine) (bool, error) {
	var err error
	p, err := path.New(r.Valid)
	if err != nil {
		return false, err
	}
	results, err := p.Find(vm)
	if err != nil {
		return false, err
	}
	return results.Len() > 0, nil
}

func (r *Rule) IsAppliableOn(vm *k6tv1.VirtualMachine) (bool, error) {
	if r.Valid == "" {
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
