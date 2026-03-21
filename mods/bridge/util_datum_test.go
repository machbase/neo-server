package bridge

import (
	"database/sql"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestConvertDatumTimePreservesUnixNano(t *testing.T) {
	original := time.Date(2026, 3, 14, 5, 29, 1, 123456789, time.FixedZone("KST", 9*60*60))

	datums, err := ConvertToDatum(original)
	require.NoError(t, err)
	require.Len(t, datums, 1)

	values, err := ConvertFromDatum(datums...)
	require.NoError(t, err)
	require.Len(t, values, 1)

	converted, ok := values[0].(time.Time)
	require.True(t, ok)
	require.Equal(t, original.UnixNano(), converted.UnixNano())
	require.True(t, original.UTC().Equal(converted.UTC()))
}

func TestConvertDatumValidNullTimeBecomesTime(t *testing.T) {
	original := &sql.NullTime{
		Time:  time.Date(2026, 3, 15, 6, 30, 2, 987654321, time.UTC),
		Valid: true,
	}

	datums, err := ConvertToDatum(original)
	require.NoError(t, err)
	require.Len(t, datums, 1)

	values, err := ConvertFromDatum(datums...)
	require.NoError(t, err)
	require.Len(t, values, 1)

	converted, ok := values[0].(time.Time)
	require.True(t, ok)
	require.Equal(t, original.Time.UnixNano(), converted.UnixNano())
}

func TestConvertDatumInvalidNullTimeBecomesNil(t *testing.T) {
	original := &sql.NullTime{Valid: false}

	datums, err := ConvertToDatum(original)
	require.NoError(t, err)
	require.Len(t, datums, 1)

	values, err := ConvertFromDatum(datums...)
	require.NoError(t, err)
	require.Len(t, values, 1)
	require.Nil(t, values[0])
}

func TestConvertDatumMixedTimeValues(t *testing.T) {
	base := time.Date(2026, 3, 16, 7, 31, 3, 456789123, time.UTC)
	originals := []any{
		base,
		&base,
		&sql.NullTime{Time: base.Add(2 * time.Second), Valid: true},
		&sql.NullTime{Valid: false},
	}

	datums, err := ConvertToDatum(originals...)
	require.NoError(t, err)
	require.Len(t, datums, len(originals))

	values, err := ConvertFromDatum(datums...)
	require.NoError(t, err)
	require.Len(t, values, len(originals))

	converted0, ok := values[0].(time.Time)
	require.True(t, ok)
	require.Equal(t, base.UnixNano(), converted0.UnixNano())

	converted1, ok := values[1].(time.Time)
	require.True(t, ok)
	require.Equal(t, base.UnixNano(), converted1.UnixNano())

	converted2, ok := values[2].(time.Time)
	require.True(t, ok)
	require.Equal(t, base.Add(2*time.Second).UnixNano(), converted2.UnixNano())

	require.Nil(t, values[3])
}
