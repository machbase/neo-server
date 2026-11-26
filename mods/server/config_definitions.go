package server

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/zclconf/go-cty/cty"
	"github.com/zclconf/go-cty/cty/function"
)

type configEvalContext struct {
	Args      []string
	Arguments []serverArgument
	Pname     string
	Version   string
}

type configModuleDefinition struct {
	Id       string
	Name     string
	Priority int
	Disabled bool
	Config   cty.Value
	Injects  []configInjection // key: target module id, value: fieldName
}

type configInjection struct {
	Target    string
	FieldName string
}

func loadServerConfigDefinitions(context configEvalContext, options serverBootOptions) ([]*configModuleDefinition, error) {
	evalContext := &hcl.EvalContext{
		Functions: configFunctions(context),
		Variables: make(map[string]cty.Value),
	}
	if options.configFile != "" {
		return loadConfigDefinitionFiles([]string{options.configFile}, evalContext)
	}
	if options.configDir == "" {
		return loadConfigDefinitions(DefaultFallbackConfig, evalContext)
	}
	entries, err := os.ReadDir(options.configDir)
	if err != nil {
		return nil, fmt.Errorf("invalid config directory: %w", err)
	}
	files := make([]string, 0, len(entries))
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".conf") {
			files = append(files, filepath.Join(options.configDir, entry.Name()))
		}
	}
	return loadConfigDefinitionFiles(files, evalContext)
}

func loadConfigDefinitionFiles(files []string, evalCtx *hcl.EvalContext) ([]*configModuleDefinition, error) {
	body, err := loadConfigFiles(files...)
	if err != nil {
		return nil, err
	}
	return parseConfigDefinitions(body, evalCtx)
}

func loadConfigDefinitions(content []byte, evalCtx *hcl.EvalContext) ([]*configModuleDefinition, error) {
	body, err := loadConfigContent(content)
	if err != nil {
		return nil, err
	}
	return parseConfigDefinitions(body, evalCtx)
}

func parseConfigDefinitions(body hcl.Body, evalCtx *hcl.EvalContext) ([]*configModuleDefinition, error) {
	if evalCtx == nil {
		evalCtx = &hcl.EvalContext{
			Functions: configFunctions(configEvalContext{Args: os.Args}),
			Variables: make(map[string]cty.Value),
		}
	} else {
		if evalCtx.Functions == nil {
			evalCtx.Functions = make(map[string]function.Function)
		}
		if evalCtx.Variables == nil {
			evalCtx.Variables = make(map[string]cty.Value)
		}
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

	priorityBase := 1000
	result := make([]*configModuleDefinition, 0)
	for i, m := range modules {
		moduleId := m.Labels[0]
		moduleName := fmt.Sprintf("$mod_%d", i+1)
		if offset := strings.LastIndex(moduleId, "/"); offset > 0 && offset < len(moduleId)-1 {
			moduleName = fmt.Sprintf("%s%02d", moduleId[offset+1:], i)
		}
		moduleDef := &configModuleDefinition{
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
			if !value.IsKnown() || value.IsNull() {
				return nil, fmt.Errorf("module %s %s must not be null or unknown", moduleId, name)
			}
			switch name {
			case "priority":
				if value.Type() != cty.Number {
					return nil, fmt.Errorf("module %s priority must be an integer", moduleId)
				}
				priority, err := int64FromCty(value)
				if err != nil || int64(int(priority)) != priority {
					return nil, fmt.Errorf("module %s priority must be an integer in range", moduleId)
				}
				moduleDef.Priority = int(priority)
			case "disabled":
				disabled, err := boolFromCty(value)
				if err != nil {
					return nil, fmt.Errorf("module %s disabled: %w", moduleId, err)
				}
				moduleDef.Disabled = disabled
			case "name":
				if value.Type() != cty.String {
					return nil, fmt.Errorf("module %s name must be a string", moduleId)
				}
				moduleDef.Name = value.AsString()
			}
		}
		if moduleDef.Disabled && moduleId != loggingModuleID && moduleId != serverModuleID {
			result = append(result, moduleDef)
			continue
		}
		for _, c := range content.Blocks {
			if c.Type == "config" {
				obj, err := configObjectFromBody(c.Body.(*hclsyntax.Body), evalCtx)
				if err != nil {
					return nil, err
				}
				moduleDef.Config = obj
			} else if c.Type == "inject" {
				target := c.Labels[0]
				fieldName := c.Labels[1]
				if target == "" {
					return nil, fmt.Errorf("module %s inject target not defined", moduleDef.Id)
				}
				if fieldName == "" {
					return nil, fmt.Errorf("module %s inject %s requires target field", moduleDef.Id, target)
				}
				if moduleDef.Injects == nil {
					moduleDef.Injects = []configInjection{}
				}
				moduleDef.Injects = append(moduleDef.Injects, configInjection{Target: target, FieldName: fieldName})
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

func loadConfigFiles(files ...string) (hcl.Body, error) {
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

func loadConfigContent(content []byte) (hcl.Body, error) {
	hclFile, hclDiag := hclsyntax.ParseConfig(content, "nofile.hcl", hcl.Pos{Line: 1})
	if hclDiag.HasErrors() {
		return nil, errors.New(hclDiag.Error())
	}
	return hclFile.Body, nil
}

func configObjectFromBody(body *hclsyntax.Body, evalCtx *hcl.EvalContext) (cty.Value, error) {
	rt := make(map[string]cty.Value)
	for _, attr := range body.Attributes {
		value, diag := attr.Expr.Value(evalCtx)
		if diag.HasErrors() {
			return cty.NilVal, errors.New(diag.Error())
		}
		rt[attr.Name] = value
	}
	for _, block := range body.Blocks {
		bval, err := configObjectFromBody(block.Body, evalCtx)
		if err != nil {
			return cty.NilVal, err
		}
		rt[block.Type] = bval
	}
	return cty.ObjectVal(rt), nil
}

func evalConfigObject(objName string, obj any, value cty.Value) error {
	ref := reflect.ValueOf(obj)
	return evalConfigValue(objName, ref, value)
}

func evalConfigValue(refName string, ref reflect.Value, value cty.Value) error {
	if value.Type() == cty.NilType || !value.IsKnown() || value.IsNull() {
		return fmt.Errorf("%s must not be null or unknown", refName)
	}
	for ref.Kind() == reflect.Pointer {
		if ref.IsNil() {
			if !ref.CanSet() {
				return fmt.Errorf("%s cannot be set", refName)
			}
			ref.Set(reflect.New(ref.Type().Elem()))
		}
		ref = ref.Elem()
	}
	if !ref.IsValid() || !ref.CanSet() {
		return fmt.Errorf("%s cannot be set", refName)
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
				err := evalConfigValue(fmt.Sprintf("%s.%s", refName, k), field, v)
				if err != nil {
					return err
				}
			}
		} else if ref.Type() == reflect.TypeOf(url.URL{}) && value.Type() == cty.String {
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
			if v, err := boolFromCty(value); err != nil {
				return fmt.Errorf("%s: %w", refName, err)
			} else {
				ref.SetBool(v)
			}
		} else {
			return fmt.Errorf("%s should be bool", refName)
		}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if value.Type() == cty.Number || value.Type() == cty.String {
			if v, err := int64FromCty(value); err != nil {
				return fmt.Errorf("%s: %w", refName, err)
			} else if ref.OverflowInt(v) {
				return fmt.Errorf("%s overflows %s", refName, ref.Type())
			} else {
				ref.SetInt(v)
			}
		} else {
			return fmt.Errorf("%s should be int", refName)
		}
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		if value.Type() == cty.Number || value.Type() == cty.String {
			if v, err := uint64FromCty(value); err != nil {
				return fmt.Errorf("%s: %w", refName, err)
			} else if ref.OverflowUint(v) {
				return fmt.Errorf("%s overflows %s", refName, ref.Type())
			} else {
				ref.SetUint(v)
			}
		} else {
			return fmt.Errorf("%s should be uint", refName)
		}
	case reflect.Float32, reflect.Float64:
		if value.Type() == cty.Number || value.Type() == cty.String {
			if v, err := float64FromCty(value); err != nil {
				return fmt.Errorf("%s: %w", refName, err)
			} else if ref.OverflowFloat(v) {
				return fmt.Errorf("%s overflows %s", refName, ref.Type())
			} else {
				ref.SetFloat(v)
			}
		} else {
			return fmt.Errorf("%s should be float", refName)
		}
	case reflect.Slice:
		if !value.Type().IsListType() && !value.Type().IsTupleType() && !value.Type().IsSetType() {
			return fmt.Errorf("%s should be a list", refName)
		}
		vs := value.AsValueSlice()
		slice := reflect.MakeSlice(ref.Type(), len(vs), len(vs))
		for i, elm := range vs {
			elmName := fmt.Sprintf("%s[%d]", refName, i)
			err := evalConfigValue(elmName, slice.Index(i), elm)
			if err != nil {
				return err
			}
		}
		ref.Set(slice)
	default:
		return fmt.Errorf("unsupported reflection %s type: %s", refName, ref.Kind())
	}
	return nil
}
