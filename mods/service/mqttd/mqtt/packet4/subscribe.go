package packet4

import (
	"bytes"
	"fmt"
	"io"
)

type SubscribePacket struct {
	FixedHeader
	MessageID uint16
	Topics    []string
	Qoss      []byte
}

func (s *SubscribePacket) String() string {
	return fmt.Sprintf("%s MessageID: %d topics: %s", s.FixedHeader, s.MessageID, s.Topics)
}

func (s *SubscribePacket) Write(w io.Writer) (int64, error) {
	var body bytes.Buffer
	body.Write(encodeUint16(s.MessageID))
	for i, topic := range s.Topics {
		body.Write(encodeString(topic))
		body.WriteByte(s.Qoss[i])
	}
	s.FixedHeader.RemainingLength = body.Len()
	packet := s.FixedHeader.pack()
	packet.Write(body.Bytes())
	nbytes, err := packet.WriteTo(w)
	return nbytes, err
}

func (s *SubscribePacket) Unpack(b io.Reader) error {
	var err error
	s.MessageID, err = decodeUint16(b)
	if err != nil {
		return err
	}
	payloadLength := s.FixedHeader.RemainingLength - 2
	for payloadLength > 0 {
		topic, err := decodeString(b)
		if err != nil {
			return err
		}
		s.Topics = append(s.Topics, topic)
		qos, err := decodeByte(b)
		if err != nil {
			return err
		}
		s.Qoss = append(s.Qoss, qos)
		payloadLength -= 2 + len(topic) + 1 // 2 = string length, 1 = qos
	}
	return nil
}

func (s *SubscribePacket) Details() Details {
	return Details{Qos: 1, MessageID: s.MessageID}
}
