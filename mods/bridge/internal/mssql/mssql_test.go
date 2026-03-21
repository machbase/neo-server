package mssql_test

import (
	"database/sql"
	"fmt"
	"testing"
	"time"

	bridgepkg "github.com/machbase/neo-server/v8/mods/bridge"
	"github.com/machbase/neo-server/v8/mods/bridge/internal"
	bridgeMSSQL "github.com/machbase/neo-server/v8/mods/bridge/internal/mssql"
	"github.com/machbase/neo-server/v8/test"
	"github.com/ory/dockertest/v4"
	"github.com/stretchr/testify/require"
)

func TestMSSQLDatetimeTypes(t *testing.T) {
	if !test.SupportDockerTest() {
		t.Skip("dockertest does not work in this environment")
	}

	pool := dockertest.NewPoolT(t, "")
	resource := pool.RunT(t, "mcr.microsoft.com/mssql/server",
		dockertest.WithTag("2025-latest"),
		dockertest.WithEnv([]string{
			"ACCEPT_EULA=Y",
			"MSSQL_SA_PASSWORD=Your_password123",
		}),
	)

	hostPort := resource.GetHostPort("1433/tcp")
	dsn := fmt.Sprintf("server=%s user=sa password=Your_password123 database=master encrypt=disable", hostPort)

	err := pool.Retry(t.Context(), 60*time.Second, func() error {
		db, err := sql.Open("sqlserver", fmt.Sprintf("sqlserver://sa:Your_password123@%s?database=master", hostPort))
		if err != nil {
			return err
		}
		defer db.Close()
		return db.Ping()
	})
	require.NoError(t, err)

	br := bridgeMSSQL.New("ms", dsn)
	require.NoError(t, br.BeforeRegister())
	defer br.AfterUnregister()

	ctx := t.Context()
	sqlConn, err := br.Connect(ctx)
	require.NoError(t, err)
	defer sqlConn.Close()

	conn := internal.NewConn(sqlConn)
	defer conn.Close()

	result := conn.Exec(ctx, `CREATE TABLE ids (
		id INT NOT NULL PRIMARY KEY,
		event_smallint SMALLINT NULL,
		event_decimal DECIMAL(10,2) NULL,
		event_real REAL NULL,
		event_varchar VARCHAR(100) NULL,
		event_text TEXT NULL,
		event_datetime DATETIME NULL
	)`)
	require.NoError(t, result.Err())

	result = conn.Exec(ctx, `INSERT INTO ids(id, event_smallint, event_decimal, event_real, event_varchar, event_text, event_datetime) VALUES(1, 7, 123.45, 9.5, 'ms-varchar', 'ms-text', '2026-03-14 05:29:01')`)
	require.NoError(t, result.Err())

	result = conn.Exec(ctx, `INSERT INTO ids(id, event_smallint, event_decimal, event_real, event_varchar, event_text, event_datetime) VALUES(2, NULL, NULL, NULL, NULL, NULL, NULL)`)
	require.NoError(t, result.Err())

	rows, err := sqlConn.QueryContext(ctx, `SELECT id, event_smallint, event_decimal, event_real, event_varchar, event_text, event_datetime FROM ids WHERE id = 1`)
	require.NoError(t, err)
	defer rows.Close()

	columns, err := rows.ColumnTypes()
	require.NoError(t, err)
	require.Len(t, columns, 7)

	fields := make([]any, len(columns))
	for i, column := range columns {
		fields[i] = br.NewScanType(column.ScanType().String(), column.DatabaseTypeName())
	}

	require.True(t, rows.Next())
	require.NoError(t, rows.Scan(fields...))
	require.False(t, rows.Next())
	require.NoError(t, rows.Err())

	idValue, ok := fields[0].(*sql.NullInt64)
	require.True(t, ok)
	require.True(t, idValue.Valid)
	require.Equal(t, int64(1), idValue.Int64)

	smallintValue, ok := fields[1].(*sql.NullInt64)
	require.True(t, ok)
	require.True(t, smallintValue.Valid)
	require.Equal(t, int64(7), smallintValue.Int64)

	decimalValue, ok := fields[2].(*sql.NullFloat64)
	require.True(t, ok)
	require.True(t, decimalValue.Valid)
	require.InDelta(t, 123.45, decimalValue.Float64, 0.0001)

	realValue, ok := fields[3].(*sql.NullFloat64)
	require.True(t, ok)
	require.True(t, realValue.Valid)
	require.InDelta(t, 9.5, realValue.Float64, 0.0001)

	varcharValue, ok := fields[4].(*sql.NullString)
	require.True(t, ok)
	require.True(t, varcharValue.Valid)
	require.Equal(t, "ms-varchar", varcharValue.String)

	textValue, ok := fields[5].(*sql.NullString)
	require.True(t, ok)
	require.True(t, textValue.Valid)
	require.Equal(t, "ms-text", textValue.String)

	datetimeValue, ok := fields[6].(*sql.NullTime)
	require.True(t, ok)
	require.True(t, datetimeValue.Valid)
	require.Equal(t, time.Date(2026, 3, 14, 5, 29, 1, 0, time.UTC), datetimeValue.Time.UTC())

	datums, err := bridgepkg.ConvertToDatum(fields...)
	require.NoError(t, err)
	require.Len(t, datums, 7)

	values, err := bridgepkg.ConvertFromDatum(datums...)
	require.NoError(t, err)
	require.Equal(t, int64(1), values[0])
	require.Equal(t, int64(7), values[1])
	convertedDecimal, ok := values[2].(float64)
	require.True(t, ok)
	require.InDelta(t, 123.45, convertedDecimal, 0.0001)
	convertedReal, ok := values[3].(float64)
	require.True(t, ok)
	require.InDelta(t, 9.5, convertedReal, 0.0001)
	require.Equal(t, "ms-varchar", values[4])
	require.Equal(t, "ms-text", values[5])
	convertedDatetime, ok := values[6].(time.Time)
	require.True(t, ok)
	require.Equal(t, time.Date(2026, 3, 14, 5, 29, 1, 0, time.UTC), convertedDatetime.UTC())

	nullRows, err := sqlConn.QueryContext(ctx, `SELECT id, event_smallint, event_decimal, event_real, event_varchar, event_text, event_datetime FROM ids WHERE id = 2`)
	require.NoError(t, err)
	defer nullRows.Close()

	nullColumns, err := nullRows.ColumnTypes()
	require.NoError(t, err)
	require.Len(t, nullColumns, 7)

	nullFields := make([]any, len(nullColumns))
	for i, column := range nullColumns {
		nullFields[i] = br.NewScanType(column.ScanType().String(), column.DatabaseTypeName())
	}

	require.True(t, nullRows.Next())
	require.NoError(t, nullRows.Scan(nullFields...))
	require.False(t, nullRows.Next())
	require.NoError(t, nullRows.Err())

	nullIDValue, ok := nullFields[0].(*sql.NullInt64)
	require.True(t, ok)
	require.True(t, nullIDValue.Valid)
	require.Equal(t, int64(2), nullIDValue.Int64)

	nullSmallintValue, ok := nullFields[1].(*sql.NullInt64)
	require.True(t, ok)
	require.False(t, nullSmallintValue.Valid)

	nullDecimalValue, ok := nullFields[2].(*sql.NullFloat64)
	require.True(t, ok)
	require.False(t, nullDecimalValue.Valid)

	nullRealValue, ok := nullFields[3].(*sql.NullFloat64)
	require.True(t, ok)
	require.False(t, nullRealValue.Valid)

	nullVarcharValue, ok := nullFields[4].(*sql.NullString)
	require.True(t, ok)
	require.False(t, nullVarcharValue.Valid)

	nullTextValue, ok := nullFields[5].(*sql.NullString)
	require.True(t, ok)
	require.False(t, nullTextValue.Valid)

	nullDatetimeValue, ok := nullFields[6].(*sql.NullTime)
	require.True(t, ok)
	require.False(t, nullDatetimeValue.Valid)

	nullDatums, err := bridgepkg.ConvertToDatum(nullFields...)
	require.NoError(t, err)
	require.Len(t, nullDatums, 7)

	nullValues, err := bridgepkg.ConvertFromDatum(nullDatums...)
	require.NoError(t, err)
	require.Equal(t, int64(2), nullValues[0])
	require.Nil(t, nullValues[1])
	require.Nil(t, nullValues[2])
	require.Nil(t, nullValues[3])
	require.Nil(t, nullValues[4])
	require.Nil(t, nullValues[5])
	require.Nil(t, nullValues[6])
}
