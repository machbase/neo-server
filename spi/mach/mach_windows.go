//go:build windows
// +build windows

package mach

import (
	"golang.org/x/sys/windows"
	"golang.org/x/text/encoding/japanese"
	"golang.org/x/text/encoding/korean"
)

func translateCodePage(str string) string {
	switch windows.GetACP() {
	case 949:
		ub, _ := korean.EUCKR.NewEncoder().Bytes([]byte(str))
		return string(ub)
	case 932:
		ub, _ := japanese.ShiftJIS.NewEncoder().Bytes([]byte(str))
		return string(ub)
	default:
		return str
	}
}
