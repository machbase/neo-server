//go:build windows
// +build windows

package booter

func Daemonize(bootlog string, pidfile string, proc func()) {
	proc()
}
