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
		event_tinyint TINYINT NULL,
		event_int INT NULL,
		event_bigint BIGINT NULL,
		event_decimal DECIMAL(10,2) NULL,
		event_numeric NUMERIC(10,2) NULL,
		event_money MONEY NULL,
		event_smallmoney SMALLMONEY NULL,
		event_real REAL NULL,
		event_float FLOAT NULL,
		event_bit BIT NULL,
		event_varchar VARCHAR(100) NULL,
		event_nchar NCHAR(8) NULL,
		event_nvarchar NVARCHAR(100) NULL,
		event_text TEXT NULL,
		event_datetime DATETIME NULL
	)`)
	require.NoError(t, result.Err())

	result = conn.Exec(ctx, `INSERT INTO ids(id, event_smallint, event_tinyint, event_int, event_bigint, event_decimal, event_numeric, event_money, event_smallmoney, event_real, event_float, event_bit, event_varchar, event_nchar, event_nvarchar, event_text, event_datetime) VALUES(1, 7, 12, 34, 1234567890123, 123.45, 234.56, 345.67, 45.67, 9.5, 10.25, 1, 'ms-varchar', N'ms-nchar', N'ms-nvarchar', 'ms-text', '2026-03-14 05:29:01')`)
	require.NoError(t, result.Err())

	result = conn.Exec(ctx, `INSERT INTO ids(id, event_smallint, event_tinyint, event_int, event_bigint, event_decimal, event_numeric, event_money, event_smallmoney, event_real, event_float, event_bit, event_varchar, event_nchar, event_nvarchar, event_text, event_datetime) VALUES(2, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL)`)
	require.NoError(t, result.Err())

	rows, err := sqlConn.QueryContext(ctx, `SELECT id, event_smallint, event_tinyint, event_int, event_bigint, event_decimal, event_numeric, event_money, event_smallmoney, event_real, event_float, event_bit, event_varchar, event_nchar, event_nvarchar, event_text, event_datetime FROM ids WHERE id = 1`)
	require.NoError(t, err)
	defer rows.Close()

	columns, err := rows.ColumnTypes()
	require.NoError(t, err)
	require.Len(t, columns, 17)

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

	tinyintValue, ok := fields[2].(*sql.NullInt64)
	require.True(t, ok)
	require.True(t, tinyintValue.Valid)
	require.Equal(t, int64(12), tinyintValue.Int64)

	intValue, ok := fields[3].(*sql.NullInt64)
	require.True(t, ok)
	require.True(t, intValue.Valid)
	require.Equal(t, int64(34), intValue.Int64)

	bigintValue, ok := fields[4].(*sql.NullInt64)
	require.True(t, ok)
	require.True(t, bigintValue.Valid)
	require.Equal(t, int64(1234567890123), bigintValue.Int64)

	decimalValue, ok := fields[5].(*sql.NullFloat64)
	require.True(t, ok)
	require.True(t, decimalValue.Valid)
	require.InDelta(t, 123.45, decimalValue.Float64, 0.0001)

	numericValue, ok := fields[6].(*sql.NullFloat64)
	require.True(t, ok)
	require.True(t, numericValue.Valid)
	require.InDelta(t, 234.56, numericValue.Float64, 0.0001)

	moneyValue, ok := fields[7].(*sql.NullFloat64)
	require.True(t, ok)
	require.True(t, moneyValue.Valid)
	require.InDelta(t, 345.67, moneyValue.Float64, 0.0001)

	smallmoneyValue, ok := fields[8].(*sql.NullFloat64)
	require.True(t, ok)
	require.True(t, smallmoneyValue.Valid)
	require.InDelta(t, 45.67, smallmoneyValue.Float64, 0.0001)

	realValue, ok := fields[9].(*sql.NullFloat64)
	require.True(t, ok)
	require.True(t, realValue.Valid)
	require.InDelta(t, 9.5, realValue.Float64, 0.0001)

	floatValue, ok := fields[10].(*sql.NullFloat64)
	require.True(t, ok)
	require.True(t, floatValue.Valid)
	require.InDelta(t, 10.25, floatValue.Float64, 0.0001)

	bitValue, ok := fields[11].(*sql.NullBool)
	require.True(t, ok)
	require.True(t, bitValue.Valid)
	require.True(t, bitValue.Bool)

	varcharValue, ok := fields[12].(*sql.NullString)
	require.True(t, ok)
	require.True(t, varcharValue.Valid)
	require.Equal(t, "ms-varchar", varcharValue.String)

	ncharValue, ok := fields[13].(*sql.NullString)
	require.True(t, ok)
	require.True(t, ncharValue.Valid)
	require.Equal(t, "ms-nchar", ncharValue.String)

	nvarcharValue, ok := fields[14].(*sql.NullString)
	require.True(t, ok)
	require.True(t, nvarcharValue.Valid)
	require.Equal(t, "ms-nvarchar", nvarcharValue.String)

	textValue, ok := fields[15].(*sql.NullString)
	require.True(t, ok)
	require.True(t, textValue.Valid)
	require.Equal(t, "ms-text", textValue.String)

	datetimeValue, ok := fields[16].(*sql.NullTime)
	require.True(t, ok)
	require.True(t, datetimeValue.Valid)
	require.Equal(t, time.Date(2026, 3, 14, 5, 29, 1, 0, time.UTC), datetimeValue.Time.UTC())

	datums, err := bridgepkg.ConvertToDatum(fields...)
	require.NoError(t, err)
	require.Len(t, datums, 17)

	values, err := bridgepkg.ConvertFromDatum(datums...)
	require.NoError(t, err)
	require.Equal(t, int64(1), values[0])
	require.Equal(t, int64(7), values[1])
	require.Equal(t, int64(12), values[2])
	require.Equal(t, int64(34), values[3])
	require.Equal(t, int64(1234567890123), values[4])
	convertedDecimal, ok := values[5].(float64)
	require.True(t, ok)
	require.InDelta(t, 123.45, convertedDecimal, 0.0001)
	convertedNumeric, ok := values[6].(float64)
	require.True(t, ok)
	require.InDelta(t, 234.56, convertedNumeric, 0.0001)
	convertedMoney, ok := values[7].(float64)
	require.True(t, ok)
	require.InDelta(t, 345.67, convertedMoney, 0.0001)
	convertedSmallmoney, ok := values[8].(float64)
	require.True(t, ok)
	require.InDelta(t, 45.67, convertedSmallmoney, 0.0001)
	convertedReal, ok := values[9].(float64)
	require.True(t, ok)
	require.InDelta(t, 9.5, convertedReal, 0.0001)
	convertedFloat, ok := values[10].(float64)
	require.True(t, ok)
	require.InDelta(t, 10.25, convertedFloat, 0.0001)
	require.Equal(t, true, values[11])
	require.Equal(t, "ms-varchar", values[12])
	require.Equal(t, "ms-nchar", values[13])
	require.Equal(t, "ms-nvarchar", values[14])
	require.Equal(t, "ms-text", values[15])
	convertedDatetime, ok := values[16].(time.Time)
	require.True(t, ok)
	require.Equal(t, time.Date(2026, 3, 14, 5, 29, 1, 0, time.UTC), convertedDatetime.UTC())

	nullRows, err := sqlConn.QueryContext(ctx, `SELECT id, event_smallint, event_tinyint, event_int, event_bigint, event_decimal, event_numeric, event_money, event_smallmoney, event_real, event_float, event_bit, event_varchar, event_nchar, event_nvarchar, event_text, event_datetime FROM ids WHERE id = 2`)
	require.NoError(t, err)
	defer nullRows.Close()

	nullColumns, err := nullRows.ColumnTypes()
	require.NoError(t, err)
	require.Len(t, nullColumns, 17)

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

	nullTinyintValue, ok := nullFields[2].(*sql.NullInt64)
	require.True(t, ok)
	require.False(t, nullTinyintValue.Valid)

	nullIntValue, ok := nullFields[3].(*sql.NullInt64)
	require.True(t, ok)
	require.False(t, nullIntValue.Valid)

	nullBigintValue, ok := nullFields[4].(*sql.NullInt64)
	require.True(t, ok)
	require.False(t, nullBigintValue.Valid)

	nullDecimalValue, ok := nullFields[5].(*sql.NullFloat64)
	require.True(t, ok)
	require.False(t, nullDecimalValue.Valid)

	nullNumericValue, ok := nullFields[6].(*sql.NullFloat64)
	require.True(t, ok)
	require.False(t, nullNumericValue.Valid)

	nullMoneyValue, ok := nullFields[7].(*sql.NullFloat64)
	require.True(t, ok)
	require.False(t, nullMoneyValue.Valid)

	nullSmallmoneyValue, ok := nullFields[8].(*sql.NullFloat64)
	require.True(t, ok)
	require.False(t, nullSmallmoneyValue.Valid)

	nullRealValue, ok := nullFields[9].(*sql.NullFloat64)
	require.True(t, ok)
	require.False(t, nullRealValue.Valid)

	nullFloatValue, ok := nullFields[10].(*sql.NullFloat64)
	require.True(t, ok)
	require.False(t, nullFloatValue.Valid)

	nullBitValue, ok := nullFields[11].(*sql.NullBool)
	require.True(t, ok)
	require.False(t, nullBitValue.Valid)

	nullVarcharValue, ok := nullFields[12].(*sql.NullString)
	require.True(t, ok)
	require.False(t, nullVarcharValue.Valid)

	nullNCharValue, ok := nullFields[13].(*sql.NullString)
	require.True(t, ok)
	require.False(t, nullNCharValue.Valid)

	nullNVarcharValue, ok := nullFields[14].(*sql.NullString)
	require.True(t, ok)
	require.False(t, nullNVarcharValue.Valid)

	nullTextValue, ok := nullFields[15].(*sql.NullString)
	require.True(t, ok)
	require.False(t, nullTextValue.Valid)

	nullDatetimeValue, ok := nullFields[16].(*sql.NullTime)
	require.True(t, ok)
	require.False(t, nullDatetimeValue.Valid)

	nullDatums, err := bridgepkg.ConvertToDatum(nullFields...)
	require.NoError(t, err)
	require.Len(t, nullDatums, 17)

	nullValues, err := bridgepkg.ConvertFromDatum(nullDatums...)
	require.NoError(t, err)
	require.Equal(t, int64(2), nullValues[0])
	require.Nil(t, nullValues[1])
	require.Nil(t, nullValues[2])
	require.Nil(t, nullValues[3])
	require.Nil(t, nullValues[4])
	require.Nil(t, nullValues[5])
	require.Nil(t, nullValues[6])
	require.Nil(t, nullValues[7])
	require.Nil(t, nullValues[8])
	require.Nil(t, nullValues[9])
	require.Nil(t, nullValues[10])
	require.Nil(t, nullValues[11])
	require.Nil(t, nullValues[12])
	require.Nil(t, nullValues[13])
	require.Nil(t, nullValues[14])
	require.Nil(t, nullValues[15])
	require.Nil(t, nullValues[16])
}
