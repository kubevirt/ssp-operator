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

package config

import (
	"kubevirt.io/cpu-nfd-plugin/pkg/file"
	"kubevirt.io/cpu-nfd-plugin/pkg/util"
)

//Config holds data about obsolete cpus and minimal baseline cpus
type Config struct {
	ObsoleteCPUs []string `yaml:"obsoleteCPUs"`
	MinCPU       string   `yaml:"minCPU"`
}

var ConfigPath = "/etc/kubernetes/node-feature-discovery/source.d/cpu-plugin-configmap.yaml"

//LoadConfig loads config yaml file with obsolete cpus and minimal baseline cpus
func LoadConfig() (Config, error) {
	config := Config{}
	err := file.GetStructureFromYamlFile(ConfigPath, &config)
	if err != nil {
		return Config{}, err
	}
	return config, nil
}

//GetObsoleteCPUMap returns map of obsolete cpus
func (c *Config) GetObsoleteCPUMap() map[string]bool {
	return util.ConvertStringSliceToMap(c.ObsoleteCPUs)
}

//GetMinCPU returns minimal baseline cpu. If minimal cpu is not defined,
//it returns for Intel vendor Penryn cpu model, for AMD it returns Opteron_G1.
func (c *Config) GetMinCPU() string {
	return c.MinCPU
}
