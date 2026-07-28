package jshext_test

import (
	"bytes"
	"html"
	"strings"
	"testing"

	"github.com/machbase/neo-server/v8/mods/util/mdconv"
	"github.com/stretchr/testify/require"
)

func TestJshExecuteRendersSourceAndOutput(t *testing.T) {
	src := strings.Join([]string{
		"# JSH example",
		"```jsh {execute=true}",
		`console.log("hello from jsh");`,
		"```",
	}, "\n")

	var out bytes.Buffer
	conv := mdconv.New()
	err := conv.ConvertString(src, &out)
	require.NoError(t, err)

	result := out.String()
	require.Contains(t, result, `class="jshext"`)
	require.Contains(t, result, `class="jshext-result"`)
	require.Contains(t, result, "hello from jsh")
	require.NotContains(t, result, "exitCode")
}

func TestJshExecuteFalseRendersAsCodeFence(t *testing.T) {
	src := strings.Join([]string{
		"```jsh {execute=false}",
		`console.log("hello");`,
		"```",
	}, "\n")

	var out bytes.Buffer
	conv := mdconv.New()
	err := conv.ConvertString(src, &out)
	require.NoError(t, err)

	result := out.String()
	require.NotContains(t, result, `class="jshext"`)
	require.Contains(t, result, "<pre")
	require.Contains(t, result, "<code")
}

func TestJshSourceSelectionRendersOnlyRequestedLines(t *testing.T) {
	src := strings.Join([]string{
		"```jsh {execute=true,source=[\"2-3\"]}",
		"console.log('line1');",
		"console.log('line2');",
		"console.log('line3');",
		"```",
	}, "\n")

	var out bytes.Buffer
	conv := mdconv.New()
	err := conv.ConvertString(src, &out)
	require.NoError(t, err)

	result := out.String()
	require.Contains(t, result, "console.log(&#39;line2&#39;);")
	require.Contains(t, result, "console.log(&#39;line3&#39;);")
	require.NotContains(t, result, "console.log(&#39;line1&#39;);")
}

func TestJshExecuteUsesRealJshRuntime(t *testing.T) {
	src := strings.Join([]string{
		"```jsh {execute=true}",
		`console.log("runtime-ok");`,
		"```",
	}, "\n")

	var out bytes.Buffer
	conv := mdconv.New()
	err := conv.ConvertString(src, &out)
	require.NoError(t, err)

	result := out.String()
	require.Contains(t, result, "runtime-ok")
	require.NotContains(t, result, "exitCode")
}

func TestJshSourceHideIsDefaultForExecuteTrue(t *testing.T) {
	src := strings.Join([]string{
		"```jsh {execute=true}",
		`console.log("hidden-source");`,
		"```",
	}, "\n")

	var out bytes.Buffer
	conv := mdconv.New()
	err := conv.ConvertString(src, &out)
	require.NoError(t, err)

	result := out.String()
	require.NotContains(t, result, `<div class="jshext-source">`)
	require.Contains(t, result, `class="jshext-result"`)
}

func TestJshExecuteFalseIgnoresSourceOption(t *testing.T) {
	src := strings.Join([]string{
		"```jsh {execute=false,source=[\"2-3\"]}",
		"console.log('line1');",
		"console.log('line2');",
		"console.log('line3');",
		"```",
	}, "\n")

	var out bytes.Buffer
	conv := mdconv.New()
	err := conv.ConvertString(src, &out)
	require.NoError(t, err)

	result := out.String()
	require.NotContains(t, result, `class="jshext"`)
	require.Contains(t, result, "line1")
	require.Contains(t, result, "line2")
	require.Contains(t, result, "line3")
}

func TestJshSourceRenderUsesCodeBlockStyle(t *testing.T) {
	src := strings.Join([]string{
		"```jsh {execute=true,source=[\"2\"]}",
		`console.log("first line");`,
		`console.log("styled-source");`,
		"```",
	}, "\n")

	var out bytes.Buffer
	conv := mdconv.New()
	err := conv.ConvertString(src, &out)
	require.NoError(t, err)

	result := out.String()
	require.Contains(t, result, `<pre class="chroma"`)
	require.Contains(t, result, `<code class="language-javascript">`)
}

func TestJshResultJsonRendersStructuredPayload(t *testing.T) {
	src := strings.Join([]string{
		"```jsh {execute=true,result=json}",
		`console.log("json-ok");`,
		"```",
	}, "\n")

	var out bytes.Buffer
	conv := mdconv.New()
	err := conv.ConvertString(src, &out)
	require.NoError(t, err)

	result := html.UnescapeString(out.String())
	require.Contains(t, result, `"stdout"`)
	require.Contains(t, result, `"exitCode": 0`)
	require.Contains(t, result, `"timedOut": false`)
}

func TestJshTimeoutRendersTimedOutExecution(t *testing.T) {
	src := strings.Join([]string{
		"```jsh {execute=true,timeout=1ms}",
		"while (true) { }",
		"```",
	}, "\n")

	var out bytes.Buffer
	conv := mdconv.New()
	err := conv.ConvertString(src, &out)
	require.NoError(t, err)

	result := out.String()
	require.Contains(t, result, "timed out")
	require.NotContains(t, result, "timedOut=true")
}
