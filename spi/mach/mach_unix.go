//go:build !windows
// +build !windows

package mach

func translateCodePage(str string) string {
	return str
}
