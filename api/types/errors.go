package types

import (
	"errors"
	"fmt"

	we "github.com/pkg/errors"
)

var ErrDatabaseNotInitialized = errors.New("database not initialized")

var ErrDatabaseConnectionNotFound = func(name string) error {
	return fmt.Errorf("connection '%s' not found", name)
}

var ErrDatabaseConnectionInvalid = func(name string) error {
	return fmt.Errorf("invalid connection '%s'", name)
}

var ErrDatabaseConnectID = func(cause string) error {
	return fmt.Errorf("connection id fail, %s", cause)
}

var ErrDatabaseUnsupportedType = func(fn string, typ int) error {
	return fmt.Errorf("%s unsupported type %d", fn, typ)
}

var ErrDatabaseUnsupportedTypeName = func(fn string, typ string) error {
	return fmt.Errorf("%s unsupported type %s", fn, typ)
}

var ErrDatabaseScan = func(err error) error {
	return we.Wrap(err, "scan")
}

var ErrDatabaseScanTypeName = func(typ string, err error) error {
	return we.Wrapf(err, "scan %s", typ)
}
var ErrDatabaseScanType = func(from string, to any) error {
	return fmt.Errorf("scan convert from %s to %T not supported", from, to)
}

var ErrDatabaseScanIndex = func(idx int, len int) error {
	return fmt.Errorf("scan column %d is out of range %d", idx, len)
}

var ErrDatabaseScanUnsupportedType = func(to any) error {
	return fmt.Errorf("scan unsupported type %T", to)
}

var ErrDatabaseFetch = func(err error) error {
	return we.Wrap(err, "fetch")
}

var ErrDatabaseBindType = func(idx int, val any) error {
	return fmt.Errorf("bind unsupported idx %d type %T", idx, val)
}

var ErrDatabaseNoColumns = func(table string) error {
	return fmt.Errorf("table '%s' has no columns", table)
}

var ErrDatabaseLengthOfColumns = func(table string, expectColumns int, actualColumns int) error {
	return fmt.Errorf("value count %d, table '%s' requires %d columns to append", actualColumns, table, expectColumns)
}

var ErrDatabaseClosedAppender = errors.New("closed appender")

var ErrDatabaseNoConnection = errors.New("invalid connection")

var ErrDatabaseBindNull = func(idx int, err error) error {
	return fmt.Errorf("bind error idx %d with NULL, %q", idx, err.Error())
}

var ErrDatabaseBind = func(idx int, val any, err error) error {
	return we.Wrapf(err, "bind error idx %d with %T", idx, val)
}

var ErrDatabaseBindUnknownType = func(paramNo int, sqlType int) error {
	return fmt.Errorf("bind unknown type at column %d sql_type:%d", paramNo, sqlType)
}

var ErrDatabaseBindWrongType = func(paramNo int, sqlType int, value any) error {
	return fmt.Errorf("bind wrong type at column %d sql_type:%d value:%T", paramNo, sqlType, value)
}

var ErrNotImplemented = func(name string) error { return fmt.Errorf("not implemented %s", name) }

var ErrCannotConvertValue = func(from, to any) error {
	return fmt.Errorf("cannot convert value from %T to %T", from, to)
}

var ErrParamCount = func(expect int, actual int) error {
	return fmt.Errorf("params required %d, but got %d", expect, actual)
}
