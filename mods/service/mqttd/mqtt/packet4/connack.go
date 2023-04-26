package packet4

import (
	"bytes"
	"fmt"
	"io"
)

type ConnackPacket struct {
	FixedHeader
	SessionPresent bool
	ReturnCode     byte
}

func (ca *ConnackPacket) String() string {
	return fmt.Sprintf("%s sessionpresent: %t returncode: %d", ca.FixedHeader, ca.SessionPresent, ca.ReturnCode)
}

func (ca *ConnackPacket) Write(w io.Writer) (int64, error) {
	var body bytes.Buffer

	body.WriteByte(boolToByte(ca.SessionPresent))
	body.WriteByte(ca.ReturnCode)
	ca.FixedHeader.RemainingLength = 2
	packet := ca.FixedHeader.pack()
	packet.Write(body.Bytes())
	nbytes, err := packet.WriteTo(w)

	return nbytes, err
}

func (ca *ConnackPacket) Unpack(b io.Reader) error {
	flags, err := decodeByte(b)
	if err != nil {
		return err
	}
	ca.SessionPresent = 1&flags > 0
	ca.ReturnCode, err = decodeByte(b)

	return err
}

func (ca *ConnackPacket) Details() Details {
	return Details{Qos: 0, MessageID: 0}
}
