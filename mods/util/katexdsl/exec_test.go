package katexdsl

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
)

func ExampleRender() {
	b := bytes.Buffer{}
	Render(&b, []byte(`Y = A \dot X^2 + B \dot X + C`), false, false)
	fmt.Println(strings.Contains(b.String(), `<math xmlns="http://www.w3.org/1998/Math/MathML">`))

	// Output:
	// true
}

func BenchmarkRender(b *testing.B) {
	for n := 0; n < b.N; n++ {
		buf := bytes.Buffer{}
		Render(&buf, []byte(`Y = A \dot X^2 + B \dot X + C`), false, false)
	}
}

func TestRenderError(t *testing.T) {
	// with throwOnError = true
	buf := bytes.Buffer{}
	err := Render(&buf, []byte(`\invalidcommand`), false, true)
	if err == nil {
		t.Error("Expected error for invalid KaTeX with throwOnError=true, got nil")
	}

	// with throwOnError = false
	buf.Reset()
	err = Render(&buf, []byte(`\invalidcommand`), false, false)
	if err != nil {
		t.Errorf("Expected no error for invalid KaTeX with throwOnError=false, got: %v", err)
	}
	if !bytes.Contains(buf.Bytes(), []byte(`<math xmlns="http://www.w3.org/1998/Math/MathML">`)) {
		t.Error("Couldn't find MathML output when rendering invalid KaTeX with throwOnError=false.")
	}
}

func TestRenderOptionAndRenderWithOption(t *testing.T) {
	buf := bytes.Buffer{}
	err := RenderOption(&buf, []byte(`x+y`), Option{DisplayMode: true, Output: "mathml", ThrowOnError: true})
	if err != nil {
		t.Fatalf("RenderOption failed: %v", err)
	}
	if !bytes.Contains(buf.Bytes(), []byte(`display="block"`)) {
		t.Fatalf("expected display block output, got: %s", buf.String())
	}

	buf.Reset()
	err = RenderWithOption(&buf, []byte(`x+y`), Option{DisplayMode: false, ThrowOnError: false})
	if err != nil {
		t.Fatalf("RenderWithOption failed: %v", err)
	}
	if !bytes.Contains(buf.Bytes(), []byte(`<math xmlns="http://www.w3.org/1998/Math/MathML">`)) {
		t.Fatalf("expected MathML output, got: %s", buf.String())
	}
}
