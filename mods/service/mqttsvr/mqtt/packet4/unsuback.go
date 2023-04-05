package packet4

import (
	"fmt"
	"io"
)

type UnsubackPacket struct {
	FixedHeader
	MessageID uint16
}

func (ua *UnsubackPacket) String() string {
	return fmt.Sprintf("%s MessageID: %d", ua.FixedHeader, ua.MessageID)
}

func (ua *UnsubackPacket) Write(w io.Writer) (int64, error) {
	ua.FixedHeader.RemainingLength = 2
	packet := ua.FixedHeader.pack()
	packet.Write(encodeUint16(ua.MessageID))
	nbytes, err := packet.WriteTo(w)

	return nbytes, err
}

func (ua *UnsubackPacket) Unpack(b io.Reader) error {
	var err error
	ua.MessageID, err = decodeUint16(b)

	return err
}

func (ua *UnsubackPacket) Details() Details {
	return Details{Qos: 0, MessageID: ua.MessageID}
}
