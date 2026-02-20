package log

import (
	"bytes"
	"fmt"
	"log/slog"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/dop251/goja"
)

func TestPrint(t *testing.T) {
	buf := &bytes.Buffer{}
	defaultWriter = buf

	Print("hello", "world")
	output := buf.String()
	if output != "helloworld" {
		t.Errorf("expected 'helloworld', got '%s'", output)
	}
}

func TestPrintln(t *testing.T) {
	buf := &bytes.Buffer{}
	defaultWriter = buf

	Println("hello", "world")
	output := buf.String()
	if output != "hello world\n" {
		t.Errorf("expected 'hello world\\n', got '%s'", output)
	}
}

func TestLog(t *testing.T) {
	tests := []struct {
		name     string
		level    slog.Level
		args     []interface{}
		contains string
	}{
		{
			name:     "INFO level",
			level:    slog.LevelInfo,
			args:     []interface{}{"test message"},
			contains: "INFO",
		},
		{
			name:     "DEBUG level",
			level:    slog.LevelDebug,
			args:     []interface{}{"debug message"},
			contains: "DEBUG",
		},
		{
			name:     "WARN level",
			level:    slog.LevelWarn,
			args:     []interface{}{"warning message"},
			contains: "WARN",
		},
		{
			name:     "ERROR level",
			level:    slog.LevelError,
			args:     []interface{}{"error message"},
			contains: "ERROR",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := &bytes.Buffer{}
			defaultWriter = buf

			Log(tt.level, tt.args...)
			output := buf.String()
			if !strings.Contains(output, tt.contains) {
				t.Errorf("expected output to contain '%s', got '%s'", tt.contains, output)
			}
		})
	}
}

func TestSetConsole(t *testing.T) {
	vm := goja.New()
	buf := &bytes.Buffer{}

	con := SetConsole(vm, buf)
	if con == nil {
		t.Fatal("SetConsole returned nil")
	}

	// Test that console methods are set
	methods := []string{"log", "debug", "info", "warn", "error", "println", "print"}
	for _, method := range methods {
		if con.Get(method) == nil {
			t.Errorf("console.%s is not set", method)
		}
	}
}

func TestConsoleLog(t *testing.T) {
	vm := goja.New()
	buf := &bytes.Buffer{}

	con := SetConsole(vm, buf)
	vm.Set("console", con)

	_, err := vm.RunString(`console.log("test message")`)
	if err != nil {
		t.Fatalf("failed to run console.log: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "INFO") || !strings.Contains(output, "test message") {
		t.Errorf("expected output to contain 'INFO' and 'test message', got '%s'", output)
	}
}

func TestConsoleDebug(t *testing.T) {
	vm := goja.New()
	buf := &bytes.Buffer{}

	con := SetConsole(vm, buf)
	vm.Set("console", con)

	_, err := vm.RunString(`console.debug("debug message")`)
	if err != nil {
		t.Fatalf("failed to run console.debug: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "DEBUG") || !strings.Contains(output, "debug message") {
		t.Errorf("expected output to contain 'DEBUG' and 'debug message', got '%s'", output)
	}
}

func TestConsoleWarn(t *testing.T) {
	vm := goja.New()
	buf := &bytes.Buffer{}

	con := SetConsole(vm, buf)
	vm.Set("console", con)

	_, err := vm.RunString(`console.warn("warning message")`)
	if err != nil {
		t.Fatalf("failed to run console.warn: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "WARN") || !strings.Contains(output, "warning message") {
		t.Errorf("expected output to contain 'WARN' and 'warning message', got '%s'", output)
	}
}

func TestConsoleError(t *testing.T) {
	vm := goja.New()
	buf := &bytes.Buffer{}

	con := SetConsole(vm, buf)
	vm.Set("console", con)

	_, err := vm.RunString(`console.error("error message")`)
	if err != nil {
		t.Fatalf("failed to run console.error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "ERROR") || !strings.Contains(output, "error message") {
		t.Errorf("expected output to contain 'ERROR' and 'error message', got '%s'", output)
	}
}

func TestConsolePrint(t *testing.T) {
	vm := goja.New()
	buf := &bytes.Buffer{}

	con := SetConsole(vm, buf)
	vm.Set("console", con)

	_, err := vm.RunString(`console.print("hello", "world")`)
	if err != nil {
		t.Fatalf("failed to run console.print: %v", err)
	}

	output := buf.String()
	if output != "helloworld" {
		t.Errorf("expected 'helloworld', got '%s'", output)
	}
}

func TestConsolePrintln(t *testing.T) {
	vm := goja.New()
	buf := &bytes.Buffer{}

	con := SetConsole(vm, buf)
	vm.Set("console", con)

	_, err := vm.RunString(`console.println("hello", "world")`)
	if err != nil {
		t.Fatalf("failed to run console.println: %v", err)
	}

	output := buf.String()
	if output != "hello world\n" {
		t.Errorf("expected 'hello world\\n', got '%s'", output)
	}
}

func TestValueToPrintable(t *testing.T) {
	vm := goja.New()

	tests := []struct {
		name     string
		script   string
		expected any
	}{
		{
			name:     "simple string",
			script:   `"hello"`,
			expected: "hello",
		},
		{
			name:     "number",
			script:   `42`,
			expected: int64(42),
		},
		{
			name:     "object",
			script:   `({a: 1, b: 2})`,
			expected: "{a:1, b:2}",
		},
		{
			name:     "array",
			script:   `[1,2,3]`,
			expected: "[1, 2, 3]",
		},
		{
			name:     "nested array",
			script:   `[[1,2],[3,4]]`,
			expected: "[[1, 2], [3, 4]]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			val, err := vm.RunString(tt.script)
			if err != nil {
				t.Fatalf("failed to run script: %v", err)
			}

			result := valueToPrintable(val)
			if result != tt.expected {
				t.Errorf("expected result '%s(%T)', got '%s(%T)'", tt.expected, tt.expected, result, result)
			}
		})
	}
}

func TestConsoleLogMultipleArgs(t *testing.T) {
	vm := goja.New()
	buf := &bytes.Buffer{}

	con := SetConsole(vm, buf)
	vm.Set("console", con)

	_, err := vm.RunString(`console.log("arg1", "arg2", "arg3")`)
	if err != nil {
		t.Fatalf("failed to run console.log with multiple args: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "arg1") || !strings.Contains(output, "arg2") || !strings.Contains(output, "arg3") {
		t.Errorf("expected output to contain all args, got '%s'", output)
	}
}

func TestConsoleLogObject(t *testing.T) {
	vm := goja.New()
	buf := &bytes.Buffer{}

	con := SetConsole(vm, buf)
	vm.Set("console", con)

	_, err := vm.RunString(`console.log({name: "test", value: 123})`)
	if err != nil {
		t.Fatalf("failed to run console.log with object: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "name") || !strings.Contains(output, "test") {
		t.Errorf("expected output to contain object representation, got '%s'", output)
	}
}

func TestConsoleLogArray(t *testing.T) {
	vm := goja.New()
	buf := &bytes.Buffer{}

	con := SetConsole(vm, buf)
	vm.Set("console", con)

	_, err := vm.RunString(`console.log([1, 2, 3])`)
	if err != nil {
		t.Fatalf("failed to run console.log with array: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "[") && !strings.Contains(output, "1") {
		t.Errorf("expected output to contain array representation, got '%s'", output)
	}
}

func TestDefaultWriter(t *testing.T) {
	// Save original defaultWriter
	originalWriter := defaultWriter
	defer func() { defaultWriter = originalWriter }()

	// Test that defaultWriter can be changed
	buf1 := &bytes.Buffer{}
	defaultWriter = buf1
	Print("test1")

	if buf1.String() != "test1" {
		t.Errorf("expected 'test1', got '%s'", buf1.String())
	}

	// Change writer again
	buf2 := &bytes.Buffer{}
	defaultWriter = buf2
	Print("test2")

	if buf2.String() != "test2" {
		t.Errorf("expected 'test2', got '%s'", buf2.String())
	}

	// buf1 should not have changed
	if buf1.String() != "test1" {
		t.Errorf("expected buf1 to still be 'test1', got '%s'", buf1.String())
	}
}

func TestPrintf(t *testing.T) {
	buf := &bytes.Buffer{}
	defaultWriter = buf

	Printf("Hello %s, number: %d", "World", 42)
	output := buf.String()
	expected := "Hello World, number: 42"
	if output != expected {
		t.Errorf("expected '%s', got '%s'", expected, output)
	}
}

func TestConsolePrintf(t *testing.T) {
	vm := goja.New()
	buf := &bytes.Buffer{}

	con := SetConsole(vm, buf)
	vm.Set("console", con)

	tests := []struct {
		name     string
		script   string
		expected string
	}{
		{
			name:     "string formatting",
			script:   `console.printf("Hello %s", "World")`,
			expected: "Hello World",
		},
		{
			name:     "number formatting",
			script:   `console.printf("Number: %d", 42)`,
			expected: "Number: 42",
		},
		{
			name:     "multiple args",
			script:   `console.printf("%s: %d", "count", 5)`,
			expected: "count: 5",
		},
		{
			name:     "no args",
			script:   `console.printf()`,
			expected: "",
		},
		{
			name:     "format only",
			script:   `console.printf("no placeholders")`,
			expected: "no placeholders",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf.Reset()
			_, err := vm.RunString(tt.script)
			if err != nil {
				t.Fatalf("failed to run console.printf: %v", err)
			}

			output := buf.String()
			if output != tt.expected {
				t.Errorf("expected '%s', got '%s'", tt.expected, output)
			}
		})
	}
}

func TestAnyToPrintableTypes(t *testing.T) {
	vm := goja.New()

	tests := []struct {
		name     string
		script   string
		contains string
	}{
		{
			name:     "null",
			script:   `null`,
			contains: "null",
		},
		{
			name:     "boolean true",
			script:   `true`,
			contains: "true",
		},
		{
			name:     "boolean false",
			script:   `false`,
			contains: "false",
		},
		{
			name:     "float",
			script:   `3.14`,
			contains: "3.14",
		},
		{
			name:     "string",
			script:   `"test string"`,
			contains: "test string",
		},
		{
			name:     "byte array",
			script:   `new Uint8Array([72, 101, 108, 108, 111])`,
			contains: "Hello",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := &bytes.Buffer{}
			con := SetConsole(vm, buf)
			vm.Set("console", con)

			script := "console.log(" + tt.script + ")"
			_, err := vm.RunString(script)
			if err != nil {
				t.Fatalf("failed to run script: %v", err)
			}

			output := buf.String()
			if !strings.Contains(output, tt.contains) {
				t.Errorf("expected output to contain '%s', got '%s'", tt.contains, output)
			}
		})
	}
}

func TestAnyToPrintableGojaObject(t *testing.T) {
	vm := goja.New()

	// Test goja.Object with custom toString
	val, err := vm.RunString(`({ toString: function() { return "custom string"; } })`)
	if err != nil {
		t.Fatalf("failed to create object: %v", err)
	}

	obj := val.ToObject(vm)
	result := anyToPrintable(obj)

	// The toString method should be called
	if !strings.Contains(result.(goja.Value).String(), "custom string") {
		t.Errorf("expected result to contain 'custom string', got '%v'", result)
	}
}

func TestAnyToPrintableTimeType(t *testing.T) {
	buf := &bytes.Buffer{}
	defaultWriter = buf

	// Test with Go time.Time directly
	now := time.Now()
	result := anyToPrintable(now)
	expected := now.Local().Format(time.DateTime)
	if result != expected {
		t.Errorf("expected time format '%s', got '%s'", expected, result)
	}
}

func TestAnyToPrintableFloatArrays(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected string
	}{
		{
			name:     "float64 slice",
			input:    []float64{1.5, 2.5, 3.5},
			expected: "[1.5, 2.5, 3.5]",
		},
		{
			name:     "2D float64 slice",
			input:    [][]float64{{1.0, 2.0}, {3.0, 4.0}},
			expected: "[[1, 2], [3, 4]]", // %v formats 1.0 as 1
		},
		{
			name:     "empty float64 slice",
			input:    []float64{},
			expected: "[]",
		},
		{
			name:     "empty 2D float64 slice",
			input:    [][]float64{},
			expected: "[]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := anyToPrintable(tt.input)
			if result != tt.expected {
				t.Errorf("expected '%s', got '%s'", tt.expected, result)
			}
		})
	}
}

func TestAnyToPrintableComplexObjects(t *testing.T) {
	vm := goja.New()

	tests := []struct {
		name     string
		script   string
		contains []string
	}{
		{
			name:     "nested object",
			script:   `({outer: {inner: "value"}})`,
			contains: []string{"outer", "inner", "value"},
		},
		{
			name:     "object with null",
			script:   `({key: null})`,
			contains: []string{"key", "null"},
		},
		{
			name:     "mixed type array",
			script:   `[1, "string", true, null]`,
			contains: []string{"1", "string", "true", "null"},
		},
		{
			name:     "empty object",
			script:   `({})`,
			contains: []string{"{}"},
		},
		{
			name:     "empty array",
			script:   `[]`,
			contains: []string{"[]"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := &bytes.Buffer{}
			con := SetConsole(vm, buf)
			vm.Set("console", con)

			script := "console.log(" + tt.script + ")"
			_, err := vm.RunString(script)
			if err != nil {
				t.Fatalf("failed to run script: %v", err)
			}

			output := buf.String()
			for _, expected := range tt.contains {
				if !strings.Contains(output, expected) {
					t.Errorf("expected output to contain '%s', got '%s'", expected, output)
				}
			}
		})
	}
}

func TestConsoleInfo(t *testing.T) {
	vm := goja.New()
	buf := &bytes.Buffer{}

	con := SetConsole(vm, buf)
	vm.Set("console", con)

	_, err := vm.RunString(`console.info("info message")`)
	if err != nil {
		t.Fatalf("failed to run console.info: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "INFO") || !strings.Contains(output, "info message") {
		t.Errorf("expected output to contain 'INFO' and 'info message', got '%s'", output)
	}
}

func TestAnyToPrintableDefaultCase(t *testing.T) {
	// Test with a type that doesn't match any specific case
	type CustomType struct {
		Value string
	}

	custom := CustomType{Value: "test"}
	result := anyToPrintable(custom)

	resultStr := fmt.Sprintf("%v", result)
	if !strings.Contains(resultStr, "CustomType") {
		t.Errorf("expected result to contain type name 'CustomType', got '%s'", resultStr)
	}
}

func TestAnyToPrintablePointerTypes(t *testing.T) {
	// Test pointer types that are not covered
	strVal := "test string"
	intVal := 42
	int32Val := int32(32)
	int64Val := int64(64)
	floatVal := 3.14

	tests := []struct {
		name     string
		input    interface{}
		expected string
	}{
		{
			name:     "string pointer",
			input:    &strVal,
			expected: "test string",
		},
		{
			name:     "int pointer",
			input:    &intVal,
			expected: "42",
		},
		{
			name:     "int32 pointer",
			input:    &int32Val,
			expected: "32",
		},
		{
			name:     "int64 pointer",
			input:    &int64Val,
			expected: "64",
		},
		{
			name:     "float64 pointer",
			input:    &floatVal,
			expected: "3.14",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := anyToPrintable(tt.input)
			resultStr := fmt.Sprintf("%v", result)
			if !strings.Contains(resultStr, tt.expected) {
				t.Errorf("expected result to contain '%s', got '%s'", tt.expected, resultStr)
			}
		})
	}
}

func TestAnyToPrintableTimePointer(t *testing.T) {
	// Test time.Time pointer
	now := time.Now()
	result := anyToPrintable(&now)
	expected := now.Local().Format(time.DateTime)
	if result != expected {
		t.Errorf("expected time format '%s', got '%s'", expected, result)
	}
}

func TestAnyToPrintableDuration(t *testing.T) {
	// Test time.Duration
	duration := 5 * time.Second
	result := anyToPrintable(duration)
	expected := "5s"
	if result != expected {
		t.Errorf("expected duration '%s', got '%s'", expected, result)
	}
}

func TestAnyToPrintableURLTypes(t *testing.T) {
	url1, _ := url.Parse("https://example.com/path1")
	url2, _ := url.Parse("https://example.com/path2")

	tests := []struct {
		name     string
		input    interface{}
		expected string
	}{
		{
			name:     "single URL",
			input:    url1,
			expected: "https://example.com/path1",
		},
		{
			name:     "URL slice",
			input:    []*url.URL{url1, url2},
			expected: "[https://example.com/path1, https://example.com/path2]",
		},
		{
			name:     "URL slice pointer",
			input:    &[]*url.URL{url1, url2},
			expected: "[https://example.com/path1, https://example.com/path2]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := anyToPrintable(tt.input)
			resultStr := fmt.Sprintf("%v", result)
			if resultStr != tt.expected {
				t.Errorf("expected '%s', got '%s'", tt.expected, resultStr)
			}
		})
	}
}

func TestAnyToPrintableInt32AndInt64(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected string
	}{
		{
			name:     "int32",
			input:    int32(32),
			expected: "32",
		},
		{
			name:     "int64",
			input:    int64(64),
			expected: "64",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := anyToPrintable(tt.input)
			resultStr := fmt.Sprintf("%v", result)
			if resultStr != tt.expected {
				t.Errorf("expected '%s', got '%s'", tt.expected, resultStr)
			}
		})
	}
}

func TestAnyToPrintableByteArray(t *testing.T) {
	// Test []byte
	bytes := []byte("Hello, World!")
	result := anyToPrintable(bytes)
	expected := "Hello, World!"
	if result != expected {
		t.Errorf("expected '%s', got '%s'", expected, result)
	}
}

func TestAnyToPrintableEmptyByteArray(t *testing.T) {
	// Test empty []byte
	bytes := []byte{}
	result := anyToPrintable(bytes)
	expected := ""
	if result != expected {
		t.Errorf("expected empty string, got '%s'", result)
	}
}

func TestAnyToPrintableStringSlice(t *testing.T) {
	// Test []string
	strSlice := []string{"apple", "banana", "cherry"}
	result := anyToPrintable(strSlice)
	expected := "[apple, banana, cherry]"
	if result != expected {
		t.Errorf("expected '%s', got '%s'", expected, result)
	}
}

func TestAnyToPrintableEmptyStringSlice(t *testing.T) {
	// Test empty []string
	strSlice := []string{}
	result := anyToPrintable(strSlice)
	expected := "[]"
	if result != expected {
		t.Errorf("expected '[]', got '%s'", result)
	}
}

func TestAnyToPrintableMapStringAny(t *testing.T) {
	// Test map[string]any with sorted keys
	m := map[string]any{
		"zebra": "last",
		"apple": 1,
		"mango": true,
	}
	result := anyToPrintable(m)
	// Keys should be sorted alphabetically
	expected := "{apple:1, mango:true, zebra:last}"
	if result != expected {
		t.Errorf("expected '%s', got '%s'", expected, result)
	}
}

func TestAnyToPrintableEmptyMap(t *testing.T) {
	// Test empty map[string]any
	m := map[string]any{}
	result := anyToPrintable(m)
	expected := "{}"
	if result != expected {
		t.Errorf("expected '{}', got '%s'", result)
	}
}

func TestAnyToPrintableNestedMap(t *testing.T) {
	// Test nested map
	m := map[string]any{
		"outer": map[string]any{
			"inner": "value",
		},
	}
	result := anyToPrintable(m)
	// Should handle nested maps
	if !strings.Contains(result.(string), "outer") || !strings.Contains(result.(string), "inner") {
		t.Errorf("expected nested map representation, got '%s'", result)
	}
}

func TestConsolePrintfWithObjects(t *testing.T) {
	vm := goja.New()
	buf := &bytes.Buffer{}

	con := SetConsole(vm, buf)
	vm.Set("console", con)

	// Test printf with objects that need conversion
	_, err := vm.RunString(`console.printf("Object: %v", {key: "value"})`)
	if err != nil {
		t.Fatalf("failed to run console.printf with object: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "key") {
		t.Errorf("expected output to contain 'key', got '%s'", output)
	}
}

func TestConsolePrintfWithArrays(t *testing.T) {
	vm := goja.New()
	buf := &bytes.Buffer{}

	con := SetConsole(vm, buf)
	vm.Set("console", con)

	// Test printf with arrays
	_, err := vm.RunString(`console.printf("Array: %v", [1, 2, 3])`)
	if err != nil {
		t.Fatalf("failed to run console.printf with array: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "1") {
		t.Errorf("expected output to contain array elements, got '%s'", output)
	}
}
