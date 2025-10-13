package metric

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestTimeseries(t *testing.T) {
	now := time.Date(2023, 10, 1, 12, 4, 4, 400_000_000, time.UTC)
	nowFunc = func() time.Time { return now }
	timeZone = time.UTC

	ts := NewTimeSeries(time.Second, 3, NewMeter())
	ts.Add(1.0)

	now = now.Add(time.Second)
	ts.Add(2.0)

	require.JSONEq(t, `[`+
		`{"ts":"2023-10-01 12:04:05","value":{"samples":1,"max":1,"min":1,"first":1,"last":1,"sum":1}},`+
		`{"ts":"2023-10-01 12:04:06","value":{"samples":1,"max":2,"min":2,"first":2,"last":2,"sum":2}}`+
		`]`, ts.String())

	now = now.Add(time.Second)
	ts.Add(3.0)

	now = now.Add(time.Second)
	ts.Add(4.0)

	times, values := ts.All()
	require.Equal(t, []time.Time{
		time.Date(2023, time.October, 1, 12, 4, 6, 0, time.UTC),
		time.Date(2023, time.October, 1, 12, 4, 7, 0, time.UTC),
		time.Date(2023, time.October, 1, 12, 4, 8, 0, time.UTC),
	}, times)
	require.Equal(t, []Value{
		&MeterValue{Min: 2, Max: 2, First: 2, Last: 2, Sum: 2, Samples: 1},
		&MeterValue{Min: 3, Max: 3, First: 3, Last: 3, Sum: 3, Samples: 1},
		&MeterValue{Min: 4, Max: 4, First: 4, Last: 4, Sum: 4, Samples: 1},
	}, values)

	now = now.Add(100 * time.Millisecond)
	ts.Add(5.0)

	now = now.Add(200 * time.Millisecond)
	ts.Add(4.8)

	times, values = ts.All()
	require.Equal(t, []time.Time{
		time.Date(2023, time.October, 1, 12, 4, 6, 0, time.UTC),
		time.Date(2023, time.October, 1, 12, 4, 7, 0, time.UTC),
		time.Date(2023, time.October, 1, 12, 4, 8, 0, time.UTC),
	}, times)
	require.Equal(t, []Value{
		&MeterValue{Min: 2, Max: 2, First: 2, Last: 2, Sum: 2, Samples: 1},
		&MeterValue{Min: 3, Max: 3, First: 3, Last: 3, Sum: 3, Samples: 1},
		&MeterValue{Min: 4, Max: 5, First: 4, Last: 4.8, Sum: 13.8, Samples: 3},
	}, values)

	now = now.Add(1700 * time.Millisecond)
	ts.Add(6.0)

	times, values = ts.All()
	require.Equal(t, []time.Time{
		time.Date(2023, time.October, 1, 12, 4, 8, 0, time.UTC),
		time.Date(2023, time.October, 1, 12, 4, 9, 0, time.UTC),
		time.Date(2023, time.October, 1, 12, 4, 10, 0, time.UTC),
	}, times)
	require.Equal(t, []Value{
		&MeterValue{Min: 4, Max: 5, First: 4, Last: 4.8, Sum: 13.8, Samples: 3},
		nil, //&MeterValue{Min: 0, Max: 0, First: 0, Last: 0, Total: 0, Count: 0}},
		&MeterValue{Min: 6, Max: 6, First: 6, Last: 6, Sum: 6, Samples: 1},
	}, values)

	now = now.Add(5 * time.Second)
	ts.Add(7.0)

	require.JSONEq(t, `[`+
		`{"ts":"2023-10-01 12:04:15","value":{"samples":1,"max":7,"min":7,"first":7,"last":7,"sum":7}}`+
		`]`, ts.String())
}

func TestTimeSeriesSubSeconds(t *testing.T) {
	ts := NewTimeSeries(time.Second, 10, NewCounter())

	now := time.Date(2023, 10, 1, 12, 4, 5, 0, time.UTC)
	nowFunc = func() time.Time {
		ret := now
		now = now.Add(100 * time.Millisecond)
		return ret
	}

	for i := 1; i <= 10*10; i++ {
		ts.Add(float64(i))
	}

	require.JSONEq(t, `[`+
		`{"ts":"2023-10-01 12:04:06","value":{"value":55,"samples":10}},`+
		`{"ts":"2023-10-01 12:04:07","value":{"value":155,"samples":10}},`+
		`{"ts":"2023-10-01 12:04:08","value":{"value":255,"samples":10}},`+
		`{"ts":"2023-10-01 12:04:09","value":{"value":355,"samples":10}},`+
		`{"ts":"2023-10-01 12:04:10","value":{"value":455,"samples":10}},`+
		`{"ts":"2023-10-01 12:04:11","value":{"value":555,"samples":10}},`+
		`{"ts":"2023-10-01 12:04:12","value":{"value":655,"samples":10}},`+
		`{"ts":"2023-10-01 12:04:13","value":{"value":755,"samples":10}},`+
		`{"ts":"2023-10-01 12:04:14","value":{"value":855,"samples":10}},`+
		`{"ts":"2023-10-01 12:04:15","value":{"value":955,"samples":10}}`+
		`]`, ts.String())

	times, values := ts.LastN(0)
	require.Equal(t, []time.Time{
		time.Date(2023, 10, 1, 12, 4, 6, 0, time.UTC),
		time.Date(2023, 10, 1, 12, 4, 7, 0, time.UTC),
		time.Date(2023, 10, 1, 12, 4, 8, 0, time.UTC),
		time.Date(2023, 10, 1, 12, 4, 9, 0, time.UTC),
		time.Date(2023, 10, 1, 12, 4, 10, 0, time.UTC),
		time.Date(2023, 10, 1, 12, 4, 11, 0, time.UTC),
		time.Date(2023, 10, 1, 12, 4, 12, 0, time.UTC),
		time.Date(2023, 10, 1, 12, 4, 13, 0, time.UTC),
		time.Date(2023, 10, 1, 12, 4, 14, 0, time.UTC),
		time.Date(2023, 10, 1, 12, 4, 15, 0, time.UTC),
	}, times)
	require.Equal(t, []Value{
		&CounterValue{Value: 55, Samples: 10},
		&CounterValue{Value: 155, Samples: 10},
		&CounterValue{Value: 255, Samples: 10},
		&CounterValue{Value: 355, Samples: 10},
		&CounterValue{Value: 455, Samples: 10},
		&CounterValue{Value: 555, Samples: 10},
		&CounterValue{Value: 655, Samples: 10},
		&CounterValue{Value: 755, Samples: 10},
		&CounterValue{Value: 855, Samples: 10},
		&CounterValue{Value: 955, Samples: 10},
	}, values)
	require.Equal(t, time.Second, ts.Interval())
	require.Equal(t, 10, ts.MaxCount())

	ptTime, ptValue := ts.Last()
	require.Equal(t, &CounterValue{Value: 955, Samples: 10}, ptValue)
	require.Equal(t, time.Date(2023, 10, 1, 12, 4, 15, 0, time.UTC), ptTime)

	ptTimes, _ := ts.LastN(20)
	require.Equal(t, 10, len(ptTimes))

	ptTimes, ptValues := ts.After(time.Date(2023, 10, 1, 12, 4, 13, 0, time.UTC))
	require.Equal(t, 3, len(ptTimes))
	require.Equal(t, &CounterValue{Value: 755, Samples: 10}, ptValues[0])
	require.Equal(t, time.Date(2023, 10, 1, 12, 4, 13, 0, time.UTC), ptTimes[0])
	require.Equal(t, &CounterValue{Value: 855, Samples: 10}, ptValues[1])
	require.Equal(t, time.Date(2023, 10, 1, 12, 4, 14, 0, time.UTC), ptTimes[1])
	require.Equal(t, &CounterValue{Value: 955, Samples: 10}, ptValues[2])
	require.Equal(t, time.Date(2023, 10, 1, 12, 4, 15, 0, time.UTC), ptTimes[2])
}

func TestMultiTimeSeries(t *testing.T) {
	mts := MultiTimeSeries{
		NewTimeSeries(time.Second, 10, NewMeter()),
		NewTimeSeries(10*time.Second, 6, NewMeter()),
		NewTimeSeries(60*time.Second, 5, NewMeter()),
	}

	now := time.Date(2023, 10, 1, 12, 4, 5, 0, time.UTC)
	nowFunc = func() time.Time { return now }

	for i := 1; i <= 10*5*60; i++ {
		mts.Add(float64(i))
		now = now.Add(100 * time.Millisecond)
	}

	times, values := mts[0].LastN(0)
	require.Equal(t, []time.Time{
		time.Date(2023, 10, 1, 12, 8, 56, 0, time.UTC),
		time.Date(2023, 10, 1, 12, 8, 57, 0, time.UTC),
		time.Date(2023, 10, 1, 12, 8, 58, 0, time.UTC),
		time.Date(2023, 10, 1, 12, 8, 59, 0, time.UTC),
		time.Date(2023, 10, 1, 12, 9, 00, 0, time.UTC),
		time.Date(2023, 10, 1, 12, 9, 01, 0, time.UTC),
		time.Date(2023, 10, 1, 12, 9, 02, 0, time.UTC),
		time.Date(2023, 10, 1, 12, 9, 03, 0, time.UTC),
		time.Date(2023, 10, 1, 12, 9, 04, 0, time.UTC),
		time.Date(2023, 10, 1, 12, 9, 05, 0, time.UTC),
	}, times)
	require.Equal(t, []Value{
		&MeterValue{Min: 2901, Max: 2910, First: 2901, Last: 2910, Sum: 29055, Samples: 10},
		&MeterValue{Min: 2911, Max: 2920, First: 2911, Last: 2920, Sum: 29155, Samples: 10},
		&MeterValue{Min: 2921, Max: 2930, First: 2921, Last: 2930, Sum: 29255, Samples: 10},
		&MeterValue{Min: 2931, Max: 2940, First: 2931, Last: 2940, Sum: 29355, Samples: 10},
		&MeterValue{Min: 2941, Max: 2950, First: 2941, Last: 2950, Sum: 29455, Samples: 10},
		&MeterValue{Min: 2951, Max: 2960, First: 2951, Last: 2960, Sum: 29555, Samples: 10},
		&MeterValue{Min: 2961, Max: 2970, First: 2961, Last: 2970, Sum: 29655, Samples: 10},
		&MeterValue{Min: 2971, Max: 2980, First: 2971, Last: 2980, Sum: 29755, Samples: 10},
		&MeterValue{Min: 2981, Max: 2990, First: 2981, Last: 2990, Sum: 29855, Samples: 10},
		&MeterValue{Min: 2991, Max: 3000, First: 2991, Last: 3000, Sum: 29955, Samples: 10},
	}, values)

	times, values = mts[1].All()
	require.Equal(t, []time.Time{
		time.Date(2023, 10, 1, 12, 8, 20, 0, time.UTC),
		time.Date(2023, 10, 1, 12, 8, 30, 0, time.UTC),
		time.Date(2023, 10, 1, 12, 8, 40, 0, time.UTC),
		time.Date(2023, 10, 1, 12, 8, 50, 0, time.UTC),
		time.Date(2023, 10, 1, 12, 9, 00, 0, time.UTC),
		time.Date(2023, 10, 1, 12, 9, 10, 0, time.UTC),
	}, times)
	require.Equal(t, []Value{
		&MeterValue{Min: 2451, Max: 2550, First: 2451, Last: 2550, Sum: 250050, Samples: 100},
		&MeterValue{Min: 2551, Max: 2650, First: 2551, Last: 2650, Sum: 260050, Samples: 100},
		&MeterValue{Min: 2651, Max: 2750, First: 2651, Last: 2750, Sum: 270050, Samples: 100},
		&MeterValue{Min: 2751, Max: 2850, First: 2751, Last: 2850, Sum: 280050, Samples: 100},
		&MeterValue{Min: 2851, Max: 2950, First: 2851, Last: 2950, Sum: 290050, Samples: 100},
		&MeterValue{Min: 2951, Max: 3000, First: 2951, Last: 3000, Sum: 148775, Samples: 50},
	}, values)

	times, values = mts[2].All()
	require.Equal(t, []time.Time{
		time.Date(2023, 10, 1, 12, 6, 0, 0, time.UTC),
		time.Date(2023, 10, 1, 12, 7, 0, 0, time.UTC),
		time.Date(2023, 10, 1, 12, 8, 0, 0, time.UTC),
		time.Date(2023, 10, 1, 12, 9, 0, 0, time.UTC),
		time.Date(2023, 10, 1, 12, 10, 0, 0, time.UTC),
	}, times)
	require.Equal(t, []Value{
		&MeterValue{Min: 551, Max: 1150, First: 551, Last: 1150, Sum: 510300, Samples: 600},
		&MeterValue{Min: 1151, Max: 1750, First: 1151, Last: 1750, Sum: 870300, Samples: 600},
		&MeterValue{Min: 1751, Max: 2350, First: 1751, Last: 2350, Sum: 1230300, Samples: 600},
		&MeterValue{Min: 2351, Max: 2950, First: 2351, Last: 2950, Sum: 1590300, Samples: 600},
		&MeterValue{Min: 2951, Max: 3000, First: 2951, Last: 3000, Sum: 148775, Samples: 50},
	}, values)
}

func TestTimeSeriesCounter(t *testing.T) {
	ts := NewTimeSeries(1*time.Second, 10, NewCounter())

	now := time.Date(2025, 07, 21, 17, 31, 12, 0, time.FixedZone("Asia/Seoul", 9*60*60))
	nowFunc = func() time.Time {
		ret := now
		now = now.Add(time.Millisecond * 100)
		return ret
	}

	for i := 1; i <= 100; i++ {
		ts.Add(float64(i))
	}

	times, values := ts.LastN(0)
	require.Equal(t, []time.Time{
		time.Date(2025, 07, 21, 17, 31, 13, 0, time.FixedZone("Asia/Seoul", 9*60*60)),
		time.Date(2025, 07, 21, 17, 31, 14, 0, time.FixedZone("Asia/Seoul", 9*60*60)),
		time.Date(2025, 07, 21, 17, 31, 15, 0, time.FixedZone("Asia/Seoul", 9*60*60)),
		time.Date(2025, 07, 21, 17, 31, 16, 0, time.FixedZone("Asia/Seoul", 9*60*60)),
		time.Date(2025, 07, 21, 17, 31, 17, 0, time.FixedZone("Asia/Seoul", 9*60*60)),
		time.Date(2025, 07, 21, 17, 31, 18, 0, time.FixedZone("Asia/Seoul", 9*60*60)),
		time.Date(2025, 07, 21, 17, 31, 19, 0, time.FixedZone("Asia/Seoul", 9*60*60)),
		time.Date(2025, 07, 21, 17, 31, 20, 0, time.FixedZone("Asia/Seoul", 9*60*60)),
		time.Date(2025, 07, 21, 17, 31, 21, 0, time.FixedZone("Asia/Seoul", 9*60*60)),
		time.Date(2025, 07, 21, 17, 31, 22, 0, time.FixedZone("Asia/Seoul", 9*60*60)),
	}, times)
	require.Equal(t, []Value{
		&CounterValue{Samples: 10, Value: 55},
		&CounterValue{Samples: 10, Value: 155},
		&CounterValue{Samples: 10, Value: 255},
		&CounterValue{Samples: 10, Value: 355},
		&CounterValue{Samples: 10, Value: 455},
		&CounterValue{Samples: 10, Value: 555},
		&CounterValue{Samples: 10, Value: 655},
		&CounterValue{Samples: 10, Value: 755},
		&CounterValue{Samples: 10, Value: 855},
		&CounterValue{Samples: 10, Value: 955},
	}, values)
}

func TestTimeSeriesCounterWithSlidingWindow(t *testing.T) {
	ts := NewTimeSeries(1*time.Second, 10,
		NewCounter().WithDerivers(
			NewMovingAverage("ma3", 3),
			NewMovingAverage("ma5", 5),
		),
	)

	now := time.Date(2025, 07, 21, 17, 31, 12, 0, time.FixedZone("Asia/Seoul", 9*60*60))
	nowFunc = func() time.Time {
		ret := now
		now = now.Add(time.Millisecond * 100)
		return ret
	}

	for i := 1; i <= 100; i++ {
		ts.Add(float64(i))
	}

	times, values := ts.LastN(0)
	require.Equal(t, []time.Time{
		time.Date(2025, 07, 21, 17, 31, 13, 0, time.FixedZone("Asia/Seoul", 9*60*60)),
		time.Date(2025, 07, 21, 17, 31, 14, 0, time.FixedZone("Asia/Seoul", 9*60*60)),
		time.Date(2025, 07, 21, 17, 31, 15, 0, time.FixedZone("Asia/Seoul", 9*60*60)),
		time.Date(2025, 07, 21, 17, 31, 16, 0, time.FixedZone("Asia/Seoul", 9*60*60)),
		time.Date(2025, 07, 21, 17, 31, 17, 0, time.FixedZone("Asia/Seoul", 9*60*60)),
		time.Date(2025, 07, 21, 17, 31, 18, 0, time.FixedZone("Asia/Seoul", 9*60*60)),
		time.Date(2025, 07, 21, 17, 31, 19, 0, time.FixedZone("Asia/Seoul", 9*60*60)),
		time.Date(2025, 07, 21, 17, 31, 20, 0, time.FixedZone("Asia/Seoul", 9*60*60)),
		time.Date(2025, 07, 21, 17, 31, 21, 0, time.FixedZone("Asia/Seoul", 9*60*60)),
		time.Date(2025, 07, 21, 17, 31, 22, 0, time.FixedZone("Asia/Seoul", 9*60*60)),
	}, times)
	require.Equal(t, &CounterValue{Samples: 10, Value: 55, DerivedValues: map[string]Value{
		"ma3": &CounterValue{Samples: 10, Value: 55},
		"ma5": &CounterValue{Samples: 10, Value: 55},
	}}, values[0])
	require.Equal(t, &CounterValue{Samples: 10, Value: 155, DerivedValues: map[string]Value{
		"ma3": &CounterValue{Samples: 20, Value: 105},
		"ma5": &CounterValue{Samples: 20, Value: 105},
	}}, values[1])
	require.Equal(t, &CounterValue{Samples: 10, Value: 255, DerivedValues: map[string]Value{
		"ma3": &CounterValue{Samples: 30, Value: 155},
		"ma5": &CounterValue{Samples: 30, Value: 155},
	}}, values[2])
	require.Equal(t, &CounterValue{Samples: 10, Value: 355, DerivedValues: map[string]Value{
		"ma3": &CounterValue{Samples: 30, Value: 255},
		"ma5": &CounterValue{Samples: 40, Value: 205},
	}}, values[3])
	require.Equal(t, &CounterValue{Samples: 10, Value: 455, DerivedValues: map[string]Value{
		"ma3": &CounterValue{Samples: 30, Value: 355},
		"ma5": &CounterValue{Samples: 50, Value: 255},
	}}, values[4])
	require.Equal(t, &CounterValue{Samples: 10, Value: 555, DerivedValues: map[string]Value{
		"ma3": &CounterValue{Samples: 30, Value: 455},
		"ma5": &CounterValue{Samples: 50, Value: 355},
	}}, values[5])
	require.Equal(t, &CounterValue{Samples: 10, Value: 655, DerivedValues: map[string]Value{
		"ma3": &CounterValue{Samples: 30, Value: 555},
		"ma5": &CounterValue{Samples: 50, Value: 455},
	}}, values[6])
	require.Equal(t, &CounterValue{Samples: 10, Value: 755, DerivedValues: map[string]Value{
		"ma3": &CounterValue{Samples: 30, Value: 655},
		"ma5": &CounterValue{Samples: 50, Value: 555},
	}}, values[7])
	require.Equal(t, &CounterValue{Samples: 10, Value: 855, DerivedValues: map[string]Value{
		"ma3": &CounterValue{Samples: 30, Value: 755},
		"ma5": &CounterValue{Samples: 50, Value: 655},
	}}, values[8])
	require.Equal(t, &CounterValue{Samples: 10, Value: 955, DerivedValues: map[string]Value{
		"ma3": &CounterValue{Samples: 30, Value: 855},
		"ma5": &CounterValue{Samples: 50, Value: 755},
	}}, values[9])
}

func TestTimeSeriesGauge(t *testing.T) {
	ts := NewTimeSeries(time.Second, 10, NewGauge())

	now := time.Date(2025, 07, 21, 17, 31, 12, 0, time.FixedZone("Asia/Seoul", 9*60*60))
	nowFunc = func() time.Time {
		ret := now
		now = now.Add(time.Millisecond * 100)
		return ret
	}

	for i := 1; i <= 100; i++ {
		ts.Add(float64(i))
	}
	times, values := ts.LastN(-1)
	require.Equal(t, []time.Time{
		time.Date(2025, 07, 21, 17, 31, 13, 0, time.FixedZone("Asia/Seoul", 9*60*60)),
		time.Date(2025, 07, 21, 17, 31, 14, 0, time.FixedZone("Asia/Seoul", 9*60*60)),
		time.Date(2025, 07, 21, 17, 31, 15, 0, time.FixedZone("Asia/Seoul", 9*60*60)),
		time.Date(2025, 07, 21, 17, 31, 16, 0, time.FixedZone("Asia/Seoul", 9*60*60)),
		time.Date(2025, 07, 21, 17, 31, 17, 0, time.FixedZone("Asia/Seoul", 9*60*60)),
		time.Date(2025, 07, 21, 17, 31, 18, 0, time.FixedZone("Asia/Seoul", 9*60*60)),
		time.Date(2025, 07, 21, 17, 31, 19, 0, time.FixedZone("Asia/Seoul", 9*60*60)),
		time.Date(2025, 07, 21, 17, 31, 20, 0, time.FixedZone("Asia/Seoul", 9*60*60)),
		time.Date(2025, 07, 21, 17, 31, 21, 0, time.FixedZone("Asia/Seoul", 9*60*60)),
		time.Date(2025, 07, 21, 17, 31, 22, 0, time.FixedZone("Asia/Seoul", 9*60*60)),
	}, times)
	require.Equal(t, []Value{
		&GaugeValue{Samples: 10, Sum: 55, Value: 10},
		&GaugeValue{Samples: 10, Sum: 155, Value: 20},
		&GaugeValue{Samples: 10, Sum: 255, Value: 30},
		&GaugeValue{Samples: 10, Sum: 355, Value: 40},
		&GaugeValue{Samples: 10, Sum: 455, Value: 50},
		&GaugeValue{Samples: 10, Sum: 555, Value: 60},
		&GaugeValue{Samples: 10, Sum: 655, Value: 70},
		&GaugeValue{Samples: 10, Sum: 755, Value: 80},
		&GaugeValue{Samples: 10, Sum: 855, Value: 90},
		&GaugeValue{Samples: 10, Sum: 955, Value: 100},
	}, values)
}

func TestTimeSeriesGaugeWithSlidingWindow(t *testing.T) {
	ts := NewTimeSeries(time.Second, 10,
		NewGauge().WithDerivers(
			NewMovingAverage("ma3", 3),
			NewMovingAverage("ma5", 5),
		),
	)

	now := time.Date(2025, 07, 21, 17, 31, 12, 0, time.FixedZone("Asia/Seoul", 9*60*60))
	nowFunc = func() time.Time {
		ret := now
		now = now.Add(time.Millisecond * 100)
		return ret
	}

	for i := 1; i <= 100; i++ {
		ts.Add(float64(i))
	}
	times, values := ts.LastN(-1)
	require.Equal(t, []time.Time{
		time.Date(2025, 07, 21, 17, 31, 13, 0, time.FixedZone("Asia/Seoul", 9*60*60)),
		time.Date(2025, 07, 21, 17, 31, 14, 0, time.FixedZone("Asia/Seoul", 9*60*60)),
		time.Date(2025, 07, 21, 17, 31, 15, 0, time.FixedZone("Asia/Seoul", 9*60*60)),
		time.Date(2025, 07, 21, 17, 31, 16, 0, time.FixedZone("Asia/Seoul", 9*60*60)),
		time.Date(2025, 07, 21, 17, 31, 17, 0, time.FixedZone("Asia/Seoul", 9*60*60)),
		time.Date(2025, 07, 21, 17, 31, 18, 0, time.FixedZone("Asia/Seoul", 9*60*60)),
		time.Date(2025, 07, 21, 17, 31, 19, 0, time.FixedZone("Asia/Seoul", 9*60*60)),
		time.Date(2025, 07, 21, 17, 31, 20, 0, time.FixedZone("Asia/Seoul", 9*60*60)),
		time.Date(2025, 07, 21, 17, 31, 21, 0, time.FixedZone("Asia/Seoul", 9*60*60)),
		time.Date(2025, 07, 21, 17, 31, 22, 0, time.FixedZone("Asia/Seoul", 9*60*60)),
	}, times)
	require.Equal(t, &GaugeValue{Samples: 10, Sum: 55, Value: 10, DerivedValues: map[string]Value{
		"ma3": &GaugeValue{Samples: 10, Sum: 55, Value: 10},
		"ma5": &GaugeValue{Samples: 10, Sum: 55, Value: 10},
	}}, values[0])
	require.Equal(t, &GaugeValue{Samples: 10, Sum: 155, Value: 20, DerivedValues: map[string]Value{
		"ma3": &GaugeValue{Samples: 20, Sum: 210, Value: 15},
		"ma5": &GaugeValue{Samples: 20, Sum: 210, Value: 15},
	}}, values[1])
	require.Equal(t, &GaugeValue{Samples: 10, Sum: 255, Value: 30, DerivedValues: map[string]Value{
		"ma3": &GaugeValue{Samples: 30, Sum: 465, Value: 20},
		"ma5": &GaugeValue{Samples: 30, Sum: 465, Value: 20},
	}}, values[2])
	require.Equal(t, &GaugeValue{Samples: 10, Sum: 355, Value: 40, DerivedValues: map[string]Value{
		"ma3": &GaugeValue{Samples: 30, Sum: 765, Value: 30},
		"ma5": &GaugeValue{Samples: 40, Sum: 820, Value: 25},
	}}, values[3])
	require.Equal(t, &GaugeValue{Samples: 10, Sum: 455, Value: 50, DerivedValues: map[string]Value{
		"ma3": &GaugeValue{Samples: 30, Sum: 1065, Value: 40},
		"ma5": &GaugeValue{Samples: 50, Sum: 1275, Value: 30},
	}}, values[4])
	require.Equal(t, &GaugeValue{Samples: 10, Sum: 555, Value: 60, DerivedValues: map[string]Value{
		"ma3": &GaugeValue{Samples: 30, Sum: 1365, Value: 50},
		"ma5": &GaugeValue{Samples: 50, Sum: 1775, Value: 40},
	}}, values[5])
	require.Equal(t, &GaugeValue{Samples: 10, Sum: 655, Value: 70, DerivedValues: map[string]Value{
		"ma3": &GaugeValue{Samples: 30, Sum: 1665, Value: 60},
		"ma5": &GaugeValue{Samples: 50, Sum: 2275, Value: 50},
	}}, values[6])
	require.Equal(t, &GaugeValue{Samples: 10, Sum: 755, Value: 80, DerivedValues: map[string]Value{
		"ma3": &GaugeValue{Samples: 30, Sum: 1965, Value: 70},
		"ma5": &GaugeValue{Samples: 50, Sum: 2775, Value: 60},
	}}, values[7])
	require.Equal(t, &GaugeValue{Samples: 10, Sum: 855, Value: 90, DerivedValues: map[string]Value{
		"ma3": &GaugeValue{Samples: 30, Sum: 2265, Value: 80},
		"ma5": &GaugeValue{Samples: 50, Sum: 3275, Value: 70},
	}}, values[8])
	require.Equal(t, &GaugeValue{Samples: 10, Sum: 955, Value: 100, DerivedValues: map[string]Value{
		"ma3": &GaugeValue{Samples: 30, Sum: 2565, Value: 90},
		"ma5": &GaugeValue{Samples: 50, Sum: 3775, Value: 80},
	}}, values[9])
}

func TestTimeSeriesMeter(t *testing.T) {
	ts := NewTimeSeries(time.Second, 10, NewMeter())

	now := time.Date(2025, 07, 21, 17, 31, 12, 0, time.FixedZone("Asia/Seoul", 9*60*60))
	nowFunc = func() time.Time {
		ret := now
		now = now.Add(time.Millisecond * 100)
		return ret
	}

	for i := 1; i <= 100; i++ {
		ts.Add(float64(i))
	}

	times, values := ts.All()
	require.Equal(t, []time.Time{
		time.Date(2025, 07, 21, 17, 31, 13, 0, time.FixedZone("Asia/Seoul", 9*60*60)),
		time.Date(2025, 07, 21, 17, 31, 14, 0, time.FixedZone("Asia/Seoul", 9*60*60)),
		time.Date(2025, 07, 21, 17, 31, 15, 0, time.FixedZone("Asia/Seoul", 9*60*60)),
		time.Date(2025, 07, 21, 17, 31, 16, 0, time.FixedZone("Asia/Seoul", 9*60*60)),
		time.Date(2025, 07, 21, 17, 31, 17, 0, time.FixedZone("Asia/Seoul", 9*60*60)),
		time.Date(2025, 07, 21, 17, 31, 18, 0, time.FixedZone("Asia/Seoul", 9*60*60)),
		time.Date(2025, 07, 21, 17, 31, 19, 0, time.FixedZone("Asia/Seoul", 9*60*60)),
		time.Date(2025, 07, 21, 17, 31, 20, 0, time.FixedZone("Asia/Seoul", 9*60*60)),
		time.Date(2025, 07, 21, 17, 31, 21, 0, time.FixedZone("Asia/Seoul", 9*60*60)),
		time.Date(2025, 07, 21, 17, 31, 22, 0, time.FixedZone("Asia/Seoul", 9*60*60)),
	}, times)
	require.Equal(t, []Value{
		&MeterValue{Min: 1, Max: 10, First: 1, Last: 10, Sum: 55, Samples: 10},
		&MeterValue{Min: 11, Max: 20, First: 11, Last: 20, Sum: 155, Samples: 10},
		&MeterValue{Min: 21, Max: 30, First: 21, Last: 30, Sum: 255, Samples: 10},
		&MeterValue{Min: 31, Max: 40, First: 31, Last: 40, Sum: 355, Samples: 10},
		&MeterValue{Min: 41, Max: 50, First: 41, Last: 50, Sum: 455, Samples: 10},
		&MeterValue{Min: 51, Max: 60, First: 51, Last: 60, Sum: 555, Samples: 10},
		&MeterValue{Min: 61, Max: 70, First: 61, Last: 70, Sum: 655, Samples: 10},
		&MeterValue{Min: 71, Max: 80, First: 71, Last: 80, Sum: 755, Samples: 10},
		&MeterValue{Min: 81, Max: 90, First: 81, Last: 90, Sum: 855, Samples: 10},
		&MeterValue{Min: 91, Max: 100, First: 91, Last: 100, Sum: 955, Samples: 10},
	}, values)
}

func TestTimeSeriesMeterWithSlidingWindow(t *testing.T) {
	ts := NewTimeSeries(time.Second, 10,
		NewMeter().WithDerivers(
			NewMovingAverage("ma3", 3),
			NewMovingAverage("ma5", 5),
		),
	)

	now := time.Date(2025, 07, 21, 17, 31, 12, 0, time.FixedZone("Asia/Seoul", 9*60*60))
	nowFunc = func() time.Time {
		ret := now
		now = now.Add(time.Millisecond * 100)
		return ret
	}

	for i := 1; i <= 100; i++ {
		ts.Add(float64(i))
	}

	times, values := ts.All()
	require.Equal(t, []time.Time{
		time.Date(2025, 07, 21, 17, 31, 13, 0, time.FixedZone("Asia/Seoul", 9*60*60)),
		time.Date(2025, 07, 21, 17, 31, 14, 0, time.FixedZone("Asia/Seoul", 9*60*60)),
		time.Date(2025, 07, 21, 17, 31, 15, 0, time.FixedZone("Asia/Seoul", 9*60*60)),
		time.Date(2025, 07, 21, 17, 31, 16, 0, time.FixedZone("Asia/Seoul", 9*60*60)),
		time.Date(2025, 07, 21, 17, 31, 17, 0, time.FixedZone("Asia/Seoul", 9*60*60)),
		time.Date(2025, 07, 21, 17, 31, 18, 0, time.FixedZone("Asia/Seoul", 9*60*60)),
		time.Date(2025, 07, 21, 17, 31, 19, 0, time.FixedZone("Asia/Seoul", 9*60*60)),
		time.Date(2025, 07, 21, 17, 31, 20, 0, time.FixedZone("Asia/Seoul", 9*60*60)),
		time.Date(2025, 07, 21, 17, 31, 21, 0, time.FixedZone("Asia/Seoul", 9*60*60)),
		time.Date(2025, 07, 21, 17, 31, 22, 0, time.FixedZone("Asia/Seoul", 9*60*60)),
	}, times)
	require.Equal(t, &MeterValue{Min: 1, Max: 10, First: 1, Last: 10, Sum: 55, Samples: 10, DerivedValues: map[string]Value{
		"ma3": &MeterValue{Min: 1, Max: 10, First: 1, Last: 10, Sum: 55, Samples: 10},
		"ma5": &MeterValue{Min: 1, Max: 10, First: 1, Last: 10, Sum: 55, Samples: 10},
	}}, values[0])
	require.Equal(t, &MeterValue{Min: 11, Max: 20, First: 11, Last: 20, Sum: 155, Samples: 10, DerivedValues: map[string]Value{
		"ma3": &MeterValue{Min: 6, Max: 15, First: 6, Last: 15, Sum: 210, Samples: 20},
		"ma5": &MeterValue{Min: 6, Max: 15, First: 6, Last: 15, Sum: 210, Samples: 20},
	}}, values[1])
	require.Equal(t, &MeterValue{Min: 21, Max: 30, First: 21, Last: 30, Sum: 255, Samples: 10, DerivedValues: map[string]Value{
		"ma3": &MeterValue{Min: 11, Max: 20, First: 11, Last: 20, Sum: 465, Samples: 30},
		"ma5": &MeterValue{Min: 11, Max: 20, First: 11, Last: 20, Sum: 465, Samples: 30},
	}}, values[2])
	require.Equal(t, &MeterValue{Min: 31, Max: 40, First: 31, Last: 40, Sum: 355, Samples: 10, DerivedValues: map[string]Value{
		"ma3": &MeterValue{Min: 21, Max: 30, First: 21, Last: 30, Sum: 765, Samples: 30},
		"ma5": &MeterValue{Min: 16, Max: 25, First: 16, Last: 25, Sum: 820, Samples: 40},
	}}, values[3])
	require.Equal(t, &MeterValue{Min: 41, Max: 50, First: 41, Last: 50, Sum: 455, Samples: 10, DerivedValues: map[string]Value{
		"ma3": &MeterValue{Min: 31, Max: 40, First: 31, Last: 40, Sum: 1065, Samples: 30},
		"ma5": &MeterValue{Min: 21, Max: 30, First: 21, Last: 30, Sum: 1275, Samples: 50},
	}}, values[4])
	require.Equal(t, &MeterValue{Min: 51, Max: 60, First: 51, Last: 60, Sum: 555, Samples: 10, DerivedValues: map[string]Value{
		"ma3": &MeterValue{Min: 41, Max: 50, First: 41, Last: 50, Sum: 1365, Samples: 30},
		"ma5": &MeterValue{Min: 31, Max: 40, First: 31, Last: 40, Sum: 1775, Samples: 50},
	}}, values[5])
	require.Equal(t, &MeterValue{Min: 61, Max: 70, First: 61, Last: 70, Sum: 655, Samples: 10, DerivedValues: map[string]Value{
		"ma3": &MeterValue{Min: 51, Max: 60, First: 51, Last: 60, Sum: 1665, Samples: 30},
		"ma5": &MeterValue{Min: 41, Max: 50, First: 41, Last: 50, Sum: 2275, Samples: 50},
	}}, values[6])
	require.Equal(t, &MeterValue{Min: 71, Max: 80, First: 71, Last: 80, Sum: 755, Samples: 10, DerivedValues: map[string]Value{
		"ma3": &MeterValue{Min: 61, Max: 70, First: 61, Last: 70, Sum: 1965, Samples: 30},
		"ma5": &MeterValue{Min: 51, Max: 60, First: 51, Last: 60, Sum: 2775, Samples: 50},
	}}, values[7])
	require.Equal(t, &MeterValue{Min: 81, Max: 90, First: 81, Last: 90, Sum: 855, Samples: 10, DerivedValues: map[string]Value{
		"ma3": &MeterValue{Min: 71, Max: 80, First: 71, Last: 80, Sum: 2265, Samples: 30},
		"ma5": &MeterValue{Min: 61, Max: 70, First: 61, Last: 70, Sum: 3275, Samples: 50},
	}}, values[8])
	require.Equal(t, &MeterValue{Min: 91, Max: 100, First: 91, Last: 100, Sum: 955, Samples: 10, DerivedValues: map[string]Value{
		"ma3": &MeterValue{Min: 81, Max: 90, First: 81, Last: 90, Sum: 2565, Samples: 30},
		"ma5": &MeterValue{Min: 71, Max: 80, First: 71, Last: 80, Sum: 3775, Samples: 50},
	}}, values[9])
}

func TestTimeSeriesTimer(t *testing.T) {
	ts := NewTimeSeries(time.Second, 10, NewTimer())

	now := time.Date(2025, 07, 21, 17, 31, 12, 0, time.FixedZone("Asia/Seoul", 9*60*60))
	nowFunc = func() time.Time {
		ret := now
		now = now.Add(time.Millisecond * 100)
		return ret
	}

	for i := 1; i <= 100; i++ {
		ts.Add(float64(time.Duration(i) * time.Second))
	}

	times, values := ts.All()
	require.Equal(t, []time.Time{
		time.Date(2025, 07, 21, 17, 31, 13, 0, time.FixedZone("Asia/Seoul", 9*60*60)),
		time.Date(2025, 07, 21, 17, 31, 14, 0, time.FixedZone("Asia/Seoul", 9*60*60)),
		time.Date(2025, 07, 21, 17, 31, 15, 0, time.FixedZone("Asia/Seoul", 9*60*60)),
		time.Date(2025, 07, 21, 17, 31, 16, 0, time.FixedZone("Asia/Seoul", 9*60*60)),
		time.Date(2025, 07, 21, 17, 31, 17, 0, time.FixedZone("Asia/Seoul", 9*60*60)),
		time.Date(2025, 07, 21, 17, 31, 18, 0, time.FixedZone("Asia/Seoul", 9*60*60)),
		time.Date(2025, 07, 21, 17, 31, 19, 0, time.FixedZone("Asia/Seoul", 9*60*60)),
		time.Date(2025, 07, 21, 17, 31, 20, 0, time.FixedZone("Asia/Seoul", 9*60*60)),
		time.Date(2025, 07, 21, 17, 31, 21, 0, time.FixedZone("Asia/Seoul", 9*60*60)),
		time.Date(2025, 07, 21, 17, 31, 22, 0, time.FixedZone("Asia/Seoul", 9*60*60)),
	}, times)
	require.Equal(t, []Value{
		&TimerValue{Min: time.Duration(1) * time.Second, Max: time.Duration(10) * time.Second, Sum: time.Duration(55) * time.Second, Samples: 10},
		&TimerValue{Min: time.Duration(11) * time.Second, Max: time.Duration(20) * time.Second, Sum: time.Duration(155) * time.Second, Samples: 10},
		&TimerValue{Min: time.Duration(21) * time.Second, Max: time.Duration(30) * time.Second, Sum: time.Duration(255) * time.Second, Samples: 10},
		&TimerValue{Min: time.Duration(31) * time.Second, Max: time.Duration(40) * time.Second, Sum: time.Duration(355) * time.Second, Samples: 10},
		&TimerValue{Min: time.Duration(41) * time.Second, Max: time.Duration(50) * time.Second, Sum: time.Duration(455) * time.Second, Samples: 10},
		&TimerValue{Min: time.Duration(51) * time.Second, Max: time.Duration(60) * time.Second, Sum: time.Duration(555) * time.Second, Samples: 10},
		&TimerValue{Min: time.Duration(61) * time.Second, Max: time.Duration(70) * time.Second, Sum: time.Duration(655) * time.Second, Samples: 10},
		&TimerValue{Min: time.Duration(71) * time.Second, Max: time.Duration(80) * time.Second, Sum: time.Duration(755) * time.Second, Samples: 10},
		&TimerValue{Min: time.Duration(81) * time.Second, Max: time.Duration(90) * time.Second, Sum: time.Duration(855) * time.Second, Samples: 10},
		&TimerValue{Min: time.Duration(91) * time.Second, Max: time.Duration(100) * time.Second, Sum: time.Duration(955) * time.Second, Samples: 10},
	}, values)
}

func TestTimeSeriesTimerWithSlidingWindow(t *testing.T) {
	ts := NewTimeSeries(time.Second, 10, NewTimer().WithDerivers(
		NewMovingAverage("ma3", 3),
		NewMovingAverage("ma5", 5),
	))

	now := time.Date(2025, 07, 21, 17, 31, 12, 0, time.FixedZone("Asia/Seoul", 9*60*60))
	nowFunc = func() time.Time {
		ret := now
		now = now.Add(time.Millisecond * 100)
		return ret
	}

	for i := 1; i <= 100; i++ {
		ts.Add(float64(time.Duration(i) * time.Second))
	}

	times, values := ts.All()
	require.Equal(t, []time.Time{
		time.Date(2025, 07, 21, 17, 31, 13, 0, time.FixedZone("Asia/Seoul", 9*60*60)),
		time.Date(2025, 07, 21, 17, 31, 14, 0, time.FixedZone("Asia/Seoul", 9*60*60)),
		time.Date(2025, 07, 21, 17, 31, 15, 0, time.FixedZone("Asia/Seoul", 9*60*60)),
		time.Date(2025, 07, 21, 17, 31, 16, 0, time.FixedZone("Asia/Seoul", 9*60*60)),
		time.Date(2025, 07, 21, 17, 31, 17, 0, time.FixedZone("Asia/Seoul", 9*60*60)),
		time.Date(2025, 07, 21, 17, 31, 18, 0, time.FixedZone("Asia/Seoul", 9*60*60)),
		time.Date(2025, 07, 21, 17, 31, 19, 0, time.FixedZone("Asia/Seoul", 9*60*60)),
		time.Date(2025, 07, 21, 17, 31, 20, 0, time.FixedZone("Asia/Seoul", 9*60*60)),
		time.Date(2025, 07, 21, 17, 31, 21, 0, time.FixedZone("Asia/Seoul", 9*60*60)),
		time.Date(2025, 07, 21, 17, 31, 22, 0, time.FixedZone("Asia/Seoul", 9*60*60)),
	}, times)
	require.Equal(t, &TimerValue{Min: time.Duration(1) * time.Second, Max: time.Duration(10) * time.Second, Sum: time.Duration(55) * time.Second, Samples: 10, DerivedValues: map[string]Value{
		"ma3": &TimerValue{Min: time.Duration(1) * time.Second, Max: time.Duration(10) * time.Second, Sum: time.Duration(55) * time.Second, Samples: 10},
		"ma5": &TimerValue{Min: time.Duration(1) * time.Second, Max: time.Duration(10) * time.Second, Sum: time.Duration(55) * time.Second, Samples: 10},
	}}, values[0])
	require.Equal(t, &TimerValue{Min: time.Duration(11) * time.Second, Max: time.Duration(20) * time.Second, Sum: time.Duration(155) * time.Second, Samples: 10, DerivedValues: map[string]Value{
		"ma3": &TimerValue{Min: time.Duration(6) * time.Second, Max: time.Duration(15) * time.Second, Sum: time.Duration(210) * time.Second, Samples: 20},
		"ma5": &TimerValue{Min: time.Duration(6) * time.Second, Max: time.Duration(15) * time.Second, Sum: time.Duration(210) * time.Second, Samples: 20},
	}}, values[1])
	require.Equal(t, &TimerValue{Min: time.Duration(21) * time.Second, Max: time.Duration(30) * time.Second, Sum: time.Duration(255) * time.Second, Samples: 10, DerivedValues: map[string]Value{
		"ma3": &TimerValue{Min: time.Duration(11) * time.Second, Max: time.Duration(20) * time.Second, Sum: time.Duration(465) * time.Second, Samples: 30},
		"ma5": &TimerValue{Min: time.Duration(11) * time.Second, Max: time.Duration(20) * time.Second, Sum: time.Duration(465) * time.Second, Samples: 30},
	}}, values[2])
	require.Equal(t, &TimerValue{Min: time.Duration(31) * time.Second, Max: time.Duration(40) * time.Second, Sum: time.Duration(355) * time.Second, Samples: 10, DerivedValues: map[string]Value{
		"ma3": &TimerValue{Min: time.Duration(21) * time.Second, Max: time.Duration(30) * time.Second, Sum: time.Duration(765) * time.Second, Samples: 30},
		"ma5": &TimerValue{Min: time.Duration(16) * time.Second, Max: time.Duration(25) * time.Second, Sum: time.Duration(820) * time.Second, Samples: 40},
	}}, values[3])
	require.Equal(t, &TimerValue{Min: time.Duration(41) * time.Second, Max: time.Duration(50) * time.Second, Sum: time.Duration(455) * time.Second, Samples: 10, DerivedValues: map[string]Value{
		"ma3": &TimerValue{Min: time.Duration(31) * time.Second, Max: time.Duration(40) * time.Second, Sum: time.Duration(1065) * time.Second, Samples: 30},
		"ma5": &TimerValue{Min: time.Duration(21) * time.Second, Max: time.Duration(30) * time.Second, Sum: time.Duration(1275) * time.Second, Samples: 50},
	}}, values[4])
	require.Equal(t, &TimerValue{Min: time.Duration(51) * time.Second, Max: time.Duration(60) * time.Second, Sum: time.Duration(555) * time.Second, Samples: 10, DerivedValues: map[string]Value{
		"ma3": &TimerValue{Min: time.Duration(41) * time.Second, Max: time.Duration(50) * time.Second, Sum: time.Duration(1365) * time.Second, Samples: 30},
		"ma5": &TimerValue{Min: time.Duration(31) * time.Second, Max: time.Duration(40) * time.Second, Sum: time.Duration(1775) * time.Second, Samples: 50},
	}}, values[5])
	require.Equal(t, &TimerValue{Min: time.Duration(61) * time.Second, Max: time.Duration(70) * time.Second, Sum: time.Duration(655) * time.Second, Samples: 10, DerivedValues: map[string]Value{
		"ma3": &TimerValue{Min: time.Duration(51) * time.Second, Max: time.Duration(60) * time.Second, Sum: time.Duration(1665) * time.Second, Samples: 30},
		"ma5": &TimerValue{Min: time.Duration(41) * time.Second, Max: time.Duration(50) * time.Second, Sum: time.Duration(2275) * time.Second, Samples: 50},
	}}, values[6])
	require.Equal(t, &TimerValue{Min: time.Duration(71) * time.Second, Max: time.Duration(80) * time.Second, Sum: time.Duration(755) * time.Second, Samples: 10, DerivedValues: map[string]Value{
		"ma3": &TimerValue{Min: time.Duration(61) * time.Second, Max: time.Duration(70) * time.Second, Sum: time.Duration(1965) * time.Second, Samples: 30},
		"ma5": &TimerValue{Min: time.Duration(51) * time.Second, Max: time.Duration(60) * time.Second, Sum: time.Duration(2775) * time.Second, Samples: 50},
	}}, values[7])
	require.Equal(t, &TimerValue{Min: time.Duration(81) * time.Second, Max: time.Duration(90) * time.Second, Sum: time.Duration(855) * time.Second, Samples: 10, DerivedValues: map[string]Value{
		"ma3": &TimerValue{Min: time.Duration(71) * time.Second, Max: time.Duration(80) * time.Second, Sum: time.Duration(2265) * time.Second, Samples: 30},
		"ma5": &TimerValue{Min: time.Duration(61) * time.Second, Max: time.Duration(70) * time.Second, Sum: time.Duration(3275) * time.Second, Samples: 50},
	}}, values[8])
	require.Equal(t, &TimerValue{Min: time.Duration(91) * time.Second, Max: time.Duration(100) * time.Second, Sum: time.Duration(955) * time.Second, Samples: 10, DerivedValues: map[string]Value{
		"ma3": &TimerValue{Min: time.Duration(81) * time.Second, Max: time.Duration(90) * time.Second, Sum: time.Duration(2565) * time.Second, Samples: 30},
		"ma5": &TimerValue{Min: time.Duration(71) * time.Second, Max: time.Duration(80) * time.Second, Sum: time.Duration(3775) * time.Second, Samples: 50},
	}}, values[9])
}

func TestTimeSeriesHistogram(t *testing.T) {
	ts := NewTimeSeries(time.Second, 10, NewHistogram(100, 0.5, 0.75, 0.99))

	now := time.Date(2025, 07, 21, 17, 31, 12, 0, time.FixedZone("Asia/Seoul", 9*60*60))
	nowFunc = func() time.Time {
		ret := now
		now = now.Add(time.Millisecond * 100)
		return ret
	}

	for i := 1; i <= 100; i++ {
		ts.Add(float64(i))
	}

	times, values := ts.LastN(0)
	require.Equal(t, []time.Time{
		time.Date(2025, 07, 21, 17, 31, 13, 0, time.FixedZone("Asia/Seoul", 9*60*60)),
		time.Date(2025, 07, 21, 17, 31, 14, 0, time.FixedZone("Asia/Seoul", 9*60*60)),
		time.Date(2025, 07, 21, 17, 31, 15, 0, time.FixedZone("Asia/Seoul", 9*60*60)),
		time.Date(2025, 07, 21, 17, 31, 16, 0, time.FixedZone("Asia/Seoul", 9*60*60)),
		time.Date(2025, 07, 21, 17, 31, 17, 0, time.FixedZone("Asia/Seoul", 9*60*60)),
		time.Date(2025, 07, 21, 17, 31, 18, 0, time.FixedZone("Asia/Seoul", 9*60*60)),
		time.Date(2025, 07, 21, 17, 31, 19, 0, time.FixedZone("Asia/Seoul", 9*60*60)),
		time.Date(2025, 07, 21, 17, 31, 20, 0, time.FixedZone("Asia/Seoul", 9*60*60)),
		time.Date(2025, 07, 21, 17, 31, 21, 0, time.FixedZone("Asia/Seoul", 9*60*60)),
		time.Date(2025, 07, 21, 17, 31, 22, 0, time.FixedZone("Asia/Seoul", 9*60*60)),
	}, times)
	require.Equal(t, []Value{
		&HistogramValue{Samples: 10, P: []float64{0.5, 0.75, 0.99}, Values: []float64{5, 8, 10}},
		&HistogramValue{Samples: 10, P: []float64{0.5, 0.75, 0.99}, Values: []float64{15, 18, 20}},
		&HistogramValue{Samples: 10, P: []float64{0.5, 0.75, 0.99}, Values: []float64{25, 28, 30}},
		&HistogramValue{Samples: 10, P: []float64{0.5, 0.75, 0.99}, Values: []float64{35, 38, 40}},
		&HistogramValue{Samples: 10, P: []float64{0.5, 0.75, 0.99}, Values: []float64{45, 48, 50}},
		&HistogramValue{Samples: 10, P: []float64{0.5, 0.75, 0.99}, Values: []float64{55, 58, 60}},
		&HistogramValue{Samples: 10, P: []float64{0.5, 0.75, 0.99}, Values: []float64{65, 68, 70}},
		&HistogramValue{Samples: 10, P: []float64{0.5, 0.75, 0.99}, Values: []float64{75, 78, 80}},
		&HistogramValue{Samples: 10, P: []float64{0.5, 0.75, 0.99}, Values: []float64{85, 88, 90}},
		&HistogramValue{Samples: 10, P: []float64{0.5, 0.75, 0.99}, Values: []float64{95, 98, 100}},
	}, values)
}

func TestTimeSeriesHistogramWithSlidingWindow(t *testing.T) {
	ts := NewTimeSeries(time.Second, 10, NewHistogram(100, 0.5, 0.75, 0.99).WithDerivers(
		NewMovingAverage("ma3", 3),
		NewMovingAverage("ma5", 5),
	))

	now := time.Date(2025, 07, 21, 17, 31, 12, 0, time.FixedZone("Asia/Seoul", 9*60*60))
	nowFunc = func() time.Time {
		ret := now
		now = now.Add(time.Millisecond * 100)
		return ret
	}

	for i := 1; i <= 100; i++ {
		ts.Add(float64(i))
	}

	times, values := ts.LastN(0)
	require.Equal(t, []time.Time{
		time.Date(2025, 07, 21, 17, 31, 13, 0, time.FixedZone("Asia/Seoul", 9*60*60)),
		time.Date(2025, 07, 21, 17, 31, 14, 0, time.FixedZone("Asia/Seoul", 9*60*60)),
		time.Date(2025, 07, 21, 17, 31, 15, 0, time.FixedZone("Asia/Seoul", 9*60*60)),
		time.Date(2025, 07, 21, 17, 31, 16, 0, time.FixedZone("Asia/Seoul", 9*60*60)),
		time.Date(2025, 07, 21, 17, 31, 17, 0, time.FixedZone("Asia/Seoul", 9*60*60)),
		time.Date(2025, 07, 21, 17, 31, 18, 0, time.FixedZone("Asia/Seoul", 9*60*60)),
		time.Date(2025, 07, 21, 17, 31, 19, 0, time.FixedZone("Asia/Seoul", 9*60*60)),
		time.Date(2025, 07, 21, 17, 31, 20, 0, time.FixedZone("Asia/Seoul", 9*60*60)),
		time.Date(2025, 07, 21, 17, 31, 21, 0, time.FixedZone("Asia/Seoul", 9*60*60)),
		time.Date(2025, 07, 21, 17, 31, 22, 0, time.FixedZone("Asia/Seoul", 9*60*60)),
	}, times)
	require.Equal(t, &HistogramValue{Samples: 10, P: []float64{0.5, 0.75, 0.99}, Values: []float64{5, 8, 10}, DerivedValues: map[string]Value{
		"ma3": &HistogramValue{Samples: 10, P: []float64{0.5, 0.75, 0.99}, Values: []float64{5, 8, 10}},
		"ma5": &HistogramValue{Samples: 10, P: []float64{0.5, 0.75, 0.99}, Values: []float64{5, 8, 10}},
	}}, values[0])
	require.Equal(t, &HistogramValue{Samples: 10, P: []float64{0.5, 0.75, 0.99}, Values: []float64{15, 18, 20}, DerivedValues: map[string]Value{
		"ma3": &HistogramValue{Samples: 20, P: []float64{0.5, 0.75, 0.99}, Values: []float64{10, 13, 15}},
		"ma5": &HistogramValue{Samples: 20, P: []float64{0.5, 0.75, 0.99}, Values: []float64{10, 13, 15}},
	}}, values[1])
	require.Equal(t, &HistogramValue{Samples: 10, P: []float64{0.5, 0.75, 0.99}, Values: []float64{25, 28, 30}, DerivedValues: map[string]Value{
		"ma3": &HistogramValue{Samples: 30, P: []float64{0.5, 0.75, 0.99}, Values: []float64{15, 18, 20}},
		"ma5": &HistogramValue{Samples: 30, P: []float64{0.5, 0.75, 0.99}, Values: []float64{15, 18, 20}},
	}}, values[2])
	require.Equal(t, &HistogramValue{Samples: 10, P: []float64{0.5, 0.75, 0.99}, Values: []float64{35, 38, 40}, DerivedValues: map[string]Value{
		"ma3": &HistogramValue{Samples: 30, P: []float64{0.5, 0.75, 0.99}, Values: []float64{25, 28, 30}},
		"ma5": &HistogramValue{Samples: 40, P: []float64{0.5, 0.75, 0.99}, Values: []float64{20, 23, 25}},
	}}, values[3])
	require.Equal(t, &HistogramValue{Samples: 10, P: []float64{0.5, 0.75, 0.99}, Values: []float64{45, 48, 50}, DerivedValues: map[string]Value{
		"ma3": &HistogramValue{Samples: 30, P: []float64{0.5, 0.75, 0.99}, Values: []float64{35, 38, 40}},
		"ma5": &HistogramValue{Samples: 50, P: []float64{0.5, 0.75, 0.99}, Values: []float64{25, 28, 30}},
	}}, values[4])
	require.Equal(t, &HistogramValue{Samples: 10, P: []float64{0.5, 0.75, 0.99}, Values: []float64{55, 58, 60}, DerivedValues: map[string]Value{
		"ma3": &HistogramValue{Samples: 30, P: []float64{0.5, 0.75, 0.99}, Values: []float64{45, 48, 50}},
		"ma5": &HistogramValue{Samples: 50, P: []float64{0.5, 0.75, 0.99}, Values: []float64{35, 38, 40}},
	}}, values[5])
	require.Equal(t, &HistogramValue{Samples: 10, P: []float64{0.5, 0.75, 0.99}, Values: []float64{65, 68, 70}, DerivedValues: map[string]Value{
		"ma3": &HistogramValue{Samples: 30, P: []float64{0.5, 0.75, 0.99}, Values: []float64{55, 58, 60}},
		"ma5": &HistogramValue{Samples: 50, P: []float64{0.5, 0.75, 0.99}, Values: []float64{45, 48, 50}},
	}}, values[6])
	require.Equal(t, &HistogramValue{Samples: 10, P: []float64{0.5, 0.75, 0.99}, Values: []float64{75, 78, 80}, DerivedValues: map[string]Value{
		"ma3": &HistogramValue{Samples: 30, P: []float64{0.5, 0.75, 0.99}, Values: []float64{65, 68, 70}},
		"ma5": &HistogramValue{Samples: 50, P: []float64{0.5, 0.75, 0.99}, Values: []float64{55, 58, 60}},
	}}, values[7])
	require.Equal(t, &HistogramValue{Samples: 10, P: []float64{0.5, 0.75, 0.99}, Values: []float64{85, 88, 90}, DerivedValues: map[string]Value{
		"ma3": &HistogramValue{Samples: 30, P: []float64{0.5, 0.75, 0.99}, Values: []float64{75, 78, 80}},
		"ma5": &HistogramValue{Samples: 50, P: []float64{0.5, 0.75, 0.99}, Values: []float64{65, 68, 70}},
	}}, values[8])
	require.Equal(t, &HistogramValue{Samples: 10, P: []float64{0.5, 0.75, 0.99}, Values: []float64{95, 98, 100}, DerivedValues: map[string]Value{
		"ma3": &HistogramValue{Samples: 30, P: []float64{0.5, 0.75, 0.99}, Values: []float64{85, 88, 90}},
		"ma5": &HistogramValue{Samples: 50, P: []float64{0.5, 0.75, 0.99}, Values: []float64{75, 78, 80}},
	}}, values[9])
}
