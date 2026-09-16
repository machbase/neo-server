//go:build linux && arm
// +build linux,arm

package mach

// #cgo LDFLAGS: ${SRCDIR}/native/libmachengine_standard_linux_arm32.a -lm -ldl
import "C"

const LibMachLinkInfo = "static_standard_linux_arm32"
