package mqtt

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"io"

	"github.com/machbase/neo-server/mods/service/mqttd/mqtt/packet4"
	"github.com/machbase/neo-server/mods/service/mqttd/mqtt/packet5"
)

func accept(r io.Reader) (byte, any, int64, error) {
	var nlen int64
	var err error

	t := [1]byte{}
	n, err := io.ReadFull(r, t[:])
	if err != nil {
		return 0, nil, nlen, err
	}
	nlen += int64(n)

	pt := t[0] >> 4
	if pt != packet5.CONNECT {
		return 0, nil, nlen, fmt.Errorf("invalid protocol control: %d", t[0])
	}

	//flags := t[0] & 0xF // reserved flags
	vbi, err := getVBI(r)
	if err != nil {
		return 0, nil, nlen, err
	}
	nlen += int64(vbi.Len())

	remainingLength, err := decodeVBI(vbi)
	if err != nil {
		return 0, nil, nlen, err
	}

	var content bytes.Buffer
	content.Grow(remainingLength)

	n64, err := io.CopyN(&content, r, int64(remainingLength))
	if err != nil {
		return 0, nil, nlen, err
	}
	nlen += n64

	if n64 != int64(remainingLength) {
		return 0, nil, nlen, fmt.Errorf("failed to read packet, expected %d bytes, read %d", remainingLength, n)
	}

	b := content.Bytes()
	if b[2] == 'M' && b[3] == 'Q' && b[4] == 'T' && b[5] == 'T' && b[6] == 5 {
		cp := &packet5.ControlPacket{FixedHeader: packet5.FixedHeader{Type: pt, Flags: t[0] & 0xF}}
		cp.Content = &packet5.Connect{
			ProtocolName:    "MQTT",
			ProtocolVersion: 5,
			Properties:      &packet5.Properties{},
		}
		cp.Content.Unpack(&content)
		return 5, cp, nlen, nil
	} else if b[2] == 'M' && b[3] == 'Q' && b[4] == 'T' && b[5] == 'T' && b[6] == 4 {
		cp := &packet4.ConnectPacket{
			FixedHeader:     packet4.FixedHeader{},
			ProtocolName:    "MQTT",
			ProtocolVersion: 4,
		}
		cp.MessageType = t[0] >> 4
		cp.Dup = (t[0]>>3)&0x01 > 0
		cp.Qos = (t[0] >> 1) & 0x03
		cp.Retain = t[0]&0x01 > 0
		cp.RemainingLength = remainingLength
		cp.Unpack(&content)
		return b[6], cp, nlen, nil
	} else if b[2] == 'M' && b[3] == 'Q' && b[4] == 'I' && b[5] == 's' && b[6] == 'd' && b[7] == 'p' && b[8] == 0x03 {
		cp := &packet4.ConnectPacket{
			FixedHeader:     packet4.FixedHeader{},
			ProtocolName:    "MQIsdp",
			ProtocolVersion: 3,
		}
		cp.MessageType = t[0] >> 4
		cp.Dup = (t[0]>>3)&0x01 > 0
		cp.Qos = (t[0] >> 1) & 0x03
		cp.Retain = t[0]&0x01 > 0
		cp.RemainingLength = remainingLength
		cp.Unpack(&content)
		return 4, cp, nlen, nil
	} else {
		return 0, nil, nlen, fmt.Errorf("invalid CONNECT: %s", hex.Dump(b))
	}
}

func getVBI(r io.Reader) (*bytes.Buffer, error) {
	var ret bytes.Buffer
	digit := [1]byte{}
	for {
		_, err := io.ReadFull(r, digit[:])
		if err != nil {
			return nil, err
		}
		ret.WriteByte(digit[0])
		if digit[0] <= 0x7f {
			return &ret, nil
		}
	}
}

func decodeVBI(r *bytes.Buffer) (int, error) {
	var vbi uint32
	var multiplier uint32
	for {
		digit, err := r.ReadByte()
		if err != nil && err != io.EOF {
			return 0, err
		}
		vbi |= uint32(digit&127) << multiplier
		if (digit & 128) == 0 {
			break
		}
		multiplier += 7
	}
	return int(vbi), nil
}
