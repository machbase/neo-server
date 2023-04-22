package packet4

import (
	"bytes"
	"fmt"
	"io"
)

type SubackPacket struct {
	FixedHeader
	MessageID   uint16
	ReturnCodes []byte
}

func (sa *SubackPacket) String() string {
	return fmt.Sprintf("%s MessageID: %d", sa.FixedHeader, sa.MessageID)
}

func (sa *SubackPacket) Write(w io.Writer) (int64, error) {
	var body bytes.Buffer
	body.Write(encodeUint16(sa.MessageID))
	body.Write(sa.ReturnCodes)
	sa.FixedHeader.RemainingLength = body.Len()
	packet := sa.FixedHeader.pack()
	packet.Write(body.Bytes())
	nbytes, err := packet.WriteTo(w)
	return nbytes, err
}

func (sa *SubackPacket) Unpack(b io.Reader) error {
	var qosBuffer bytes.Buffer
	var err error
	sa.MessageID, err = decodeUint16(b)
	if err != nil {
		return err
	}
	_, err = qosBuffer.ReadFrom(b)
	if err != nil {
		return err
	}
	sa.ReturnCodes = qosBuffer.Bytes()
	return nil
}

func (sa *SubackPacket) Details() Details {
	return Details{Qos: 0, MessageID: sa.MessageID}
}
