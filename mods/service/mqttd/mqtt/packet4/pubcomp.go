package packet4

import (
	"fmt"
	"io"
)

type PubcompPacket struct {
	FixedHeader
	MessageID uint16
}

func (pc *PubcompPacket) String() string {
	return fmt.Sprintf("%s MessageID: %d", pc.FixedHeader, pc.MessageID)
}

func (pc *PubcompPacket) Write(w io.Writer) (int64, error) {
	pc.FixedHeader.RemainingLength = 2
	packet := pc.FixedHeader.pack()
	packet.Write(encodeUint16(pc.MessageID))
	nbytes, err := packet.WriteTo(w)

	return nbytes, err
}

func (pc *PubcompPacket) Unpack(b io.Reader) error {
	var err error
	pc.MessageID, err = decodeUint16(b)

	return err
}

func (pc *PubcompPacket) Details() Details {
	return Details{Qos: pc.Qos, MessageID: pc.MessageID}
}
