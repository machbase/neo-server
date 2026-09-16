//go:build darwin && arm64
// +build darwin,arm64

package mach

// #cgo LDFLAGS: ${SRCDIR}/native/libmachengine_standard_darwin_arm64.a
import "C"

const LibMachLinkInfo = "static_standard_darwin_arm64"
