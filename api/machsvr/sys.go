//go:build !windows
// +build !windows

package machsvr

func translateCodePage(str string) string {
	return str
}
