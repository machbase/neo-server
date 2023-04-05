package packet4

import (
	"fmt"
	"io"
)

type PubrelPacket struct {
	FixedHeader
	MessageID uint16
}

func (pr *PubrelPacket) String() string {
	return fmt.Sprintf("%s MessageID: %d", pr.FixedHeader, pr.MessageID)
}

func (pr *PubrelPacket) Write(w io.Writer) (int64, error) {
	pr.FixedHeader.RemainingLength = 2
	packet := pr.FixedHeader.pack()
	packet.Write(encodeUint16(pr.MessageID))
	nbytes, err := packet.WriteTo(w)
	return nbytes, err
}

func (pr *PubrelPacket) Unpack(b io.Reader) error {
	var err error
	pr.MessageID, err = decodeUint16(b)
	return err
}

func (pr *PubrelPacket) Details() Details {
	return Details{Qos: pr.Qos, MessageID: pr.MessageID}
}
