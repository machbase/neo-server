package tql

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestTimeWindowSetColumns(t *testing.T) {
	timeWindow := NewTimeWindow()
	require.EqualError(t, timeWindow.SetColumns([]string{"sum"}), "invalid columns count or no time column specified")
	require.EqualError(t, timeWindow.SetColumns([]string{"time", "sum:unknown"}), `unknown interpolation method "unknown"`)

	timeWindow = NewTimeWindow()
	require.NoError(t, timeWindow.SetColumns([]string{"time", "sum"}))
	require.Equal(t, 0, timeWindow.timeIdx)
	require.Len(t, timeWindow.columns, 2)
}

func TestTimeWindowOnEOF(t *testing.T) {
	node := NewNode(NewTask())
	node.output = make(chan *Record, 2)

	timeWindow := NewTimeWindow()
	timeWindow.tsFrom = time.Unix(0, 0)
	timeWindow.tsUntil = time.Unix(3, 0)
	timeWindow.period = time.Second
	require.NoError(t, timeWindow.SetColumns([]string{"time", "sum"}))

	timeWindow.onEOF(node)

	var records []*Record
	for len(records) < 2 {
		records = append(records, <-node.output)
	}
	require.Len(t, records, 2)
	for index, record := range records {
		require.Equal(t, time.Unix(int64(index+1), 0), record.Key())
		require.Equal(t, []any{time.Unix(int64(index+1), 0), nil}, record.Value())
	}
}

func TestTimeWindowInterpolationBuffer(t *testing.T) {
	node := NewNode(NewTask())
	node.output = make(chan *Record, 3)

	timeWindow := NewTimeWindow()
	timeWindow.bufferSize = 1
	timeWindow.hasInterp = true
	timeWindow.fillers = []TimeWindowFiller{
		&TimeWindowFillerConstant{value: nil},
		&TimeWindowFillerLinearRegression{fallback: &TimeWindowFillerConstant{value: -1.0}},
	}

	timeWindow.pushBuffer(node, time.Unix(1, 0), []any{time.Unix(1, 0), 1.0})
	timeWindow.pushBuffer(node, time.Unix(2, 0), []any{time.Unix(2, 0), 3.0})
	timeWindow.pushBuffer(node, time.Unix(3, 0), []any{time.Unix(3, 0), nil})
	timeWindow.flushBuffer(node)

	records := []*Record{<-node.output, <-node.output, <-node.output}
	require.Equal(t, []any{time.Unix(1, 0), 1.0}, records[0].Value())
	require.Equal(t, []any{time.Unix(2, 0), 3.0}, records[1].Value())
	require.Equal(t, []any{time.Unix(3, 0), 5.0}, records[2].Value())
}
