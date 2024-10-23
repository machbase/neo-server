package types_test

import (
	"database/sql/driver"
	"net"
	"testing"
	"time"

	"github.com/machbase/neo-server/api/types"
	"github.com/stretchr/testify/require"
)

func TestScan(t *testing.T) {
	t.Parallel()

	now := time.Unix(0, 1729578712564320000)

	tests := []struct {
		name   string
		src    any
		dst    any
		expect any
	}{
		///////////////////////////////////
		// src: int
		{name: "int to int   ", src: int(321), dst: new(int), expect: int(321)},
		{name: "int to uint  ", src: int(123), dst: new(uint), expect: uint(123)},
		{name: "int to int16 ", src: int(123), dst: new(int16), expect: uint(123)},
		{name: "int to uint16", src: int(123), dst: new(uint16), expect: uint(123)},
		{name: "int to int32 ", src: int(123), dst: new(int32), expect: int32(123)},
		{name: "int to uint32", src: int(123), dst: new(uint32), expect: uint32(123)},
		{name: "int to int64 ", src: int(123), dst: new(int64), expect: int64(123)},
		{name: "int to uint64", src: int(123), dst: new(uint64), expect: uint64(123)},
		{name: "int to string", src: int(123), dst: new(string), expect: "123"},
		///////////////////////////////////
		// src: int16
		{name: "int16 to int   ", src: int16(321), dst: new(int), expect: int(321)},
		{name: "int16 to uint  ", src: int16(123), dst: new(uint), expect: uint(123)},
		{name: "int16 to int16 ", src: int16(123), dst: new(int16), expect: uint(123)},
		{name: "int16 to uint16", src: int16(123), dst: new(uint16), expect: uint(123)},
		{name: "int16 to int32 ", src: int16(123), dst: new(int32), expect: int32(123)},
		{name: "int16 to uint32", src: int16(123), dst: new(uint32), expect: uint32(123)},
		{name: "int16 to int64 ", src: int16(123), dst: new(int64), expect: int64(123)},
		{name: "int16 to uint64", src: int16(123), dst: new(uint64), expect: uint64(123)},
		{name: "int16 to string", src: int16(123), dst: new(string), expect: "123"},
		///////////////////////////////////
		// src: int32
		{name: "int32 to int   ", src: int32(321), dst: new(int), expect: int(321)},
		{name: "int32 to uint  ", src: int32(123), dst: new(uint), expect: uint(123)},
		{name: "int32 to int16 ", src: int32(123), dst: new(int16), expect: uint(123)},
		{name: "int32 to uint16", src: int32(123), dst: new(uint16), expect: uint(123)},
		{name: "int32 to int32 ", src: int32(123), dst: new(int32), expect: int32(123)},
		{name: "int32 to uint32", src: int32(123), dst: new(uint32), expect: uint32(123)},
		{name: "int32 to int64 ", src: int32(123), dst: new(int64), expect: int64(123)},
		{name: "int32 to uint64", src: int32(123), dst: new(uint64), expect: uint64(123)},
		{name: "int32 to string", src: int32(123), dst: new(string), expect: "123"},
		///////////////////////////////////
		// src: int64
		{name: "int64 to int   ", src: int64(987654321), dst: new(int), expect: int(987654321)},
		{name: "int64 to uint  ", src: int64(987654321), dst: new(uint), expect: uint(987654321)},
		{name: "int64 to int16 ", src: int64(987654321), dst: new(int16), expect: int16(26801)},
		{name: "int64 to uint16", src: int64(987654321), dst: new(uint16), expect: uint16(26801)},
		{name: "int64 to int32 ", src: int64(987654321), dst: new(int32), expect: int32(987654321)},
		{name: "int64 to uint32", src: int64(987654321), dst: new(uint32), expect: uint32(987654321)},
		{name: "int64 to int64 ", src: int64(987654321), dst: new(int64), expect: int64(987654321)},
		{name: "int64 to uint64", src: int64(987654321), dst: new(uint64), expect: uint64(987654321)},
		{name: "int64 to string", src: int64(987654321), dst: new(string), expect: "987654321"},
		///////////////////////////////////
		// src: int64
		{name: "time to int64   ", src: now, dst: new(int64), expect: int64(1729578712564320000)},
		{name: "time to time    ", src: now, dst: new(time.Time), expect: now},
		{name: "time to string  ", src: now, dst: new(string), expect: "2024-10-22T06:31:52Z"},
		///////////////////////////////////
		// src: float32
		{name: "float32 to float32", src: float32(3.141592), dst: new(float32), expect: float32(3.141592)},
		{name: "float32 to float64", src: float32(3.141592), dst: new(float64), expect: float64(float32(3.141592))},
		{name: "float32 to string ", src: float32(3.141592), dst: new(string), expect: "3.141592"},
		///////////////////////////////////
		// src: float64
		{name: "float64 to float32", src: float64(3.141592), dst: new(float32), expect: float32(3.141592)},
		{name: "float64 to float64", src: float64(3.141592), dst: new(float64), expect: float64(3.141592)},
		{name: "float64 to string ", src: float64(3.141592), dst: new(string), expect: "3.141592"},
		///////////////////////////////////
		// src: string
		{name: "string to string", src: "1.2.3.4.5", dst: new(string), expect: "1.2.3.4.5"},
		{name: "string to []byte", src: "1.2.3.4.5", dst: new([]byte), expect: []byte("1.2.3.4.5")},
		{name: "string to net.IP", src: "192.168.1.10", dst: new(net.IP), expect: net.ParseIP("192.168.1.10")},
		///////////////////////////////////
		// src: []byte
		{name: "[]byte to []byte", src: []byte("1.2.3.4.5"), dst: new([]byte), expect: []byte("1.2.3.4.5")},
		{name: "[]byte to string", src: []byte("1.2.3.4.5"), dst: new(string), expect: "1.2.3.4.5"},
		///////////////////////////////////
		// src: net.IP
		{name: "net.IP to []byte", src: net.ParseIP("192.168.1.10"), dst: new(net.IP), expect: net.ParseIP("192.168.1.10")},
		{name: "net.IP to string", src: net.ParseIP("192.168.1.10"), dst: new(string), expect: "192.168.1.10"},
	}

	for _, tt := range tests {
		if err := types.Scan(tt.src, tt.dst); err != nil {
			t.Errorf("%s: Scan(%v, %v) got error: %v", tt.name, tt.src, tt.dst, err)
		}
		result := types.Unbox(tt.dst)
		require.EqualValues(t, tt.expect, result, "%s: Scan(%T, %T) got %v, want %v", tt.name, tt.src, tt.dst, result, tt.expect)

		if err := types.Scan(box(tt.src), tt.dst); err != nil {
			t.Errorf("%s: Scan(*%v, %v) got error: %v", tt.name, tt.src, tt.dst, err)
		}
		result = types.Unbox(tt.dst)
		require.EqualValues(t, tt.expect, result, "%s: Scan(*%T, %T) got %v, want %v", tt.name, tt.src, tt.dst, result, tt.expect)
	}
}

func box(val any) any {
	switch v := val.(type) {
	case int:
		return &v
	case uint:
		return &v
	case int16:
		return &v
	case uint16:
		return &v
	case int32:
		return &v
	case uint32:
		return &v
	case int64:
		return &v
	case uint64:
		return &v
	case float64:
		return &v
	case float32:
		return &v
	case string:
		return &v
	case time.Time:
		return &v
	case []byte:
		return &v
	case net.IP:
		return &v
	case driver.Value:
		return &v
	default:
		return val
	}
}
