package system_test

import (
	"strings"
	"testing"
	"time"

	"github.com/machbase/neo-server/v8/jsh/test_engine"
	"github.com/stretchr/testify/require"
)

func TestGC(t *testing.T) {
	tests := []test_engine.TestCase{
		{
			Name: "gc",
			Script: `
				const sys = require("system");
				sys.gc();
				sys.free_os_memory();
			`,
			ExpectFunc: func(t *testing.T, result string) {
			},
		},
	}
	for _, tc := range tests {
		test_engine.RunTest(t, tc)
	}
}

func TestNow(t *testing.T) {
	tests := []test_engine.TestCase{
		{
			Name: "now",
			Script: `
				const now = require("system").now;
				console.println(now());
			`,
			ExpectFunc: func(t *testing.T, result string) {
				result = strings.TrimSpace(result)
				_, err := time.Parse("2006-01-02 15:04:05", result)
				require.NoError(t, err, "expected to parse time in format '2006-01-02 15:04:05', got %q", result)
			},
		},
	}

	for _, tc := range tests {
		test_engine.RunTest(t, tc)
	}
}

func TestTimeLocation(t *testing.T) {
	tests := []test_engine.TestCase{
		{
			Name: "timeLocation",
			Script: `
				const timeLocation = require("system").timeLocation;
				console.println(timeLocation("UTC").string());
				console.println(timeLocation("Asia/Shanghai").string());
			`,
			ExpectFunc: func(t *testing.T, result string) {
				lines := strings.Split(strings.TrimSpace(result), "\n")
				require.Equal(t, "UTC", lines[0])
				require.Equal(t, "Asia/Shanghai", lines[1])
			},
		},
	}

	for _, tc := range tests {
		test_engine.RunTest(t, tc)
	}
}
