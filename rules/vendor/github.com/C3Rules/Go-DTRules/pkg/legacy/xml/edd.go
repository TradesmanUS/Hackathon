package xml

import (
	"fmt"
	"strings"

	"github.com/C3Rules/Go-DTRules/pkg/dt"
	"github.com/C3Rules/Go-DTRules/pkg/vm"
)

type EDD struct {
	Slice[*Entity]
}

type Entity struct {
	Name    string   `xml:"name,attr"`
	Access  string   `xml:"access,attr"`
	Comment string   `xml:"comment,attr"`
	Fields  []*Field `xml:"field"`
}

type Field struct {
	Name         string `xml:"name,attr"`
	Type         string `xml:"type,attr"`
	Subtype      string `xml:"subtype,attr"`
	Access       string `xml:"access,attr"`
	Required     string `xml:"required,attr"`
	Input        string `xml:"input,attr"`
	DefaultValue string `xml:"default_value,attr"`
	Comment      string `xml:"comment,attr"`
}

func (edd EDD) Compile() (map[string]dt.EntityDefinition, error) {
	out := map[string]dt.EntityDefinition{}
	for _, e := range edd.Slice {
		out[e.Name] = dt.EntityDefinition{}
		for _, field := range e.Fields {
			var f dt.EntityDefinitionField
			var err error
			f.Type, err = convertType(field.Type)
			if err != nil {
				return nil, err
			}
			if field.DefaultValue == "" {
				f.Default = vm.Null
			} else {
				f.Default, err = vm.ParseValueString(field.DefaultValue)
				if err != nil {
					return nil, err
				}
			}
			switch strings.ToLower(field.Access) {
			case "rw", "read/write":
				f.Writable = true
			}
			switch strings.ToLower(field.Required) {
			case "yes", "true", "required":
				f.Required = true
			}
			out[e.Name][field.Name] = f
		}
	}
	return out, nil
}

func convertType(s string) (vm.Type, error) {
	switch s {
	case "entity":
		return dt.EntityType, nil
	case "integer":
		return vm.NumberType, nil
	case "string":
		return vm.StringType, nil
	case "array":
		return vm.ArrayType, nil
	case "boolean":
		return vm.BooleanType, nil
	}
	return 0, fmt.Errorf("unknown type %q", s)
}
