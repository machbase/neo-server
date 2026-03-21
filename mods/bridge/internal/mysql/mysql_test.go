package mysql_test

import (
	"database/sql"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"

	bridgepkg "github.com/machbase/neo-server/v8/mods/bridge"
	"github.com/machbase/neo-server/v8/mods/bridge/internal"
	bridgeMySQL "github.com/machbase/neo-server/v8/mods/bridge/internal/mysql"
	"github.com/machbase/neo-server/v8/test"
	"github.com/ory/dockertest/v4"
	"github.com/stretchr/testify/require"
)

func TestMySQLDateTypes(t *testing.T) {
	if !test.SupportDockerTest() {
		t.Skip("dockertest does not work in this environment")
	}

	pool := dockertest.NewPoolT(t, "")
	resource := pool.RunT(t, "mysql",
		dockertest.WithTag("8.0"),
		dockertest.WithEnv([]string{
			"MYSQL_ROOT_PASSWORD=secret",
			"MYSQL_DATABASE=db",
			"MYSQL_USER=dbuser",
			"MYSQL_PASSWORD=secret",
		}),
	)

	hostPort := resource.GetHostPort("3306/tcp")
	host, port, err := net.SplitHostPort(hostPort)
	require.NoError(t, err)

	dsn := fmt.Sprintf("dbuser:secret@tcp(%s:%s)/db?parseTime=true&loc=UTC", host, port)
	err = pool.Retry(t.Context(), 30*time.Second, func() error {
		db, err := sql.Open("mysql", dsn)
		if err != nil {
			return err
		}
		defer db.Close()
		return db.Ping()
	})
	require.NoError(t, err)

	br := bridgeMySQL.New("my", dsn)
	require.NoError(t, br.BeforeRegister())
	defer br.AfterUnregister()

	ctx := t.Context()
	sqlConn, err := br.Connect(ctx)
	require.NoError(t, err)
	defer sqlConn.Close()

	conn := internal.NewConn(sqlConn)
	defer conn.Close()

	result := conn.Exec(ctx, "SET time_zone = '+00:00'")
	require.NoError(t, result.Err())

	result = conn.Exec(ctx, `CREATE TABLE test_dates (
		id INT NOT NULL AUTO_INCREMENT PRIMARY KEY,
		event_bigint BIGINT,
		event_int INT,
		event_smallint SMALLINT,
		event_double DOUBLE,
		event_varchar VARCHAR(64),
		event_char CHAR(4),
		event_text TEXT,
		event_blob BLOB,
		event_date DATE,
		event_datetime DATETIME,
		event_timestamp TIMESTAMP NULL
	)`)
	require.NoError(t, result.Err())

	result = conn.Exec(ctx, `INSERT INTO test_dates(event_bigint, event_int, event_smallint, event_double, event_varchar, event_char, event_text, event_blob, event_date, event_datetime, event_timestamp) VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		int64(4200000000),
		123456,
		int16(12),
		3.5,
		"my-varchar",
		"ABCD",
		"my-text",
		[]byte{0x01, 0x02, 0x03},
		"2026-03-14",
		"2026-03-14 05:29:01",
		"2026-03-14 05:29:01",
	)
	require.NoError(t, result.Err())

	result = conn.Exec(ctx, `INSERT INTO test_dates(event_bigint, event_int, event_smallint, event_double, event_varchar, event_char, event_text, event_blob, event_date, event_datetime, event_timestamp) VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		"2026-03-15",
		"2026-03-15 06:30:02",
		nil,
	)
	require.NoError(t, result.Err())

	rows, err := sqlConn.QueryContext(ctx, `SELECT event_bigint, event_int, event_smallint, event_double, event_varchar, event_char, event_text, event_blob, event_date, event_datetime, event_timestamp FROM test_dates WHERE event_timestamp IS NOT NULL`)
	require.NoError(t, err)
	defer rows.Close()

	columns, err := rows.ColumnTypes()
	require.NoError(t, err)
	require.Len(t, columns, 11)

	fields := make([]any, len(columns))
	for i, column := range columns {
		fields[i] = br.NewScanType(column.ScanType().String(), strings.ToUpper(column.DatabaseTypeName()))
	}

	require.True(t, rows.Next())
	require.NoError(t, rows.Scan(fields...))
	require.False(t, rows.Next())
	require.NoError(t, rows.Err())

	requireMySQLIntegerField(t, fields[0], 4200000000)
	requireMySQLIntegerField(t, fields[1], 123456)
	requireMySQLIntegerField(t, fields[2], 12)

	doubleValue, ok := fields[3].(*sql.NullFloat64)
	require.True(t, ok)
	require.True(t, doubleValue.Valid)
	require.InDelta(t, 3.5, doubleValue.Float64, 0.0001)

	varcharValue, ok := fields[4].(*sql.NullString)
	require.True(t, ok)
	require.True(t, varcharValue.Valid)
	require.Equal(t, "my-varchar", varcharValue.String)

	charValue, ok := fields[5].(*sql.NullString)
	require.True(t, ok)
	require.True(t, charValue.Valid)
	require.Equal(t, "ABCD", charValue.String)

	textValue, ok := fields[6].(*sql.NullString)
	require.True(t, ok)
	require.True(t, textValue.Valid)
	require.Equal(t, "my-text", textValue.String)

	blobValue, ok := fields[7].(*[]byte)
	require.True(t, ok)
	require.Equal(t, []byte{0x01, 0x02, 0x03}, *blobValue)

	dateValue, ok := fields[8].(*sql.NullString)
	require.True(t, ok)
	require.True(t, dateValue.Valid)
	require.Equal(t, "2026-03-14T00:00:00Z", dateValue.String)

	datetimeValue, ok := fields[9].(*sql.NullTime)
	require.True(t, ok)
	require.True(t, datetimeValue.Valid)
	require.Equal(t, time.Date(2026, 3, 14, 5, 29, 1, 0, time.UTC), datetimeValue.Time.UTC())

	timestampValue, ok := fields[10].(*sql.NullTime)
	require.True(t, ok)
	require.True(t, timestampValue.Valid)
	require.Equal(t, time.Date(2026, 3, 14, 5, 29, 1, 0, time.UTC), timestampValue.Time.UTC())

	datums, err := bridgepkg.ConvertToDatum(fields...)
	require.NoError(t, err)
	require.Len(t, datums, 11)

	values, err := bridgepkg.ConvertFromDatum(datums...)
	require.NoError(t, err)
	requireMySQLIntegerValue(t, values[0], 4200000000)
	requireMySQLIntegerValue(t, values[1], 123456)
	requireMySQLIntegerValue(t, values[2], 12)
	convertedDouble, ok := values[3].(float64)
	require.True(t, ok)
	require.InDelta(t, 3.5, convertedDouble, 0.0001)
	require.Equal(t, "my-varchar", values[4])
	require.Equal(t, "ABCD", values[5])
	require.Equal(t, "my-text", values[6])
	require.Equal(t, []byte{0x01, 0x02, 0x03}, values[7])
	dateString, ok := values[8].(string)
	require.True(t, ok)
	require.Equal(t, "2026-03-14T00:00:00Z", dateString)
	convertedDatetime, ok := values[9].(time.Time)
	require.True(t, ok)
	require.Equal(t, time.Date(2026, 3, 14, 5, 29, 1, 0, time.UTC), convertedDatetime.UTC())
	convertedTimestamp, ok := values[10].(time.Time)
	require.True(t, ok)
	require.Equal(t, time.Date(2026, 3, 14, 5, 29, 1, 0, time.UTC), convertedTimestamp.UTC())

	nullRows, err := sqlConn.QueryContext(ctx, `SELECT event_bigint, event_int, event_smallint, event_double, event_varchar, event_char, event_text, event_blob, event_date, event_datetime, event_timestamp FROM test_dates WHERE event_timestamp IS NULL`)
	require.NoError(t, err)
	defer nullRows.Close()

	nullColumns, err := nullRows.ColumnTypes()
	require.NoError(t, err)
	require.Len(t, nullColumns, 11)

	nullFields := make([]any, len(nullColumns))
	for i, column := range nullColumns {
		nullFields[i] = br.NewScanType(column.ScanType().String(), strings.ToUpper(column.DatabaseTypeName()))
	}

	require.True(t, nullRows.Next())
	require.NoError(t, nullRows.Scan(nullFields...))
	require.False(t, nullRows.Next())
	require.NoError(t, nullRows.Err())

	requireMySQLNullIntegerField(t, nullFields[0])
	requireMySQLNullIntegerField(t, nullFields[1])
	requireMySQLNullIntegerField(t, nullFields[2])

	nullDoubleValue, ok := nullFields[3].(*sql.NullFloat64)
	require.True(t, ok)
	require.False(t, nullDoubleValue.Valid)

	nullVarcharValue, ok := nullFields[4].(*sql.NullString)
	require.True(t, ok)
	require.False(t, nullVarcharValue.Valid)

	nullCharValue, ok := nullFields[5].(*sql.NullString)
	require.True(t, ok)
	require.False(t, nullCharValue.Valid)

	nullTextValue, ok := nullFields[6].(*sql.NullString)
	require.True(t, ok)
	require.False(t, nullTextValue.Valid)

	nullBlobValue, ok := nullFields[7].(*[]byte)
	require.True(t, ok)
	require.Nil(t, *nullBlobValue)

	nullDateValue, ok := nullFields[8].(*sql.NullString)
	require.True(t, ok)
	require.True(t, nullDateValue.Valid)
	require.Equal(t, "2026-03-15T00:00:00Z", nullDateValue.String)

	nullDatetimeValue, ok := nullFields[9].(*sql.NullTime)
	require.True(t, ok)
	require.True(t, nullDatetimeValue.Valid)
	require.Equal(t, time.Date(2026, 3, 15, 6, 30, 2, 0, time.UTC), nullDatetimeValue.Time.UTC())

	nullTimestampValue, ok := nullFields[10].(*sql.NullTime)
	require.True(t, ok)
	require.False(t, nullTimestampValue.Valid)

	nullDatums, err := bridgepkg.ConvertToDatum(nullFields...)
	require.NoError(t, err)
	require.Len(t, nullDatums, 11)

	nullValues, err := bridgepkg.ConvertFromDatum(nullDatums...)
	require.NoError(t, err)
	require.Nil(t, nullValues[0])
	require.Nil(t, nullValues[1])
	require.Nil(t, nullValues[2])
	require.Nil(t, nullValues[3])
	require.Nil(t, nullValues[4])
	require.Nil(t, nullValues[5])
	require.Nil(t, nullValues[6])
	require.Equal(t, []byte{}, nullValues[7])
	nullDateString, ok := nullValues[8].(string)
	require.True(t, ok)
	require.Equal(t, "2026-03-15T00:00:00Z", nullDateString)
	convertedNullDatetime, ok := nullValues[9].(time.Time)
	require.True(t, ok)
	require.Equal(t, time.Date(2026, 3, 15, 6, 30, 2, 0, time.UTC), convertedNullDatetime.UTC())
	require.Nil(t, nullValues[10])
}

func requireMySQLIntegerField(t *testing.T, field any, want int64) {
	t.Helper()

	switch value := field.(type) {
	case *sql.NullInt16:
		require.True(t, value.Valid)
		require.Equal(t, want, int64(value.Int16))
	case *sql.NullInt32:
		require.True(t, value.Valid)
		require.Equal(t, want, int64(value.Int32))
	case *sql.NullInt64:
		require.True(t, value.Valid)
		require.Equal(t, want, value.Int64)
	default:
		require.Failf(t, "unexpected mysql integer field type", "%T", field)
	}
}

func requireMySQLNullIntegerField(t *testing.T, field any) {
	t.Helper()

	switch value := field.(type) {
	case *sql.NullInt16:
		require.False(t, value.Valid)
	case *sql.NullInt32:
		require.False(t, value.Valid)
	case *sql.NullInt64:
		require.False(t, value.Valid)
	default:
		require.Failf(t, "unexpected mysql integer field type", "%T", field)
	}
}

func requireMySQLIntegerValue(t *testing.T, value any, want int64) {
	t.Helper()

	switch actual := value.(type) {
	case int32:
		require.Equal(t, want, int64(actual))
	case int64:
		require.Equal(t, want, actual)
	default:
		require.Failf(t, "unexpected mysql integer value type", "%T", value)
	}
}
