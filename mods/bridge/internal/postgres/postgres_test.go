package postgres_test

import (
	"context"
	"database/sql"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/machbase/neo-client/api"
	bridgepkg "github.com/machbase/neo-server/v8/mods/bridge"
	"github.com/machbase/neo-server/v8/mods/bridge/internal"
	bridgePostgres "github.com/machbase/neo-server/v8/mods/bridge/internal/postgres"
	"github.com/machbase/neo-server/v8/test"
	"github.com/ory/dockertest/v4"
	"github.com/stretchr/testify/require"
)

func TestPostgres(t *testing.T) {
	if !test.SupportDockerTest() {
		t.Skip("dockertest does not work in this environment")
	}
	pool := dockertest.NewPoolT(t, "")
	postgresRepository, postgresTag := test.PostgresDockerImage.Resolve()
	postgres := pool.RunT(t, postgresRepository,
		dockertest.WithTag(postgresTag),
		dockertest.WithEnv([]string{
			"POSTGRES_USER=dbuser",
			"POSTGRES_PASSWORD=dbpass",
			"POSTGRES_DB=db",
		}),
	)
	hostPort := postgres.GetHostPort("5432/tcp")
	host, port, _ := net.SplitHostPort(hostPort)
	dsn := fmt.Sprintf("host=%s port=%s dbname=db user=dbuser password=dbpass sslmode=disable", host, port)
	// wait for postgres to be ready
	err := pool.Retry(t.Context(), 30*time.Second, func() error {
		db, err := sql.Open("postgres", dsn)
		if err != nil {
			return err
		}
		return db.Ping()
	})
	if err != nil {
		t.Fatalf("could not connect to postgres: %v", err)
	}

	bridge := bridgePostgres.New("pg", dsn)
	bridge.BeforeRegister()
	defer bridge.AfterUnregister()

	newConn := func(ctx context.Context) api.Conn {
		conn, err := bridge.Connect(ctx)
		if err != nil {
			panic(err)
		}
		return internal.NewConn(conn)
	}

	ctx := t.Context()
	conn := newConn(ctx)
	defer conn.Close()

	conn.Exec(ctx, `CREATE TABLE test (id SERIAL PRIMARY KEY, name TEXT)`)
	conn.Exec(ctx, `INSERT INTO test (name) VALUES ($1)`, "foo")
	conn.Exec(ctx, `INSERT INTO test (name) VALUES ($1)`, "bar")

	rows, err := conn.Query(ctx, `SELECT * FROM test ORDER BY id`)
	require.NoError(t, err)
	defer rows.Close()
	for rows.Next() {
		var id int64
		var name string
		require.NoError(t, rows.Scan(&id, &name))
	}
}

func TestPostgresDateTypes(t *testing.T) {
	if !test.SupportDockerTest() {
		t.Skip("dockertest does not work in this environment")
	}

	pool := dockertest.NewPoolT(t, "")
	postgresRepository, postgresTag := test.PostgresDockerImage.Resolve()
	postgres := pool.RunT(t, postgresRepository,
		dockertest.WithTag(postgresTag),
		dockertest.WithEnv([]string{
			"POSTGRES_USER=dbuser",
			"POSTGRES_PASSWORD=dbpass",
			"POSTGRES_DB=db",
		}),
	)
	hostPort := postgres.GetHostPort("5432/tcp")
	host, port, err := net.SplitHostPort(hostPort)
	require.NoError(t, err)
	dsn := fmt.Sprintf("host=%s port=%s dbname=db user=dbuser password=dbpass sslmode=disable", host, port)

	err = pool.Retry(t.Context(), 30*time.Second, func() error {
		db, err := sql.Open("postgres", dsn)
		if err != nil {
			return err
		}
		defer db.Close()
		return db.Ping()
	})
	require.NoError(t, err)

	br := bridgePostgres.New("pg", dsn)
	require.NoError(t, br.BeforeRegister())
	defer br.AfterUnregister()

	ctx := t.Context()
	sqlConn, err := br.Connect(ctx)
	require.NoError(t, err)
	defer sqlConn.Close()

	conn := internal.NewConn(sqlConn)
	defer conn.Close()

	result := conn.Exec(ctx, `SET TIME ZONE 'UTC'`)
	require.NoError(t, result.Err())

	result = conn.Exec(ctx, `CREATE TABLE test_dates (
		id SERIAL PRIMARY KEY,
		event_bool BOOLEAN,
		event_int INTEGER,
		event_bigint BIGINT,
		event_real REAL,
		event_text TEXT,
		event_uuid UUID,
		event_date DATE,
		event_timestamp TIMESTAMP,
		event_timestamptz TIMESTAMPTZ
	)`)
	require.NoError(t, result.Err())

	result = conn.Exec(ctx, `INSERT INTO test_dates(event_bool, event_int, event_bigint, event_real, event_text, event_uuid, event_date, event_timestamp, event_timestamptz) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		true,
		int32(42),
		int64(4200000000),
		float32(3.25),
		"pg-text",
		"550e8400-e29b-41d4-a716-446655440000",
		"2026-03-14",
		"2026-03-14 05:29:01",
		"2026-03-14 05:29:01+00",
	)
	require.NoError(t, result.Err())

	result = conn.Exec(ctx, `INSERT INTO test_dates(event_bool, event_int, event_bigint, event_real, event_text, event_uuid, event_date, event_timestamp, event_timestamptz) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		nil,
		nil,
		nil,
		float32(7.5),
		nil,
		nil,
		"2026-03-15",
		nil,
		nil,
	)
	require.NoError(t, result.Err())

	rows, err := sqlConn.QueryContext(ctx, `SELECT event_bool, event_int, event_bigint, event_real, event_text, event_uuid, event_date, event_timestamp, event_timestamptz FROM test_dates WHERE event_timestamp IS NOT NULL`)
	require.NoError(t, err)
	defer rows.Close()

	columns, err := rows.ColumnTypes()
	require.NoError(t, err)
	require.Len(t, columns, 9)

	fields := make([]any, len(columns))
	for i, column := range columns {
		fields[i] = br.NewScanType(column.ScanType().String(), strings.ToUpper(column.DatabaseTypeName()))
	}

	require.True(t, rows.Next())
	require.NoError(t, rows.Scan(fields...))
	require.False(t, rows.Next())
	require.NoError(t, rows.Err())

	boolValue, ok := fields[0].(*sql.NullBool)
	require.True(t, ok)
	require.True(t, boolValue.Valid)
	require.True(t, boolValue.Bool)

	intValue, ok := fields[1].(*sql.NullInt32)
	require.True(t, ok)
	require.True(t, intValue.Valid)
	require.Equal(t, int32(42), intValue.Int32)

	bigintValue, ok := fields[2].(*sql.NullInt64)
	require.True(t, ok)
	require.True(t, bigintValue.Valid)
	require.Equal(t, int64(4200000000), bigintValue.Int64)

	realValue, ok := fields[3].(*float32)
	require.True(t, ok)
	require.InDelta(t, 3.25, *realValue, 0.0001)

	textValue, ok := fields[4].(*sql.NullString)
	require.True(t, ok)
	require.True(t, textValue.Valid)
	require.Equal(t, "pg-text", textValue.String)

	uuidValue, ok := fields[5].(*sql.NullString)
	require.True(t, ok)
	require.True(t, uuidValue.Valid)
	require.Equal(t, "550e8400-e29b-41d4-a716-446655440000", uuidValue.String)

	dateValue, ok := fields[6].(*sql.NullTime)
	require.True(t, ok)
	require.True(t, dateValue.Valid)
	require.Equal(t, time.Date(2026, 3, 14, 0, 0, 0, 0, time.UTC), dateValue.Time.UTC())

	timestampValue, ok := fields[7].(*sql.NullTime)
	require.True(t, ok)
	require.True(t, timestampValue.Valid)
	require.Equal(t, time.Date(2026, 3, 14, 5, 29, 1, 0, time.UTC), timestampValue.Time.UTC())

	timestampTZValue, ok := fields[8].(*sql.NullTime)
	require.True(t, ok)
	require.True(t, timestampTZValue.Valid)
	require.Equal(t, time.Date(2026, 3, 14, 5, 29, 1, 0, time.UTC), timestampTZValue.Time.UTC())

	values := make([]any, len(fields))
	for i, v := range fields {
		values[i] = bridgepkg.UnboxValueToNative(v)
	}
	require.Equal(t, true, values[0])
	require.Equal(t, int32(42), values[1])
	require.Equal(t, int64(4200000000), values[2])
	convertedReal, ok := values[3].(float32)
	require.True(t, ok, fmt.Sprintf("value type:%T", values[3]))
	require.InDelta(t, 3.25, convertedReal, 0.0001)
	require.Equal(t, "pg-text", values[4])
	require.Equal(t, "550e8400-e29b-41d4-a716-446655440000", values[5])

	convertedDate, ok := values[6].(time.Time)
	require.True(t, ok)
	require.Equal(t, time.Date(2026, 3, 14, 0, 0, 0, 0, time.UTC), convertedDate.UTC())

	convertedTimestamp, ok := values[7].(time.Time)
	require.True(t, ok)
	require.Equal(t, time.Date(2026, 3, 14, 5, 29, 1, 0, time.UTC), convertedTimestamp.UTC())

	convertedTimestampTZ, ok := values[8].(time.Time)
	require.True(t, ok)
	require.Equal(t, time.Date(2026, 3, 14, 5, 29, 1, 0, time.UTC), convertedTimestampTZ.UTC())

	nullRows, err := sqlConn.QueryContext(ctx, `SELECT event_bool, event_int, event_bigint, event_real, event_text, event_uuid, event_date, event_timestamp, event_timestamptz FROM test_dates WHERE event_timestamp IS NULL`)
	require.NoError(t, err)
	defer nullRows.Close()

	nullColumns, err := nullRows.ColumnTypes()
	require.NoError(t, err)
	require.Len(t, nullColumns, 9)

	nullFields := make([]any, len(nullColumns))
	for i, column := range nullColumns {
		nullFields[i] = br.NewScanType(column.ScanType().String(), strings.ToUpper(column.DatabaseTypeName()))
	}

	require.True(t, nullRows.Next())
	require.NoError(t, nullRows.Scan(nullFields...))
	require.False(t, nullRows.Next())
	require.NoError(t, nullRows.Err())

	nullBoolValue, ok := nullFields[0].(*sql.NullBool)
	require.True(t, ok)
	require.False(t, nullBoolValue.Valid)

	nullIntValue, ok := nullFields[1].(*sql.NullInt32)
	require.True(t, ok)
	require.False(t, nullIntValue.Valid)

	nullBigintValue, ok := nullFields[2].(*sql.NullInt64)
	require.True(t, ok)
	require.False(t, nullBigintValue.Valid)

	nullRealValue, ok := nullFields[3].(*float32)
	require.True(t, ok)
	require.Equal(t, float32(7.5), *nullRealValue)

	nullTextValue, ok := nullFields[4].(*sql.NullString)
	require.True(t, ok)
	require.False(t, nullTextValue.Valid)

	nullUUIDValue, ok := nullFields[5].(*sql.NullString)
	require.True(t, ok)
	require.False(t, nullUUIDValue.Valid)

	nullDateValue, ok := nullFields[6].(*sql.NullTime)
	require.True(t, ok)
	require.True(t, nullDateValue.Valid)
	require.Equal(t, time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC), nullDateValue.Time.UTC())

	nullTimestampValue, ok := nullFields[7].(*sql.NullTime)
	require.True(t, ok)
	require.False(t, nullTimestampValue.Valid)

	nullTimestampTZValue, ok := nullFields[8].(*sql.NullTime)
	require.True(t, ok)
	require.False(t, nullTimestampTZValue.Valid)

	nullValues := make([]any, len(nullFields))
	for i, v := range nullFields {
		nullValues[i] = bridgepkg.UnboxValueToNative(v)
	}
	require.Nil(t, nullValues[0])
	require.Nil(t, nullValues[1])
	require.Nil(t, nullValues[2])
	convertedNullReal, ok := nullValues[3].(float32)
	require.True(t, ok)
	require.Equal(t, float32(7.5), convertedNullReal)
	require.Nil(t, nullValues[4])
	require.Nil(t, nullValues[5])

	convertedNullDate, ok := nullValues[6].(time.Time)
	require.True(t, ok)
	require.Equal(t, time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC), convertedNullDate.UTC())
	require.Nil(t, nullValues[7])
	require.Nil(t, nullValues[8])
}
