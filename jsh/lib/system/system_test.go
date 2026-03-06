package system_test

import (
	"regexp"
	"strings"
	"testing"

	"github.com/machbase/neo-server/v8/jsh/test_engine"
)

func TestLog(t *testing.T) {
	tests := []test_engine.TestCase{
		{
			Name: "log",
			Script: `
				console.info("test info");
				console.error("test error");
				console.warn("test warn");
				console.debug("test debug");
				`,
			ExpectFunc: func(t *testing.T, result string) {
				lines := strings.Split(result, "\n")
				output := []string{
					`^INFO  test info`,
					`^ERROR test error`,
					`^WARN  test warn`,
					`^DEBUG test debug`,
				}
				for n, line := range output {
					r := regexp.MustCompile(line)
					if !r.MatchString(lines[n]) {
						t.Errorf("log line %d: expected to match %q, got %q", n, line, lines[n])
					}
				}
			},
		},
	}

	for _, tc := range tests {
		test_engine.RunTest(t, tc)
	}
}
