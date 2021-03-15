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

	k6tv1 "kubevirt.io/client-go/api/v1"
)

const (
	MaxDisks         uint = 64
	MaxIfaces        uint = 64
	MaxPortsPerIface uint = 16
	MaxNTPServers    uint = 8
	MaxItems         int  = 64
)

// NewVirtualMachine returns a fully zero-value VirtualMachine with all optional fields
func NewDefaultVirtualMachine() *k6tv1.VirtualMachine {
	domSpec := k6tv1.DomainSpec{}
	numItems := NumItems(map[string]int{
		"Disks":      int(MaxDisks),
		"Interfaces": int(MaxIfaces),
		"Ports":      int(MaxPortsPerIface),
		"NTPServers": int(MaxNTPServers),
	})
	// caution: the reflect.Value must be addressable. You may want
	// to read carefully https://blog.golang.org/laws-of-reflection
	makeStruct(reflect.TypeOf(domSpec), reflect.ValueOf(&domSpec).Elem(), numItems)

	tmpl := k6tv1.VirtualMachineInstanceTemplateSpec{}
	tmpl.Spec.Domain = domSpec

	vm := k6tv1.VirtualMachine{}
	vm.Spec.Template = &tmpl
	k6tv1.SetObjectDefaults_VirtualMachine(&vm)
	// workaround for k6t limitation
	setVirtualMachineDefaults(&vm)
	return &vm
}

func setVirtualMachineDefaults(in *k6tv1.VirtualMachine) {
	if in.Spec.Template != nil {
		for i := range in.Spec.Template.Spec.Domain.Devices.Disks {
			a := &in.Spec.Template.Spec.Domain.Devices.Disks[i]
			if a.DiskDevice.CDRom != nil {
				setCDRomTargetDefaults(a.DiskDevice.CDRom)
			}
		}
	}
}

func setCDRomTargetDefaults(obj *k6tv1.CDRomTarget) {
	_true := true
	obj.ReadOnly = &_true
	if obj.Tray == "" {
		obj.Tray = k6tv1.TrayStateClosed
	}
}
