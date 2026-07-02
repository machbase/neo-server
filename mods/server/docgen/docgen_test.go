package main

import (
	"bytes"
	"encoding/json"
	"go/ast"
	"go/parser"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// createCommentGroup is a helper to create a CommentGroup from text
func createCommentGroup(text string) *ast.CommentGroup {
	lines := strings.Split(strings.TrimSpace(text), "\n")
	comments := make([]*ast.Comment, len(lines))
	for i, line := range lines {
		comments[i] = &ast.Comment{Text: "// " + line}
	}
	return &ast.CommentGroup{List: comments}
}

// TestParseParamDocLine tests parsing of parameter documentation lines.
func TestParseParamDocLine(t *testing.T) {
	tests := []struct {
		name        string
		line        string
		expectName  string
		expectDesc  string
		expectFound bool
	}{
		{
			name:        "simple_param",
			line:        "markdown: markdown source text",
			expectName:  "markdown",
			expectDesc:  "markdown source text",
			expectFound: true,
		},
		{
			name:        "param_with_spaces",
			line:        "  darkMode  :  whether to render with dark-mode style  ",
			expectName:  "darkMode",
			expectDesc:  "whether to render with dark-mode style",
			expectFound: true,
		},
		{
			name:        "no_colon",
			line:        "invalid_param",
			expectName:  "",
			expectDesc:  "",
			expectFound: false,
		},
		{
			name:        "colon_only",
			line:        ":",
			expectName:  "",
			expectDesc:  "",
			expectFound: false,
		},
		{
			name:        "empty_name",
			line:        ": description",
			expectName:  "",
			expectDesc:  "",
			expectFound: false,
		},
		{
			name:        "empty_description",
			line:        "param:",
			expectName:  "",
			expectDesc:  "",
			expectFound: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			name, desc, found := parseParamDocLine(tt.line)
			require.Equal(t, tt.expectFound, found)
			if tt.expectFound {
				require.Equal(t, tt.expectName, name)
				require.Equal(t, tt.expectDesc, desc)
			}
		})
	}
}

// TestParseSectionItem tests parsing of section items (with "- " or "* " prefix).
func TestParseSectionItem(t *testing.T) {
	tests := []struct {
		name        string
		line        string
		expectName  string
		expectDesc  string
		expectFound bool
	}{
		{
			name:        "dash_prefix",
			line:        "- markdown: markdown source text",
			expectName:  "markdown",
			expectDesc:  "markdown source text",
			expectFound: true,
		},
		{
			name:        "star_prefix",
			line:        "* darkMode: whether to render",
			expectName:  "darkMode",
			expectDesc:  "whether to render",
			expectFound: true,
		},
		{
			name:        "no_prefix",
			line:        "referer: the referer URL",
			expectName:  "referer",
			expectDesc:  "the referer URL",
			expectFound: true,
		},
		{
			name:        "no_colon",
			line:        "- invalid_item",
			expectName:  "",
			expectDesc:  "",
			expectFound: false,
		},
		{
			name:        "empty_name",
			line:        "-  : description",
			expectName:  "",
			expectDesc:  "",
			expectFound: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			name, desc, found := parseSectionItem(tt.line)
			require.Equal(t, tt.expectFound, found)
			if tt.expectFound {
				require.Equal(t, tt.expectName, name)
				require.Equal(t, tt.expectDesc, desc)
			}
		})
	}
}

// TestSplitDocAndParamDocBasic tests basic documentation and parameter separation.
func TestSplitDocAndParamDocBasic(t *testing.T) {
	// Create a mock comment group with simple documentation
	docText := "This is a simple function.\n\nparams:\n- markdown: markdown source text\n- darkMode: whether to render with dark-mode style"
	commentGroup := createCommentGroup(docText)

	paramNames := map[string]struct{}{
		"markdown": {},
		"darkMode": {},
	}

	doc, paramDoc, returnDoc, returnFieldDoc := splitDocAndParamDoc(commentGroup, paramNames, []fieldInfo{})

	require.NotEmpty(t, doc)
	require.Equal(t, "This is a simple function.", doc)
	require.Equal(t, "markdown source text", paramDoc["markdown"])
	require.Equal(t, "whether to render with dark-mode style", paramDoc["darkMode"])
	require.Empty(t, returnDoc)
	require.Empty(t, returnFieldDoc)
}

// TestSplitDocAndParamDocMultiline tests multi-line parameter descriptions (the key fix).
func TestSplitDocAndParamDocMultiline(t *testing.T) {
	// This test validates the fix for handling multi-line parameter descriptions
	// especially for the 'referer' parameter in rpcMarkdownRender
	docText := `rpcMarkdownRender renders markdown to HTML.

params:
- markdown: markdown source text
- darkMode: whether to render with dark-mode style
- referer: the referer URL
    "http://127.0.0.1:5654/web/api/tql/sample_image.wrk" // if file has been saved
    "http://127.0.0.1:5654/web/ui" // file is not saved`

	commentGroup := createCommentGroup(docText)

	paramNames := map[string]struct{}{
		"markdown": {},
		"darkMode": {},
		"referer":  {},
	}

	doc, paramDoc, _, _ := splitDocAndParamDoc(commentGroup, paramNames, []fieldInfo{})

	// Function description should not include the referer continuation lines
	require.Equal(t, "rpcMarkdownRender renders markdown to HTML.", doc)

	// Validate parameter descriptions
	require.Equal(t, "markdown source text", paramDoc["markdown"])
	require.Equal(t, "whether to render with dark-mode style", paramDoc["darkMode"])

	// The key test: referer should include the multi-line description
	refererDesc := paramDoc["referer"]
	require.NotEmpty(t, refererDesc)
	require.Contains(t, refererDesc, "the referer URL")
	require.Contains(t, refererDesc, "http://127.0.0.1:5654/web/api/tql/sample_image.wrk")
	require.Contains(t, refererDesc, "http://127.0.0.1:5654/web/ui")
	require.Contains(t, refererDesc, "if file has been saved")
	require.Contains(t, refererDesc, "file is not saved")
}

// TestSplitDocAndParamDocWithReturn tests parameter and return section separation.
func TestSplitDocAndParamDocWithReturn(t *testing.T) {
	docText := `Example function with params and return.

params:
- name: parameter name
- value: parameter value

return: result description`

	commentGroup := createCommentGroup(docText)

	paramNames := map[string]struct{}{
		"name":  {},
		"value": {},
	}

	doc, paramDoc, returnDoc, returnFieldDoc := splitDocAndParamDoc(commentGroup, paramNames, []fieldInfo{
		{Name: "", Type: "string"},
	})

	require.Equal(t, "Example function with params and return.", doc)
	require.Equal(t, "parameter name", paramDoc["name"])
	require.Equal(t, "parameter value", paramDoc["value"])
	require.Equal(t, "result description", returnDoc["1"])
	require.Empty(t, returnFieldDoc)
}

// TestSplitDocAndParamDocNoParams tests documentation with no parameters section.
func TestSplitDocAndParamDocNoParams(t *testing.T) {
	docText := `Simple function without parameters.

This is additional documentation.`

	commentGroup := &ast.CommentGroup{
		List: []*ast.Comment{
			{Text: "// " + strings.ReplaceAll(docText, "\n", "\n// ")},
		},
	}

	paramNames := map[string]struct{}{}

	doc, paramDoc, returnDoc, returnFieldDoc := splitDocAndParamDoc(commentGroup, paramNames, []fieldInfo{})

	require.Contains(t, doc, "Simple function without parameters.")
	require.Contains(t, doc, "This is additional documentation.")
	require.Empty(t, paramDoc)
	require.Empty(t, returnDoc)
	require.Empty(t, returnFieldDoc)
}

// TestSplitDocAndParamDocBlankLineResetsSection tests that blank lines reset the current section.
func TestSplitDocAndParamDocBlankLineResetsSection(t *testing.T) {
	docText := `Function with blank line handling.

params:
- param1: first parameter

This line should be part of function doc, not param.
- param2: second parameter`

	commentGroup := createCommentGroup(docText)

	paramNames := map[string]struct{}{
		"param1": {},
		"param2": {},
	}

	doc, paramDoc, _, _ := splitDocAndParamDoc(commentGroup, paramNames, []fieldInfo{})

	// After blank line, we exit params section, so the next lines should go to doc
	require.Contains(t, doc, "This line should be part of function doc")
	require.Equal(t, "first parameter", paramDoc["param1"])
	// param2 should not be in paramDoc because it's after blank line (section reset)
	require.Empty(t, paramDoc["param2"])
}

// TestSplitDocAndParamDocEmptyComment tests handling of nil or empty comments.
func TestSplitDocAndParamDocEmptyComment(t *testing.T) {
	// Test with nil comment group
	doc, paramDoc, returnDoc, returnFieldDoc := splitDocAndParamDoc(nil, map[string]struct{}{}, []fieldInfo{})

	require.Empty(t, doc)
	require.Empty(t, paramDoc)
	require.Empty(t, returnDoc)
	require.Empty(t, returnFieldDoc)
}

func TestDocgenHelpers(t *testing.T) {
	current, err := currentDir()
	require.NoError(t, err)
	require.NotEmpty(t, current)

	require.Equal(t, "jsonrpc.gen.md", pathBase("/tmp/jsonrpc.gen.md"))
	require.Equal(t, "neo", pathBase("github.com/machbase/neo-server/v8/neo"))

	importSpec := &ast.ImportSpec{Path: &ast.BasicLit{Value: `"github.com/machbase/neo-server/v8/mods/scheduler"`}}
	require.Equal(t, "scheduler", importAlias(importSpec, "github.com/machbase/neo-server/v8/mods/scheduler"))
	importSpec.Name = &ast.Ident{Name: "sched"}
	require.Equal(t, "sched", importAlias(importSpec, "github.com/machbase/neo-server/v8/mods/scheduler"))

	require.Equal(t, "Server", normalizeReceiverType(&ast.Ident{Name: "Server"}))
	require.Equal(t, "Server", normalizeReceiverType(&ast.StarExpr{X: &ast.Ident{Name: "Server"}}))

	quoted := &ast.BasicLit{Value: "`json:\"name,omitempty\"`"}
	name, optional, skip := jsonFieldName("Name", quoted)
	require.False(t, skip)
	require.Equal(t, "name", name)
	require.True(t, optional)

	name, optional, skip = jsonFieldName("Name", &ast.BasicLit{Value: "`json:\"-\"`"})
	require.True(t, skip)
	require.False(t, optional)
	require.Empty(t, name)

	name, optional, skip = jsonFieldName("Name", nil)
	require.False(t, skip)
	require.False(t, optional)
	require.Equal(t, "Name", name)

	require.True(t, isPointerExpr(&ast.StarExpr{X: &ast.Ident{Name: "Meta"}}))
	require.False(t, isPointerExpr(&ast.Ident{Name: "Meta"}))

	require.True(t, isImplicitParam("context.Context"))
	require.True(t, isImplicitParam("json.RawMessage"))
	require.False(t, isImplicitParam("string"))

	retKey, retDesc, ok := parseReturnHeaderLine("return: result description", map[string]struct{}{}, 1, 1)
	require.True(t, ok)
	require.Equal(t, "1", retKey)
	require.Equal(t, "result description", retDesc)

	var desc string
	name, desc, ok = parseSectionItem("- field: field description")
	require.True(t, ok)
	require.Equal(t, "field", name)
	require.Equal(t, "field description", desc)

	retKey, ok = matchReturnDocKey("result", map[string]struct{}{}, 2, 1)
	require.True(t, ok)
	require.Equal(t, "1", retKey)
	retKey, ok = matchReturnDocKey("return2", map[string]struct{}{}, 3, 1)
	require.True(t, ok)
	require.Equal(t, "2", retKey)

	params := []fieldInfo{{Name: "first"}, {Name: "second"}}
	require.Equal(t, []string{"first", "second"}, paramNames(params))

	returns := []fieldInfo{{Name: "value", Type: "string"}, {Name: "err", Type: "error"}}
	filtered := explicitReturns(returns)
	require.Len(t, filtered, 1)
	require.Equal(t, "value", filtered[0].Name)
	require.True(t, isImplicitParam("context.Context"))

	ident := func(expr string) ast.Expr {
		parsed, err := parser.ParseExpr(expr)
		require.NoError(t, err)
		return parsed
	}
	require.Equal(t, "string", jsonType(ident("string"), ""))
	require.Equal(t, "bool", jsonType(ident("bool"), ""))
	require.Equal(t, "int", jsonType(ident("int"), ""))
	require.Equal(t, "any", jsonType(ident("any"), ""))
	require.Equal(t, "array<string>", jsonType(ident("[]string"), ""))
	require.Equal(t, "object", jsonType(ident("map[string]any"), ""))
	require.Equal(t, "object<example.Type>", jsonType(ident("example.Type"), ""))

	require.Equal(t, "string", sampleValue(ident("string"), ""))
}

func TestDocgenEndToEndRenderAndResolution(t *testing.T) {
	tmp := t.TempDir()
	moduleRoot := filepath.Join(tmp)
	hostDir := filepath.Join(moduleRoot, "docgentesthost")
	depDir := filepath.Join(moduleRoot, "testpkg")
	require.NoError(t, os.MkdirAll(hostDir, 0o755))
	require.NoError(t, os.MkdirAll(depDir, 0o755))

	require.NoError(t, os.WriteFile(filepath.Join(depDir, "dep.go"), []byte(`package testpkg

type Payload struct {
	Name string `+"`json:\"name\"`"+`
}

func Add(name string) error {
	return nil
}
`), 0o644))

	require.NoError(t, os.WriteFile(filepath.Join(hostDir, "host.go"), []byte(`package docgentesthost

import (
	"context"
	tp "github.com/machbase/neo-server/v8/testpkg"
)

type Meta struct {
	Count int `+"`json:\"count\"`"+`
}

type Request struct {
	Name string `+"`json:\"name\"`"+`
	Meta *Meta `+"`json:\"meta,omitempty\"`"+`
	Ext *tp.Payload `+"`json:\"ext,omitempty\"`"+`
	Tags []string `+"`json:\"tags,omitempty\"`"+`
	Opts map[string]any `+"`json:\"opts,omitempty\"`"+`
}

type Response struct {
	ID string `+"`json:\"id\"`"+`
	Meta *Meta `+"`json:\"meta,omitempty\"`"+`
}

type Server struct{}

// Handle processes a request.
//
// params:
// - req: request payload
//
// return: response payload
// - id: response identifier
func (s *Server) Handle(ctx context.Context, req Request) (Response, error) {
	return Response{ID: req.Name}, nil
}

func (s *Server) Ping(ctx context.Context) error {
	return nil
}

func register(ctl interface{ RegisterJsonRpcHandler(string, any) }, s *Server) {
	ctl.RegisterJsonRpcHandler("local.handle", s.Handle)
	ctl.RegisterJsonRpcHandler("local.ping", s.Ping)
	ctl.RegisterJsonRpcHandler("dep.add", tp.Add)
}
`), 0o644))

	serverPkg, err := parsePackage(hostDir)
	require.NoError(t, err)
	require.Equal(t, "docgentesthost", serverPkg.Name)
	require.Contains(t, serverPkg.Structs, "Request")
	require.Contains(t, serverPkg.Methods, "Server.Handle")

	depPkg, err := loadImportedPackage("github.com/machbase/neo-server/v8/testpkg", map[string]*packageInfo{}, moduleRoot)
	require.NoError(t, err)
	require.Equal(t, "testpkg", depPkg.Name)
	require.Contains(t, depPkg.Functions, "Add")

	regs, err := collectRegistrations(filepath.Join(hostDir, "host.go"), serverPkg, moduleRoot)
	require.NoError(t, err)
	require.Len(t, regs, 3)

	md, err := renderMarkdown(regs, moduleRoot)
	require.NoError(t, err)
	output := string(md)
	require.Contains(t, output, "#### local.handle")
	require.Contains(t, output, "`local.handle(req)`")
	require.Contains(t, output, "req.meta.count")
	require.Contains(t, output, "req.ext.name")
	require.Contains(t, output, "req.opts")
	require.Contains(t, output, "`id` *string*")
	require.Contains(t, output, "`meta` *object, optional*")
	require.Contains(t, output, "`meta.count` *int*")
	require.Contains(t, output, "#### dep.add")
	require.Contains(t, output, "`dep.add(name)`")
	require.Contains(t, output, "response payload")

	request, response := sampleMessages("local.handle", explicitParams(serverPkg.Methods["Server.Handle"].Params), explicitReturns(serverPkg.Methods["Server.Handle"].Returns), serverPkg, moduleRoot, map[string]*packageInfo{})
	require.Equal(t, "local.handle", request.RPC.Method)
	require.NotEmpty(t, request.RPC.Params)
	require.IsType(t, map[string]any{}, request.RPC.Params[0])
	require.NotNil(t, response.RPC.Result)

	encodedReq, err := json.Marshal(request)
	require.NoError(t, err)
	require.Contains(t, string(encodedReq), "\"meta\"")
	require.Contains(t, string(encodedReq), "\"ext\"")

	writeJSONBuffer := &bytes.Buffer{}
	writeJSON(writeJSONBuffer, map[string]any{"ok": true})
	require.Contains(t, writeJSONBuffer.String(), "```json")

	respBuf := &bytes.Buffer{}
	writeRequestResponseJSON(respBuf, request, response)
	require.Contains(t, respBuf.String(), "Request/Response JSON")

	entries := paramSchemaEntries("req", serverPkg.Methods["Server.Handle"].Params[1], serverPkg, moduleRoot, map[string]*packageInfo{})
	require.NotEmpty(t, entries)
	require.Contains(t, entries[0].Path, "req.")

	structName, st, ok := resolveStructType(serverPkg, serverPkg.Methods["Server.Handle"].Params[1].Expr, moduleRoot, map[string]*packageInfo{})
	require.True(t, ok)
	require.Contains(t, structName, "Request")
	require.NotNil(t, st)

	extFieldExpr := st.Fields[2].Expr
	importedStructName, importedStruct, importedOk := resolveStructType(serverPkg, extFieldExpr, moduleRoot, map[string]*packageInfo{})
	require.True(t, importedOk)
	require.Equal(t, "testpkg.Payload", importedStructName)
	require.NotNil(t, importedStruct)
	require.NotEmpty(t, importedStruct.Fields)
	require.Equal(t, "name", importedStruct.Fields[0].Name)

	require.True(t, isStructLikeField(serverPkg, serverPkg.Methods["Server.Handle"].Params[1].Expr, moduleRoot, map[string]*packageInfo{}))

	titleCases := map[string]string{
		"service.port.list": "Service Port List",
		"lsp":               "Lsp",
		"":                  "RPC",
	}
	for input, want := range titleCases {
		require.Equal(t, want, title(input))
	}

	require.Equal(t, "Hello", upperFirst("hello"))
	require.Equal(t, "", upperFirst(""))
}

func TestDocgenReturnRenderingHelpers(t *testing.T) {
	docs := map[string]string{
		"value": "named return",
		"1":     "first return",
		"2":     "second return",
	}
	fields := map[string][]docEntry{
		"1": {
			{Name: "id", Desc: "identifier"},
		},
		"value": {
			{Name: "count", Desc: "value count"},
		},
	}
	returns := []fieldInfo{
		{Name: "value", Type: "string"},
		{Name: "other", Type: "int"},
	}

	require.Equal(t, []string{"string|error - named return", "int|error - second return"}, returnLines(returns, docs))
	require.Equal(t, "first return", returnDocByIndexOrName(docs, 1, ""))
	require.Equal(t, "named return", returnDocByIndexOrName(docs, 1, "value"))
	require.Equal(t, "", returnDocByIndexOrName(nil, 1, "value"))

	buf := &bytes.Buffer{}
	renderReturnLines(buf, returns, docs, fields, nil, "", nil)
	output := buf.String()
	require.Contains(t, output, "`string|error - named return`")
	require.Contains(t, output, "`count`: value count")

	require.Nil(t, sampleReturnValue(nil))
	require.Equal(t, "string", sampleValue(&ast.Ident{Name: "string"}, ""))
	require.Equal(t, map[string]any{}, sampleValue(&ast.MapType{}, ""))
	request, response := sampleMessages("demo.method", []fieldInfo{{Expr: &ast.Ident{Name: "string"}, Type: "string"}}, []fieldInfo{{Expr: &ast.Ident{Name: "string"}, Type: "string"}}, nil, "", nil)
	require.Equal(t, "demo.method", request.RPC.Method)
	require.NotNil(t, response.RPC.Result)
}
