//go:build ignore
// +build ignore

package main

import (
	"bytes"
	"fmt"
	"go/format"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"

	"github.com/machbase/neo-server/mods/codec/opts/defs"
	"github.com/machbase/neo-server/mods/tql/fcom"
)

var EOL = "\n"

func main() {
	header := []string{
		`//go:generate go run generate.go`,
		``,
		`package fcom`,
		``,
		`// Code generated by go generate; DO NOT EDIT.`,
		``,
		`import(`,
		`   "math"`,
		``,
		`	"github.com/machbase/neo-server/mods/expression"`,
		`	"github.com/machbase/neo-server/mods/tql/conv"`,
		`	"github.com/machbase/neo-server/mods/codec/opts"`,
		`)`,
		``,
		`var GenFunctions = map[string]expression.Function{`,
		``,
	}
	w := &bytes.Buffer{}
	fmt.Fprintf(w, strings.Join(header, EOL))
	for _, def := range fcom.Definitions {
		fmt.Fprintf(w, `	"%s": gen_%s,`+EOL, def.Name, def.Name)
	}
	for _, def := range defs.Definitions {
		fmt.Fprintf(w, `   "%s": gen_%s,`+EOL, def.Name, def.Name)
	}
	fmt.Fprintf(w, `}`+EOL)

	for _, def := range fcom.Definitions {
		writeMapFunc(w, def.Name, def.Func)
	}
	for _, def := range defs.Definitions {
		writeMapFunc(w, def.Name, def.Func)
	}

	content := w.Bytes()
	content, err := format.Source(content)
	if err != nil {
		fmt.Println(string(content))
		panic(err)
	}
	file, err := os.Create("generate.gen.go")
	if err != nil {
		panic(err)
	}
	file.Write(content)
	file.Close()
}

func writeMapFunc(w io.Writer, name string, f any) {
	fv := reflect.ValueOf(f)
	fullFuncName := runtime.FuncForPC(fv.Pointer()).Name()
	realFuncName := strings.TrimPrefix(filepath.Ext(fullFuncName), ".")
	if !strings.HasPrefix(filepath.Base(fullFuncName), "fcom.") {
		realFuncName = filepath.Base(fullFuncName)
	}
	wrapFuncName := fmt.Sprintf("gen_%s", name)

	methodType := fv.Type()
	numParams := methodType.NumIn()
	strParams := []string{}
	typeParams := []string{}
	convParams := []string{}
	for i := 0; i < numParams; i++ {
		param := methodType.In(i)
		if methodType.IsVariadic() && i == numParams-1 {
			strParams = append(strParams, fmt.Sprintf("p%d...", i))
		} else {
			strParams = append(strParams, fmt.Sprintf("p%d", i))
		}
		if param.Kind() == reflect.Slice {
			elmType := param.Elem()
			typeParams = append(typeParams, fmt.Sprintf("[]%s", elmType))
			convParams = append(convParams,
				fmt.Sprintf(`p%d, ok := args[%d].([]%s)`, i, i, elmType),
				`if !ok {`,
				fmt.Sprintf(`return nil, conv.ErrWrongTypeOfArgs("%s", %d, "[]%s", args[%d])`, name, i, elmType, i),
				`}`,
			)
		} else {
			ptype := param.Name()
			typeParams = append(typeParams, ptype)
			convFunc := ""
			convOptionalFunc := ""
			switch ptype {
			case "float32":
				convFunc = "Float32"
			case "float64":
				convFunc = "Float64"
			case "string":
				convFunc = "String"
			case "int":
				convFunc = "Int"
			case "int64":
				convFunc = "Int64"
			case "bool":
				convFunc = "Bool"
			case "OptionInt":
				convOptionalFunc = "Int"
			default:
				switch param.String() {
				case "interface {}":
					convFunc = "Any"
				case "*time.Location":
					convFunc = "TimeLocation"
				case "spec.OutputStream":
					convFunc = "OutputStream"
				case "spec.InputStream":
					convFunc = "InputStream"
				case "transcoder.Transcoder":
					convFunc = "Transcoder"
				default:
					panic(fmt.Sprintf("unhandled param type '%v' %s of %s\n", param, ptype, realFuncName))
				}
			}
			if convFunc == "" {
				convParams = append(convParams,
					fmt.Sprintf(`p%d := conv.Empty%s()`, i, convOptionalFunc),
					fmt.Sprintf(`if len(args) >= %d {`, i+1),
					fmt.Sprintf(`v, err := conv.%s(args, %d, "%s", "%s")`, convOptionalFunc, i, name, ptype),
					`if err != nil {`,
					`return nil, err`,
					`} else {`,
					fmt.Sprintf(`p%d = conv.OptionInt{Value:v}`, i),
					`}`,
					`}`,
				)
			} else {
				convParams = append(convParams,
					fmt.Sprintf(`p%d, err := conv.%s(args, %d, "%s", "%s")`, i, convFunc, i, name, ptype),
					`if err != nil {`,
					`return nil, err`,
					`}`,
				)
			}
		}
	}

	lines := []string{}
	lines = append(lines,
		``,
		fmt.Sprintf(`// %s`, wrapFuncName),
		`//`,
		fmt.Sprintf(`// syntax: %s(%s)`, name, strings.Join(typeParams, ", ")),
		fmt.Sprintf(`func %s(args ...any) (any, error) {`, wrapFuncName),
		fmt.Sprintf(`if len(args) != %d {`, numParams),
		fmt.Sprintf(`return nil, conv.ErrInvalidNumOfArgs("%s", %d, len(args))`, name, numParams),
		`}`,
	)
	lines = append(lines, convParams...)

	strCall := fmt.Sprintf(`	%s(%s)`, realFuncName, strings.Join(strParams, ","))
	numOuts := methodType.NumOut()
	if numOuts == 0 {
		lines = append(lines, strCall, `return nil, nil`)
	} else if numOuts == 1 {
		lines = append(lines, fmt.Sprintf(`ret := %s`, strCall), `return ret, nil`)
	} else if numOuts == 2 {
		lines = append(lines, fmt.Sprintf(`return %s`, strCall))
	} else {
		panic(fmt.Sprintf("function %s returns too many", name))
	}

	lines = append(lines, `}`, ``)

	fmt.Fprintf(w, strings.Join(lines, EOL))
}
