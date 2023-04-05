package packet4

import (
	"bytes"
	"fmt"
	"io"
)

type PublishPacket struct {
	FixedHeader
	TopicName string
	MessageID uint16
	Payload   []byte
}

func (p *PublishPacket) String() string {
	return fmt.Sprintf("%s topicName: %s MessageID: %d payload: %s",
		p.FixedHeader, p.TopicName, p.MessageID, string(p.Payload))
}

func (p *PublishPacket) Write(w io.Writer) (int64, error) {
	var body bytes.Buffer

	body.Write(encodeString(p.TopicName))
	if p.Qos > 0 {
		body.Write(encodeUint16(p.MessageID))
	}
	p.FixedHeader.RemainingLength = body.Len() + len(p.Payload)
	packet := p.FixedHeader.pack()
	packet.Write(body.Bytes())
	packet.Write(p.Payload)
	nbytes, err := w.Write(packet.Bytes())

	return int64(nbytes), err
}

func (p *PublishPacket) Unpack(b io.Reader) error {
	var payloadLength = p.FixedHeader.RemainingLength
	var err error
	p.TopicName, err = decodeString(b)
	if err != nil {
		return err
	}

	if p.Qos > 0 {
		p.MessageID, err = decodeUint16(b)
		if err != nil {
			return err
		}
		payloadLength -= len(p.TopicName) + 4
	} else {
		payloadLength -= len(p.TopicName) + 2
	}
	if payloadLength < 0 {
		return fmt.Errorf("error unpacking publish, payload length < 0")
	}
	p.Payload = make([]byte, payloadLength)
	_, err = b.Read(p.Payload)

	return err
}

// Copy publish packet with the same topic and payload
// but an empty fixed header
func (p *PublishPacket) Copy() *PublishPacket {
	newP := NewControlPacket(PUBLISH).(*PublishPacket)
	newP.TopicName = p.TopicName
	newP.Payload = p.Payload
	return newP
}

func (p *PublishPacket) Details() Details {
	return Details{Qos: p.Qos, MessageID: p.MessageID}
}
