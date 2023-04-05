package packet4

import "io"

type PingrespPacket struct {
	FixedHeader
}

func (pr *PingrespPacket) String() string {
	return pr.FixedHeader.String()
}

func (pr *PingrespPacket) Write(w io.Writer) (int64, error) {
	packet := pr.FixedHeader.pack()
	nbytes, err := packet.WriteTo(w)
	return nbytes, err
}

func (pr *PingrespPacket) Unpack(b io.Reader) error {
	return nil
}

func (pr *PingrespPacket) Details() Details {
	return Details{Qos: 0, MessageID: 0}
}
