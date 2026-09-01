//go:build linux && arm64
// +build linux,arm64

package mach

// #cgo LDFLAGS: ${SRCDIR}/native/libmachengine_standard_linux_arm64.a -lm -ldl
import "C"

const LibMachLinkInfo = "static_standard_linux_arm64"
