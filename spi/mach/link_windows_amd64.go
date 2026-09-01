//go:build windows && amd64
// +build windows,amd64

package mach

/*
#cgo LDFLAGS: ${SRCDIR}/native/libmachengine_standard_windows_amd64.a -lm -lws2_32 -lnetapi32 -ladvapi32 -liphlpapi -ldbghelp -lshell32 -luser32 -lbcrypt
*/
import "C"

const LibMachLinkInfo = "static_standard_windows_amd64"
