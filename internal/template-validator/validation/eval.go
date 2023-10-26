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
	"io"
	"io/ioutil"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	k6tv1 "kubevirt.io/api/core/v1"
	"kubevirt.io/client-go/log"

	k6tobjs "kubevirt.io/ssp-operator/internal/template-validator/kubevirtjobs"
)

var (
	ErrUnrecognizedRuleType = errors.New("unrecognized Rule type")
	ErrDuplicateRuleName    = errors.New("duplicate Rule Name")
	ErrMissingRequiredKey   = errors.New("missing required key")
	ErrUnsatisfiedRule      = errors.New("rule is not satisfied")
)

type Report struct {
	Ref       *Rule
	Skipped   bool   // because not valid, with `valid` defined as per spec
	Satisfied bool   // applied rule, with this result
	Message   string // human-friendly application output (debug/troubleshooting)
	Error     error  // *internal* error
}

type Result struct {
	Status []Report
	failed bool
}

//Warn logs warnings into pod's log.
//Warnings are not included in result response.
func (r *Result) Warn(message string, e error) {
	log.Log.Warningf(fmt.Sprintf("%s: %s", message, e.Error()))
}

func (r *Result) Fail(ru *Rule, e error) {
	r.Status = append(r.Status, Report{
		Ref:   ru,
		Error: e,
	})
	// rule errors should never go unnoticed.
	// IOW, if you have a rule, you want to have it applied.
	r.failed = true
}

func (r *Result) Skip(ru *Rule) {
	r.Status = append(r.Status, Report{
		Ref:     ru,
		Skipped: true,
	})
}

func (r *Result) Applied(ru *Rule, satisfied bool, message string) {
	r.Status = append(r.Status, Report{
		Ref:       ru,
		Satisfied: satisfied,
		Message:   message,
	})

	if !satisfied {
		if ru.JustWarning {
			r.Warn(ru.Message, ErrUnsatisfiedRule)
		} else {
			r.failed = true
		}
	}
}

func (r *Result) Succeeded() bool {
	return !r.failed
}

// checks if a report needs to be translated to a StatusCause, and if so
// return the message describing the cause
func needsCause(rr *Report) (bool, string) {
	if rr.Error != nil {
		// internal errors need explanation
		return true, fmt.Sprintf("%v", rr.Error)
	}
	// rules we should check, and which failed (external errors?)
	if !rr.Skipped && !rr.Satisfied {
		return true, rr.Message
	}
	return false, ""
}

func (r *Result) ToStatusCauses() []metav1.StatusCause {
	if !r.failed {
		return nil
	}

	var causes []metav1.StatusCause
	for _, rr := range r.Status {
		if ok, message := needsCause(&rr); ok {
			causes = append(causes, metav1.StatusCause{
				Type:    metav1.CauseTypeFieldValueInvalid,
				Field:   rr.Ref.Path.Expr(),
				Message: fmt.Sprintf("%s: %s", rr.Ref.Message, message),
			})
		}
	}
	return causes
}

type Evaluator struct {
	Sink io.Writer
}

func NewEvaluator() *Evaluator {
	return &Evaluator{Sink: ioutil.Discard}
}

func validateRule(r *Rule) error {
	if !r.Rule.IsValid() {
		return ErrUnrecognizedRuleType
	}

	if r.Message == "" {
		return ErrMissingRequiredKey
	}
	return nil
}

// Evaluate applies *all* the rules (greedy evaluation) to the given VM.
// Returns a Report for each applied Rule, but ordering isn't guaranteed.
// Use Report.Ref to crosslink Reports with Rules.
// The 'bool' return value is a syntetic result, it is true if Evaluation succeeded.
// The 'error' return value signals *internal* evaluation error.
// IOW 'false' evaluation *DOES NOT* imply error != nil
func (ev *Evaluator) Evaluate(rules []Rule, vm *k6tv1.VirtualMachine) *Result {
	// We can argue that this stage is needed because the parsing layer is too poor/dumb
	// still, we need to do what we need to do.
	uniqueNames := make(map[string]struct{})
	result := Result{}

	refVm := k6tobjs.NewDefaultVirtualMachine()

	for i := range rules {
		r := &rules[i]

		// we reject let all validation fail if a rule is not well formed (e.g. syntax error)
		// to let the cluster admin quickly identify the error in the rules. Otherwise, it
		// we simply skip the malformed rule, the error can go unnoticed.
		// IOW, this is a policy decision
		if _, ok := uniqueNames[r.Name]; ok {
			fmt.Fprintf(ev.Sink, "%s failed: duplicate name\n", r.Name)
			result.Fail(r, ErrDuplicateRuleName)
			continue
		}
		uniqueNames[r.Name] = struct{}{}

		if err := validateRule(r); err != nil {
			fmt.Fprintf(ev.Sink, "%s failed: %v\n", r.Name, err)
			result.Fail(r, err)
			continue
		}

		// Specialize() may be costly, so we do this before.
		if !r.IsAppliableOn(vm) {
			// Legit case. Nothing to do or to complain.
			fmt.Fprintf(ev.Sink, "%s SKIPPED: not appliable\n", r.Name)
			result.Skip(r)
			continue
		}

		ra, err := r.Specialize(vm, refVm)
		if err != nil {
			fmt.Fprintf(ev.Sink, "%s failed: cannot specialize: %v\n", r.Name, err)
			result.Fail(r, err)
			continue
		}

		satisfied, err := ra.Apply(vm, refVm)
		if err != nil {
			fmt.Fprintf(ev.Sink, "%s failed: cannot apply: %v\n", r.Name, err)
			result.Fail(r, err)
			continue
		}

		applicationText := ra.String()
		fmt.Fprintf(ev.Sink, "%s applied: %v, %s\n", r.Name, boolAsStatus(satisfied), applicationText)
		result.Applied(r, satisfied, applicationText)
	}

	return &result
}

func boolAsStatus(val bool) string {
	if val {
		return "OK"
	} else {
		return "FAIL"
	}
}
