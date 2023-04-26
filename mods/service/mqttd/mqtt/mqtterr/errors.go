package mqtterr

import "fmt"

func MaxMessageSizeExceededError(remaining int, sizeLimit int) ErrMaxMessageSizeExceeded {
	return ErrMaxMessageSizeExceeded{
		remainingLength: remaining,
		maxSizeLimit:    sizeLimit,
	}
}

type ErrMaxMessageSizeExceeded struct {
	remainingLength int
	maxSizeLimit    int
}

func (me ErrMaxMessageSizeExceeded) Error() string {
	return fmt.Sprintf("message %d bytes exceeded limit %d", me.remainingLength, me.maxSizeLimit)
}
