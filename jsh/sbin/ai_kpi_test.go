package sbin_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAIKPIHelp(t *testing.T) {
	workDir := t.TempDir()

	output, err := runCommand(workDir, nil, "ai_kpi", "--help")
	if err != nil {
		t.Fatalf("ai_kpi --help failed: %v\n%s", err, output)
	}

	if !strings.Contains(output, "Usage: ai_kpi [options]") {
		t.Fatalf("help output missing usage:\n%s", output)
	}
	if !strings.Contains(output, "--scenarios") {
		t.Fatalf("help output missing --scenarios option:\n%s", output)
	}
}

func TestAIKPIDryRunWritesReport(t *testing.T) {
	workDir := t.TempDir()

	scenariosPath := filepath.Join(workDir, "scenarios.txt")
	reportPath := filepath.Join(workDir, "report.json")
	scenarios := strings.Join([]string{
		"# comment line",
		"{\"id\":\"s1\",\"prompt\":\"show me table status\"}",
		"find recent rows",
		"",
	}, "\n")
	if err := os.WriteFile(scenariosPath, []byte(scenarios), 0o644); err != nil {
		t.Fatalf("write scenarios file: %v", err)
	}

	output, err := runCommand(workDir, nil,
		"ai_kpi",
		"--dry-run",
		"--scenarios", "scenarios.txt",
		"--out", "report.json",
	)
	if err != nil {
		t.Fatalf("ai_kpi dry-run failed: %v\n%s", err, output)
	}
	if !strings.Contains(output, "AI KPI Report") {
		t.Fatalf("missing report header in output:\n%s", output)
	}
	if !strings.Contains(output, "Scenarios: 2") {
		t.Fatalf("unexpected scenario count in output:\n%s", output)
	}

	raw, err := os.ReadFile(reportPath)
	if err != nil {
		t.Fatalf("read report file: %v", err)
	}

	var report map[string]any
	if err := json.Unmarshal(raw, &report); err != nil {
		t.Fatalf("parse report json: %v\n%s", err, string(raw))
	}

	totals, ok := report["totals"].(map[string]any)
	if !ok {
		t.Fatalf("totals section missing in report: %#v", report)
	}
	if totals["scenarioCount"] != float64(2) {
		t.Fatalf("scenarioCount = %#v, want 2", totals["scenarioCount"])
	}
	if totals["failureCount"] != float64(0) {
		t.Fatalf("failureCount = %#v, want 0", totals["failureCount"])
	}
}

func TestAIKPIDryRunWritesNDJSONAndCSV(t *testing.T) {
	workDir := t.TempDir()

	scenariosPath := filepath.Join(workDir, "scenarios.jsonl")
	reportPath := filepath.Join(workDir, "report.json")
	ndjsonPath := filepath.Join(workDir, "report.ndjson")
	csvPath := filepath.Join(workDir, "report.csv")
	scenarios := strings.Join([]string{
		`{"id":"s1","prompt":"show table list"}`,
		`{"id":"s2","prompt":"describe table sample"}`,
	}, "\n")
	if err := os.WriteFile(scenariosPath, []byte(scenarios), 0o644); err != nil {
		t.Fatalf("write scenarios file: %v", err)
	}

	output, err := runCommand(workDir, nil,
		"ai_kpi",
		"--dry-run",
		"--scenarios", "scenarios.jsonl",
		"--out", "report.json",
		"--out-ndjson", "report.ndjson",
		"--out-csv", "report.csv",
	)
	if err != nil {
		t.Fatalf("ai_kpi dry-run with ndjson/csv failed: %v\n%s", err, output)
	}

	ndjsonRaw, err := os.ReadFile(ndjsonPath)
	if err != nil {
		t.Fatalf("read ndjson file: %v", err)
	}
	ndjsonLines := strings.Split(strings.TrimSpace(string(ndjsonRaw)), "\n")
	if len(ndjsonLines) != 2 {
		t.Fatalf("ndjson line count = %d, want 2\n%s", len(ndjsonLines), string(ndjsonRaw))
	}

	csvRaw, err := os.ReadFile(csvPath)
	if err != nil {
		t.Fatalf("read csv file: %v", err)
	}
	csvText := string(csvRaw)
	if !strings.Contains(csvText, "index,id,ok,dryRun") {
		t.Fatalf("csv header missing:\n%s", csvText)
	}
	if !strings.Contains(csvText, ",s1,") || !strings.Contains(csvText, ",s2,") {
		t.Fatalf("csv body missing scenario ids:\n%s", csvText)
	}

	if _, err := os.Stat(reportPath); err != nil {
		t.Fatalf("report json file missing: %v", err)
	}
}

func TestAIKPIRealModeSmoke(t *testing.T) {
	apiKey := strings.TrimSpace(os.Getenv("OPENAI_API_KEY"))
	if apiKey == "" {
		t.Skip("OPENAI_API_KEY is not set; skipping real-mode smoke test")
	}

	workDir := t.TempDir()
	scenariosPath := filepath.Join(workDir, "real-scenarios.jsonl")
	reportPath := filepath.Join(workDir, "real-report.json")

	if err := os.WriteFile(scenariosPath, []byte(`{"id":"real-1","prompt":"Say hello in one short sentence."}`+"\n"), 0o644); err != nil {
		t.Fatalf("write real scenarios file: %v", err)
	}

	output, err := runCommand(workDir, map[string]any{
		"OPENAI_API_KEY": apiKey,
	},
		"ai_kpi",
		"--scenarios", "real-scenarios.jsonl",
		"--out", "real-report.json",
		"--provider", "openai",
		"--model", "gpt-5.4-mini",
		"--noExec",
	)
	if err != nil {
		t.Fatalf("ai_kpi real-mode smoke failed: %v\n%s", err, output)
	}
	if !strings.Contains(output, "AI KPI Report") {
		t.Fatalf("real-mode output missing report header:\n%s", output)
	}

	raw, err := os.ReadFile(reportPath)
	if err != nil {
		t.Fatalf("read real report file: %v", err)
	}

	var report map[string]any
	if err := json.Unmarshal(raw, &report); err != nil {
		t.Fatalf("parse real report json: %v\n%s", err, string(raw))
	}
	totals, ok := report["totals"].(map[string]any)
	if !ok {
		t.Fatalf("totals section missing in real report: %#v", report)
	}
	if totals["scenarioCount"] != float64(1) {
		t.Fatalf("real scenarioCount = %#v, want 1", totals["scenarioCount"])
	}
}

func TestAIKPIDryRunWithRepositoryScenarioPack(t *testing.T) {
	workDir := t.TempDir()
	reportPath := filepath.Join(workDir, "ci-pack-report.json")
	scenarioSrc := filepath.Join("..", "test", "ai-kpi-scenarios-ci.jsonl")
	scenarioDst := filepath.Join(workDir, "ai-kpi-scenarios-ci.jsonl")

	rawScenario, err := os.ReadFile(scenarioSrc)
	if err != nil {
		t.Fatalf("read repository scenario pack: %v", err)
	}
	if err := os.WriteFile(scenarioDst, rawScenario, 0o644); err != nil {
		t.Fatalf("copy scenario pack into workdir: %v", err)
	}

	output, err := runCommand(workDir, nil,
		"ai_kpi",
		"--dry-run",
		"--scenarios", "ai-kpi-scenarios-ci.jsonl",
		"--out", "ci-pack-report.json",
	)
	if err != nil {
		t.Fatalf("ai_kpi dry-run with repository scenario pack failed: %v\n%s", err, output)
	}
	if !strings.Contains(output, "Scenarios: 20") {
		t.Fatalf("expected 20 scenarios in output:\n%s", output)
	}

	raw, err := os.ReadFile(reportPath)
	if err != nil {
		t.Fatalf("read ci-pack report file: %v", err)
	}
	var report map[string]any
	if err := json.Unmarshal(raw, &report); err != nil {
		t.Fatalf("parse ci-pack report json: %v\n%s", err, string(raw))
	}
	totals, ok := report["totals"].(map[string]any)
	if !ok {
		t.Fatalf("totals section missing in ci-pack report: %#v", report)
	}
	if totals["scenarioCount"] != float64(20) {
		t.Fatalf("ci-pack scenarioCount = %#v, want 20", totals["scenarioCount"])
	}
}
