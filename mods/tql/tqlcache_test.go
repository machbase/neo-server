package tql

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestTqlCache(t *testing.T) {
	tests := []struct {
		Name       string
		Script     string
		Params     map[string][]string
		Payload    string
		Filename   string
		ExpectCSV  []string
		ExpectText []string
		ExpectErr  string
	}{
		{
			Name: "cache-enlist",
			Script: `
				FAKE( linspace(
						parseFloat(param("begin")), 
						parseFloat(param("end")),
						parseFloat(param("count"))) )
				CSV(
					cache(param("begin") + "-" + param("end") + "-" +  param("count"), "10s")
				)`,
			Params:   map[string][]string{"begin": {"1"}, "end": {"10"}, "count": {"10"}},
			Filename: "/test/cache-enlist.tql",
			ExpectCSV: []string{
				"1", "2", "3", "4", "5", "6", "7", "8", "9", "10", "\n",
			},
		},
		{
			Name: "cache-hit",
			Script: `
				FAKE( linspace(
						parseFloat(param("begin")), 
						parseFloat(param("end")),
						parseFloat(param("count"))) )
				CSV(
					cache(param("begin") + "-" + param("end") + "-" +  param("count"), "10s")
				)`,
			Params:   map[string][]string{"begin": {"1"}, "end": {"10"}, "count": {"10"}},
			Filename: "/test/cache-enlist.tql",
			ExpectCSV: []string{
				"1", "2", "3", "4", "5", "6", "7", "8", "9", "10", "\n",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()

			output := &bytes.Buffer{}
			task := NewTaskContext(ctx)
			task.sourcePath = tc.Filename
			task.SetLogWriter(os.Stdout)
			task.SetOutputWriterJson(output, true)
			if tc.Payload != "" {
				task.SetInputReader(bytes.NewBufferString(tc.Payload))
			}
			if len(tc.Params) > 0 {
				task.SetParams(tc.Params)
			}
			if err := task.CompileString(tc.Script); err != nil {
				t.Log("ERROR:", tc.Name, err.Error())
				t.Fail()
				return
			}
			result := task.Execute()
			if tc.ExpectErr != "" {
				require.Error(t, result.Err)
				require.Equal(t, tc.ExpectErr, result.Err.Error())
				return
			}
			if result.Err != nil {
				t.Log("ERROR:", tc.Name, result.Err.Error())
				t.Fail()
				return
			}

			switch task.OutputContentType() {
			case "text/plain",
				"text/csv; charset=utf-8",
				"text/markdown",
				"application/xhtml+xml",
				"application/json",
				"application/x-ndjson":
				outputText := output.String()
				if outputText == "" && result.IsDbSink {
					if v, err := json.Marshal(result); err == nil {
						outputText = string(v)
					} else {
						outputText = "ERROR: failed to marshal result"
					}
				}
				if len(tc.ExpectCSV) > 0 {
					require.Equal(t, strings.Join(tc.ExpectCSV, "\n"), outputText)
				} else if len(tc.ExpectText) > 0 {
					require.Equal(t, strings.Join(tc.ExpectText, "\n"), outputText)
				} else {
					t.Fatalf("unhandled output %q: %s", task.OutputContentType(), outputText)
				}
			default:
				t.Fatal("ERROR:", tc.Name, "unexpected content type:", task.OutputContentType())
			}
		})
	}
}
