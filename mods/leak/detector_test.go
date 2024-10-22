package leak_test

import (
	"testing"
	"time"

	"github.com/machbase/neo-server/mods/leak"
	"github.com/stretchr/testify/require"
)

//go:generate moq -out ./mock_test.go -pkg leak_test ../../api Rows Appender

func TestDetector(t *testing.T) {
	det := leak.NewDetector(leak.Timer(100 * time.Millisecond))
	require.NotNil(t, det)

	det.Start()

	rows := &RowsMock{
		CloseFunc: func() error { return nil },
	}
	rowsWrap := det.DetainRows(rows, "select * from example")
	det.EnlistDetective(rows, "select * from example limit 10")
	det.UpdateDetective(rows)

	appender := &AppenderMock{
		CloseFunc: func() (int64, int64, error) { return 100, 0, nil },
	}
	appenderWrap := det.DetainAppender(appender, "example")

	for i := 0; i < 10; i++ {
		time.Sleep(20 * time.Millisecond)
		det.Detect()
	}

	var err error

	rowsWrap, err = det.Rows(rowsWrap.Id())
	require.Nil(t, err)
	rowsWrap.Release()

	appenderWrap, err = det.Appender(appenderWrap.Id())
	require.Nil(t, err)
	appenderWrap.Release()

	det.DelistDetective(rows)
	det.Detect()

	det.Stop()
}
