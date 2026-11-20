package spi_test

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSQLUnsignedBindings(t *testing.T) {
	ctx := t.Context()
	dsn := fmt.Sprintf("server=127.0.0.1:%d;user=sys;password=manager", testServer.MachPort())
	db, err := sql.Open("machbase", dsn)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, db.Close()) })

	_, err = db.ExecContext(ctx, "CREATE TABLE SQL_UNSIGNED_BIND (id INTEGER, u32 UINTEGER, u64 ULONG)")
	require.NoError(t, err)
	t.Cleanup(func() {
		_, err := db.ExecContext(context.Background(), "DROP TABLE SQL_UNSIGNED_BIND")
		require.NoError(t, err)
	})

	insert, err := db.PrepareContext(ctx, "INSERT INTO SQL_UNSIGNED_BIND VALUES (:id, :u32, :u64)")
	require.NoError(t, err)
	defer insert.Close()

	const high32 = uint32(math.MaxInt32) + 1
	const high64 = uint64(math.MaxInt64) + 1
	max32, max64 := uint32(math.MaxUint32-1), uint64(math.MaxUint64-1)
	_, err = insert.ExecContext(ctx, sql.Named("id", int32(1)), sql.Named("u32", high32), sql.Named("u64", high64))
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, "INSERT INTO SQL_UNSIGNED_BIND VALUES (?, ?, ?)", int32(2), &max32, &max64)
	require.NoError(t, err)
	_, err = insert.ExecContext(ctx, sql.Named("id", int32(3)), sql.Named("u32", uint32(0)), sql.Named("u64", uint64(math.MaxInt64)))
	require.NoError(t, err)
	_, err = insert.ExecContext(ctx, sql.Named("id", int32(4)), sql.Named("u32", (*uint32)(nil)), sql.Named("u64", (*uint64)(nil)))
	require.NoError(t, err)

	for _, row := range []struct {
		id  int32
		u32 uint32
		u64 uint64
		raw any
	}{
		{1, high32, high64, "9223372036854775808"},
		{2, max32, max64, "18446744073709551614"},
		{3, 0, math.MaxInt64, int64(math.MaxInt64)},
	} {
		var got32 uint32
		var got64 uint64
		err := db.QueryRowContext(ctx, "SELECT u32, u64 FROM SQL_UNSIGNED_BIND WHERE id = ?", row.id).Scan(&got32, &got64)
		require.NoError(t, err)
		require.Equal(t, row.u32, got32)
		require.Equal(t, row.u64, got64)

		var raw any
		err = db.QueryRowContext(ctx, "SELECT u64 FROM SQL_UNSIGNED_BIND WHERE u32 = ? AND u64 = ?", row.u32, row.u64).Scan(&raw)
		require.NoError(t, err)
		require.Equal(t, row.raw, raw)
	}

	var null32 sql.Null[uint32]
	var null64 sql.Null[uint64]
	err = db.QueryRowContext(ctx, "SELECT u32, u64 FROM SQL_UNSIGNED_BIND WHERE id = 4").Scan(&null32, &null64)
	require.NoError(t, err)
	require.False(t, null32.Valid)
	require.False(t, null64.Valid)

	_, err = insert.ExecContext(ctx, int32(5), uint32(math.MaxUint32), uint64(0))
	require.ErrorContains(t, err, "reserved for Machbase NULL")
	_, err = insert.ExecContext(ctx, int32(5), uint32(0), uint64(math.MaxUint64))
	require.ErrorContains(t, err, "reserved for Machbase NULL")
}
