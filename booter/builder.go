package booter

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/zclconf/go-cty/cty"
	"github.com/zclconf/go-cty/cty/function"
	"github.com/zclconf/go-cty/cty/gocty"
)

type Builder interface {
	Build(definitions []*Definition) (Booter, error)
	BuildWithContent(content []byte) (Booter, error)
	BuildWithFiles(files []string) (Booter, error)
	BuildWithDir(configDir string) (Booter, error)

	AddStartupHook(hooks ...func())
	AddShutdownHook(hooks ...func())
	SetFunction(name string, f function.Function)
	SetVariable(name string, value any) error
	SetConfigFileSuffix(ext string)
}

type builder struct {
	startupHooks  []func()
	shutdownHooks []func()
	functions     map[string]function.Function
	variables     map[string]cty.Value
	fileSuffix    string
}

func NewBuilder() Builder {
	b := &builder{
		functions:  make(map[string]function.Function),
		variables:  make(map[string]cty.Value),
		fileSuffix: ".hcl",
	}
	for k, v := range DefaultFunctions {
		b.functions[k] = v
	}
	return b
}

func (bld *builder) Build(definitions []*Definition) (Booter, error) {
	b, err := NewWithDefinitions(definitions)
	if err != nil {
		return nil, err
	}
	rt := b.(*boot)
	rt.startupHooks = bld.startupHooks
	rt.shutdownHooks = bld.shutdownHooks
	return rt, nil
}

func (bld *builder) BuildWithContent(content []byte) (Booter, error) {
	definitions, err := LoadDefinitions(content, bld.makeContext())
	if err != nil {
		return nil, err
	}
	return bld.Build(definitions)
}

func (bld *builder) BuildWithFiles(files []string) (Booter, error) {
	definitions, err := LoadDefinitionFiles(files, bld.makeContext())
	if err != nil {
		return nil, err
	}
	return bld.Build(definitions)
}

func (bld *builder) BuildWithDir(configDir string) (Booter, error) {
	entries, err := os.ReadDir(configDir)
	if err != nil {
		return nil, fmt.Errorf("invalid config directory, %s", err.Error())
	}

	files := make([]string, 0)
	for _, file := range entries {
		if !strings.HasSuffix(file.Name(), bld.fileSuffix) {
			continue
		}
		files = append(files, file.Name())
	}
	sort.Slice(files, func(i, j int) bool {
		return files[i] < files[j]
	})
	result := make([]string, 0)
	for _, file := range files {
		result = append(result, filepath.Join(configDir, file))
	}
	return bld.BuildWithFiles(result)
}

func (bld *builder) AddStartupHook(hooks ...func()) {
	bld.startupHooks = append(bld.startupHooks, hooks...)
}

func (bld *builder) AddShutdownHook(hooks ...func()) {
	bld.shutdownHooks = append(bld.shutdownHooks, hooks...)
}

func (bld *builder) makeContext() *hcl.EvalContext {
	evalCtx := &hcl.EvalContext{
		Functions: bld.functions,
		Variables: bld.variables,
	}
	if evalCtx.Functions == nil {
		evalCtx.Functions = make(map[string]function.Function)
	}
	return evalCtx
}

func (bld *builder) SetFunction(name string, f function.Function) {
	bld.functions[name] = f
}

func (bld *builder) SetVariable(name string, value any) (err error) {
	if len(name) == 0 {
		return errors.New("can not define with empty name")
	}
	var v cty.Value
	switch raw := value.(type) {
	case string:
		v, err = gocty.ToCtyValue(raw, cty.String)
	case bool:
		v, err = gocty.ToCtyValue(raw, cty.Bool)
	case int, int32, int64, float32, float64:
		v, err = gocty.ToCtyValue(raw, cty.Number)
	default:
		return fmt.Errorf("can not define %s with value type %T", name, value)
	}

	if err == nil {
		bld.variables[name] = v
	}
	return
}

func (bld *builder) SetConfigFileSuffix(ext string) {
	bld.fileSuffix = ext
}
