/*
 * This file is part of the KubeVirt project
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 * Copyright 2019 Red Hat, Inc.
 */

package validation

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	k6tv1 "kubevirt.io/api/core/v1"

	"kubevirt.io/ssp-operator/internal/template-validator/validation/path"
)

type RuleType string

const (
	IntegerRule RuleType = "integer"
	StringRule  RuleType = "string"
	EnumRule    RuleType = "enum"
	RegexRule   RuleType = "regex"
)

func (r RuleType) IsValid() bool {
	switch r {
	case IntegerRule:
		fallthrough
	case StringRule:
		fallthrough
	case EnumRule:
		fallthrough
	case RegexRule:
		return true
	}
	return false
}

type RuleApplier interface {
	Apply(vm, ref *k6tv1.VirtualMachine) (bool, error)
	String() string
}

var ErrNoValuesFound = errors.New("no values were found")

// we need a vm reference to specialize a rule because few key fields may
// be JSONPath, and we need to walk them to get e.g. the value to check,
// or the limits to enforce.
func (r *Rule) Specialize(vm, ref *k6tv1.VirtualMachine) (RuleApplier, error) {
	switch r.Rule {
	case IntegerRule:
		return NewIntRule(r, vm, ref)
	case StringRule:
		return NewStringRule(r, vm, ref)
	case EnumRule:
		return NewEnumRule(r, vm, ref)
	case RegexRule:
		return NewRegexRule(r)
	}
	return nil, fmt.Errorf("usupported rule: %s", r.Rule)
}

type Range struct {
	MinSet bool
	Min    int64
	MaxSet bool
	Max    int64
}

func (r *Range) Decode(min, max *path.IntOrPath, vm, ref *k6tv1.VirtualMachine) error {
	if min != nil {
		v, err := decodeInt(*min, vm, ref)
		if err != nil {
			return err
		}
		r.Min = v
		r.MinSet = true
	}
	if max != nil {
		v, err := decodeInt(*max, vm, ref)
		if err != nil {
			return err
		}
		r.Max = v
		r.MaxSet = true
	}
	return nil
}

func (r *Range) Includes(v int64) bool {
	if r.MinSet && v < r.Min {
		return false
	}
	if r.MaxSet && v > r.Max {
		return false
	}
	return true
}

// These are the specializedrules
type intRule struct {
	Ref       *Rule
	Value     Range
	Current   []int64
	Satisfied bool
}

// JSONPATH lookup logic, aka what this "ref" object and why we need it
//
// When we need to fetch the value of a Rule.Path which happens to be a JSONPath,
// first we just try if the given VM object has the path we need.
// If the lookup succeeds, everyone's happy and we stop here.
// Else, the vm obj has not the path we were looking for.
// It could be either:
// - the path is bogus. We check lazily, so this is the first time we see this
//   and we need to make a decision. But mayne
// - the path is legal, but it refers to an optional subpath which is missing.
//   so we try again with the zero-initialized "reference" object.
//   if even this lookup fails, we mark the path as bogus.
//   Otherwise we use the zero, default, value for our logic.

func findPathOnVmOrRef(path *path.Path, vm, ref *k6tv1.VirtualMachine) (path.Results, error) {
	res, err := path.Find(vm)
	if err == nil {
		return res, nil
	}

	res, err = path.Find(ref)
	if err == nil {
		return res, nil
	}

	return nil, err
}

func decodeInts(path *path.Path, vm, ref *k6tv1.VirtualMachine) ([]int64, error) {
	res, err := findPathOnVmOrRef(path, vm, ref)
	if err != nil {
		return nil, err
	}
	return res.AsInt64()
}

func decodeInt(ip path.IntOrPath, vm, ref *k6tv1.VirtualMachine) (int64, error) {
	if ip.IsInt() {
		return ip.Int, nil
	}

	v, err := decodeInts(ip.Path, vm, ref)
	if err != nil {
		return 0, err
	}
	if len(v) != 1 {
		return 0, fmt.Errorf("expected one value, found %v", len(v))
	}
	return v[0], nil
}

func decodeStrings(path *path.Path, vm, ref *k6tv1.VirtualMachine) ([]string, error) {
	res, err := findPathOnVmOrRef(path, vm, ref)
	if err != nil {
		return nil, err
	}
	return res.AsString()
}

func decodeString(sp path.StringOrPath, vm, ref *k6tv1.VirtualMachine) (string, error) {
	if sp.IsString() {
		return sp.Str, nil
	}

	vals, err := decodeStrings(sp.Path, vm, ref)
	if err != nil {
		return "", err
	}
	if len(vals) != 1 {
		return "", fmt.Errorf("expected one value, found %v", len(vals))
	}
	return vals[0], nil
}

func NewIntRule(r *Rule, vm, ref *k6tv1.VirtualMachine) (RuleApplier, error) {
	ir := intRule{Ref: r}
	err := ir.Value.Decode(r.Min, r.Max, vm, ref)
	if err != nil {
		return nil, err
	}
	return &ir, nil
}

func (ir *intRule) Apply(vm, ref *k6tv1.VirtualMachine) (bool, error) {
	vals, err := decodeInts(&ir.Ref.Path, vm, ref)
	if err != nil {
		return false, err
	}
	if len(vals) == 0 {
		return false, ErrNoValuesFound
	}

	ir.Current = vals
	satisfied := true
	for _, val := range vals {
		if !ir.Value.Includes(val) {
			satisfied = false
			break
		}
	}

	ir.Satisfied = satisfied
	return ir.Satisfied, nil
}

func (ir *intRule) String() string {
	lowerBound := "N/A"
	if ir.Value.MinSet {
		lowerBound = strconv.FormatInt(ir.Value.Min, 10)
	}
	upperBound := "N/A"
	if ir.Value.MaxSet {
		upperBound = strconv.FormatInt(ir.Value.Max, 10)
	}

	if ir.Satisfied {
		return fmt.Sprintf("All values %v are in interval [%s, %s]", ir.Current, lowerBound, upperBound)
	}

	errorMessage := ""
	for i, value := range ir.Current {
		if ir.Value.MinSet && value < ir.Value.Min {
			errorMessage += fmt.Sprintf("value %v is lower than minimum [%s]", value, lowerBound)
		}
		if ir.Value.MaxSet && value > ir.Value.Max {
			errorMessage += fmt.Sprintf("value %v is higher than maximum [%s]", value, upperBound)
		}
		if i != (len(ir.Current) - 1) {
			errorMessage += ", "
		}
	}
	return errorMessage
}

type stringRule struct {
	Ref       *Rule
	Length    Range
	Current   []string
	Satisfied bool
}

func NewStringRule(r *Rule, vm, ref *k6tv1.VirtualMachine) (RuleApplier, error) {
	sr := stringRule{Ref: r}
	err := sr.Length.Decode(r.MinLength, r.MaxLength, vm, ref)
	if err != nil {
		return nil, err
	}
	return &sr, nil
}

func (sr *stringRule) Apply(vm, ref *k6tv1.VirtualMachine) (bool, error) {
	vals, err := decodeStrings(&sr.Ref.Path, vm, ref)
	if err != nil {
		return false, err
	}
	if len(vals) == 0 {
		return false, ErrNoValuesFound
	}

	sr.Current = vals
	satisfied := true
	for _, val := range vals {
		if !sr.Length.Includes(int64(len(val))) {
			satisfied = false
			break
		}
	}

	sr.Satisfied = satisfied
	return sr.Satisfied, nil
}

func (sr *stringRule) String() string {
	lowerBound := "N/A"
	if sr.Length.MinSet {
		lowerBound = strconv.FormatInt(sr.Length.Min, 10)
	}
	upperBound := "N/A"
	if sr.Length.MaxSet {
		upperBound = strconv.FormatInt(sr.Length.Max, 10)
	}

	if sr.Satisfied {
		return fmt.Sprintf("Lengts of all strings are in interval [%s, %s]", lowerBound, upperBound)
	} else {
		return fmt.Sprintf("Lengths of some strings are not in interval [%s, %s]", lowerBound, upperBound)
	}
}

type enumRule struct {
	Ref       *Rule
	Values    []string
	Current   []string
	Satisfied bool
}

func NewEnumRule(r *Rule, vm, ref *k6tv1.VirtualMachine) (RuleApplier, error) {
	er := enumRule{Ref: r}
	for _, v := range r.Values {
		s, err := decodeString(v, vm, ref)
		if err != nil {
			return nil, err
		}
		er.Values = append(er.Values, s)
	}
	return &er, nil
}

func (er *enumRule) Apply(vm, ref *k6tv1.VirtualMachine) (bool, error) {
	vals, err := decodeStrings(&er.Ref.Path, vm, ref)
	if err != nil {
		return false, err
	}
	if len(vals) == 0 {
		return false, ErrNoValuesFound
	}

	er.Current = vals
	er.Satisfied = containsOnly(vals, er.Values)
	return er.Satisfied, nil
}

func containsOnly(data []string, expected []string) bool {
outerLoop:
	for _, val := range data {
		for _, expectedVal := range expected {
			if val == expectedVal {
				continue outerLoop
			}
		}
		return false
	}
	return true
}

func (er *enumRule) String() string {
	if er.Satisfied {
		return fmt.Sprintf("All [%s] are in [%s]",
			strings.Join(er.Current, ", "),
			strings.Join(er.Values, ", "))
	} else {
		return fmt.Sprintf("Some of [%s] are not in [%s]",
			strings.Join(er.Current, ", "),
			strings.Join(er.Values, ", "))
	}
}

type regexRule struct {
	Ref       *Rule
	Regex     *regexp.Regexp
	Current   []string
	Satisfied bool
}

func NewRegexRule(r *Rule) (RuleApplier, error) {
	regex, err := regexp.Compile(r.Regex)
	if err != nil {
		return nil, err
	}
	return &regexRule{
		Ref:   r,
		Regex: regex,
	}, nil
}

func (rr *regexRule) Apply(vm, ref *k6tv1.VirtualMachine) (bool, error) {
	vals, err := decodeStrings(&rr.Ref.Path, vm, ref)
	if err != nil {
		return false, err
	}
	if len(vals) == 0 {
		return false, ErrNoValuesFound
	}

	rr.Current = vals
	satisfied := true
	for _, val := range vals {
		if !rr.Regex.MatchString(val) {
			satisfied = false
			break
		}
	}

	rr.Satisfied = satisfied
	return rr.Satisfied, nil
}

func (rr *regexRule) String() string {
	if rr.Satisfied {
		return fmt.Sprintf("All [%s] match %s", strings.Join(rr.Current, ", "), rr.Regex)
	} else {
		return fmt.Sprintf("Some of [%s] do not match %s", strings.Join(rr.Current, ", "), rr.Regex)
	}
}
