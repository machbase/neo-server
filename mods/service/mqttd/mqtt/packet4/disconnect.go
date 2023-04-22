package packet4

import "io"

type DisconnectPacket struct {
	FixedHeader
}

func (d *DisconnectPacket) String() string {
	return d.FixedHeader.String()
}

func (d *DisconnectPacket) Write(w io.Writer) (int64, error) {
	packet := d.FixedHeader.pack()
	nbytes, err := packet.WriteTo(w)
	return nbytes, err
}

func (d *DisconnectPacket) Unpack(b io.Reader) error {
	return nil
}

func (d *DisconnectPacket) Details() Details {
	return Details{Qos: 0, MessageID: 0}
}
