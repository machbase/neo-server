package tql

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type recordReceiver struct {
	name     string
	received *Record
}

func (r *recordReceiver) Name() string { return r.name }

func (r *recordReceiver) Receive(rec *Record) {
	r.received = rec
}

func TestRecordKindsAndArrayAccessors(t *testing.T) {
	t.Run("array record", func(t *testing.T) {
		items := []*Record{NewRecord("k", 1), NewRecord("k2", 2)}
		rec := ArrayRecord(items)

		require.True(t, rec.IsArray())
		require.False(t, rec.IsTuple())
		require.Equal(t, items, rec.Array())
		require.Equal(t, "ARRAY", rec.String())
	})

	t.Run("bytes and image records", func(t *testing.T) {
		bytesRec := NewBytesRecord([]byte("abc"))
		require.True(t, bytesRec.IsBytes())
		require.False(t, bytesRec.IsTuple())
		require.Equal(t, "BYTES", bytesRec.String())
		require.Nil(t, bytesRec.Array())

		imageRec := NewImageRecord([]byte("img"), "image/png")
		require.True(t, imageRec.IsImage())
		require.False(t, imageRec.IsTuple())
		require.Equal(t, "IMAGE", imageRec.String())
		require.Equal(t, "image/png", imageRec.contentType)
	})

	t.Run("special records", func(t *testing.T) {
		require.Equal(t, "EOF", EofRecord.String())
		require.Equal(t, "CIRCUITBREAK", BreakRecord.String())

		errRec := ErrorRecord(errors.New("boom"))
		require.True(t, errRec.IsError())
		require.EqualError(t, errRec.Error(), "boom")
		require.Equal(t, "ERROR boom", errRec.String())

		normal := NewRecord("key", 1)
		require.Nil(t, normal.Error())
		require.True(t, normal.IsTuple())
		require.Equal(t, "K:string(key) V:int", normal.String())
	})

	t.Run("nil record string", func(t *testing.T) {
		var rec *Record
		require.Equal(t, "<nil>", rec.String())
	})
}

func TestRecordVariableAndTell(t *testing.T) {
	rec := NewRecord("key", 1)
	rec.SetVariable("answer", 42)

	v, err := rec.GetVariable("$answer")
	require.NoError(t, err)
	require.Equal(t, 42, v)

	v, err = rec.GetVariable("$missing")
	require.NoError(t, err)
	require.Nil(t, v)

	v, err = rec.GetVariable("answer")
	require.EqualError(t, err, "undefined variable 'answer'")
	require.Nil(t, v)

	v, err = NewRecord("other", 2).GetVariable("$answer")
	require.EqualError(t, err, "undefined variable '$answer'")
	require.Nil(t, v)

	rec.Tell(nil)
	rcv := &recordReceiver{name: "capture"}
	rec.Tell(rcv)
	require.Same(t, rec, rcv.received)
}

func TestRecordFlattenAndStringValueTypes(t *testing.T) {
	t.Run("flatten variants", func(t *testing.T) {
		require.Equal(t, []any{"k", 1, "two"}, NewRecord("k", []any{1, "two"}).Flatten())
		require.Equal(t, []any{"k", "value"}, NewRecord("k", "value").Flatten())
		require.Equal(t, []any{"k"}, NewRecord("k", nil).Flatten())
	})

	t.Run("string value types", func(t *testing.T) {
		rec := NewRecord("k", []any{1, "two", []any{true, 3.14, "x", 7}, 9})
		require.Equal(t, "int, string, []any{bool,float64,string,int,... (len=4)}, int, ... (len=4)", rec.StringValueTypes())

		matrix := NewRecord("k", [][]any{{1, "a"}, {true}, {3.14}, {"tail", 2}})
		require.Equal(t, "(len=4) [][]any{[0]{int, string},[1]{bool},[2]{float64},[3]{string, int}, ...}", matrix.StringValueTypes())
	})
}

func TestRecordEqualKeyAndValue(t *testing.T) {
	base := time.Date(2024, 1, 2, 3, 4, 5, 6, time.UTC)
	require.True(t, NewRecord(base, nil).EqualKey(NewRecord(base, nil)))
	require.False(t, NewRecord(base, nil).EqualKey(NewRecord(base.Add(time.Nanosecond), nil)))

	require.True(t, NewRecord([]int{1, 2}, nil).EqualKey(NewRecord([]int{1, 2}, nil)))
	require.False(t, NewRecord([]int{1, 2}, nil).EqualKey(NewRecord([]int{1, 3}, nil)))
	require.False(t, NewRecord("x", nil).EqualKey(nil))

	require.True(t, NewRecord("k", []any{1, "a"}).EqualValue(NewRecord("k", []any{1, "a"})))
	require.False(t, NewRecord("k", []any{1, "a"}).EqualValue(NewRecord("k", []any{2, "a"})))
	require.False(t, NewRecord("k", 1).EqualValue(nil))
}
