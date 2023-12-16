package facility

import "time"

type VolatileFileWriter interface {
	VolatileFilePrefix() string
	VolatileFileWrite(name string, data []byte, deadline time.Time)
}
