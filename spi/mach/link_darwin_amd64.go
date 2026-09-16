//go:build darwin && amd64
// +build darwin,amd64

package mach

// #cgo LDFLAGS: ${SRCDIR}/native/libmachengine_standard_darwin_amd64.a
import "C"

const LibMachLinkInfo = "static_standard_darwin_amd64"
