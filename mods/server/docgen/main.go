package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"unicode"
)

const modulePath = "github.com/machbase/neo-server/v8"

type registeredMethod struct {
	Name       string
	Handler    string
	Function   *functionInfo
	SourceFile string
}

type functionInfo struct {
	Name    string
	Doc     string
	Params  []fieldInfo
	Returns []fieldInfo
}

type fieldInfo struct {
	Name string
	Type string
	Expr ast.Expr
}

type packageInfo struct {
	Name      string
	Imports   map[string]string
	Functions map[string]*functionInfo
	Methods   map[string]*functionInfo
}

type rpcRequestEnvelope struct {
	Type    string            `json:"type"`
	Session string            `json:"session"`
	RPC     rpcRequestPayload `json:"rpc"`
}

type rpcRequestPayload struct {
	JSONRPC string `json:"jsonrpc"`
	ID      int    `json:"id"`
	Method  string `json:"method"`
	Params  []any  `json:"params"`
}

type rpcResponseEnvelope struct {
	Type    string             `json:"type"`
	Session string             `json:"session"`
	RPC     rpcResponsePayload `json:"rpc"`
}

type rpcResponsePayload struct {
	JSONRPC string `json:"jsonrpc"`
	ID      int    `json:"id"`
	Result  any    `json:"result"`
}

func main() {
	docgenDir, err := currentDir()
	if err != nil {
		panic(err)
	}
	serverDir := filepath.Clean(filepath.Join(docgenDir, ".."))
	moduleRoot := filepath.Clean(filepath.Join(serverDir, "..", ".."))

	serverPackage, err := parsePackage(serverDir)
	if err != nil {
		panic(err)
	}

	registrations, err := collectRegistrations(filepath.Join(serverDir, "server.go"), serverPackage, moduleRoot)
	if err != nil {
		panic(err)
	}

	markdown, err := renderMarkdown(registrations)
	if err != nil {
		panic(err)
	}
	if err := os.WriteFile(filepath.Join(serverDir, "..", "..", "docs", "jsonrpc.gen.md"), markdown, 0o644); err != nil {
		panic(err)
	}
}

func currentDir() (string, error) {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		return "", fmt.Errorf("failed to resolve generator path")
	}
	return filepath.Dir(filename), nil
}

func parsePackage(dir string) (*packageInfo, error) {
	fileSet := token.NewFileSet()
	packages, err := parser.ParseDir(fileSet, dir, func(info os.FileInfo) bool {
		name := info.Name()
		return strings.HasSuffix(name, ".go") && !strings.HasSuffix(name, "_test.go")
	}, parser.ParseComments)
	if err != nil {
		return nil, err
	}

	var parsedPackage *ast.Package
	for _, candidate := range packages {
		if candidate.Name == "main" {
			continue
		}
		parsedPackage = candidate
		break
	}
	if parsedPackage == nil {
		return nil, fmt.Errorf("no package found in %s", dir)
	}

	info := &packageInfo{
		Name:      parsedPackage.Name,
		Imports:   map[string]string{},
		Functions: map[string]*functionInfo{},
		Methods:   map[string]*functionInfo{},
	}

	for _, file := range parsedPackage.Files {
		for _, importSpec := range file.Imports {
			importPath := strings.Trim(importSpec.Path.Value, "\"")
			alias := importAlias(importSpec, importPath)
			if alias != "_" && alias != "." {
				info.Imports[alias] = importPath
			}
		}

		for _, declaration := range file.Decls {
			function, ok := declaration.(*ast.FuncDecl)
			if !ok {
				continue
			}
			fn := functionSignature(function)
			if function.Recv == nil {
				info.Functions[function.Name.Name] = fn
				continue
			}
			for _, receiver := range expandFields(function.Recv.List) {
				receiverType := normalizeReceiverType(receiver.Expr)
				if receiverType != "" {
					info.Methods[receiverType+"."+function.Name.Name] = fn
				}
			}
		}
	}
	return info, nil
}

func importAlias(importSpec *ast.ImportSpec, importPath string) string {
	if importSpec.Name != nil {
		return importSpec.Name.Name
	}
	return pathBase(importPath)
}

func pathBase(importPath string) string {
	importPath = strings.TrimSuffix(importPath, "/")
	idx := strings.LastIndex(importPath, "/")
	if idx < 0 {
		return importPath
	}
	return importPath[idx+1:]
}

func functionSignature(function *ast.FuncDecl) *functionInfo {
	return &functionInfo{
		Name:    function.Name.Name,
		Doc:     cleanDoc(function.Doc),
		Params:  expandFields(function.Type.Params.List),
		Returns: expandResults(function.Type.Results),
	}
}

func cleanDoc(group *ast.CommentGroup) string {
	if group == nil {
		return ""
	}
	return strings.TrimSpace(group.Text())
}

func expandResults(results *ast.FieldList) []fieldInfo {
	if results == nil {
		return nil
	}
	return expandFields(results.List)
}

func expandFields(fields []*ast.Field) []fieldInfo {
	var ret []fieldInfo
	for _, field := range fields {
		fieldType := renderExpr(field.Type)
		if len(field.Names) == 0 {
			ret = append(ret, fieldInfo{Type: fieldType, Expr: field.Type})
			continue
		}
		for _, name := range field.Names {
			ret = append(ret, fieldInfo{Name: name.Name, Type: fieldType, Expr: field.Type})
		}
	}
	return ret
}

func renderExpr(expr ast.Expr) string {
	buffer := &bytes.Buffer{}
	if err := format.Node(buffer, token.NewFileSet(), expr); err != nil {
		return "any"
	}
	return buffer.String()
}

func normalizeReceiverType(expr ast.Expr) string {
	switch typed := expr.(type) {
	case *ast.Ident:
		return typed.Name
	case *ast.StarExpr:
		return normalizeReceiverType(typed.X)
	default:
		return renderExpr(expr)
	}
}

func collectRegistrations(path string, serverPackage *packageInfo, moduleRoot string) ([]registeredMethod, error) {
	fileSet := token.NewFileSet()
	file, err := parser.ParseFile(fileSet, path, nil, parser.ParseComments)
	if err != nil {
		return nil, err
	}

	importedPackages := map[string]*packageInfo{}
	var registrations []registeredMethod
	var firstErr error

	ast.Inspect(file, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok || !isRegisterJsonRpcHandlerCall(call) || len(call.Args) < 2 {
			return true
		}
		methodLiteral, ok := call.Args[0].(*ast.BasicLit)
		if !ok {
			firstErr = fmt.Errorf("json-rpc method argument must be a string literal at %s", fileSet.Position(call.Pos()))
			return false
		}
		methodName, err := strconv.Unquote(methodLiteral.Value)
		if err != nil {
			firstErr = fmt.Errorf("unquote json-rpc method at %s: %w", fileSet.Position(methodLiteral.Pos()), err)
			return false
		}
		handlerName, function, err := resolveHandler(call.Args[1], serverPackage, importedPackages, moduleRoot)
		if err != nil {
			firstErr = fmt.Errorf("resolve handler for %s: %w", methodName, err)
			return false
		}
		registrations = append(registrations, registeredMethod{
			Name:       methodName,
			Handler:    handlerName,
			Function:   function,
			SourceFile: path,
		})
		return true
	})
	if firstErr != nil {
		return nil, firstErr
	}
	return registrations, nil
}

func isRegisterJsonRpcHandlerCall(call *ast.CallExpr) bool {
	selector, ok := call.Fun.(*ast.SelectorExpr)
	return ok && selector.Sel.Name == "RegisterJsonRpcHandler"
}

func resolveHandler(expr ast.Expr, serverPackage *packageInfo, importedPackages map[string]*packageInfo, moduleRoot string) (string, *functionInfo, error) {
	switch handler := expr.(type) {
	case *ast.Ident:
		fn := serverPackage.Functions[handler.Name]
		if fn == nil {
			return handler.Name, nil, fmt.Errorf("function %s not found", handler.Name)
		}
		return handler.Name, fn, nil
	case *ast.SelectorExpr:
		owner := renderExpr(handler.X)
		if owner == "s" {
			key := "Server." + handler.Sel.Name
			fn := serverPackage.Methods[key]
			if fn == nil {
				return key, nil, fmt.Errorf("method %s not found", key)
			}
			return key, fn, nil
		}
		importPath, ok := serverPackage.Imports[owner]
		if !ok {
			return owner + "." + handler.Sel.Name, nil, fmt.Errorf("import alias %s not found", owner)
		}
		pkg, err := loadImportedPackage(importPath, importedPackages, moduleRoot)
		if err != nil {
			return owner + "." + handler.Sel.Name, nil, err
		}
		fn := pkg.Functions[handler.Sel.Name]
		if fn == nil {
			return owner + "." + handler.Sel.Name, nil, fmt.Errorf("function %s not found in %s", handler.Sel.Name, importPath)
		}
		return owner + "." + handler.Sel.Name, fn, nil
	default:
		return renderExpr(expr), nil, fmt.Errorf("unsupported handler expression %s", renderExpr(expr))
	}
}

func loadImportedPackage(importPath string, cache map[string]*packageInfo, moduleRoot string) (*packageInfo, error) {
	if pkg := cache[importPath]; pkg != nil {
		return pkg, nil
	}
	if !strings.HasPrefix(importPath, modulePath+"/") {
		return nil, fmt.Errorf("external import %s is not supported", importPath)
	}
	dir := filepath.Join(moduleRoot, strings.TrimPrefix(importPath, modulePath+"/"))
	pkg, err := parsePackage(dir)
	if err != nil {
		return nil, err
	}
	cache[importPath] = pkg
	return pkg, nil
}

func renderMarkdown(registrations []registeredMethod) ([]byte, error) {
	buffer := &bytes.Buffer{}
	buffer.WriteString("<!-- Code generated by go generate; DO NOT EDIT. -->\n\n")
	buffer.WriteString("# Server JSON-RPC Methods\n\n")
	buffer.WriteString("This document is generated from `service.Controller.RegisterJsonRpcHandler()` registrations in `mods/server/server.go`.\n")
	buffer.WriteString("Implicit runtime parameters such as `context.Context`, `*gin.Context`, and `*WebConsole` are omitted from `params`.\n\n")

	for groupIndex, group := range groupRegistrations(registrations) {
		if groupIndex > 0 {
			buffer.WriteByte('\n')
		}
		fmt.Fprintf(buffer, "### %s\n\n", title(group.Name))
		for _, registration := range group.Methods {
			renderMethod(buffer, registration)
		}
	}
	return buffer.Bytes(), nil
}

type methodGroup struct {
	Name    string
	Methods []registeredMethod
}

func groupRegistrations(registrations []registeredMethod) []methodGroup {
	groups := []methodGroup{}
	indexes := map[string]int{}
	for _, registration := range registrations {
		name := strings.SplitN(registration.Name, ".", 2)[0]
		index, ok := indexes[name]
		if !ok {
			index = len(groups)
			indexes[name] = index
			groups = append(groups, methodGroup{Name: name})
		}
		groups[index].Methods = append(groups[index].Methods, registration)
	}
	return groups
}

func renderMethod(buffer *bytes.Buffer, registration registeredMethod) {
	params := explicitParams(registration.Function.Params)
	returns := explicitReturns(registration.Function.Returns)

	fmt.Fprintf(buffer, "#### %s\n\n", registration.Name)
	if registration.Function.Doc != "" {
		fmt.Fprintf(buffer, "%s\n\n", registration.Function.Doc)
	}
	fmt.Fprintf(buffer, "`%s(%s)`\n\n", registration.Name, strings.Join(paramNames(params), ", "))

	buffer.WriteString("*Params*\n")
	if len(params) == 0 {
		buffer.WriteString("\n- none\n")
	} else {
		for index, param := range params {
			name := param.Name
			if name == "" {
				name = fmt.Sprintf("param%d", index+1)
			}
			fmt.Fprintf(buffer, "- `%s` *%s*\n", name, jsonType(param.Expr, param.Type))
		}
	}

	buffer.WriteString("\n*Return*\n\n")
	for _, line := range returnLines(returns) {
		fmt.Fprintf(buffer, "- `%s`\n", line)
	}

	request, response := sampleMessages(registration.Name, params, returns)
	buffer.WriteByte('\n')
	writeRequestResponseJSON(buffer, request, response)
	buffer.WriteByte('\n')
}

func explicitParams(params []fieldInfo) []fieldInfo {
	ret := []fieldInfo{}
	for _, param := range params {
		if isImplicitParam(param.Type) {
			continue
		}
		ret = append(ret, param)
	}
	return ret
}

func explicitReturns(returns []fieldInfo) []fieldInfo {
	ret := []fieldInfo{}
	for _, result := range returns {
		if result.Type == "error" {
			continue
		}
		ret = append(ret, result)
	}
	return ret
}

func isImplicitParam(paramType string) bool {
	switch paramType {
	case "context.Context", "*gin.Context", "*WebConsole", "json.RawMessage":
		return true
	default:
		return false
	}
}

func paramNames(params []fieldInfo) []string {
	names := make([]string, 0, len(params))
	for index, param := range params {
		name := param.Name
		if name == "" {
			name = fmt.Sprintf("param%d", index+1)
		}
		names = append(names, name)
	}
	return names
}

func returnLines(returns []fieldInfo) []string {
	if len(returns) == 0 {
		return []string{"null|error"}
	}
	lines := make([]string, 0, len(returns))
	for _, result := range returns {
		lines = append(lines, jsonType(result.Expr, result.Type)+"|error")
	}
	return lines
}

func sampleMessages(method string, params []fieldInfo, returns []fieldInfo) (rpcRequestEnvelope, rpcResponseEnvelope) {
	sampleParams := make([]any, 0, len(params))
	for _, param := range params {
		sampleParams = append(sampleParams, sampleValue(param.Expr, param.Type))
	}
	request := rpcRequestEnvelope{
		Type:    "rpc_req",
		Session: "client-session-#1",
		RPC: rpcRequestPayload{
			JSONRPC: "2.0",
			ID:      20,
			Method:  method,
			Params:  sampleParams,
		},
	}
	response := rpcResponseEnvelope{
		Type:    "rpc_rsp",
		Session: "client-session-#1",
		RPC: rpcResponsePayload{
			JSONRPC: "2.0",
			ID:      20,
			Result:  sampleReturnValue(returns),
		},
	}
	return request, response
}

func sampleReturnValue(returns []fieldInfo) any {
	if len(returns) == 0 {
		return nil
	}
	if len(returns) == 1 {
		return sampleValue(returns[0].Expr, returns[0].Type)
	}
	values := make([]any, 0, len(returns))
	for _, result := range returns {
		values = append(values, sampleValue(result.Expr, result.Type))
	}
	return values
}

func writeJSON(buffer *bytes.Buffer, value any) {
	encoded, err := json.MarshalIndent(value, "", "    ")
	if err != nil {
		panic(err)
	}
	buffer.WriteString("```json\n")
	buffer.Write(encoded)
	buffer.WriteString("\n```\n")
}

func writeRequestResponseJSON(buffer *bytes.Buffer, request rpcRequestEnvelope, response rpcResponseEnvelope) {
	buffer.WriteString("<details>\n<summary>Request/Response JSON</summary>\n\n")
	buffer.WriteString("*Request*\n\n")
	writeJSON(buffer, request)
	buffer.WriteString("\n*Response*\n\n")
	writeJSON(buffer, response)
	buffer.WriteString("\n</details>\n")
}

func jsonType(expr ast.Expr, fallback string) string {
	switch typed := expr.(type) {
	case *ast.Ident:
		switch typed.Name {
		case "string":
			return "string"
		case "bool":
			return "bool"
		case "int", "int8", "int16", "int32", "int64", "uint", "uint8", "uint16", "uint32", "uint64", "float32", "float64":
			return typed.Name
		case "any", "interface{}":
			return "any"
		default:
			return "object<" + typed.Name + ">"
		}
	case *ast.StarExpr:
		return jsonType(typed.X, renderExpr(typed.X))
	case *ast.ArrayType:
		return "array<" + jsonType(typed.Elt, renderExpr(typed.Elt)) + ">"
	case *ast.MapType:
		return "object"
	case *ast.InterfaceType:
		return "any"
	case *ast.SelectorExpr:
		return "object<" + renderExpr(typed) + ">"
	default:
		return fallback
	}
}

func sampleValue(expr ast.Expr, fallback string) any {
	switch typed := expr.(type) {
	case *ast.Ident:
		switch typed.Name {
		case "string":
			return "string"
		case "bool":
			return false
		case "int", "int8", "int16", "int32", "int64", "uint", "uint8", "uint16", "uint32", "uint64", "float32", "float64":
			return 0
		case "any", "interface{}":
			return map[string]any{}
		default:
			return map[string]any{}
		}
	case *ast.StarExpr:
		return sampleValue(typed.X, renderExpr(typed.X))
	case *ast.ArrayType:
		return []any{}
	case *ast.MapType:
		return map[string]any{}
	case *ast.InterfaceType:
		return map[string]any{}
	case *ast.SelectorExpr:
		return map[string]any{}
	default:
		_ = fallback
		return map[string]any{}
	}
}

func title(value string) string {
	if value == "" {
		return "RPC"
	}
	parts := strings.FieldsFunc(value, func(runeValue rune) bool {
		return runeValue == '.' || runeValue == '_' || runeValue == '-'
	})
	for index, part := range parts {
		parts[index] = upperFirst(part)
	}
	return strings.Join(parts, " ")
}

func upperFirst(value string) string {
	runes := []rune(value)
	if len(runes) == 0 {
		return value
	}
	runes[0] = unicode.ToUpper(runes[0])
	return string(runes)
}
