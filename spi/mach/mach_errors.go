package mach

import "fmt"

var ErrDatabaseMach = func(code int, msg string) error {
	return fmt.Errorf("MACH-ERR %d %s", code, msg)
}
var ErrDatabaseReturns = func(fn string, rt int) error {
	return fmt.Errorf("%s returns %d", fn, rt)
}
var ErrDatabaseReturnsAtIdx = func(fn string, idx int, rt int) error {
	return fmt.Errorf("%s idx %d returns %d", fn, idx, rt)
}
var ErrDatabaseWrap = func(fn string, cause error) error {
	return fmt.Errorf("%s %s", fn, cause.Error())
}
var ErrDatabaseAppendUnknownType = func(typ string) error {
	return fmt.Errorf("MachAppendData unknown column type '%s'", typ)
}
var ErrDatabaseAppendWrongType = func(actual any, column string, typ string) error {
	return fmt.Errorf("MachAppendData cannot apply %T to %s (%s)", actual, column, typ)
}
var ErrDatabaseAppendWrongTimeValueType = func(actual string, column string, typ string) error {
	return fmt.Errorf("MachAppendData cannot apply %s to %s (%s)", actual, column, typ)
}
var ErrDatabaseAppendWrongValueCount = func(expect int, actual int) error {
	return fmt.Errorf("MachAppendData required %d, but got %d", expect, actual)
}
