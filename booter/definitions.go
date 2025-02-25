package booter

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"reflect"
	"sort"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/zclconf/go-cty/cty"
	"github.com/zclconf/go-cty/cty/function"
)

type Definition struct {
	Id       string
	Name     string
	Priority int
	Disabled bool
	Config   cty.Value
	Injects  []InjectionDef // key: target module id, value: fieldName
}

type InjectionDef struct {
	Target    string
	FieldName string
}

func LoadDefinitionFiles(files []string, evalCtx *hcl.EvalContext) ([]*Definition, error) {
	body, err := LoadFile(files...)
	if err != nil {
		return nil, err
	}
	return ParseDefinitions(body, evalCtx)
}

func LoadDefinitions(content []byte, evalCtx *hcl.EvalContext) ([]*Definition, error) {
	body, err := Load(content)
	if err != nil {
		return nil, err
	}
	return ParseDefinitions(body, evalCtx)
}

func ParseDefinitions(body hcl.Body, evalCtx *hcl.EvalContext) ([]*Definition, error) {
	if evalCtx == nil {
		evalCtx = &hcl.EvalContext{
			Functions: DefaultFunctions,
			Variables: make(map[string]cty.Value),
		}
	} else if evalCtx.Functions == nil {
		evalCtx.Functions = make(map[string]function.Function)
	} else if evalCtx.Variables == nil {
		evalCtx.Variables = make(map[string]cty.Value)
	}

	moduleSchema := &hcl.BodySchema{
		Attributes: []hcl.AttributeSchema{},
		Blocks: []hcl.BlockHeaderSchema{
			{Type: "module", LabelNames: []string{"id"}},
			{Type: "define", LabelNames: []string{"id"}},
		},
	}
	content, diag := body.Content(moduleSchema)
	if diag.HasErrors() {
		return nil, errors.New(diag.Error())
	}

	defines := make([]*hcl.Block, 0)
	modules := make([]*hcl.Block, 0)

	for _, block := range content.Blocks {
		if block.Type == "define" {
			defines = append(defines, block)
		} else if block.Type == "module" {
			modules = append(modules, block)
		}
	}

	for _, d := range defines {
		id := d.Labels[0]
		sb := d.Body.(*hclsyntax.Body)
		for _, attr := range sb.Attributes {
			name := fmt.Sprintf("%s_%s", id, attr.Name)
			value, diag := attr.Expr.Value(evalCtx)
			if diag.HasErrors() {
				return nil, errors.New(diag.Error())
			}
			evalCtx.Variables[name] = value
		}
	}

	moduleSchema = &hcl.BodySchema{
		Attributes: []hcl.AttributeSchema{
			{Name: "priority", Required: false},
			{Name: "disabled", Required: false},
			{Name: "name", Required: false},
		},
		Blocks: []hcl.BlockHeaderSchema{
			{Type: "config", LabelNames: []string{}},
			{Type: "reference", LabelNames: []string{"refer"}},
			{Type: "inject", LabelNames: []string{"target", "field"}},
		},
	}

	// injectSchema := &hcl.BodySchema{
	// 	Attributes: []hcl.AttributeSchema{
	// 		{Name: "target", Required: true},
	// 		{Name: "field", Required: true},
	// 	},
	// }

	priorityBase := 1000
	result := make([]*Definition, 0)
	for i, m := range modules {
		moduleId := m.Labels[0]
		moduleName := fmt.Sprintf("$mod_%d", i+1)
		if offset := strings.LastIndex(moduleId, "/"); offset > 0 && offset < len(moduleId)-1 {
			moduleName = fmt.Sprintf("%s%02d", moduleId[offset+1:], i)
		}
		moduleDef := &Definition{
			Id:       moduleId,
			Name:     moduleName,
			Priority: priorityBase + i,
		}

		content, diag := m.Body.Content(moduleSchema)
		if diag.HasErrors() {
			return nil, errors.New(diag.Error())
		}
		// module attributes
		for _, attr := range content.Attributes {
			name := attr.Name
			value, diag := attr.Expr.Value(evalCtx)
			if diag.HasErrors() {
				return nil, errors.New(diag.Error())
			}
			switch name {
			case "priority":
				moduleDef.Priority = PriorityFromCty(value)
			case "disabled":
				moduleDef.Disabled, _ = BoolFromCty(value)
			case "name":
				moduleDef.Name = StringFromCty(value)
			}
		}
		for _, c := range content.Blocks {
			if c.Type == "config" {
				obj, err := ObjectValFromBody(c.Body.(*hclsyntax.Body), evalCtx)
				if err != nil {
					return nil, err
				}
				moduleDef.Config = obj
			} else if c.Type == "inject" {
				target := c.Labels[0]
				fieldName := c.Labels[1]
				// inject, diag := c.Body.Content(injectSchema)
				// if diag.HasErrors() {
				// 	return nil, errors.New(diag.Error())
				// }
				// // inject attributes
				// for _, attr := range inject.Attributes {
				// 	attrName := attr.Name
				// 	attrValue, diag := attr.Expr.Value(evalCtx)
				// 	if diag.HasErrors() {
				// 		return nil, errors.New(diag.Error())
				// 	}
				// 	switch attrName {
				// 	case "field":
				// 		fieldName = StringFromCty(attrValue)
				// 	case "target":
				// 		target = StringFromCty(attrValue)
				// 	}
				// }
				if target == "" {
					return nil, fmt.Errorf("module %s inject target not defined", moduleDef.Id)
				}
				if fieldName == "" {
					return nil, fmt.Errorf("module %s inject %s requires target field", moduleDef.Id, target)
				}
				if moduleDef.Injects == nil {
					moduleDef.Injects = []InjectionDef{}
				}
				moduleDef.Injects = append(moduleDef.Injects, InjectionDef{Target: target, FieldName: fieldName})
			} else {
				return nil, fmt.Errorf("unknown block %s", c.Type)
			}
		}
		result = append(result, moduleDef)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Priority < result[j].Priority
	})

	return result, nil
}

func LoadFile(files ...string) (hcl.Body, error) {
	hclFiles := make([]*hcl.File, 0)
	for _, file := range files {
		content, err := os.ReadFile(file)
		if err != nil {
			return nil, err
		}
		hclFile, hclDiag := hclsyntax.ParseConfig(content, file, hcl.Pos{Line: 1})
		if hclDiag.HasErrors() {
			return nil, errors.New(hclDiag.Error())
		}
		hclFiles = append(hclFiles, hclFile)
	}
	return hcl.MergeFiles(hclFiles), nil
}

func Load(content []byte) (hcl.Body, error) {
	hclFile, hclDiag := hclsyntax.ParseConfig(content, "nofile.hcl", hcl.Pos{Line: 1})
	if hclDiag.HasErrors() {
		return nil, errors.New(hclDiag.Error())
	}
	return hclFile.Body, nil
}

func ObjectValFromBody(body *hclsyntax.Body, evalCtx *hcl.EvalContext) (cty.Value, error) {
	rt := make(map[string]cty.Value)
	for _, attr := range body.Attributes {
		value, diag := attr.Expr.Value(evalCtx)
		if diag.HasErrors() {
			return cty.NilVal, errors.New(diag.Error())
		}
		rt[attr.Name] = value
	}
	for _, block := range body.Blocks {
		bval, err := ObjectValFromBody(block.Body, evalCtx)
		if err != nil {
			return cty.NilVal, err
		}
		rt[block.Type] = bval
	}
	return cty.ObjectVal(rt), nil
}

func EvalObject(objName string, obj any, value cty.Value) error {
	ref := reflect.ValueOf(obj)
	return EvalReflectValue(objName, ref, value)
}

func EvalReflectValue(refName string, ref reflect.Value, value cty.Value) error {
	if ref.Kind() == reflect.Pointer {
		ref = reflect.Indirect(ref)
	}
	switch ref.Kind() {
	case reflect.Struct:
		if value.Type().IsObjectType() {
			valmap := value.AsValueMap()
			for k, v := range valmap {
				field := ref.FieldByName(k)
				if !field.IsValid() {
					return fmt.Errorf("%s field not found in %s", k, refName)
				}
				err := EvalReflectValue(fmt.Sprintf("%s.%s", refName, k), field, v)
				if err != nil {
					return err
				}
			}
		} else if ref.Type() == reflect.TypeOf(url.URL{}) && value.Type() == cty.String {
			// string config를 url.URL struct로 변환
			v, err := url.Parse(value.AsString())
			if err != nil {
				return fmt.Errorf("%s should be url, %s", refName, err.Error())
			}
			ref.Set(reflect.ValueOf(*v))
		} else {
			return fmt.Errorf("%s should be object as %s", refName, ref.Type().Name())
		}
	case reflect.String:
		if value.Type() == cty.String {
			ref.SetString(value.AsString())
		} else {
			return fmt.Errorf("%s should be string", refName)
		}
	case reflect.Bool:
		if value.Type() == cty.Bool || value.Type() == cty.String {
			if v, err := BoolFromCty(value); err != nil {
				return err
			} else {
				ref.SetBool(v)
			}
		} else {
			return fmt.Errorf("%s should be bool", refName)
		}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if value.Type() == cty.Number || value.Type() == cty.String {
			if v, err := Int64FromCty(value); err != nil {
				return err
			} else {
				ref.SetInt(v)
			}
		} else {
			return fmt.Errorf("%s should be int", refName)
		}
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		if value.Type() == cty.Number || value.Type() == cty.String {
			if v, err := Uint64FromCty(value); err != nil {
				return err
			} else {
				ref.SetUint(v)
			}
		} else {
			return fmt.Errorf("%s should be uint", refName)
		}
	case reflect.Float32, reflect.Float64:
		if value.Type() == cty.Number || value.Type() == cty.String {
			if v, err := Float64FromCty(value); err != nil {
				return err
			} else {
				ref.SetFloat(v)
			}
		} else {
			return fmt.Errorf("%s should be float", refName)
		}
	case reflect.Slice:
		vs := value.AsValueSlice()
		slice := reflect.MakeSlice(ref.Type(), len(vs), len(vs))
		for i, elm := range vs {
			elmName := fmt.Sprintf("%s[%d]", refName, i)
			err := EvalReflectValue(elmName, slice.Index(i), elm)
			if err != nil {
				return err
			}
		}
		ref.Set(slice)
	case reflect.Map:
		vm := value.AsValueMap()
		maps := reflect.MakeMap(ref.Type())
		keyType := ref.Type().Key()
		if keyType.Kind() != reflect.String {
			panic(fmt.Errorf("unsupported map key type: %v", keyType))
		}
		valType := ref.Type().Elem()
		if valType.Kind() == reflect.Pointer {
			fmt.Printf("pointer map val type: %v", valType)
		}
		for k, v := range vm {
			val := reflect.Indirect(reflect.New(valType))
			elmName := fmt.Sprintf("%s[\"%s\"]", refName, k)
			EvalReflectValue(elmName, val, v)
			maps.SetMapIndex(reflect.ValueOf(k), val)
		}
		ref.Set(maps)
	default:
		return fmt.Errorf("unsupported reflection %s type: %s", refName, ref.Kind())
	}
	return nil
}
