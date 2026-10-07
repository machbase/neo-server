//go:build linux && amd64
// +build linux,amd64

package mach

// #cgo LDFLAGS: ${SRCDIR}/native/libmachengine_standard_linux_amd64.a -lm -ldl -lrt -lpthread
import "C"

const LibMachLinkInfo = "static_standard_linux_amd64"
