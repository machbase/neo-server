package api_test

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/machbase/neo-server/api"
	"github.com/machbase/neo-server/api/testsuite"
	"github.com/stretchr/testify/require"
)

func BenchmarkTagDataAppend(b *testing.B) {
	db := testsuite.Database_machsvr(b)

	ctx := context.TODO()
	conn, err := db.Connect(ctx, api.WithTrustUser("sys"))
	require.NoError(b, err, "connect fail")
	defer conn.Close()

	appender, err := conn.Appender(ctx, "tag_data")
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
}

func BenchmarkTagSimpleAppend(b *testing.B) {
	db := testsuite.Database_machsvr(b)

	ctx := context.TODO()
	conn, err := db.Connect(ctx, api.WithTrustUser("sys"))
	require.NoError(b, err, "connect fail")
	defer conn.Close()

	appender, err := conn.Appender(ctx, "tag_simple")
	require.NoError(b, err, "appender fail")
	defer appender.Close()

	for i := 0; i < b.N; i++ {
		err = appender.Append(
			"bench-append",
			time.Now().UnixNano(),
			1.001*float64(i+1),
		)
		require.NoError(b, err, "append fail")
	}
}
