package api

import (
	"fmt"
)

func IsDatabaseError(err error) bool {
	_, ok := err.(*DatabaseError)
	return ok
}

type DatabaseError struct {
	msg string
}

func (e *DatabaseError) Error() string {
	return e.msg
}

func NewError(msg string) error {
	return &DatabaseError{msg: msg}
}

func NewErrorf(format string, args ...any) error {
	return &DatabaseError{msg: fmt.Sprintf(format, args...)}
}

var ErrDatabaseNotInitialized = NewError("database not initialized")

var ErrDatabaseConnectionNotFound = func(name string) error {
	return NewErrorf("connection '%s' not found", name)
}

var ErrDatabaseConnectionInvalid = func(name string) error {
	return NewErrorf("invalid connection '%s'", name)
}

var ErrDatabaseConnectID = func(cause string) error {
	return NewErrorf("connection id fail, %s", cause)
}

var ErrDatabaseUnsupportedType = func(fn string, typ int) error {
	return NewErrorf("%s unsupported type %d", fn, typ)
}

var ErrDatabaseUnsupportedTypeName = func(fn string, typ string) error {
	return NewErrorf("%s unsupported type %q", fn, typ)
}

var ErrDatabaseScan = func(err error) error {
	return NewErrorf("scan %s", err.Error())
}

var ErrDatabaseScanTypeName = func(typ string, err error) error {
	return NewErrorf("scan %s, %s", typ, err.Error())
}
var ErrDatabaseScanType = func(from string, to any) error {
	return NewErrorf("scan convert from %s to %T not supported", from, to)
}

var ErrDatabaseScanNull = func(typ string) error {
	return NewErrorf("scan NULL %s", typ)
}

var ErrDatabaseScanIndex = func(idx int, len int) error {
	return NewErrorf("scan column %d is out of range %d", idx, len)
}

var ErrDatabaseScanUnsupportedType = func(to any) error {
	return NewErrorf("scan unsupported type %T", to)
}

var ErrDatabaseFetch = func(err error) error {
	return NewErrorf("fetch %s", err.Error())
}

var ErrDatabaseBindType = func(idx int, val any) error {
	return NewErrorf("bind unsupported idx %d type %T", idx, val)
}

var ErrDatabaseNoColumns = func(table string) error {
	return NewErrorf("table '%s' has no columns", table)
}

var ErrDatabaseLengthOfColumns = func(table string, expectColumns int, actualColumns int) error {
	return NewErrorf("value count %d, table '%s' requires %d columns to append", actualColumns, table, expectColumns)
}

var ErrDatabaseClosedAppender = NewError("closed appender")

var ErrDatabaseNoConnection = NewError("invalid connection")

var ErrDatabaseBindNull = func(idx int, err error) error {
	return NewErrorf("bind error idx %d with NULL, %q", idx, err.Error())
}

var ErrDatabaseBind = func(idx int, val any, err error) error {
	return NewErrorf("bind error idx %d with %T, %s", idx, val, err.Error())
}

var ErrDatabaseBindUnknownType = func(paramNo int, sqlType int) error {
	return NewErrorf("bind unknown type at column %d sql_type:%d", paramNo, sqlType)
}

var ErrDatabaseBindWrongType = func(paramNo int, sqlType int, value any) error {
	return NewErrorf("bind wrong type at column %d sql_type:%d value:%T", paramNo, sqlType, value)
}

var ErrNotImplemented = func(name string) error { return NewErrorf("not implemented %s", name) }

var ErrCannotConvertValue = func(from, to any) error {
	return NewErrorf("cannot convert value from %T to %T", from, to)
}

var ErrParamCount = func(expect int, actual int) error {
	return NewErrorf("params required %d, but got %d", expect, actual)
}
