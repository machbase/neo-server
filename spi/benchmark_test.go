package spi_test

import (
	"database/sql/driver"
	"fmt"
	"net"
	"testing"
	"time"

	client "github.com/machbase/neo-client/v2"
	"github.com/machbase/neo-server/v8/spi"
	"github.com/stretchr/testify/require"
)

func BenchmarkTagDataAppend(b *testing.B) {
	conn, err := spi.Connect(b.Context(), "sys")
	require.NoError(b, err, "connect fail")
	defer conn.Close()
	_, err = conn.ExecContext(b.Context(), "truncate table tag_data")
	require.NoError(b, err, "truncate table fail")

	conn.Raw(func(driverConn any) error {
		rawConn := driverConn.(*client.Conn)
		appender, err := rawConn.Appender(b.Context(), "tag_data")
		require.NoError(b, err, "appender fail")
		defer appender.Close()

		for i := 0; i < b.N; i++ {
			err = appender.Append(
				fmt.Sprintf("append-bench-%d", i%100),
				time.Now().UnixNano(),
				1.001*float64(i+1),
				int16(i),
				uint16(i),
				int32(i),
				uint32(i),
				int64(i),
				uint64(i),
				fmt.Sprintf("str_value-%d", i),
				`{"t":"json"}`,
				net.IP([]byte{0x7f, 0x00, 0x00, 0x01}),
				net.IP([]byte{0x7f, 0x00, 0x00, 0x01}),
			)
			require.NoError(b, err, "append fail")
		}
		rawConn.Close()
		appender.Close()
		return driver.ErrBadConn
	})
}

func BenchmarkTagSimpleAppend(b *testing.B) {
	conn, err := spi.Connect(b.Context(), "sys")
	require.NoError(b, err, "connect fail")
	defer conn.Close()

	conn.Raw(func(driverConn any) error {
		rawConn := driverConn.(*client.Conn)
		appender, err := rawConn.Appender(b.Context(), "tag_simple")
		require.NoError(b, err, "appender fail")
		for i := 0; i < b.N; i++ {
			err = appender.Append(
				"bench-append",
				time.Now().UnixNano(),
				1.001*float64(i+1),
			)
			require.NoError(b, err, "append fail")
		}
		appender.Close()
		rawConn.Close()
		return driver.ErrBadConn
	})
}
