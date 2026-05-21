//go:build darwin || linux || freebsd || openbsd || netbsd

package pretty

import (
	"errors"
	"os"

	"golang.org/x/sys/unix"
)

func readPauseKey(input *os.File) (byte, error) {
	fd := int(input.Fd())
	buf := []byte{0}
	for {
		n, err := input.Read(buf)
		if err != nil {
			return 0, err
		}
		if n > 0 {
			break
		}
	}

	drain := make([]byte, 32)
	for {
		pollFds := []unix.PollFd{{Fd: int32(fd), Events: unix.POLLIN}}
		ready, err := unix.Poll(pollFds, 10)
		if errors.Is(err, unix.EINTR) {
			continue
		}
		if err != nil || ready <= 0 || pollFds[0].Revents&unix.POLLIN == 0 {
			break
		}
		if n, err := input.Read(drain); err != nil || n == 0 {
			break
		}
	}

	return buf[0], nil
}
