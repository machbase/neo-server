//go:build ignore

package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
)

// const pkgName = "github.com/machbase/neo-server/mods/codec/internal/echart"
const pkgName = "mods/codec/internal/echart"

func main() {
	var pkgs map[string]*ast.Package
	pkgs, err := parser.ParseDir(token.NewFileSet(), pkgName, nil, 0)
	if err != nil {
		panic(err)
	}
	for _, pkg := range pkgs {
		fmt.Println("package:", pkg.Name)
		for _, file := range pkg.Files {
			for _, decl := range file.Decls {
				if function, ok := decl.(*ast.FuncDecl); ok {
					funcName := function.Name.Name
					if strings.HasPrefix(funcName, "Set") {
						funcType := function.Type
						if funcType.TypeParams != nil {
							continue
						}
						paramList := funcType.Params.List
						params := []string{}
						for _, param := range paramList {
							pname := param.Names[0]
							switch ptype := param.Type.(type) {
							case *ast.Ellipsis:
								params = append(params, fmt.Sprintf("%v ...%v", pname, ptype.Elt))
							case *ast.Ident:
								params = append(params, fmt.Sprintf("%v %v", pname, ptype.Name))
							case *ast.SelectorExpr:
								params = append(params, fmt.Sprintf("%v %v.%s", pname, ptype.X, ptype.Sel.Name))
							default:
								panic(fmt.Sprintf("**** unhandled param type  %T\n", ptype))
							}
						}
						signature := fmt.Sprintf("%s(%s)", function.Name, strings.Join(params, ", "))
						fmt.Printf("    %s\n", signature)
					}
				}
			}
		}
	}
}
