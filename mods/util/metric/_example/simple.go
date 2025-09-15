package main

import (
	"fmt"
	"time"

	"github.com/machbase/neo-server/v8/mods/util/metric"
)

func main() {
	// store 12 time-windowed data-bins, each data-bin contains samples that collected 5 seconds
	// in this case, the collector keeps 1min (12 data bins of 5s) data history.
	series1, _ := metric.NewSeriesID("METRIC_1M", "timeseries of 5s bins", 5*time.Second, 12)
	// additionally, keep 1 hour data history with 1min data-bins
	series2, _ := metric.NewSeriesID("METRIC_1H", "timeseries of 1m bins", 1*time.Minute, 60)
	collector := metric.NewCollector(
		// make collector to gather metrics every second
		metric.WithSamplingInterval(1*time.Second),
		metric.WithSeries(series1, series2),
	)

	// add a custom metric input function that increases a counter every time it is called
	// and report it as a gauge metric named "custom:metric1"
	// the input function is called every sampling interval (1 second in this case)
	// so the reported metric value will be 1, 2, 3, ... every second
	var count = 0
	collector.AddInputFunc(func(g *metric.Gather) error {
		count++
		g.Add("custom:metric1", float64(count), metric.GaugeType(metric.UnitShort))
		return nil
	})

	// add an output function that prints the collected metrics to the console
	// the output function is called every series interval (3 seconds in this case)
	var rows int
	collector.AddOutputFunc(func(p metric.Product) error {
		rows++
		value := p.Value.(*metric.GaugeValue)
		fmt.Printf("[%02d] %q: %s Name:%q Sum:%.f Value:%.f Samples:%d\n",
			rows, p.SeriesTitle, p.Time, p.Name, value.Sum, value.Value, value.Samples)
		return nil
	})

	// start collector
	collector.Start()

	// simulate running for 10 seconds
	<-time.After(70 * time.Second)

	// stop collector
	collector.Stop()
}

// Each 5 seconds, the output function prints the collected metrics to the console
// The line [10] is the 1 minute summary of the metric.
// [01]'s sum is 6 = 1+2+3 and value is 3 that is the last value of the bin
// [02]'s sum is 30 = 4+5+6+7+8 and value is 8 that is the last value of the bin
// [03]'s sum is 55 = 9+10+11+12+13 and value is 13 that is the last value of the bin
// ...
// [10]'s sum is 946 = 1+2+3+...+43 and value is 43 that is the last value of the 1 minute bin
//
// Output:
//
// [01] "timeseries of 5s bins": 2025-09-11 16:53:20 Name:"custom:metric1" Sum:6 Value:3 Samples:3
// [02] "timeseries of 5s bins": 2025-09-11 16:53:25 Name:"custom:metric1" Sum:30 Value:8 Samples:5
// [03] "timeseries of 5s bins": 2025-09-11 16:53:30 Name:"custom:metric1" Sum:55 Value:13 Samples:5
// [04] "timeseries of 5s bins": 2025-09-11 16:53:35 Name:"custom:metric1" Sum:80 Value:18 Samples:5
// [05] "timeseries of 5s bins": 2025-09-11 16:53:40 Name:"custom:metric1" Sum:105 Value:23 Samples:5
// [06] "timeseries of 5s bins": 2025-09-11 16:53:45 Name:"custom:metric1" Sum:130 Value:28 Samples:5
// [07] "timeseries of 5s bins": 2025-09-11 16:53:50 Name:"custom:metric1" Sum:155 Value:33 Samples:5
// [08] "timeseries of 5s bins": 2025-09-11 16:53:55 Name:"custom:metric1" Sum:180 Value:38 Samples:5
// [09] "timeseries of 5s bins": 2025-09-11 16:54:00 Name:"custom:metric1" Sum:205 Value:43 Samples:5
// [10] "timeseries of 1m bins": 2025-09-11 16:54:00 Name:"custom:metric1" Sum:946 Value:43 Samples:43
// [11] "timeseries of 5s bins": 2025-09-11 16:54:05 Name:"custom:metric1" Sum:230 Value:48 Samples:5
// [12] "timeseries of 5s bins": 2025-09-11 16:54:10 Name:"custom:metric1" Sum:255 Value:53 Samples:5
// [13] "timeseries of 5s bins": 2025-09-11 16:54:15 Name:"custom:metric1" Sum:280 Value:58 Samples:5
// [14] "timeseries of 5s bins": 2025-09-11 16:54:20 Name:"custom:metric1" Sum:305 Value:63 Samples:5
// [15] "timeseries of 5s bins": 2025-09-11 16:54:25 Name:"custom:metric1" Sum:330 Value:68 Samples:5
