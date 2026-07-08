package tql

import (
	"testing"

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
