package bridge

import (
	"database/sql"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestUnboxValueToNativeTZ(t *testing.T) {
	rtz := time.FixedZone("KST", 9*60*60)

	f32 := float32(1.25)
	f64 := 2.5
	i := int(3)
	i8 := int8(4)
	i16 := int16(5)
	i32 := int32(6)
	i64 := int64(7)
	u := uint(8)
	u8 := uint8(9)
	u16 := uint16(10)
	u32 := uint32(11)
	u64 := uint64(12)
	b := true
	s := "str"
	tm := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)

	require.Equal(t, f32, UnboxValueToNativeTZ(&f32, rtz))
	require.Equal(t, f64, UnboxValueToNativeTZ(&f64, rtz))
	require.Equal(t, i, UnboxValueToNativeTZ(&i, rtz))
	require.Equal(t, i8, UnboxValueToNativeTZ(&i8, rtz))
	require.Equal(t, i16, UnboxValueToNativeTZ(&i16, rtz))
	require.Equal(t, i32, UnboxValueToNativeTZ(&i32, rtz))
	require.Equal(t, i64, UnboxValueToNativeTZ(&i64, rtz))
	require.Equal(t, u, UnboxValueToNativeTZ(&u, rtz))
	require.Equal(t, u8, UnboxValueToNativeTZ(&u8, rtz))
	require.Equal(t, u16, UnboxValueToNativeTZ(&u16, rtz))
	require.Equal(t, u32, UnboxValueToNativeTZ(&u32, rtz))
	require.Equal(t, u64, UnboxValueToNativeTZ(&u64, rtz))
	require.Equal(t, b, UnboxValueToNativeTZ(&b, rtz))
	require.Equal(t, s, UnboxValueToNativeTZ(&s, rtz))
	require.Equal(t, tm, UnboxValueToNativeTZ(&tm, rtz))

	var nilF32 *float32
	require.Nil(t, UnboxValueToNativeTZ(nilF32, rtz))

	require.Equal(t, "ok", UnboxValueToNativeTZ(&sql.NullString{String: "ok", Valid: true}, rtz))
	require.Nil(t, UnboxValueToNativeTZ(&sql.NullString{}, rtz))
	require.EqualValues(t, 16, UnboxValueToNativeTZ(&sql.NullInt16{Int16: 16, Valid: true}, rtz))
	require.Nil(t, UnboxValueToNativeTZ(&sql.NullInt16{}, rtz))
	require.EqualValues(t, 32, UnboxValueToNativeTZ(&sql.NullInt32{Int32: 32, Valid: true}, rtz))
	require.Nil(t, UnboxValueToNativeTZ(&sql.NullInt32{}, rtz))
	require.EqualValues(t, 64, UnboxValueToNativeTZ(&sql.NullInt64{Int64: 64, Valid: true}, rtz))
	require.Nil(t, UnboxValueToNativeTZ(&sql.NullInt64{}, rtz))
	require.Equal(t, 1.5, UnboxValueToNativeTZ(&sql.NullFloat64{Float64: 1.5, Valid: true}, rtz))
	require.Nil(t, UnboxValueToNativeTZ(&sql.NullFloat64{}, rtz))
	require.Equal(t, true, UnboxValueToNativeTZ(&sql.NullBool{Bool: true, Valid: true}, rtz))
	require.Nil(t, UnboxValueToNativeTZ(&sql.NullBool{}, rtz))
	require.EqualValues(t, 7, UnboxValueToNativeTZ(&sql.NullByte{Byte: 7, Valid: true}, rtz))
	require.Nil(t, UnboxValueToNativeTZ(&sql.NullByte{}, rtz))

	nullTime := &sql.NullTime{Time: tm, Valid: true}
	conv, ok := UnboxValueToNativeTZ(nullTime, rtz).(time.Time)
	require.True(t, ok)
	require.Equal(t, tm.In(rtz), conv)
	require.Nil(t, UnboxValueToNativeTZ(&sql.NullTime{}, rtz))

	rb := sql.RawBytes([]byte("raw"))
	rbConv, ok := UnboxValueToNativeTZ(&rb, rtz).([]byte)
	require.True(t, ok)
	require.Equal(t, []byte("raw"), rbConv)
	rbConv[0] = 'R'
	require.Equal(t, byte('r'), rb[0])

	bts := []uint8("bytes")
	btsConv, ok := UnboxValueToNativeTZ(&bts, rtz).([]byte)
	require.True(t, ok)
	require.Equal(t, []byte("bytes"), btsConv)
	btsConv[0] = 'B'
	require.Equal(t, uint8('b'), bts[0])

	var nilBts *[]uint8
	require.Nil(t, UnboxValueToNativeTZ(nilBts, rtz))

	require.Equal(t, 123, UnboxValueToNativeTZ(123, rtz))
}
