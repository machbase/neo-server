package packet4

import "io"

type PingreqPacket struct {
	FixedHeader
}

func (pr *PingreqPacket) String() string {
	return pr.FixedHeader.String()
}

func (pr *PingreqPacket) Write(w io.Writer) (int64, error) {
	packet := pr.FixedHeader.pack()
	nbytes, err := packet.WriteTo(w)
	return nbytes, err
}

func (pr *PingreqPacket) Unpack(b io.Reader) error {
	return nil
}

func (pr *PingreqPacket) Details() Details {
	return Details{Qos: 0, MessageID: 0}
}
