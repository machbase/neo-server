package main

import (
	"go/ast"
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
