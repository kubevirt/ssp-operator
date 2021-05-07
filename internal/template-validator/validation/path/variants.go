package path

import (
	"encoding/json"
	"fmt"
)

type IntOrPath struct {
	Int  int64
	Path *Path
}

func (r *IntOrPath) IsInt() bool {
	return r.Path == nil
}

func (r *IntOrPath) UnmarshalJSON(bytes []byte) error {
	var number float64
	err := json.Unmarshal(bytes, &number)
	if err == nil {
		if intVal, ok := toInt64(number); ok {
			r.Int = intVal
			r.Path = nil
			return nil
		}
	}

	var path Path
	err = json.Unmarshal(bytes, &path)
	if err == nil {
		r.Int = 0
		r.Path = &path
		return nil
	}

	return fmt.Errorf("cannot unmarshall IntOrPath from JSON: %w", err)
}

type StringOrPath struct {
	Str  string
	Path *Path
}

func (r *StringOrPath) IsString() bool {
	return r.Path == nil
}

func (r *StringOrPath) UnmarshalJSON(bytes []byte) error {
	var str string
	err := json.Unmarshal(bytes, &str)
	if err != nil {
		return err
	}

	if !isJSONPath(str) {
		r.Str = str
		r.Path = nil
		return nil
	}

	path, err := New(str)
	if err != nil {
		return err
	}
	r.Str = ""
	r.Path = path
	return nil
}
