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

package kubevirtobjs

import (
	"reflect"
	"unicode"
)

func isStruct(k reflect.Kind) bool {
	return k == reflect.Struct
}

func isUnexported(name string) bool {
	return unicode.IsLower(rune(name[0]))
}

type NumItems map[string]int

func (ni NumItems) ForField(name string) int {
	num, ok := ni[name]
	if !ok {
		return MaxItems
	}
	return num
}

func makeStruct(t reflect.Type, v reflect.Value, numItems NumItems) {
	for i := 0; i < v.NumField(); i++ {
		ft := t.Field(i)
		if isUnexported(ft.Name) {
			continue
		}

		f := v.Field(i)
		switch ft.Type.Kind() {
		case reflect.Map:
			f.Set(reflect.MakeMap(ft.Type))
		case reflect.Slice:
			num := numItems.ForField(ft.Name)
			f.Set(reflect.MakeSlice(ft.Type, num, num))
			// caution: check the type of the *items* of the slice,
			// not the slice itself
			if isStruct(ft.Type.Elem().Kind()) {
				for i := 0; i < num; i++ {
					makeStruct(ft.Type.Elem(), f.Index(i), numItems)
				}
			}
		case reflect.Chan:
			f.Set(reflect.MakeChan(ft.Type, 0))
		case reflect.Ptr:
			// caution: create a pointeD type, not a pointeR type.
			fv := reflect.New(ft.Type.Elem())
			if isStruct(fv.Elem().Type().Kind()) {
				makeStruct(ft.Type.Elem(), fv.Elem(), numItems)
			}
			f.Set(fv)
		case reflect.Struct:
			makeStruct(ft.Type, f, numItems)
		default:
			// nothing to do here
		}
	}
}
