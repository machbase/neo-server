package packet4

import (
	"fmt"
	"io"
)

type PubackPacket struct {
	FixedHeader
	MessageID uint16
}

func (pa *PubackPacket) String() string {
	return fmt.Sprintf("%s MessageID: %d", pa.FixedHeader, pa.MessageID)
}

func (pa *PubackPacket) Write(w io.Writer) (int64, error) {
	pa.FixedHeader.RemainingLength = 2
	packet := pa.FixedHeader.pack()
	packet.Write(encodeUint16(pa.MessageID))
	nbytes, err := packet.WriteTo(w)

	return nbytes, err
}

func (pa *PubackPacket) Unpack(b io.Reader) error {
	var err error
	pa.MessageID, err = decodeUint16(b)

	return err
}

func (pa *PubackPacket) Details() Details {
	return Details{Qos: pa.Qos, MessageID: pa.MessageID}
}
