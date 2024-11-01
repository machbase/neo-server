//go:build ignore

package main

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

var scanDirs = []string{
	"../internal/chart",
	"../internal/box",
	"../internal/csv",
	"../internal/json",
	"../internal/ndjson",
	"../internal/markdown",
	"../internal/geomap",
}

// add function names here not to generate
var ignores = []string{
	// "SetXYZ",
}

//go:generate go run generate.go

func main() {
	genFx := false
	if len(os.Args) == 2 && os.Args[1] == "fx" {
		genFx = true
		for i := range scanDirs {
			scanDirs[i] = "../codec/internal/" + strings.TrimPrefix(scanDirs[i], "../internal/")
		}
	}
	resultSets := map[string]*SetX{}
	resultImports := map[string]*ImportX{}
	for _, dir := range scanDirs {
		var pkgs map[string]*ast.Package
		var fileSet = token.NewFileSet()
		pkgs, err := parser.ParseDir(fileSet, dir, nil, 0)
		if err != nil {
			panic(err)
		}
		for _, pkg := range pkgs {
			if strings.HasSuffix(pkg.Name, "_test") {
				continue
			}
			for _, file := range pkg.Files {
				fileImports := map[string]*ImportX{}
				for _, imp := range file.Imports {
					path := imp.Path.Value
					name := ""
					if imp.Name != nil && imp.Name.Name != "" {
						name = imp.Name.Name
					}
					if prev, ok := fileImports[path]; ok {
						for _, n := range prev.Names {
							if name != n {
								prev.Names = append(prev.Names, n)
								break
							}
						}
					} else {
						importX := &ImportX{
							Path:  strings.Trim(imp.Path.Value, `"`),
							Names: []string{name},
						}
						fileImports[importX.Path] = importX
					}
				}
				for _, decl := range file.Decls {
					function, isFuncDecl := decl.(*ast.FuncDecl)
					if !isFuncDecl {
						continue
					}
					funcName := function.Name.Name
					if !strings.HasPrefix(funcName, "Set") {
						continue
					}
					shouldIgnore := false
					for _, ign := range ignores {
						if ign == funcName {
							shouldIgnore = true
							break
						}
					}
					if shouldIgnore {
						continue
					}
					funcType := function.Type
					if funcType.TypeParams != nil {
						continue
					}
					params := []string{}
					paramNames := []string{}
					paramTypes := []string{}
					for _, param := range funcType.Params.List {
						if ptype := strTypeExpr(param.Type, fileImports); ptype == "" {
							panic(fmt.Sprintf("**** unhandled param type  %T %s\n", ptype, fileSet.Position(param.Pos())))
						} else {
							for _, pname := range param.Names {
								params = append(params, fmt.Sprintf("%s %s", pname, ptype))
								paramNames = append(paramNames, pname.String())
								paramTypes = append(paramTypes, ptype)
							}
						}
					}
					setx := &SetX{
						Name:       function.Name.String(),
						Signature:  fmt.Sprintf("%s", strings.Join(params, ", ")),
						ParamNames: paramNames,
						ParamTypes: paramTypes,
						Sources:    []string{"mods/codec/" + strings.TrimPrefix(fileSet.Position(function.Pos()).String(), "../")},
					}
					if prev, exists := resultSets[setx.Name]; exists {
						if strings.Join(prev.ParamTypes, ",") != strings.Join(setx.ParamTypes, ",") {
							panic(fmt.Sprintf("redefined %s in %s and %s", setx.Name, prev.Sources, setx.Sources))
						} else {
							prev.Sources = append(prev.Sources, setx.Sources...)
						}
					} else {
						resultSets[setx.Name] = setx
					}
				}

				for path, x := range fileImports {
					if x.RefCount == 0 {
						continue
					}
					if imports, exists := resultImports[path]; exists {
						imports.Names = append(imports.Names, x.Names...)
						imports.RefCount += x.RefCount
					} else {
						resultImports[path] = x
					}
				}
			}
		}
	}
	if genFx {
		generatesFx(resultSets, nil)
	} else {
		generatesOpts(resultSets, resultImports)
	}
}

type SetX struct {
	Name       string
	Signature  string
	ParamNames []string
	ParamTypes []string
	Sources    []string
}

type ImportX struct {
	Path     string
	Names    []string
	RefCount int
}

func (s *SetX) String() string {
	return s.Signature
}

func findImport(name string, imports map[string]*ImportX) *ImportX {
	for _, x := range imports {
		if len(x.Names) == 1 && x.Names[0] == "" {
			if filepath.Base(x.Path) == name {
				x.RefCount++
				return x
			}
		} else {
			for _, n := range x.Names {
				if n == name {
					x.RefCount++
					return x
				}
			}
		}
	}
	return nil
}

func strTypeExpr(field ast.Expr, imports map[string]*ImportX) string {
	switch ptype := field.(type) {
	case *ast.Ellipsis:
		if sel, ok := ptype.Elt.(*ast.SelectorExpr); ok {
			name := fmt.Sprintf("%v", sel.X)
			findImport(name, imports)
			return fmt.Sprintf("...%v.%s", sel.X, sel.Sel.Name)
		} else {
			return fmt.Sprintf("...%v", ptype.Elt)
		}
	case *ast.ArrayType:
		return fmt.Sprintf("[]%s", ptype.Elt)
	case *ast.Ident:
		return ptype.Name
	case *ast.SelectorExpr:
		name := fmt.Sprintf("%v", ptype.X)
		for _, x := range imports {
			if len(x.Names) == 1 && x.Names[0] == "" {
				if filepath.Base(x.Path) == name {
					x.RefCount++
					break
				}
			} else {
				for _, n := range x.Names {
					if n == name {
						x.RefCount++
						break
					}
				}
			}
		}
		return fmt.Sprintf("%v.%s", ptype.X, ptype.Sel.Name)
	case *ast.StarExpr:
		return fmt.Sprintf("*%s", strTypeExpr(ptype.X, imports))
	case *ast.MapType:
		return fmt.Sprintf("map[%s]%s", ptype.Key, ptype.Value)
	default:
		return ""
	}
}

var EOL = "\n"

func generatesOpts(sets map[string]*SetX, imports map[string]*ImportX) {
	header := []string{
		`//go:generate go run generate.go`,
		``,
		`package opts`,
		``,
		`// Code generated by go generate; DO NOT EDIT.`,
		``,
		`import(`,
	}
	for path, x := range imports {
		if len(x.Names) > 0 {
			for _, n := range x.Names {
				header = append(header, fmt.Sprintf(`%s "%s"`, n, path))
			}
		} else {
			header = append(header, `"%s"`, path)
		}
	}
	header = append(header,
		`)`,
		``,
	)
	w := &bytes.Buffer{}
	fmt.Fprintf(w, strings.Join(header, EOL))
	orderedNames := []string{}
	for _, def := range sets {
		orderedNames = append(orderedNames, def.Name)
	}
	sort.Slice(orderedNames, func(i, j int) bool { return orderedNames[i] < orderedNames[j] })
	for _, name := range orderedNames {
		def := sets[name]
		writeFunction(w, def)
	}
	content, err := format.Source(w.Bytes())
	if err != nil {
		//fmt.Println(string(w.Bytes()))
		panic(err)
	}
	file, err := os.Create("generate.gen.go")
	if err != nil {
		panic(err)
	}
	file.Write(content)
	file.Close()
}

func writeFunction(w io.Writer, def *SetX) {
	lines := []string{
		fmt.Sprintf(`// %s`, def.Name),
	}
	sort.Slice(def.Sources, func(i, j int) bool { return def.Sources[i] < def.Sources[j] })
	for _, src := range def.Sources {
		lines = append(lines, fmt.Sprintf(`//   %s`, src))
	}
	lines = append(lines, fmt.Sprintf(`type Can%s interface {`, def.Name))
	lines = append(lines, fmt.Sprintf(`%s(%s)`, def.Name, def.Signature))
	lines = append(lines, `}`)
	lines = append(lines, fmt.Sprintf(`func %s(%s) Option {`, strings.TrimPrefix(def.Name, "Set"), def.Signature))
	lines = append(lines, `return func(_one any){`)
	lines = append(lines, fmt.Sprintf(`  if _o, ok := _one.(Can%s); ok {`, def.Name))
	params := []string{}
	for i, t := range def.ParamTypes {
		if strings.HasPrefix(t, "...") {
			params = append(params, def.ParamNames[i]+"...")
		} else {
			params = append(params, def.ParamNames[i])
		}
	}

	lines = append(lines, fmt.Sprintf(`_o.%s(%s)`, def.Name, strings.Join(params, ", ")))
	lines = append(lines, `  }`)
	lines = append(lines, "}")
	lines = append(lines, `}`)
	lines = append(lines, EOL)
	w.Write([]byte(strings.Join(lines, EOL)))
}

func generatesFx(sets map[string]*SetX, imports map[string]*ImportX) {
	header := []string{
		`//go:generate go run ../codec/opts/generate.go fx`,
		``,
		`package tql`,
		``,
		`// Code generated by go generate; DO NOT EDIT.`,
		``,
		`import(`,
		`"github.com/machbase/neo-server/mods/codec/opts"`,
	}

	for path, x := range imports {
		if len(x.Names) > 0 {
			for _, n := range x.Names {
				header = append(header, fmt.Sprintf(`%s "%s"`, n, path))
			}
		} else {
			header = append(header, `"%s"`, path)
		}
	}
	header = append(header, []string{
		`)`,
		``,
		`var CodecOptsDefinitions = []Definition {`,
		``,
	}...)

	footer := []string{
		`}`,
	}

	orderedNames := []string{}
	for _, def := range sets {
		orderedNames = append(orderedNames, def.Name)
	}
	sort.Slice(orderedNames, func(i, j int) bool { return orderedNames[i] < orderedNames[j] })
	lines := []string{
		`{Name: "httpHeader", Func: opts.HttpHeader},`,
	}
	for _, name := range orderedNames {
		x := sets[name]
		fname := strings.TrimPrefix(x.Name, "Set")
		name := strings.ToLower(fname[0:1]) + fname[1:]
		lines = append(lines, fmt.Sprintf(`{Name: "%s", Func: opts.%s},`, name, fname))
	}
	lines = append(lines, ``)
	w := &bytes.Buffer{}
	w.Write([]byte(strings.Join(header, EOL)))
	w.Write([]byte(strings.Join(lines, EOL)))
	w.Write([]byte(strings.Join(footer, EOL)))
	content, err := format.Source(w.Bytes())
	if err != nil {
		panic(err)
	}
	file, err := os.Create("./fx_codec_opts.gen.go")
	if err != nil {
		panic(err)
	}
	file.Write(content)
	file.Close()
}
