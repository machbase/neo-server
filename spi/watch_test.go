package spi_test

import (
	"database/sql"
	"testing"
	"time"

	"github.com/machbase/neo-server/v8/spi"
	"github.com/stretchr/testify/require"
)

func TestWatchLogTableInitialEmptyExecuteDoesNotError(t *testing.T) {
	db, err := spi.DefaultPool()
	require.NoError(t, err, "connect fail")

	conn, err := db.Conn(t.Context())
	require.NoError(t, err, "connect fail")
	_, err = conn.ExecContext(t.Context(), `create table if not exists watch_log_empty (
		time datetime,
		value double,
		memo varchar(80)
	)`)
	require.NoError(t, err, "create table fail")
	conn.Close()

	conf := spi.WatcherConfig{
		ConnProvider: func() (*sql.Conn, error) {
			return db.Conn(t.Context())
		},
		Timeformat: "2006-01-02 15:04:05.999999",
		Timezone:   time.UTC,
		TableName:  "watch_log_empty",
		MaxRowNum:  20,
	}
	w, err := spi.NewWatcher(t.Context(), conf)
	require.NoError(t, err, "new watcher fail")
	defer w.Close()

	w.Execute()

	select {
	case data := <-w.C:
		require.Failf(t, "unexpected watcher output", "%T: %#v", data, data)
	case <-time.After(250 * time.Millisecond):
	}
}
