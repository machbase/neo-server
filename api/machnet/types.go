package machnet

import (
	"errors"
	"fmt"
	"net"
	"strings"
	"time"
)

// cmi protocol version: 4.0.2
const (
	cmiProtocolMajor = 4
	cmiProtocolMinor = 0
	cmiProtocolFix   = 2
)

const (
	cmiPacketMaxBody = 64 * 1024

	cmiProtoCnt         = 9
	cmiHandshakePrefix  = "CMI_INET"
	cmiHandshakeEndian  = "0"
	cmiHandshakeReady   = "CMI_READY"
	cmiHandshakePayload = cmiHandshakePrefix + cmiHandshakeEndian
)

const (
	cmiConnectProtocol     = 0
	cmiDisconnectProtocol  = 1
	cmiPrepareProtocol     = 6
	cmiExecuteProtocol     = 7
	cmiExecDirectProtocol  = 8
	cmiFetchProtocol       = 9
	cmiFreeProtocol        = 10
	cmiAppendOpenProtocol  = 11
	cmiAppendDataProtocol  = 12
	cmiAppendCloseProtocol = 13
)

const (
	cmiCVersionID  = 0x00000001
	cmiCClientID   = 0x00000002
	cmiCDatabaseID = 0x00000004
	cmiCEndianID   = 0x00000005
	cmiCUserID     = 0x00000006
	cmiCPasswordID = 0x00000007
	cmiCTimeoutID  = 0x00000008
	cmiCSIDID      = 0x00000040
	cmiCSHCID      = 0x00000041
	cmiCIPID       = 0x00000042
	cmiCTimezoneID = 0x00000070

	cmiRResultID   = 0x00000010
	cmiRMessageID  = 0x00000011
	cmiREMessageID = 0x00000012

	cmiPStatementID = 0x00000020
	cmiPBindsID     = 0x00000021
	cmiPIDID        = 0x00000022
	cmiPRowsID      = 0x00000023
	cmiPColumnsID   = 0x00000024
	cmiPTableID     = 0x00000025
	cmiPColNameID   = 0x00000026
	cmiPColTypeID   = 0x00000027
	cmiPParamTypeID = 0x00000029

	cmiEParamID  = 0x00000031
	cmiEEndianID = 0x00000034

	cmiDStatementID = 0x00000040

	cmiFIDID    = 0x00000050
	cmiFRowsID  = 0x00000051
	cmiFValueID = 0x00000052

	cmiXIDID            = 0x00000060
	cmiXAppendSuccessID = 0x00000061
	cmiXAppendFailureID = 0x00000062
)

const (
	cmimIDStmtType = 200
)

const (
	cmiStringType = 0x00000002
	cmiBinaryType = 0x00000003
	cmiSCharType  = 0x00000004
	cmiUCharType  = 0x00000005
	cmiSShortType = 0x00000006
	cmiUShortType = 0x00000007
	cmiSIntType   = 0x00000008
	cmiUIntType   = 0x00000009
	cmiSLongType  = 0x0000000a
	cmiULongType  = 0x0000000b
	cmiDateType   = 0x0000000c
	cmiRowsType   = 0x0000000d
	cmiTNumType   = 0x000000f1
	cmiNumType    = 0x000000f2
)

const (
	cmdFixFlag  = 0x0000
	cmdVarFlag  = 0x0001
	cmdTimeFlag = 0x0002

	cmdVarcharType = (0x0001 << 2) | cmdVarFlag
	cmdDateType    = (0x0001 << 2) | cmdTimeFlag
	cmdInt16Type   = (0x0001 << 2) | cmdFixFlag
	cmdInt32Type   = (0x0002 << 2) | cmdFixFlag
	cmdInt64Type   = (0x0003 << 2) | cmdFixFlag
	cmdFlt32Type   = (0x0004 << 2) | cmdFixFlag
	cmdFlt64Type   = (0x0005 << 2) | cmdFixFlag
	cmdNulType     = (0x0006 << 2) | cmdFixFlag
	cmdIpv4Type    = (0x0008 << 2) | cmdFixFlag
	cmdIpv6Type    = (0x0009 << 2) | cmdFixFlag
	cmdBoolType    = (0x000a << 2) | cmdFixFlag
	cmdCharType    = (0x000b << 2) | cmdVarFlag
	cmdTextType    = (0x000c << 2) | cmdVarFlag
	cmdClobType    = (0x000d << 2) | cmdVarFlag
	cmdBlobType    = (0x000e << 2) | cmdVarFlag
	cmdJSONType    = (0x000f << 2) | cmdVarFlag
	cmdBinaryType  = (0x0018 << 2) | cmdVarFlag
	cmdIPNetType   = (0x0019 << 2) | cmdVarFlag
	cmdUInt16Type  = (0x001a << 2) | cmdFixFlag
	cmdUInt32Type  = (0x001b << 2) | cmdFixFlag
	cmdUInt64Type  = (0x001c << 2) | cmdFixFlag
)

const (
	cmiOKResult      uint64 = 0x724f4b5f00000000
	cmiCMErrorResult uint64 = 0x72434d5f00000000
	cmiLastResult    uint64 = 0x724c535400000000
)

const (
	shortNull    = int16(-32768)
	ushortNull   = uint16(0xffff)
	intNull      = int32(-2147483648)
	uintNull     = uint32(0xffffffff)
	longNull     = int64(-9223372036854775808)
	ulongNull    = uint64(0xffffffffffffffff)
	floatNull    = float32(3.402823466e+38)
	doubleNull   = float64(1.7976931348623158e+308)
	datetimeNull = uint64(0xffffffffffffffff)
)

const (
	sqlParamInput byte = 1
)

const (
	defaultConnectTimeout  = 5 * time.Second
	defaultQueryTimeout    = 60 * time.Second
	defaultReadBufferSize  = 128 * 1024
	defaultWriteBufferSize = 128 * 1024
)

func align8(v int) int {
	return (v + 7) &^ 7
}

type SqlType int

const (
	MACHCLI_SQL_TYPE_INT16    SqlType = 0
	MACHCLI_SQL_TYPE_INT32    SqlType = 1
	MACHCLI_SQL_TYPE_INT64    SqlType = 2
	MACHCLI_SQL_TYPE_DATETIME SqlType = 3
	MACHCLI_SQL_TYPE_FLOAT    SqlType = 4
	MACHCLI_SQL_TYPE_DOUBLE   SqlType = 5
	MACHCLI_SQL_TYPE_IPV4     SqlType = 6
	MACHCLI_SQL_TYPE_IPV6     SqlType = 7
	MACHCLI_SQL_TYPE_STRING   SqlType = 8
	MACHCLI_SQL_TYPE_BINARY   SqlType = 9
)

type CType int

const (
	MACHCLI_C_TYPE_INT16  CType = 101
	MACHCLI_C_TYPE_INT32  CType = 102
	MACHCLI_C_TYPE_INT64  CType = 103
	MACHCLI_C_TYPE_FLOAT  CType = 104
	MACHCLI_C_TYPE_DOUBLE CType = 105
	MACHCLI_C_TYPE_CHAR   CType = 106
)

type StmtType int

func (typ StmtType) IsSelect() bool       { return typ == 512 }
func (typ StmtType) IsDDL() bool          { return typ >= 1 && typ <= 255 }
func (typ StmtType) IsAlterSystem() bool  { return typ >= 256 && typ <= 511 }
func (typ StmtType) IsInsert() bool       { return typ == 513 }
func (typ StmtType) IsDelete() bool       { return typ >= 514 && typ <= 518 }
func (typ StmtType) IsInsertSelect() bool { return typ == 519 }
func (typ StmtType) IsUpdate() bool       { return typ == 520 }
func (typ StmtType) IsExecRollup() bool   { return typ >= 522 && typ <= 524 }

type ParamDesc struct {
	Type      SqlType
	Precision int
	Scale     int
	Nullable  bool
}

type StatusError struct {
	code int
	msg  string
}

func (e *StatusError) Error() string {
	if e.msg == "" {
		return fmt.Sprintf("server error code=%d", e.code)
	}
	if e.code > 0 {
		return fmt.Sprintf("server error code=%d message=%s", e.code, e.msg)
	}
	return e.msg
}

func (st *StatusError) setErr(err error) {
	if st == nil {
		return
	}
	if err == nil {
		st.code = 0
		st.msg = ""
		return
	}
	var se *StatusError
	if errors.As(err, &se) {
		st.code = se.code
		st.msg = se.msg
		if st.msg == "" {
			st.msg = err.Error()
		}
		return
	}
	st.code = 0
	st.msg = err.Error()
}

func protocolVersion() uint64 {
	return (uint64(cmiProtocolMajor&0xffff) << 48) |
		(uint64(cmiProtocolMinor&0xffff) << 32) |
		uint64(cmiProtocolFix&0xffffffff)
}

func makeClientErr(msg string) error {
	if msg == "" {
		msg = "unknown client error"
	}
	return &StatusError{code: 0, msg: msg}
}

func makeServerErr(code int, msg string) error {
	return &StatusError{code: code, msg: msg}
}

func sqlTypeToCmdType(sqlType SqlType) int {
	switch sqlType {
	case MACHCLI_SQL_TYPE_INT16:
		return cmdInt16Type
	case MACHCLI_SQL_TYPE_INT32:
		return cmdInt32Type
	case MACHCLI_SQL_TYPE_INT64:
		return cmdInt64Type
	case MACHCLI_SQL_TYPE_DATETIME:
		return cmdDateType
	case MACHCLI_SQL_TYPE_FLOAT:
		return cmdFlt32Type
	case MACHCLI_SQL_TYPE_DOUBLE:
		return cmdFlt64Type
	case MACHCLI_SQL_TYPE_IPV4:
		return cmdIpv4Type
	case MACHCLI_SQL_TYPE_IPV6:
		return cmdIpv6Type
	case MACHCLI_SQL_TYPE_BINARY:
		return cmdBinaryType
	default:
		return cmdVarcharType
	}
}

func spinerTypeToSQLType(spinerType int) SqlType {
	switch spinerType {
	case cmdBoolType, cmdInt16Type, cmdUInt16Type:
		return MACHCLI_SQL_TYPE_INT16
	case cmdInt32Type, cmdUInt32Type:
		return MACHCLI_SQL_TYPE_INT32
	case cmdInt64Type, cmdUInt64Type:
		return MACHCLI_SQL_TYPE_INT64
	case cmdDateType:
		return MACHCLI_SQL_TYPE_DATETIME
	case cmdFlt32Type:
		return MACHCLI_SQL_TYPE_FLOAT
	case cmdFlt64Type:
		return MACHCLI_SQL_TYPE_DOUBLE
	case cmdIpv4Type:
		return MACHCLI_SQL_TYPE_IPV4
	case cmdIpv6Type:
		return MACHCLI_SQL_TYPE_IPV6
	case cmdBinaryType, cmdBlobType:
		return MACHCLI_SQL_TYPE_BINARY
	default:
		return MACHCLI_SQL_TYPE_STRING
	}
}

func parseConnString(connStr string) (host string, port int, user string, pass string, alt []net.TCPAddr, fetchRows int64, err error) {
	m := map[string]string{}
	for _, entry := range strings.Split(connStr, ";") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		kv := strings.SplitN(entry, "=", 2)
		if len(kv) != 2 {
			continue
		}
		k := strings.ToUpper(strings.TrimSpace(kv[0]))
		v := strings.TrimSpace(kv[1])
		m[k] = v
	}
	host = m["SERVER"]
	if host == "" {
		host = "127.0.0.1"
	}
	port = 5656
	if p := strings.TrimSpace(m["PORT_NO"]); p != "" {
		_, scanErr := fmt.Sscanf(p, "%d", &port)
		if scanErr != nil {
			return "", 0, "", "", nil, 0, fmt.Errorf("invalid PORT_NO: %w", scanErr)
		}
	}
	user = m["UID"]
	pass = m["PWD"]
	fetchRows = defaultFetchRows
	if rowsStr := strings.TrimSpace(m["FETCH_ROWS"]); rowsStr != "" {
		if _, scanErr := fmt.Sscanf(rowsStr, "%d", &fetchRows); scanErr != nil {
			return "", 0, "", "", nil, 0, fmt.Errorf("invalid FETCH_ROWS: %w", scanErr)
		}
		if fetchRows <= 0 {
			return "", 0, "", "", nil, 0, fmt.Errorf("invalid FETCH_ROWS: %d", fetchRows)
		}
	}
	if altEntry := strings.TrimSpace(m["ALTERNATIVE_SERVERS"]); altEntry != "" {
		for _, token := range strings.Split(altEntry, ",") {
			token = strings.TrimSpace(token)
			if token == "" {
				continue
			}
			h, pStr, ok := strings.Cut(token, ":")
			if !ok {
				continue
			}
			var p int
			if _, scanErr := fmt.Sscanf(strings.TrimSpace(pStr), "%d", &p); scanErr != nil {
				continue
			}
			alt = append(alt, net.TCPAddr{IP: net.ParseIP(strings.TrimSpace(h)), Port: p})
		}
	}
	return host, port, user, pass, alt, fetchRows, nil
}

func inferStmtType(sql string) int {
	t := strings.TrimSpace(strings.ToUpper(sql))
	if t == "" {
		return 0
	}
	head := t
	if idx := strings.IndexByte(t, ' '); idx >= 0 {
		head = t[:idx]
	}
	switch head {
	case "SELECT":
		return 512
	case "INSERT":
		if strings.Contains(t, "SELECT") {
			return 519
		}
		return 513
	case "DELETE":
		return 514
	case "UPDATE":
		return 520
	case "ALTER":
		if strings.HasPrefix(t, "ALTER SYSTEM") {
			return 256
		}
		return 1
	case "CREATE", "DROP", "TRUNCATE":
		return 1
	case "EXEC":
		return 522
	default:
		return 0
	}
}

func isVariableSpinerType(spinerType int) bool {
	return (spinerType & cmdVarFlag) == cmdVarFlag
}

func computeColumnLength(spinerType int, precision int) int {
	switch spinerType {
	case cmdInt16Type, cmdUInt16Type:
		return 2
	case cmdInt32Type, cmdUInt32Type:
		return 4
	case cmdInt64Type, cmdUInt64Type:
		return 8
	case cmdFlt32Type:
		return 4
	case cmdFlt64Type:
		return 8
	case cmdDateType:
		return 8
	case cmdIpv4Type:
		return 5
	case cmdIpv6Type:
		return 17
	case cmdBoolType:
		return 2
	case cmdNulType:
		return 0
	default:
		return precision
	}
}

func extractSpinerType(cmType uint64) int {
	return int((cmType >> 56) & 0xff)
}

func extractPrecision(cmType uint64) int {
	return int((cmType >> 28) & 0x0fffffff)
}

func extractScale(cmType uint64) int {
	return int(cmType & 0x0fffffff)
}

func statusCode(v uint64) uint64 {
	return v & 0xffffffff00000000
}

func statusErrNo(v uint64) int {
	return int(v & 0xffffffff)
}
