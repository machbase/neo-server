package metric

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func ExampleTimer() {
	timer := &Timer{}

	// Simulate some work
	timer.Mark(1100 * time.Millisecond)

	timer.Mark(400 * time.Millisecond)

	fmt.Println("Avg:", timer.String())

	s := timer.Snapshot()
	fmt.Printf("%+v\n", s)

	// Output:
	//
	// Avg: 750ms
	// {Samples:2 TotalDuration:1.5s MinDuration:400ms MaxDuration:1.1s}
}

func TestTimer(t *testing.T) {
	timer := &Timer{}

	timer.Mark(10 * time.Millisecond)

	timer.Mark(20 * time.Millisecond)

	for i := 3; i <= 100; i++ {
		timer.Mark(time.Duration(i*10) * time.Millisecond)
	}
	require.Equal(t, timer.totalDuration, 50500*time.Millisecond)
	require.Equal(t, timer.samples, int64(100))
	require.Equal(t, 10*time.Millisecond, timer.minDuration)
	require.Equal(t, 1000*time.Millisecond, timer.maxDuration)
	require.Equal(t, `505ms`, timer.String())
}

func TestTimerJSON(t *testing.T) {
	tm := NewTimer()
	tm.Mark(100 * time.Millisecond)
	tm.Mark(200 * time.Millisecond)
	tm.Mark(300 * time.Millisecond)

	data, err := json.Marshal(tm)
	require.NoError(t, err)

	expected := `{"samples":3,"total":600000000,"min":100000000,"max":300000000}`
	require.JSONEq(t, expected, string(data))

	var tm2 Timer
	err = json.Unmarshal(data, &tm2)
	require.NoError(t, err)

	require.Equal(t, tm.samples, tm2.samples)
	require.Equal(t, tm.totalDuration, tm2.totalDuration)
	require.Equal(t, tm.minDuration, tm2.minDuration)
	require.Equal(t, tm.maxDuration, tm2.maxDuration)
}
