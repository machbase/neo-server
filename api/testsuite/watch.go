package testsuite

import (
	"context"
	"testing"
	"time"

	"github.com/machbase/neo-server/api"
	"github.com/stretchr/testify/require"
)

func WatchLogTable(t *testing.T, db api.Database, ctx context.Context) {
	conf := api.WatcherConfig{
		ConnProvider: func() (api.Conn, error) {
			return db.Connect(ctx, api.WithTrustUser("sys"))
		},
		Timeformat: "2006-01-02 15:04:05.999999",
		Timezone:   time.UTC,
		TableName:  "tag_data",
		TagNames:   []string{"tag1", "tag2"},
	}
	w, err := api.NewWatcher(ctx, conf)
	require.NoError(t, err, "new watcher fail")
	defer w.Close()

	tick := time.NewTicker(2 * time.Second)
	defer tick.Stop()
	tickCount := 0

	for {
		select {
		case data := <-w.C:
			if err, ok := data.(error); ok {
				t.Log("Error", err.Error())
				t.Fail()
				return
			} else if rec, ok := data.(api.WatchData); !ok {
				t.Log("Data", data)
				t.Fail()
				return
			} else {
				if tickCount > 5 {
					return
				}
				require.Equal(t, 4, len(rec["NAME"].(string)), "NAME")
				require.IsType(t, "", rec["TIME"], "TIME")
				require.LessOrEqual(t, 1.23, rec["VALUE"], "VALUE")
				require.Equal(t, int16(1), rec["SHORT_VALUE"], "SHORT_VALUE")
				require.Less(t, int32(0), rec["INT_VALUE"], "INT_VALUE")
				require.Equal(t, int64(2), rec["LONG_VALUE"], "LONG_VALUE")
				require.Equal(t, "str1", rec["STR_VALUE"], "STR_VALUE")
				require.Equal(t, `{"key1":"value1"}`, rec["JSON_VALUE"], "JSON_VALUE")
			}
		case <-tick.C:
			tickCount++
			conn, err := conf.ConnProvider()
			require.NoError(t, err, "connect fail")
			name := "tag1"
			if tickCount%2 == 0 {
				name = "tag2"
			}
			values := []any{name, time.Now(), 1.23 * float64(tickCount), 1, tickCount, 2, "str1", `{"key1":"value1"}`}
			result := conn.Exec(ctx, `insert into tag_data (name, time, value, short_value, int_value, long_value, str_value, json_value) values(?, ?, ?, ?, ?, ?, ?, ?)`, values...)
			conn.Close()
			require.NoError(t, result.Err(), "insert fail")
			time.Sleep(100 * time.Millisecond)
			w.Execute()
		}
	}
}
