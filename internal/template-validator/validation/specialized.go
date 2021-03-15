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
	"reflect"
	"regexp"
	"strconv"
	"strings"

	k6tv1 "kubevirt.io/client-go/api/v1"
)

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
	case "integer":
		return NewIntRule(r, vm, ref)
	case "string":
		return NewStringRule(r, vm, ref)
	case "enum":
		return NewEnumRule(r, vm, ref)
	case "regex":
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

func (r *Range) Decode(Min, Max interface{}, vm, ref *k6tv1.VirtualMachine) error {
	if Min != nil {
		v, err := decodeInt(Min, vm, ref)
		if err != nil {
			return err
		}
		r.Min = v
		r.MinSet = true
	}
	if Max != nil {
		v, err := decodeInt(Max, vm, ref)

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

// The first argument is either a single literal integer or a JSON path to one or more integers.
// Currently the function does not support multiple literal integers.
func decodeInts(obj interface{}, vm, ref *k6tv1.VirtualMachine) ([]int64, error) {
	if val, ok := toInt64(obj); ok {
		return []int64{val}, nil
	}

	jsonPath, ok := obj.(string)
	if !ok {
		return nil, fmt.Errorf("unsupported type %v (%v)", obj, reflect.TypeOf(obj).Name())
	}
	if !isJSONPath(jsonPath) {
		return nil, fmt.Errorf("parameter is not JSONPath: %v", jsonPath)
	}

	v, err := decodeInt64FromJSONPath(jsonPath, vm)
	if err != nil {
		v, err = decodeInt64FromJSONPath(jsonPath, ref)
	}
	return v, err
}

func decodeInt(obj interface{}, vm, ref *k6tv1.VirtualMachine) (int64, error) {
	v, err := decodeInts(obj, vm, ref)
	if err != nil {
		return 0, err
	}
	if len(v) != 1 {
		return 0, fmt.Errorf("expected one value, found %v", len(v))
	}
	return v[0], nil
}

// The first argument is either a single literal string or a JSON path to one or more strings.
// Currently the function does not support multiple literal strings.
func decodeStrings(s string, vm, ref *k6tv1.VirtualMachine) ([]string, error) {
	if !isJSONPath(s) {
		return []string{s}, nil
	}
	v, err := decodeJSONPathString(s, vm)
	if err != nil {
		v, err = decodeJSONPathString(s, ref)
	}
	return v, err
}

func decodeString(s string, vm, ref *k6tv1.VirtualMachine) (string, error) {
	vals, err := decodeStrings(s, vm, ref)
	if err != nil {
		return "", err
	}
	if len(vals) != 1 {
		return "", fmt.Errorf("expected one value, found %v", len(vals))
	}
	return vals[0], nil
}

func decodeInt64FromJSONPath(jsonPath string, vm *k6tv1.VirtualMachine) ([]int64, error) {
	path, err := findJsonPath(jsonPath, vm)
	if err != nil {
		return nil, err
	}
	return path.AsInt64()
}

func decodeJSONPathString(jsonPath string, vm *k6tv1.VirtualMachine) ([]string, error) {
	path, err := findJsonPath(jsonPath, vm)
	if err != nil {
		return nil, err
	}
	return path.AsString()
}

func findJsonPath(jsonPath string, vm *k6tv1.VirtualMachine) (*Path, error) {
	path, err := NewPath(jsonPath)
	if err != nil {
		return nil, err
	}
	err = path.Find(vm)
	if err != nil {
		return nil, err
	}
	return path, nil
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
	vals, err := decodeInts(ir.Ref.Path, vm, ref)
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
	} else {
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
	vals, err := decodeStrings(sr.Ref.Path, vm, ref)
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
	vals, err := decodeStrings(er.Ref.Path, vm, ref)
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
	vals, err := decodeStrings(rr.Ref.Path, vm, ref)
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
