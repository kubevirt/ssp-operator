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
	"fmt"
	"reflect"
	"strings"

	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/client-go/util/jsonpath"

	k6tv1 "kubevirt.io/client-go/api/v1"
)

var (
	ErrInvalidJSONPath = fmt.Errorf("invalid JSONPath")
)

const (
	JSONPathPrefix string = "jsonpath::"
)

func isJSONPath(s string) bool {
	return strings.HasPrefix(s, JSONPathPrefix)
}

type Path struct {
	jp      *jsonpath.JSONPath
	results [][]reflect.Value
}

func TrimJSONPath(path string) string {
	s := strings.TrimPrefix(path, JSONPathPrefix)
	// we always need to interpret the user-supplied path as relative path
	return strings.TrimPrefix(s, "$")
}

func NewJSONPathFromString(path string) (string, error) {
	if !isJSONPath(path) {
		return "", ErrInvalidJSONPath
	}
	expr := TrimJSONPath(path)
	return fmt.Sprintf("{.spec.template%s}", expr), nil
}

func NewPath(expr string) (*Path, error) {
	var err error
	pathExpr, err := NewJSONPathFromString(expr)
	if err != nil {
		return nil, err
	}

	jp := jsonpath.New(expr) // we don't really care about the name
	err = jp.Parse(pathExpr)
	if err != nil {
		return nil, err
	}
	return &Path{jp: jp}, nil
}

func (p *Path) Find(vm *k6tv1.VirtualMachine) error {
	var err error
	p.results, err = p.jp.FindResults(vm)
	if err != nil {
		return ErrInvalidJSONPath
	}
	return nil
}

func (p *Path) Len() int {
	totalCount := 0
	for _, result := range p.results {
		totalCount += len(result)
	}
	return totalCount
}

func (p *Path) AsString() ([]string, error) {
	var ret []string
	for i := range p.results {
		res := p.results[i]
		for j := range res {
			obj := res[j].Interface()
			strObj, ok := obj.(string)
			if ok {
				ret = append(ret, strObj)
				continue
			}
			return nil, fmt.Errorf("mismatching type: %v, not string", res[j].Type().Name())
		}
	}
	return ret, nil
}

func (p *Path) AsInt64() ([]int64, error) {
	var ret []int64
	for i := range p.results {
		res := p.results[i]
		for j := range res {
			obj := res[j].Interface()
			if intObj, ok := toInt64(obj); ok {
				ret = append(ret, intObj)
				continue
			}
			if quantityObj, ok := obj.(resource.Quantity); ok {
				v, ok := quantityObj.AsInt64()
				if ok {
					ret = append(ret, v)
					continue
				}
			}
			return nil, fmt.Errorf("mismatching type: %v, not int or resource.Quantity", res[j].Type().Name())
		}
	}
	return ret, nil
}
