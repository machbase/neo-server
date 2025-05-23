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

	"github.com/machbase/neo-server/v8/mods/tql"
)

var EOL = "\n"

func main() {
	definitions := []tql.Definition{}
	for _, def := range tql.FxDefinitions {
		definitions = append(definitions, def)
	}
	definitions = append(definitions, tql.Definition{Name: "// codec.opts"})
	for _, def := range tql.CodecOptsDefinitions {
		definitions = append(definitions, def)
	}
	header := []string{
		`package tql`,
		``,
		`// Code generated by go generate; DO NOT EDIT.`,
		``,
		`import(`,
		`   "math"`,
		``,
		`	"github.com/machbase/neo-server/v8/api"`,
		`	"github.com/machbase/neo-server/v8/mods/tql/internal/expression"`,
		`	"github.com/machbase/neo-server/v8/mods/codec/opts"`,
		`	"github.com/machbase/neo-server/v8/mods/nums"`,
		`)`,
		``,
		`func NewNode(task *Task) *Node {`,
		`  x := &Node{task: task}`,
		`  x.functions = map[string]expression.Function {`,
		``,
	}
	footer := []string{
		`  }`,
		`  return x`,
		`}`,
		``,
	}
	w := &bytes.Buffer{}
	fmt.Fprintf(w, strings.Join(header, EOL))
	for _, def := range definitions {
		if strings.HasPrefix(def.Name, "//") {
			fmt.Fprintf(w, "%s%s", def.Name, EOL)
			continue
		}
		if expr, ok := def.Func.(string); ok {
			fmt.Fprintf(w, `	"%s": %s,%s`, def.Name, expr, EOL)
		} else {
			fmt.Fprintf(w, `	"%s": x.gen_%s,%s`, def.Name, def.Name, EOL)
		}
	}
	fmt.Fprintf(w, strings.Join(footer, EOL))

	for _, def := range definitions {
		if _, ok := def.Func.(string); ok {
			continue
		} else if strings.HasPrefix(def.Name, "//") {
			continue
		} else {
			writeMapFunc(w, def.Name, def.Func)
		}
	}

	content := w.Bytes()
	content, err := format.Source(content)
	if err != nil {
		fmt.Println(string(content))
		panic(err)
	}
	file, err := os.Create("fx_generate.gen.go")
	if err != nil {
		panic(err)
	}
	file.Write(content)
	file.Close()
}

func writeMapFunc(w io.Writer, name string, f any) {
	fv := reflect.ValueOf(f)
	fullFuncName := runtime.FuncForPC(fv.Pointer()).Name()
	realFuncName := filepath.Base(fullFuncName)
	if strings.HasPrefix(realFuncName, "tql.(*Node).") {
		realFuncName = strings.TrimPrefix(realFuncName, "tql.(*Node).")
		realFuncName = strings.TrimSuffix(realFuncName, "-fm")
		realFuncName = fmt.Sprintf("x.%s", realFuncName)
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
			if i == numParams-1 && methodType.IsVariadic() {
				typeParams = append(typeParams, fmt.Sprintf("...%s", elmType))
				typeConvFunc := getConvFunc(elmType.String(), param.Name(), realFuncName)
				convParams = append(convParams,
					fmt.Sprintf(`p%d := []%s{}`, i, elmType),
					fmt.Sprintf(`for n := %d; n < len(args); n++ {`, i),
					fmt.Sprintf(`argv, err := conv%s(args, n, "%s", "...%s")`, typeConvFunc, name, elmType),
					`if err != nil {`,
					`return nil, err`,
					`}`,
					fmt.Sprintf(`p%d = append(p%d, argv)`, i, i),
					`}`,
				)
			} else {
				typeParams = append(typeParams, fmt.Sprintf("[]%s", elmType))
				convParams = append(convParams,
					fmt.Sprintf(`p%d, ok := args[%d].([]%s)`, i, i, elmType),
					`if !ok {`,
					fmt.Sprintf(`return nil, ErrWrongTypeOfArgs("%s", %d, "[]%s", args[%d])`, name, i, elmType, i),
					`}`,
				)
			}
		} else {
			ptype := param.Name()
			strParam := param.String()
			typeParams = append(typeParams, ptype)
			convParams = append(convParams, getConvStatement(strParam, i, param.Name(), name))
			if i == 0 && strParam == "*tql.Node" {
				convParams[len(convParams)-1] = fmt.Sprintf("/* args[%d] %s */%s", i, strParam, EOL) + convParams[len(convParams)-1]
			}
		}
	}

	lines := []string{}
	lines = append(lines,
		``,
		fmt.Sprintf(`// %s`, wrapFuncName),
		`//`,
		fmt.Sprintf(`// syntax: %s(%s)`, name, strings.Join(typeParams, ", ")),
		fmt.Sprintf(`func (x *Node) %s(args ...any) (any, error) {`, wrapFuncName),
	)
	if len(typeParams) > 0 && strings.HasPrefix(typeParams[len(typeParams)-1], "...") {
		// the last parameter is variadic
		if numParams > 1 { // if func takes only variadic param, there no need to check
			lines = append(lines,
				fmt.Sprintf(`if len(args) < %d {`, numParams-1),
				fmt.Sprintf(`return nil, ErrInvalidNumOfArgs("%s", %d, len(args))`, name, numParams-1),
				`}`,
			)
		}
	} else {
		lines = append(lines,
			fmt.Sprintf(`if len(args) != %d {`, numParams),
			fmt.Sprintf(`return nil, ErrInvalidNumOfArgs("%s", %d, len(args))`, name, numParams),
			`}`,
		)
	}
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

func getConvFunc(ptype string, pname string, funcName string) string {
	switch ptype {
	case "float32":
		return "Float32"
	case "float64":
		return "Float64"
	case "string":
		return "String"
	case "int":
		return "Int"
	case "int64":
		return "Int64"
	case "bool":
		return "Bool"
	case "interface {}":
		return "Any"
	case "*nums.LatLng":
		return "LatLng"
	case "map[string]interface {}":
		return "Dictionary"
	case "api.DataType":
		return "DataType"
	default:
		panic(fmt.Sprintf("unhandled param type '%v' %s of %s\n", pname, ptype, funcName))
	}
}

func getConvStatement(ptype string, idx int, pname string, funcName string) string {
	var convFunc string
	switch ptype {
	case "uint8":
		convFunc = "Byte"
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
	case "interface {}":
		convFunc = "Any"
	case "*time.Location":
		convFunc = "TimeLocation"
	case "io.Writer":
		convFunc = "OutputStream"
	case "io.Reader":
		convFunc = "InputStream"
	case "transcoder.Transcoder":
		convFunc = "Transcoder"
	case "*tql.Node":
		convFunc = "Node"
	case "*tql.SubContext":
		convFunc = "Context"
	case "facility.Logger":
		convFunc = "Logger"
	case "facility.VolatileFileWriter":
		convFunc = "VolatileFileWriter"
	case "encoding.Encoding":
		convFunc = "Charset"
	case "*nums.LatLon":
		convFunc = "LatLon"
	case "*nums.SingleLatLng":
		convFunc = "SingleLatLng"
	case "*nums.MultiLatLng":
		convFunc = "MultiLatLng"
	case "nums.GeoMarker":
		convFunc = "GeoMarker"
	case "nums.Geography":
		convFunc = "Geography"
	case "map[string]interface {}":
		convFunc = "MapStringAny"
	default:
		panic(fmt.Sprintf("unhandled param type '%v' %s of %s\n", pname, ptype, funcName))
	}

	lines := []string{}
	if convFunc != "" {
		lines = []string{
			fmt.Sprintf(`p%d, err := conv%s(args, %d, "%s", "%s")`, idx, convFunc, idx, funcName, ptype),
			`if err != nil {`,
			`return nil, err`,
			`}`,
		}
	} else {
		return ""
	}
	return strings.Join(lines, EOL)
}
