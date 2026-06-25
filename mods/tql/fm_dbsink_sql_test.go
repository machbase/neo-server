package tql

import (
	"testing"

	"github.com/machbase/neo-server/v8/spi"
	"github.com/stretchr/testify/require"
)

func TestValidateSqlVerbForSink(t *testing.T) {
	require.NoError(t, validateSqlVerbForSink("insert into t values(1)"))
	require.NoError(t, validateSqlVerbForSink("update t set v = 1"))
	require.NoError(t, validateSqlVerbForSink("delete from t"))
	require.NoError(t, validateSqlVerbForSink("show tables"))

	err := validateSqlVerbForSink("select * from t")
	require.Error(t, err)
	require.Equal(t, `f(SQL) sink does not allow fetch verb "SELECT"`, err.Error())
}

func TestFormatSqlSinkMessage(t *testing.T) {
	require.Equal(t, "2 rows inserted.", formatSqlSinkMessage(spi.SQLStatementTypeInsert, 2))
	require.Equal(t, "1 row updated.", formatSqlSinkMessage(spi.SQLStatementTypeUpdate, 1))
	require.Equal(t, "3 rows deleted.", formatSqlSinkMessage(spi.SQLStatementTypeDelete, 3))
	require.Equal(t, "4 rows affected.", formatSqlSinkMessage(spi.SQLStatementTypeCreate, 4))
}

func TestParseRowsAffectedFromMessage(t *testing.T) {
	n, ok := parseRowsAffectedFromMessage("2 rows inserted.")
	require.True(t, ok)
	require.EqualValues(t, 2, n)

	n, ok = parseRowsAffectedFromMessage("a row updated.")
	require.True(t, ok)
	require.EqualValues(t, 1, n)

	n, ok = parseRowsAffectedFromMessage("Created successfully.")
	require.False(t, ok)
	require.EqualValues(t, 0, n)
}
