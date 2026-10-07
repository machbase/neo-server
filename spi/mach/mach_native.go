package mach

import (
	"fmt"
	"net"
	"strings"
	"sync"
	"time"
	"unsafe"
)

/*
#cgo CFLAGS: -I${SRCDIR}/native
#include <machEngine.h>
#include <stdlib.h>
#include <stdio.h>
#include <signal.h>
#include <string.h>
#include <time.h>

extern void cliDefaultAppendErrorCallback(void* aStmtHandle, int aErrorCode, char* aErrorMessage, long aErrorBufLen, char* aRowBuf, long aRowBufLen);

static inline void cliAppendErrorCallback(void* aStmtHandle,
										  int   aErrorCode,
										  char* aErrorMessage,
										  long  aErrorBufLen,
										  char* aRowBuf,
										 long  aRowBufLen) {
	cliDefaultAppendErrorCallback(aStmtHandle, aErrorCode, aErrorMessage, aErrorBufLen, aRowBuf, aRowBufLen);
}
*/
import "C"

func LinkInfo() string {
	return LibMachLinkInfo
}

func EngInitialize(homeDir string, machPort int, flag int, envHandle *unsafe.Pointer) error {
	cstr := C.CString(homeDir)
	defer C.free(unsafe.Pointer(cstr))
	var tmpHandle unsafe.Pointer
	if rt := C.MachInitialize(cstr, C.int(machPort), C.int(flag), &tmpHandle); rt == 0 {
		*envHandle = tmpHandle
		return nil
	} else {
		return ErrDatabaseReturns("MachInitialize", int(rt))
	}
}

func EngFinalize(envHandle unsafe.Pointer) {
	C.MachFinalize(envHandle)
}

func EngCreateDatabase(envHandle unsafe.Pointer) error {
	if rt := C.MachCreateDB(envHandle); rt == 0 {
		return nil
	} else {
		return ErrDatabaseReturns("MachCreateDB", int(rt))
	}
}

func EngDestroyDatabase(envHandle unsafe.Pointer) error {
	if rt := C.MachDestroyDB(envHandle); rt == 0 {
		return nil
	} else {
		return ErrDatabaseReturns("MachDestroyDB", int(rt))
	}
}

func EngExistsDatabase(envHandle unsafe.Pointer) bool {
	rt := C.MachIsDBCreated(envHandle)
	return rt == 1
}

func EngRestoreDatabase(envHandle unsafe.Pointer, dbPath string) error {
	cstr := C.CString(dbPath)
	defer C.free(unsafe.Pointer(cstr))
	if rt := C.MachRestoreDB(envHandle, cstr); rt == 0 {
		return nil
	} else {
		return ErrDatabaseReturns("MachRestoreDB", int(rt))
	}
}

func EngStartup(envHandle unsafe.Pointer) error {
	if rt := C.MachStartupDB(envHandle); rt != 0 {
		dbErr := EngError(envHandle)
		if dbErr != nil {
			return dbErr
		} else {
			return ErrDatabaseReturns("MachStartupDB", int(rt))
		}
	}
	return nil
}

func EngShutdown(envHandle unsafe.Pointer) error {
	if rt := C.MachShutdownDB(envHandle); rt == 0 {
		return nil
	} else {
		dbErr := EngError(envHandle)
		if dbErr != nil {
			return dbErr
		} else {
			return ErrDatabaseReturns("MachShutdownDB", int(rt))
		}
	}
}

func EngConnectionCount(envHandle unsafe.Pointer) int {
	ret := C.MachGetConnectionCount(envHandle)
	return int(ret)
}

func EngConnect(envHandle unsafe.Pointer, username string, password string, conn *unsafe.Pointer) error {
	cusername := C.CString(username)
	cpassword := C.CString(password)
	defer func() {
		C.free(unsafe.Pointer(cusername))
		C.free(unsafe.Pointer(cpassword))
	}()
	var tmpConn unsafe.Pointer
	if rt := C.MachConnect(envHandle, cusername, cpassword, &tmpConn); rt == 0 {
		*conn = tmpConn
		return nil
	} else {
		dbErr := EngError(envHandle)
		if dbErr != nil {
			return dbErr
		} else {
			return ErrDatabaseReturns("MachConnect", int(rt))
		}
	}
}

func EngConnectTrust(envHandle unsafe.Pointer, username string, conn *unsafe.Pointer) error {
	cusername := C.CString(username)
	defer func() {
		C.free(unsafe.Pointer(cusername))
	}()
	var tmpConn unsafe.Pointer
	if rt := C.MachConnectNoAuth(envHandle, cusername, &tmpConn); rt == 0 {
		*conn = tmpConn
		return nil
	} else {
		dbErr := EngError(envHandle)
		if dbErr != nil {
			return dbErr
		} else {
			return ErrDatabaseReturns("MachConnect", int(rt))
		}
	}
}

func EngDisconnect(conn unsafe.Pointer) error {
	if rt := C.MachDisconnect(conn); rt == 0 {
		return nil
	} else {
		dbErr := EngError(conn)
		if dbErr != nil {
			return dbErr
		} else {
			return ErrDatabaseReturns("MachDisconnect", int(rt))
		}
	}
}

func EngCancel(conn unsafe.Pointer) error {
	if rt := C.MachCancel(conn); rt == 0 {
		return nil
	} else {
		dbErr := EngError(conn)
		if dbErr != nil {
			return dbErr
		} else {
			return ErrDatabaseReturns("MachCancel", int(rt))
		}
	}
}

func EngSessionID(conn unsafe.Pointer) (uint64, error) {
	rt := C.MachSessionID(conn)
	return uint64(rt), nil
}

func EngError(handle unsafe.Pointer) error {
	code := C.MachErrorCode(handle)
	msg := C.MachErrorMsg(handle)
	if code != 0 && msg != nil {
		return ErrDatabaseMach(int(code), C.GoString(msg))
	}
	return nil
}

// 0: id and password are correct
// 2080: user does not exist
// 2081: password is not correct
// int MachUserAuth(void* aEnvHandle, char* aUserName, char* aPassword);
func EngUserAuth(envHandle unsafe.Pointer, username string, password string) (bool, error) {
	cusername := C.CString(username)
	cpassword := C.CString(password)
	defer func() {
		C.free(unsafe.Pointer(cusername))
		C.free(unsafe.Pointer(cpassword))
	}()

	rt := C.MachUserAuth(envHandle, cusername, cpassword)
	switch rt {
	case 0:
		return true, nil
	case 2080:
		return false, nil
	case 2081:
		return false, nil
	default:
		return false, ErrDatabaseReturns("MachUserAuth", int(rt))
	}
}

func EngExplain(stmt unsafe.Pointer, full bool) (string, error) {
	var cstr = [1024 * 16]C.char{}
	var mode = 0
	if full {
		mode = 1
	}
	if rt := C.MachExplain(stmt, &cstr[0], C.int(len(cstr)), C.int(mode)); rt != 0 {
		stmtErr := EngError(stmt)
		if stmtErr != nil {
			return "", stmtErr
		} else {
			return "", ErrDatabaseReturns("MachExplain", int(rt))
		}
	}
	return C.GoString(&cstr[0]), nil
}

func EngAllocStmt(conn unsafe.Pointer, stmt *unsafe.Pointer) error {
	var ptr unsafe.Pointer
	if rt := C.MachAllocStmt(conn, &ptr); rt != 0 {
		dbErr := EngError(conn)
		if dbErr != nil {
			return dbErr
		} else {
			return ErrDatabaseReturns("MachAllocStmt", int(rt))
		}
	}
	*stmt = ptr
	return nil
}

func EngFreeStmt(stmt unsafe.Pointer) error {
	if rt := C.MachFreeStmt(stmt); rt != 0 {
		stmtErr := EngError(stmt)
		if stmtErr != nil {
			return stmtErr
		} else {
			return ErrDatabaseReturns("MachFreeStmt", int(rt))
		}
	}
	return nil
}

func EngPrepare(stmt unsafe.Pointer, sqlText string) error {
	cstr := C.CString(sqlText)
	defer C.free(unsafe.Pointer(cstr))
	if rt := C.MachPrepare(stmt, cstr); rt != 0 {
		stmtErr := EngError(stmt)
		if stmtErr != nil {
			return stmtErr
		} else {
			return ErrDatabaseReturns("MachPrepare", int(rt))
		}
	}
	return nil
}

func EngExecute(stmt unsafe.Pointer) error {
	if rt := C.MachExecute(stmt); rt != 0 {
		stmtErr := EngError(stmt)
		if stmtErr != nil {
			return stmtErr
		} else {
			return ErrDatabaseReturns("MachExecute", int(rt))
		}
	}
	return nil
}

func EngExecuteClean(stmt unsafe.Pointer) error {
	if rt := C.MachExecuteClean(stmt); rt != 0 {
		stmtErr := EngError(stmt)
		if stmtErr != nil {
			return stmtErr
		} else {
			return ErrDatabaseReturns("MachExecuteClean", int(rt))
		}
	}
	return nil
}

func EngDirectExecute(stmt unsafe.Pointer, sqlText string) error {
	cstr := C.CString(sqlText)
	defer C.free(unsafe.Pointer(cstr))
	if rt := C.MachDirectExecute(stmt, cstr); rt != 0 {
		stmtErr := EngError(stmt)
		if stmtErr != nil {
			return stmtErr
		} else {
			return ErrDatabaseReturns("MachDirectExecute", int(rt))
		}
	}
	return nil
}

func EngStmtType(stmt unsafe.Pointer) (int, error) {
	var typ C.int
	if rt := C.MachStmtType(stmt, &typ); rt != 0 {
		stmtErr := EngError(stmt)
		if stmtErr != nil {
			return 0, stmtErr
		} else {
			return 0, ErrDatabaseReturns("MachStmtType", int(rt))
		}
	}
	return int(typ), nil
}

func EngEffectRows(stmt unsafe.Pointer) (int64, error) {
	var rn C.ulonglong
	if rt := C.MachEffectRows(stmt, &rn); rt != 0 {
		stmtErr := EngError(stmt)
		if stmtErr != nil {
			return 0, stmtErr
		} else {
			return 0, ErrDatabaseReturns("MachEffectRows", int(rt))
		}
	}
	return int64(rn), nil
}

// return true if fetch success(record exists), otherwise false
func EngFetch(stmt unsafe.Pointer) (bool, error) {
	var fetchEnd C.int // 0 if record exists, otherwise 1
	if rt := C.MachFetch(stmt, &fetchEnd); rt != 0 {
		stmtErr := EngError(stmt)
		if stmtErr != nil {
			return false, stmtErr
		} else {
			return false, ErrDatabaseReturns("MachFetch", int(rt))
		}
	}
	return fetchEnd == 0, nil
}

func EngBindInt32(stmt unsafe.Pointer, idx int, val int32) error {
	if rt := C.MachBindInt32(stmt, C.int(idx), C.int(val)); rt != 0 {
		stmtErr := EngError(stmt)
		if stmtErr != nil {
			return stmtErr
		} else {
			return ErrDatabaseReturnsAtIdx("MachBindInt32", idx, int(rt))
		}
	}
	return nil
}

func EngBindInt64(stmt unsafe.Pointer, idx int, val int64) error {
	if rt := C.MachBindInt64(stmt, C.int(idx), C.longlong(val)); rt != 0 {
		stmtErr := EngError(stmt)
		if stmtErr != nil {
			return stmtErr
		} else {
			return ErrDatabaseReturnsAtIdx("MachBindInt64", idx, int(rt))
		}
	}
	return nil
}

func EngBindFloat64(stmt unsafe.Pointer, idx int, val float64) error {
	if rt := C.MachBindDouble(stmt, C.int(idx), C.double(val)); rt != 0 {
		stmtErr := EngError(stmt)
		if stmtErr != nil {
			return stmtErr
		} else {
			return ErrDatabaseReturnsAtIdx("MachBindDouble", idx, int(rt))
		}
	}
	return nil
}

func EngBindString(stmt unsafe.Pointer, idx int, val string) error {
	cstr := C.CString(val)
	defer C.free(unsafe.Pointer(cstr))
	if rt := C.MachBindString(stmt, C.int(idx), cstr, C.int(len(val))); rt != 0 {
		stmtErr := EngError(stmt)
		if stmtErr != nil {
			return stmtErr
		} else {
			return ErrDatabaseReturnsAtIdx("MachBindString", idx, int(rt))
		}
	}
	return nil
}

func EngBindBinary(stmt unsafe.Pointer, idx int, data []byte) error {
	if len(data) == 0 {
		// For empty slice, bind as empty binary
		data = []byte{0}
	}
	ptr := unsafe.Pointer(&data[0])
	if rt := C.MachBindBinary(stmt, C.int(idx), ptr, C.int(len(data))); rt != 0 {
		stmtErr := EngError(stmt)
		if stmtErr != nil {
			return stmtErr
		} else {
			return ErrDatabaseReturnsAtIdx("MachBindBinary", idx, int(rt))
		}
	}
	return nil
}

func EngBindNull(stmt unsafe.Pointer, idx int) error {
	if rt := C.MachBindNull(stmt, C.int(idx)); rt != 0 {
		stmtErr := EngError(stmt)
		if stmtErr != nil {
			return stmtErr
		} else {
			return ErrDatabaseReturnsAtIdx("MachBindNull", idx, int(rt))
		}
	}
	return nil
}

func EngColumnCount(stmt unsafe.Pointer) (int, error) {
	var count C.int = 0
	if rt := C.MachColumnCount(stmt, &count); rt != 0 {
		stmtErr := EngError(stmt)
		if stmtErr != nil {
			return 0, stmtErr
		} else {
			return 0, ErrDatabaseReturns("MachColumnCount", int(rt))
		}
	}
	return int(count), nil
}

func EngColumnInfo(stmt unsafe.Pointer, idx int, pName *string, pType *int, pSize *int, pLength *int) error {
	var nfo C.MachEngineColumnInfo
	if rt := C.MachColumnInfo(stmt, C.int(idx), &nfo); rt != 0 {
		stmtErr := EngError(stmt)
		if stmtErr != nil {
			return stmtErr
		} else {
			return ErrDatabaseReturns("MachColumnInfo", int(rt))
		}
	}

	*pName = C.GoString(&nfo.mColumnName[0])
	*pType = int(nfo.mColumnType)
	*pSize = int(nfo.mColumnSize)
	*pLength = int(nfo.mColumnLength)
	return nil
}

func EngColumnName(stmt unsafe.Pointer, idx int) (string, error) {
	var cstr = [100]C.char{}
	if rt := C.MachColumnName(stmt, C.int(idx), &cstr[0], C.int(len(cstr))); rt != 0 {
		stmtErr := EngError(stmt)
		if stmtErr != nil {
			return fmt.Sprintf("col-%d", idx), stmtErr
		} else {
			return fmt.Sprintf("col-%d", idx), ErrDatabaseReturns("MachColumnName", int(rt))
		}
	}
	return C.GoString(&cstr[0]), nil
}

func EngColumnType(stmt unsafe.Pointer, idx int) (int, int, error) {
	var typ C.int = 0
	var siz C.int = 0
	if rt := C.MachColumnType(stmt, C.int(idx), &typ, &siz); rt != 0 {
		stmtErr := EngError(stmt)
		if stmtErr != nil {
			return 0, 0, stmtErr
		} else {
			return 0, 0, ErrDatabaseReturnsAtIdx("MachColumnType", idx, int(rt))
		}
	}
	return int(typ), int(siz), nil
}

func EngColumnLength(stmt unsafe.Pointer, idx int) (int, error) {
	var length C.int = 0
	if rt := C.MachColumnLength(stmt, C.int(idx), &length); rt != 0 {
		stmtErr := EngError(stmt)
		if stmtErr != nil {
			return 0, stmtErr
		} else {
			return 0, ErrDatabaseReturnsAtIdx("MachColumnLength", idx, int(rt))
		}
	}
	return int(length), nil
}

// returns true if not null
func EngColumnData(stmt unsafe.Pointer, idx int, buf unsafe.Pointer, bufLen int) (bool, error) {
	var isNull C.char
	if rt := C.MachColumnData(stmt, C.int(idx), buf, C.int(bufLen), &isNull); rt != 0 {
		stmtErr := EngError(stmt)
		if stmtErr != nil {
			return false, stmtErr
		} else {
			return false, ErrDatabaseReturnsAtIdx("MachColumnData", idx, int(rt))
		}
	}
	return isNull == 0, nil
}

// returns int16 and true if NOT NULL, false if NULL
func EngColumnDataInt16(stmt unsafe.Pointer, idx int) (int16, bool, error) {
	var val C.short
	var isNull C.char
	if rt := C.MachColumnDataInt16(stmt, C.int(idx), &val, &isNull); rt != 0 {
		stmtErr := EngError(stmt)
		if stmtErr != nil {
			return 0, false, stmtErr
		} else {
			return 0, false, ErrDatabaseReturnsAtIdx("MachColumnDataInt16", idx, int(rt))
		}
	}
	return int16(val), isNull == 0, nil
}

// returns uint16 and true if NOT NULL, false if NULL
func EngColumnDataUInt16(stmt unsafe.Pointer, idx int) (uint16, bool, error) {
	var val C.ushort
	var isNull C.char
	if rt := C.MachColumnDataUInt16(stmt, C.int(idx), &val, &isNull); rt != 0 {
		stmtErr := EngError(stmt)
		if stmtErr != nil {
			return 0, false, stmtErr
		} else {
			return 0, false, ErrDatabaseReturnsAtIdx("MachColumnDataUInt16", idx, int(rt))
		}
	}
	return uint16(val), isNull == 0, nil
}

// returns int32 and true if NOT NULL, false if NULL
func EngColumnDataInt32(stmt unsafe.Pointer, idx int) (int32, bool, error) {
	var val C.int
	var isNull C.char
	if rt := C.MachColumnDataInt32(stmt, C.int(idx), &val, &isNull); rt != 0 {
		stmtErr := EngError(stmt)
		if stmtErr != nil {
			return 0, false, stmtErr
		} else {
			return 0, false, ErrDatabaseReturnsAtIdx("MachColumnDataInt32", idx, int(rt))
		}
	}
	return int32(val), isNull == 0, nil
}

// returns uint32 and true if NOT NULL, false if NULL
func EngColumnDataUInt32(stmt unsafe.Pointer, idx int) (uint32, bool, error) {
	var val C.uint
	var isNull C.char
	if rt := C.MachColumnDataUInt32(stmt, C.int(idx), &val, &isNull); rt != 0 {
		stmtErr := EngError(stmt)
		if stmtErr != nil {
			return 0, false, stmtErr
		} else {
			return 0, false, ErrDatabaseReturnsAtIdx("MachColumnDataUInt32", idx, int(rt))
		}
	}
	return uint32(val), isNull == 0, nil
}

// returns int64 and true if NOT NULL, false if NULL
func EngColumnDataInt64(stmt unsafe.Pointer, idx int) (int64, bool, error) {
	var val C.longlong
	var isNull C.char
	if rt := C.MachColumnDataInt64(stmt, C.int(idx), &val, &isNull); rt != 0 {
		stmtErr := EngError(stmt)
		if stmtErr != nil {
			return 0, false, stmtErr
		} else {
			return 0, false, ErrDatabaseReturnsAtIdx("MachColumnDataInt64", idx, int(rt))
		}
	}
	return int64(val), isNull == 0, nil
}

// returns uint64 and true if NOT NULL, false if NULL
func EngColumnDataUInt64(stmt unsafe.Pointer, idx int) (uint64, bool, error) {
	var val C.ulonglong
	var isNull C.char
	if rt := C.MachColumnDataUInt64(stmt, C.int(idx), &val, &isNull); rt != 0 {
		stmtErr := EngError(stmt)
		if stmtErr != nil {
			return 0, false, stmtErr
		} else {
			return 0, false, ErrDatabaseReturnsAtIdx("MachColumnDataUInt64", idx, int(rt))
		}
	}
	return uint64(val), isNull == 0, nil
}

// returns Time and true if NOT NULL, false if NULL
func EngColumnDataDateTime(stmt unsafe.Pointer, idx int) (time.Time, bool, error) {
	var val C.longlong
	var isNull C.char
	if rt := C.MachColumnDataDateTime(stmt, C.int(idx), &val, &isNull); rt != 0 {
		stmtErr := EngError(stmt)
		if stmtErr != nil {
			return time.Time{}, false, stmtErr
		} else {
			return time.Time{}, false, ErrDatabaseReturnsAtIdx("MachColumnDataDateTime", idx, int(rt))
		}
	}
	return time.Unix(0, int64(val)), isNull == 0, nil
}

// returns float32 and true if NOT NULL, false if NULL
func EngColumnDataFloat32(stmt unsafe.Pointer, idx int) (float32, bool, error) {
	var val C.float
	var isNull C.char
	if rt := C.MachColumnDataFloat(stmt, C.int(idx), &val, &isNull); rt != 0 {
		stmtErr := EngError(stmt)
		if stmtErr != nil {
			return 0, false, stmtErr
		} else {
			return 0, false, ErrDatabaseReturnsAtIdx("MachColumnDataFloat", idx, int(rt))
		}
	}
	return float32(val), isNull == 0, nil
}

// returns float64 and true if NOT NULL, false if NULL
func EngColumnDataFloat64(stmt unsafe.Pointer, idx int) (float64, bool, error) {
	var val C.double
	var isNull C.char
	if rt := C.MachColumnDataDouble(stmt, C.int(idx), &val, &isNull); rt != 0 {
		stmtErr := EngError(stmt)
		if stmtErr != nil {
			return 0, false, stmtErr
		} else {
			return 0, false, ErrDatabaseReturnsAtIdx("MachColumnDataDouble", idx, int(rt))
		}
	}
	return float64(val), isNull == 0, nil
}

// returns net.IP (v4) and true if NOT NULL, false if NULL
func EngColumnDataIPv4(stmt unsafe.Pointer, idx int) (net.IP, bool, error) {
	var val [net.IPv4len + 1]byte
	var isNull C.char
	// 주의) val[0]는 IP version
	if rt := C.MachColumnDataIPV4(stmt, C.int(idx), unsafe.Pointer(&val), &isNull); rt != 0 {
		stmtErr := EngError(stmt)
		if stmtErr != nil {
			return net.IPv6zero, false, stmtErr
		} else {
			return net.IPv4zero, false, ErrDatabaseReturnsAtIdx("MachColumnDataIPv4", idx, int(rt))
		}
	}
	return net.IP(val[1:]), isNull == 0, nil
}

// returns net.IP (v6) and true if NOT NULL, false if NULL
func EngColumnDataIPv6(stmt unsafe.Pointer, idx int) (net.IP, bool, error) {
	var val [net.IPv6len + 1]byte
	var isNull C.char
	// 주의) val[0]는 IP version
	if rt := C.MachColumnDataIPV6(stmt, C.int(idx), unsafe.Pointer(&val), &isNull); rt != 0 {
		stmtErr := EngError(stmt)
		if stmtErr != nil {
			return net.IPv6zero, false, stmtErr
		} else {
			return net.IPv6zero, false, ErrDatabaseReturnsAtIdx("MachColumnDataIPv6", idx, int(rt))
		}
	}
	return net.IP(val[1:]), isNull == 0, nil
}

// returns string and true if NOT NULL, false if NULL
func EngColumnDataString(stmt unsafe.Pointer, idx int) (string, bool, error) {
	length, err := EngColumnLength(stmt, idx)
	if err != nil {
		return "", false, ErrDatabaseWrap("machColumnDataString", err)
	}
	if length == 0 {
		return "", false, nil
	}
	buf := make([]byte, length)
	val := (*C.char)(unsafe.Pointer(&buf[0]))
	var isNull C.char
	if rt := C.MachColumnDataString(stmt, C.int(idx), val, C.int(length), &isNull); rt != 0 {
		stmtErr := EngError(stmt)
		if stmtErr != nil {
			return "", false, stmtErr
		} else {
			return "", false, ErrDatabaseReturnsAtIdx("MachColumnDataString", idx, int(rt))
		}
	}
	return string(buf), isNull == 0, nil
}

// returns []byte and true if NOT NULL, false if NULL
func EngColumnDataBinary(stmt unsafe.Pointer, idx int) ([]byte, bool, error) {
	length, err := EngColumnLength(stmt, idx)
	if err != nil {
		return nil, false, ErrDatabaseWrap("machColumnDataString", err)
	}
	if length == 0 {
		return []byte{}, false, nil
	}
	buf := make([]byte, length)
	var isNull C.char
	val := (*C.char)(unsafe.Pointer(&buf[0]))
	if rt := C.MachColumnDataString(stmt, C.int(idx), val, C.int(length), &isNull); rt != 0 {
		stmtErr := EngError(stmt)
		if stmtErr != nil {
			return nil, false, stmtErr
		} else {
			return nil, false, ErrDatabaseReturnsAtIdx("MachColumnDataString", idx, int(rt))
		}
	}
	return buf, isNull == 0, nil
}

func EngAppendOpen(stmt unsafe.Pointer, tableName string) error {
	cstr := C.CString(strings.ToUpper(tableName))
	defer C.free(unsafe.Pointer(cstr))
	if rt := C.MachAppendOpen(stmt, cstr); rt != 0 {
		stmtErr := EngError(stmt)
		if stmtErr != nil {
			return stmtErr
		} else {
			return ErrDatabaseReturns("MachAppendOpen", int(rt))
		}
	}
	return nil
}

func EngAppendClose(stmt unsafe.Pointer) (int64, int64, error) {
	var successCount C.ulonglong
	var failureCount C.ulonglong
	if rt := C.MachAppendClose(stmt, &successCount, &failureCount); rt != 0 {
		stmtErr := EngError(stmt)
		if stmtErr != nil {
			return 0, 0, stmtErr
		} else {
			return 0, 0, ErrDatabaseReturns("MachAppendClose", int(rt))
		}
	}
	return int64(successCount), int64(failureCount), nil
}

type AppendBuffer struct {
	sync.Mutex
	stmt        unsafe.Pointer
	columnTypes []string
	columnNames []string
	buffer      []C.MachEngineAppendParam
}

func EngMakeAppendBuffer(stmt unsafe.Pointer, columnNames []string, columnTypes []string) *AppendBuffer {
	ret := &AppendBuffer{}
	ret.stmt = stmt
	ret.columnNames = columnNames
	ret.columnTypes = columnTypes
	ret.buffer = make([]C.MachEngineAppendParam, len(columnNames))
	return ret
}

func (ab *AppendBuffer) Append(vals ...any) error {
	ab.Lock()
	defer ab.Unlock()
	if len(vals) != len(ab.columnNames) {
		return ErrDatabaseAppendWrongValueCount(len(ab.columnNames), len(vals))
	}
	for i, val := range vals {
		if val == nil {
			ab.buffer[i].mIsNull = 1
			continue
		} else {
			ab.buffer[i].mIsNull = 0
		}
		cName := ab.columnNames[i]
		cType := ab.columnTypes[i]
		buffer := ab.buffer

		switch cType {
		default:
			return ErrDatabaseAppendUnknownType(cType)
		case "short", "int16", "uint16":
			switch v := val.(type) {
			default:
				return ErrDatabaseAppendWrongType(v, cName, cType)
			case uint16:
				*(*C.short)(unsafe.Pointer(&buffer[i].mData[0])) = C.short(v)
			case *uint16:
				*(*C.short)(unsafe.Pointer(&buffer[i].mData[0])) = C.short(*v)
			case int16:
				*(*C.short)(unsafe.Pointer(&buffer[i].mData[0])) = C.short(v)
			case *int16:
				*(*C.short)(unsafe.Pointer(&buffer[i].mData[0])) = C.short(*v)
			case uint32:
				*(*C.short)(unsafe.Pointer(&buffer[i].mData[0])) = C.short(v)
			case *uint32:
				*(*C.short)(unsafe.Pointer(&buffer[i].mData[0])) = C.short(*v)
			case int32:
				*(*C.short)(unsafe.Pointer(&buffer[i].mData[0])) = C.short(v)
			case *int32:
				*(*C.short)(unsafe.Pointer(&buffer[i].mData[0])) = C.short(*v)
			case *float64:
				*(*C.short)(unsafe.Pointer(&buffer[i].mData[0])) = C.short(*v)
			case float64:
				*(*C.short)(unsafe.Pointer(&buffer[i].mData[0])) = C.short(v)
			case *float32:
				*(*C.short)(unsafe.Pointer(&buffer[i].mData[0])) = C.short(*v)
			case float32:
				*(*C.short)(unsafe.Pointer(&buffer[i].mData[0])) = C.short(v)
			}
		case "integer", "int32", "uint32":
			switch v := val.(type) {
			default:
				return ErrDatabaseAppendWrongType(v, cName, cType)
			case int16:
				*(*C.int)(unsafe.Pointer(&buffer[i].mData[0])) = C.int(v)
			case *int16:
				*(*C.int)(unsafe.Pointer(&buffer[i].mData[0])) = C.int(*v)
			case uint16:
				*(*C.int)(unsafe.Pointer(&buffer[i].mData[0])) = C.int(v)
			case *uint16:
				*(*C.int)(unsafe.Pointer(&buffer[i].mData[0])) = C.int(*v)
			case int32:
				*(*C.int)(unsafe.Pointer(&buffer[i].mData[0])) = C.int(v)
			case *int32:
				*(*C.int)(unsafe.Pointer(&buffer[i].mData[0])) = C.int(*v)
			case uint32:
				*(*C.int)(unsafe.Pointer(&buffer[i].mData[0])) = C.int(v)
			case *uint32:
				*(*C.int)(unsafe.Pointer(&buffer[i].mData[0])) = C.int(*v)
			case int:
				*(*C.int)(unsafe.Pointer(&buffer[i].mData[0])) = C.int(v)
			case *int:
				*(*C.int)(unsafe.Pointer(&buffer[i].mData[0])) = C.int(*v)
			case uint:
				*(*C.int)(unsafe.Pointer(&buffer[i].mData[0])) = C.int(v)
			case *uint:
				*(*C.int)(unsafe.Pointer(&buffer[i].mData[0])) = C.int(*v)
			case *float64:
				*(*C.int)(unsafe.Pointer(&buffer[i].mData[0])) = C.int(*v)
			case float64:
				*(*C.int)(unsafe.Pointer(&buffer[i].mData[0])) = C.int(v)
			case *float32:
				*(*C.int)(unsafe.Pointer(&buffer[i].mData[0])) = C.int(*v)
			case float32:
				*(*C.int)(unsafe.Pointer(&buffer[i].mData[0])) = C.int(v)
			}
		case "long", "int64", "uint64":
			switch v := val.(type) {
			default:
				return ErrDatabaseAppendWrongType(v, cName, cType)
			case int16:
				*(*C.longlong)(unsafe.Pointer(&buffer[i].mData[0])) = C.longlong(v)
			case *int16:
				*(*C.longlong)(unsafe.Pointer(&buffer[i].mData[0])) = C.longlong(*v)
			case uint16:
				*(*C.longlong)(unsafe.Pointer(&buffer[i].mData[0])) = C.longlong(v)
			case *uint16:
				*(*C.longlong)(unsafe.Pointer(&buffer[i].mData[0])) = C.longlong(*v)
			case int32:
				*(*C.longlong)(unsafe.Pointer(&buffer[i].mData[0])) = C.longlong(v)
			case *int32:
				*(*C.longlong)(unsafe.Pointer(&buffer[i].mData[0])) = C.longlong(*v)
			case uint32:
				*(*C.longlong)(unsafe.Pointer(&buffer[i].mData[0])) = C.longlong(v)
			case *uint32:
				*(*C.longlong)(unsafe.Pointer(&buffer[i].mData[0])) = C.longlong(*v)
			case int:
				*(*C.longlong)(unsafe.Pointer(&buffer[i].mData[0])) = C.longlong(v)
			case *int:
				*(*C.longlong)(unsafe.Pointer(&buffer[i].mData[0])) = C.longlong(*v)
			case uint:
				*(*C.longlong)(unsafe.Pointer(&buffer[i].mData[0])) = C.longlong(v)
			case *uint:
				*(*C.longlong)(unsafe.Pointer(&buffer[i].mData[0])) = C.longlong(*v)
			case int64:
				*(*C.longlong)(unsafe.Pointer(&buffer[i].mData[0])) = C.longlong(v)
			case *int64:
				*(*C.longlong)(unsafe.Pointer(&buffer[i].mData[0])) = C.longlong(*v)
			case uint64:
				*(*C.longlong)(unsafe.Pointer(&buffer[i].mData[0])) = C.longlong(v)
			case *uint64:
				*(*C.longlong)(unsafe.Pointer(&buffer[i].mData[0])) = C.longlong(*v)
			case *float64:
				*(*C.longlong)(unsafe.Pointer(&buffer[i].mData[0])) = C.longlong(*v)
			case float64:
				*(*C.longlong)(unsafe.Pointer(&buffer[i].mData[0])) = C.longlong(v)
			case *float32:
				*(*C.longlong)(unsafe.Pointer(&buffer[i].mData[0])) = C.longlong(*v)
			case float32:
				*(*C.longlong)(unsafe.Pointer(&buffer[i].mData[0])) = C.longlong(v)
			}
		case "float", "float32":
			switch v := val.(type) {
			default:
				return ErrDatabaseAppendWrongType(v, cName, cType)
			case int:
				*(*C.float)(unsafe.Pointer(&buffer[i].mData[0])) = C.float(v)
			case *int:
				*(*C.float)(unsafe.Pointer(&buffer[i].mData[0])) = C.float(*v)
			case int16:
				*(*C.float)(unsafe.Pointer(&buffer[i].mData[0])) = C.float(v)
			case *int16:
				*(*C.float)(unsafe.Pointer(&buffer[i].mData[0])) = C.float(*v)
			case int32:
				*(*C.float)(unsafe.Pointer(&buffer[i].mData[0])) = C.float(v)
			case *int32:
				*(*C.float)(unsafe.Pointer(&buffer[i].mData[0])) = C.float(*v)
			case int64:
				*(*C.float)(unsafe.Pointer(&buffer[i].mData[0])) = C.float(v)
			case *int64:
				*(*C.float)(unsafe.Pointer(&buffer[i].mData[0])) = C.float(*v)
			case float32:
				*(*C.float)(unsafe.Pointer(&buffer[i].mData[0])) = C.float(v)
			case *float32:
				*(*C.float)(unsafe.Pointer(&buffer[i].mData[0])) = C.float(*v)
			}
		case "double", "float64":
			switch v := val.(type) {
			default:
				return ErrDatabaseAppendWrongType(v, cName, cType)
			case int:
				*(*C.double)(unsafe.Pointer(&buffer[i].mData[0])) = C.double(v)
			case *int:
				*(*C.double)(unsafe.Pointer(&buffer[i].mData[0])) = C.double(*v)
			case int16:
				*(*C.double)(unsafe.Pointer(&buffer[i].mData[0])) = C.double(v)
			case *int16:
				*(*C.double)(unsafe.Pointer(&buffer[i].mData[0])) = C.double(*v)
			case int32:
				*(*C.double)(unsafe.Pointer(&buffer[i].mData[0])) = C.double(v)
			case *int32:
				*(*C.double)(unsafe.Pointer(&buffer[i].mData[0])) = C.double(*v)
			case int64:
				*(*C.double)(unsafe.Pointer(&buffer[i].mData[0])) = C.double(v)
			case *int64:
				*(*C.double)(unsafe.Pointer(&buffer[i].mData[0])) = C.double(*v)
			case float32:
				*(*C.double)(unsafe.Pointer(&buffer[i].mData[0])) = C.double(v)
			case *float32:
				*(*C.double)(unsafe.Pointer(&buffer[i].mData[0])) = C.double(*v)
			case float64:
				*(*C.double)(unsafe.Pointer(&buffer[i].mData[0])) = C.double(v)
			case *float64:
				*(*C.double)(unsafe.Pointer(&buffer[i].mData[0])) = C.double(*v)
			}
		case "datetime":
			(*C.MachEngineAppendDateTimeStruct)(unsafe.Pointer(&buffer[i].mData[0])).mDateStr = nil
			(*C.MachEngineAppendDateTimeStruct)(unsafe.Pointer(&buffer[i].mData[0])).mFormatStr = nil
			switch v := val.(type) {
			default:
				return ErrDatabaseAppendWrongTimeValueType(fmt.Sprintf("%T", v), cName, cType)
			case time.Time:
				tv := v.UnixNano()
				(*C.MachEngineAppendDateTimeStruct)(unsafe.Pointer(&buffer[i].mData[0])).mTime = C.longlong(tv)
			case *time.Time:
				tv := v.UnixNano()
				(*C.MachEngineAppendDateTimeStruct)(unsafe.Pointer(&buffer[i].mData[0])).mTime = C.longlong(tv)
			case int:
				tv := int64(v)
				(*C.MachEngineAppendDateTimeStruct)(unsafe.Pointer(&buffer[i].mData[0])).mTime = C.longlong(tv)
			case int16:
				tv := int64(v)
				(*C.MachEngineAppendDateTimeStruct)(unsafe.Pointer(&buffer[i].mData[0])).mTime = C.longlong(tv)
			case int32:
				tv := int64(v)
				(*C.MachEngineAppendDateTimeStruct)(unsafe.Pointer(&buffer[i].mData[0])).mTime = C.longlong(tv)
			case int64:
				tv := int64(v)
				(*C.MachEngineAppendDateTimeStruct)(unsafe.Pointer(&buffer[i].mData[0])).mTime = C.longlong(tv)
			case float64:
				tv := int64(v)
				(*C.MachEngineAppendDateTimeStruct)(unsafe.Pointer(&buffer[i].mData[0])).mTime = C.longlong(tv)
			}
		case "ipv4":
			var ipv4 net.IP
			switch ip := val.(type) {
			default:
				return ErrDatabaseAppendWrongType(val, cName, cType)
			case net.IP:
				if ipv4 = ip.To4(); ipv4 == nil {
					return ErrDatabaseAppendWrongType(val, cName, cType)
				}
			case string:
				if ipv4 = net.ParseIP(ip).To4(); ipv4 == nil {
					return ErrDatabaseAppendWrongType(val, cName, cType)
				}
			}
			(*C.MachEngineAppendIPStruct)(unsafe.Pointer(&buffer[i].mData[0])).mLength = C.MACH_ENGINE_APPEND_IP_IPV4
			(*C.MachEngineAppendIPStruct)(unsafe.Pointer(&buffer[i].mData[0])).mAddrString = nil
			for n := 0; n < net.IPv4len; n++ {
				(*C.MachEngineAppendIPStruct)(unsafe.Pointer(&buffer[i].mData[0])).mAddr[n] = C.uchar(ipv4[n])
			}
		case "ipv6":
			var ipv6 net.IP
			switch ip := val.(type) {
			default:
				return ErrDatabaseAppendWrongType(val, cName, cType)
			case net.IP:
				if ipv6 = ip.To16(); ipv6 == nil {
					return ErrDatabaseAppendWrongType(val, cName, cType)
				}
			case string:
				if ipv6 = net.ParseIP(ip).To16(); ipv6 == nil {
					return ErrDatabaseAppendWrongType(val, cName, cType)
				}
			}
			(*C.MachEngineAppendIPStruct)(unsafe.Pointer(&buffer[i].mData[0])).mLength = C.MACH_ENGINE_APPEND_IP_IPV6
			(*C.MachEngineAppendIPStruct)(unsafe.Pointer(&buffer[i].mData[0])).mAddrString = nil
			for n := 0; n < net.IPv6len; n++ {
				(*C.MachEngineAppendIPStruct)(unsafe.Pointer(&buffer[i].mData[0])).mAddr[n] = C.uchar(ipv6[n])
			}
		case "varchar", "string", "json", "text":
			switch v := val.(type) {
			default:
				return ErrDatabaseAppendWrongType(v, cName, cType)
			case string:
				if len(v) == 0 {
					(*C.MachEngineAppendVarStruct)(unsafe.Pointer(&buffer[i].mData[0])).mLength = C.uint(0)
				} else {
					cstr := C.CString(v)
					defer C.free(unsafe.Pointer(cstr))
					cstrlen := C.strlen(cstr)
					(*C.MachEngineAppendVarStruct)(unsafe.Pointer(&buffer[i].mData[0])).mLength = C.uint(cstrlen)
					(*C.MachEngineAppendVarStruct)(unsafe.Pointer(&buffer[i].mData[0])).mData = unsafe.Pointer(cstr)
				}
			case *string:
				if len(*v) == 0 {
					(*C.MachEngineAppendVarStruct)(unsafe.Pointer(&buffer[i].mData[0])).mLength = C.uint(0)
				} else {
					cstr := C.CString(*v)
					defer C.free(unsafe.Pointer(cstr))
					cstrlen := C.strlen(cstr)
					(*C.MachEngineAppendVarStruct)(unsafe.Pointer(&buffer[i].mData[0])).mLength = C.uint(cstrlen)
					(*C.MachEngineAppendVarStruct)(unsafe.Pointer(&buffer[i].mData[0])).mData = unsafe.Pointer(cstr)
				}
			}
		case "binary":
			switch v := val.(type) {
			default:
				return ErrDatabaseAppendWrongType(v, cName, cType)
			case string:
				if len(v) == 0 {
					(*C.MachEngineAppendVarStruct)(unsafe.Pointer(&buffer[i].mData[0])).mLength = C.uint(0)
				} else {
					cstr := C.CString(v)
					defer C.free(unsafe.Pointer(cstr))
					cstrlen := C.strlen(cstr)
					(*C.MachEngineAppendVarStruct)(unsafe.Pointer(&buffer[i].mData[0])).mLength = C.uint(cstrlen)
					(*C.MachEngineAppendVarStruct)(unsafe.Pointer(&buffer[i].mData[0])).mData = unsafe.Pointer(cstr)
				}
			case *string:
				if len(*v) == 0 {
					(*C.MachEngineAppendVarStruct)(unsafe.Pointer(&buffer[i].mData[0])).mLength = C.uint(0)
				} else {
					cstr := C.CString(*v)
					defer C.free(unsafe.Pointer(cstr))
					cstrlen := C.strlen(cstr)
					(*C.MachEngineAppendVarStruct)(unsafe.Pointer(&buffer[i].mData[0])).mLength = C.uint(cstrlen)
					(*C.MachEngineAppendVarStruct)(unsafe.Pointer(&buffer[i].mData[0])).mData = unsafe.Pointer(cstr)
				}
			case []byte:
				(*C.MachEngineAppendVarStruct)(unsafe.Pointer(&buffer[i].mData[0])).mLength = C.uint(len(v))
				if len(v) > 0 {
					(*C.MachEngineAppendVarStruct)(unsafe.Pointer(&buffer[i].mData[0])).mData = unsafe.Pointer(&v[0])
				}
			}
		}
	}

	if rt := C.MachAppendData(ab.stmt, &ab.buffer[0]); rt != 0 {
		stmtErr := EngError(ab.stmt)
		if stmtErr != nil {
			return stmtErr
		} else {
			return ErrDatabaseReturns("MachAppendBuffer", int(rt))
		}
	}
	return nil
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

// * DDL: 1-255
// * ALTER SYSTEM: 256-511
// * SELECT: 512
// * INSERT: 513
// * DELETE: 514-518
// * INSERT_SELECT: 519
// * UPDATE: 520
// * EXEC_ROLLUP: 522-524

type StmtType int

func (typ StmtType) IsSelect() bool {
	return typ == 512
}

func (typ StmtType) IsDDL() bool {
	return typ >= 1 && typ <= 255
}

func (typ StmtType) IsAlterSystem() bool {
	return typ >= 256 && typ <= 511
}

func (typ StmtType) IsInsert() bool {
	return typ == 513
}

func (typ StmtType) IsDelete() bool {
	return typ >= 514 && typ <= 518
}

func (typ StmtType) IsInsertSelect() bool {
	return typ == 519
}

func (typ StmtType) IsUpdate() bool {
	return typ == 520
}

func (typ StmtType) IsExecRollup() bool {
	return typ >= 522 && typ <= 524
}
