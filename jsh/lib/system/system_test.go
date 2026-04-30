package system_test

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/machbase/neo-server/v8/jsh/test_engine"
	"github.com/machbase/neo-server/v8/mods/util/metric"
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

func TestStatz(t *testing.T) {
	var out string
	var cnt int
	var now time.Time

	seriesID, err := metric.NewSeriesID("METRIC_1M", "1m/1s", time.Second, 60)
	require.NoError(t, err)
	c := metric.NewCollector(
		metric.WithSamplingInterval(time.Second),
		metric.WithSeries(seriesID),
	)

	c.AddOutputFunc(func(pd metric.Product) error {
		out = fmt.Sprintf("%s %s %v %s %s",
			pd.Name, pd.SeriesTitle, pd.Time.Format(time.TimeOnly), pd.Value.String(), pd.Type)
		if cnt == 0 {
			now = pd.Time
		} else {
			now = now.Add(time.Second)
		}
		cnt++
		expect := fmt.Sprintf(`m1:f1 1m/1s %s {"samples":1,"value":1} counter`, now.Format(time.TimeOnly))
		require.Equal(t, expect, out)
		return nil
	})
	c.AddInputFunc(func(g *metric.Gather) error {
		g.Add("m1:f1", 1.0, metric.CounterType(metric.UnitShort))
		return nil
	})
	c.Start()
	defer c.Stop()

	time.Sleep(3 * time.Second)

	tests := []test_engine.TestCase{
		{
			Name: "statz",
			Script: `
				const statz = require("system").statz;
				const lst = statz('1s', 'm1:f1');
				for (const item of lst) {
					if (item.values && item.values.length > 0 && item.values[0] !== null) {
						console.println(JSON.stringify(item));
					}
				}
			`,
			ExpectFunc: func(t *testing.T, result string) {
				lines := strings.Split(result, "\n")
				require.Contains(t, lines[0], `"time":"`)
				require.Contains(t, lines[0], `"m1_f1":1`)
				require.Contains(t, lines[0], `"values":[1]`)
			},
		},
	}

	for _, tc := range tests {
		test_engine.RunTest(t, tc)
	}
}
