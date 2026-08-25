package tql

import (
	"bytes"
	"log/slog"
	"testing"

	"github.com/machbase/neo-client/v2/api"
	"github.com/machbase/neo-server/v8/mods/logging"
	"github.com/stretchr/testify/require"
)

func TestJSLog(t *testing.T) {
	t.Run("plain writer", func(t *testing.T) {
		buffer := &bytes.Buffer{}
		logger := &JSLog{w: buffer}

		logger.Print("print")
		logger.Println(" line")
		logger.Printf("value=%d", 42)
		logger.Log(slog.LevelWarn, "warning")

		require.Contains(t, buffer.String(), "print line\nvalue=42")
		require.Contains(t, buffer.String(), "[WARN]")
		require.Contains(t, buffer.String(), "warning")
	})

	t.Run("logging writer", func(t *testing.T) {
		buffer := &bytes.Buffer{}
		writer := logging.NewLog("script-test", buffer)
		writer.SetLevel(logging.LevelInfo)

		(&JSLog{w: writer}).Log(slog.LevelInfo, "message")

		require.Contains(t, buffer.String(), "message")
	})

	t.Run("task writer", func(t *testing.T) {
		buffer := &bytes.Buffer{}
		task := NewTask()
		task.SetLogWriter(buffer)
		task.SetLogLevel(INFO)

		(&JSLog{w: task}).Println("message")

		require.Contains(t, buffer.String(), "message")
	})
}

func TestScriptDataTypes(t *testing.T) {
	tests := map[string]api.DataType{
		"int16":    api.DataTypeInt16,
		"int32":    api.DataTypeInt32,
		"int64":    api.DataTypeInt64,
		"datetime": api.DataTypeDatetime,
		"float":    api.DataTypeFloat32,
		"double":   api.DataTypeFloat64,
		"ipv4":     api.DataTypeIPv4,
		"ipv6":     api.DataTypeIPv6,
		"varchar":  api.DataTypeString,
		"binary":   api.DataTypeBinary,
		"numeric":  api.DataTypeDecimal,
		"bool":     api.DataTypeBoolean,
		"int8":     api.DataTypeByte,
		"unknown":  api.DataType("Unsupported DataType: unknown"),
	}

	for input, expect := range tests {
		t.Run(input, func(t *testing.T) {
			require.Equal(t, expect, toDataType(input))
		})
	}
}
