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

package file

import (
	"encoding/xml"
	"io/ioutil"

	yaml "gopkg.in/yaml.v2"
)

//GetStructureFromXMLFile load data from xml file and unmarshals them into given structure
//Given structure has to be pointer
func GetStructureFromXMLFile(path string, structure interface{}) error {
	rawFile, err := ioutil.ReadFile(path)
	if err != nil {
		return err
	}
	//unmarshal data into structure
	err = xml.Unmarshal(rawFile, structure)
	if err != nil {
		return err
	}
	return nil
}

//GetStructureFromYamlFile load data from yaml file and unmarshals them into given structure
//Given structure has to be pointer
func GetStructureFromYamlFile(path string, structure interface{}) error {
	rawFile, err := ioutil.ReadFile(path)
	if err != nil {
		return err
	}

	//unmarshal data into structure
	err = yaml.Unmarshal(rawFile, structure)
	if err != nil {
		return err
	}
	return nil
}
