package sbin_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func readTestInput(t *testing.T, sourcePath string) string {
	t.Helper()
	content, err := os.ReadFile(sourcePath)
	if err != nil {
		t.Fatalf("read test input %s: %v", sourcePath, err)
	}
	return string(content)
}

func TestVizValidateCommand(t *testing.T) {
	workDir := t.TempDir()
	copyTestFile(t, filepath.Join("..", "test", "advn-sample.json"), filepath.Join(workDir, "advn-sample.json"))

	output, err := runCommand(workDir, nil, "viz", "validate", "advn-sample.json")
	if err != nil {
		t.Fatalf("viz validate failed: %v\n%s", err, output)
	}
	trimmed := strings.TrimSpace(output)
	if trimmed != "VALID version=1 series=1 annotations=1" {
		t.Fatalf("unexpected validate output: %q", trimmed)
	}
}

func TestVizValidateCommandFromStdin(t *testing.T) {
	workDir := t.TempDir()
	input := readTestInput(t, filepath.Join("..", "test", "advn-sample.json"))

	output, err := runCommandWithInput(workDir, nil, input, "viz", "validate")
	if err != nil {
		t.Fatalf("viz validate from stdin failed: %v\n%s", err, output)
	}
	trimmed := strings.TrimSpace(output)
	if trimmed != "VALID version=1 series=1 annotations=1" {
		t.Fatalf("unexpected validate output from stdin: %q", trimmed)
	}
}

func TestVizViewCommand(t *testing.T) {
	workDir := t.TempDir()
	copyTestFile(t, filepath.Join("..", "test", "advn-sample.json"), filepath.Join(workDir, "advn-sample.json"))

	output, err := runCommand(workDir, nil, "viz", "view", "advn-sample.json")
	if err != nil {
		t.Fatalf("viz view failed: %v\n%s", err, output)
	}
	for _, want := range []string{
		"ADVN",
		"sensor-1",
		"time-bucket-band",
		"max ",
		"avg ",
		"min ",
		"sensor-1 data",
		"Annotations",
		"checkpoint",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("expected output to contain %q, got:\n%s", want, output)
		}
	}
}

func TestVizViewCommandFromStdin(t *testing.T) {
	workDir := t.TempDir()
	input := readTestInput(t, filepath.Join("..", "test", "advn-sample.json"))

	output, err := runCommandWithInput(workDir, nil, input, "viz", "view")
	if err != nil {
		t.Fatalf("viz view from stdin failed: %v\n%s", err, output)
	}
	for _, want := range []string{
		"ADVN",
		"sensor-1",
		"time-bucket-band",
		"Annotations",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("expected stdin view output to contain %q, got:\n%s", want, output)
		}
	}
}

func TestVizViewVerboseMetaCommand(t *testing.T) {
	workDir := t.TempDir()
	copyTestFile(t, filepath.Join("..", "test", "advn-sample.json"), filepath.Join(workDir, "advn-sample.json"))

	output, err := runCommand(workDir, nil, "viz", "view", "--verbose-meta", "advn-sample.json")
	if err != nil {
		t.Fatalf("viz view --verbose-meta failed: %v\n%s", err, output)
	}
	for _, want := range []string{
		"META",
		"representation",
		"time-bucket-band",
		"totalRows",
		"3",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("expected verbose meta output to contain %q, got:\n%s", want, output)
		}
	}
}

func TestVizValidateHistogramAndEventCommand(t *testing.T) {
	workDir := t.TempDir()
	copyTestFile(t, filepath.Join("..", "test", "advn-hist-event-sample.json"), filepath.Join(workDir, "advn-hist-event-sample.json"))

	output, err := runCommand(workDir, nil, "viz", "validate", "advn-hist-event-sample.json")
	if err != nil {
		t.Fatalf("viz validate for histogram/event-range failed: %v\n%s", err, output)
	}
	trimmed := strings.TrimSpace(output)
	if trimmed != "VALID version=1 series=2 annotations=0" {
		t.Fatalf("unexpected validate output: %q", trimmed)
	}
}

func TestVizViewHistogramAndEventCommand(t *testing.T) {
	oldLocal := time.Local
	time.Local = time.FixedZone("KST", 9*60*60)
	t.Cleanup(func() {
		time.Local = oldLocal
	})

	workDir := t.TempDir()
	copyTestFile(t, filepath.Join("..", "test", "advn-hist-event-sample.json"), filepath.Join(workDir, "advn-hist-event-sample.json"))

	output, err := runCommand(workDir, nil, "viz", "view", "advn-hist-event-sample.json")
	if err != nil {
		t.Fatalf("viz view for histogram/event-range failed: %v\n%s", err, output)
	}
	for _, want := range []string{
		"latency-hist",
		"distribution-histogram",
		"0-10",
		"10-20",
		"maintenance-window",
		"event-range",
		"maintenance",
		"2026-04-03T19:00:00+09:00",
		"2026-04-03T20:00:00+09:00",
		"========",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("expected output to contain %q, got:\n%s", want, output)
		}
	}
}

func TestVizValidateBoxplotCommand(t *testing.T) {
	workDir := t.TempDir()
	copyTestFile(t, filepath.Join("..", "test", "advn-boxplot-sample.json"), filepath.Join(workDir, "advn-boxplot-sample.json"))

	output, err := runCommand(workDir, nil, "viz", "validate", "advn-boxplot-sample.json")
	if err != nil {
		t.Fatalf("viz validate for boxplot failed: %v\n%s", err, output)
	}
	trimmed := strings.TrimSpace(output)
	if trimmed != "VALID version=1 series=1 annotations=0" {
		t.Fatalf("unexpected validate output: %q", trimmed)
	}
}

func TestVizViewBoxplotCommand(t *testing.T) {
	workDir := t.TempDir()
	copyTestFile(t, filepath.Join("..", "test", "advn-boxplot-sample.json"), filepath.Join(workDir, "advn-boxplot-sample.json"))

	output, err := runCommand(workDir, nil, "viz", "view", "advn-boxplot-sample.json")
	if err != nil {
		t.Fatalf("viz view for boxplot failed: %v\n%s", err, output)
	}
	for _, want := range []string{
		"latency-box",
		"distribution-boxplot",
		"group-a | 12 [18 | 21 | 27] 33",
		"group-b | 10 [15 | 19 | 24] 30",
		"outliers: 1",
		"latency-box data",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("expected output to contain %q, got:\n%s", want, output)
		}
	}
}

func TestVizViewCompactRowsCommand(t *testing.T) {
	workDir := t.TempDir()
	copyTestFile(t, filepath.Join("..", "test", "advn-boxplot-sample.json"), filepath.Join(workDir, "advn-boxplot-sample.json"))

	output, err := runCommand(workDir, nil, "viz", "view", "--compact", "--rows", "1", "advn-boxplot-sample.json")
	if err != nil {
		t.Fatalf("viz view --compact --rows 1 failed: %v\n%s", err, output)
	}
	for _, want := range []string{
		"ADVN",
		"latency-box",
		"group-a | 12 [18 | 21 | 27] 33",
		"... 1 more groups",
		"outliers: 1",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("expected compact output to contain %q, got:\n%s", want, output)
		}
	}
	for _, unwanted := range []string{
		"latency-box data",
		"distribution-boxplot",
		"group-b | 10 [15 | 19 | 24] 30",
	} {
		if strings.Contains(output, unwanted) {
			t.Fatalf("expected compact output to omit %q, got:\n%s", unwanted, output)
		}
	}
}

func TestVizLinesCommand(t *testing.T) {
	workDir := t.TempDir()
	copyTestFile(t, filepath.Join("..", "test", "advn-sample.json"), filepath.Join(workDir, "advn-sample.json"))

	output, err := runCommand(workDir, nil, "viz", "lines", "advn-sample.json")
	if err != nil {
		t.Fatalf("viz lines failed: %v\n%s", err, output)
	}
	for _, want := range []string{"max ", "avg ", "min "} {
		if !strings.Contains(output, want) {
			t.Fatalf("expected lines output to contain %q, got:\n%s", want, output)
		}
	}
	tallOutput, err := runCommand(workDir, nil, "viz", "lines", "--height", "5", "advn-sample.json")
	if err != nil {
		t.Fatalf("viz lines --height failed: %v\n%s", err, tallOutput)
	}
	lineCount := 0
	for _, line := range strings.Split(strings.TrimSpace(tallOutput), "\n") {
		if strings.TrimSpace(line) != "" {
			lineCount++
		}
	}
	if lineCount != 3 {
		t.Fatalf("expected band output to remain 3 lines regardless of height, got %d\n%s", lineCount, tallOutput)
	}
}

func TestVizLinesCommandSeriesSelection(t *testing.T) {
	workDir := t.TempDir()
	input := `{
		"version": 1,
		"domain": {"kind": "time", "from": "2026-04-03T00:00:00Z", "to": "2026-04-03T00:04:00Z"},
		"series": [
			{
				"id": "value-1",
				"representation": {"kind": "time-bucket-value", "fields": ["time", "value"]},
				"data": [["2026-04-03T00:00:00Z", 1], ["2026-04-03T00:02:00Z", 4], ["2026-04-03T00:04:00Z", 2]]
			},
			{
				"id": "value-2",
				"representation": {"kind": "time-bucket-value", "fields": ["time", "value"]},
				"data": [["2026-04-03T00:00:00Z", 8], ["2026-04-03T00:02:00Z", 2], ["2026-04-03T00:04:00Z", 7]]
			}
		]
	}`

	defaultOutput, err := runCommandWithInput(workDir, nil, input, "viz", "lines")
	if err != nil {
		t.Fatalf("viz lines from stdin failed: %v\n%s", err, defaultOutput)
	}
	selectedOutput, err := runCommandWithInput(workDir, nil, input, "viz", "lines", "--series", "value-2")
	if err != nil {
		t.Fatalf("viz lines --series failed: %v\n%s", err, selectedOutput)
	}
	if defaultOutput == selectedOutput {
		t.Fatalf("expected --series to change lines output, got:\n%s", selectedOutput)
	}
	if !strings.Contains(selectedOutput, "┤") {
		t.Fatalf("expected selected lines output to contain axis marker, got:\n%s", selectedOutput)
	}
	tallOutput, err := runCommandWithInput(workDir, nil, input, "viz", "lines", "--height", "5")
	if err != nil {
		t.Fatalf("viz lines --height from stdin failed: %v\n%s", err, tallOutput)
	}
	lineCount := 0
	for _, line := range strings.Split(strings.TrimSpace(tallOutput), "\n") {
		if strings.TrimSpace(line) != "" {
			lineCount++
		}
	}
	if lineCount != 6 {
		t.Fatalf("expected value-series lines output with axis and 5 chart rows, got %d\n%s", lineCount, tallOutput)
	}
}

func TestVizLinesCommandSeriesSelectionErrorHint(t *testing.T) {
	workDir := t.TempDir()
	input := `{
		"version": 1,
		"series": [
			{
				"id": "value-1",
				"representation": {"kind": "time-bucket-value", "fields": ["time", "value"]},
				"data": [[0, 1], [1, 4], [2, 2]]
			},
			{
				"id": "band-1",
				"representation": {"kind": "time-bucket-band", "fields": ["time", "min", "max", "avg"]},
				"data": [[0, 1, 5, 3]]
			},
			{
				"id": "hist-1",
				"representation": {"kind": "distribution-histogram", "fields": ["binStart", "binEnd", "count"]},
				"data": [[0, 10, 3]]
			}
		]
	}`

	output, err := runCommandWithInput(workDir, nil, input, "viz", "lines", "--series", "missing")
	if err == nil {
		t.Fatalf("expected viz lines --series missing to fail, got:\n%s", output)
	}
	for _, want := range []string{"series \"missing\" not found", "Selectable line series:", "value-1", "band-1"} {
		if !strings.Contains(output, want) {
			t.Fatalf("expected error output to contain %q, got:\n%s", want, output)
		}
	}
	if strings.Contains(output, "hist-1") {
		t.Fatalf("expected error hint to omit incompatible series, got:\n%s", output)
	}
}

func TestVizExportSVGCommand(t *testing.T) {
	workDir := t.TempDir()
	copyTestFile(t, filepath.Join("..", "test", "advn-sample.json"), filepath.Join(workDir, "advn-sample.json"))

	output, err := runCommand(workDir, nil, "viz", "export", "--title", "ADVN SVG", "--width", "640", "--height", "320", "advn-sample.json")
	if err != nil {
		t.Fatalf("viz export svg failed: %v\n%s", err, output)
	}
	for _, want := range []string{
		"<svg ",
		"data-advn-role=\"series\"",
		"ADVN SVG",
		"sensor-1",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("expected export output to contain %q, got:\n%s", want, output)
		}
	}
}

func TestVizExportSVGCommandFromStdin(t *testing.T) {
	workDir := t.TempDir()
	input := readTestInput(t, filepath.Join("..", "test", "advn-sample.json"))

	output, err := runCommandWithInput(workDir, nil, input, "viz", "export", "--title", "ADVN SVG")
	if err != nil {
		t.Fatalf("viz export svg from stdin failed: %v\n%s", err, output)
	}
	for _, want := range []string{
		"<svg ",
		"data-advn-role=\"series\"",
		"ADVN SVG",
		"sensor-1",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("expected export output from stdin to contain %q, got:\n%s", want, output)
		}
	}
}

func TestVizExportSVGToFileCommand(t *testing.T) {
	workDir := t.TempDir()
	copyTestFile(t, filepath.Join("..", "test", "advn-hist-event-sample.json"), filepath.Join(workDir, "advn-hist-event-sample.json"))

	outputPath := filepath.Join(workDir, "out.svg")
	output, err := runCommand(workDir, nil, "viz", "export", "--output", "out.svg", "--hide-legend", "advn-hist-event-sample.json")
	if err != nil {
		t.Fatalf("viz export svg to file failed: %v\n%s", err, output)
	}
	trimmed := strings.TrimSpace(output)
	if !strings.Contains(trimmed, "WROTE /work/out.svg") {
		t.Fatalf("unexpected export status output: %q", trimmed)
	}
	bytes, readErr := os.ReadFile(outputPath)
	if readErr != nil {
		t.Fatalf("failed to read exported svg: %v", readErr)
	}
	text := string(bytes)
	for _, want := range []string{
		"<svg ",
		"maintenance-window",
		"data-advn-series=\"maintenance-window\"",
		"data-advn-role=\"series\"",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("expected exported file to contain %q, got:\n%s", want, text)
		}
	}
	if strings.Contains(text, "data-advn-role=\"legend\"") {
		t.Fatalf("expected exported file to omit legend when --hide-legend is used, got:\n%s", text)
	}
}

func TestVizExportPNGToFileCommand(t *testing.T) {
	workDir := t.TempDir()
	copyTestFile(t, filepath.Join("..", "test", "advn-sample.json"), filepath.Join(workDir, "advn-sample.json"))

	outputPath := filepath.Join(workDir, "out.png")
	output, err := runCommand(workDir, nil, "viz", "export", "--format", "png", "--output", "out.png", "--title", "ADVN PNG", "advn-sample.json")
	if err != nil {
		t.Fatalf("viz export png to file failed: %v\n%s", err, output)
	}
	trimmed := strings.TrimSpace(output)
	if !strings.Contains(trimmed, "WROTE /work/out.png") {
		t.Fatalf("unexpected png export status output: %q", trimmed)
	}
	data, readErr := os.ReadFile(outputPath)
	if readErr != nil {
		t.Fatalf("failed to read exported png: %v", readErr)
	}
	if len(data) < 8 || !bytes.Equal(data[:8], []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}) {
		t.Fatalf("expected PNG signature, got %v", data)
	}
}

func TestVizExportPNGRequiresOutputCommand(t *testing.T) {
	workDir := t.TempDir()
	copyTestFile(t, filepath.Join("..", "test", "advn-sample.json"), filepath.Join(workDir, "advn-sample.json"))

	output, err := runCommand(workDir, nil, "viz", "export", "--format", "png", "advn-sample.json")
	if err == nil {
		t.Fatalf("expected viz export png without output to fail, got output:\n%s", output)
	}
	if !strings.Contains(output, "png export requires --output because stdout is text-only") {
		t.Fatalf("expected png stdout restriction error, got:\n%s", output)
	}
}

func TestVizExportInvalidFormatCommand(t *testing.T) {
	workDir := t.TempDir()
	copyTestFile(t, filepath.Join("..", "test", "advn-sample.json"), filepath.Join(workDir, "advn-sample.json"))

	output, err := runCommand(workDir, nil, "viz", "export", "--format", "jpeg", "advn-sample.json")
	if err == nil {
		t.Fatalf("expected viz export invalid format to fail, got output:\n%s", output)
	}
	if !strings.Contains(output, "unsupported export format: jpeg") {
		t.Fatalf("expected invalid format error, got:\n%s", output)
	}
}
