//go:build !darwin && !linux && !freebsd && !openbsd && !netbsd

package pretty

import "os"

func readPauseKey(input *os.File) (byte, error) {
	buf := []byte{0}
	for {
		n, err := input.Read(buf)
		if err != nil {
			return 0, err
		}
		if n > 0 {
			return buf[0], nil
		}
	}
}
