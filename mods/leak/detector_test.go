package leak_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/machbase/neo-server/mods/leak"
	"github.com/stretchr/testify/require"
)

/*
Require moq:

- Run

	moq -out ./mods/leak/mock_test.go -pkg leak_test ../neo-spi Rows Appender
*/

func TestDetector(t *testing.T) {
	det := leak.NewDetector(leak.Timer(100 * time.Millisecond))
	require.NotNil(t, det)

	det.Start()

	rows := &RowsMock{
		CloseFunc: func() error { return nil },
	}
	rowsWrap := det.DetainRows(rows, "select * from example")
	det.EnlistDetective(rows, "select * from exmaple limit 10")
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
	var count int

	for i, item := range det.Inflights() {
		count++
		fmt.Println(i, item.Id, item.Elapsed, item.Type, item.SqlText)
	}
	require.Equal(t, 3, count)

	rowsWrap, err = det.Rows(rowsWrap.Id())
	require.Nil(t, err)
	rowsWrap.Release()

	appenderWrap, err = det.Appender(appenderWrap.Id())
	require.Nil(t, err)
	appenderWrap.Release()

	det.DelistDetective(rows)
	det.Detect()

	count = 0
	for i, item := range det.Inflights() {
		count++
		fmt.Println(i, item.Id, item.Elapsed, item.Type, item.SqlText)
	}
	require.Equal(t, 0, count)

	count = 0
	for i, item := range det.Postflights() {
		count++
		fmt.Println(i, item.Count, item.SqlText)
	}
	require.Equal(t, 2, count)

	det.Stop()
}
