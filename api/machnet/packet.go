package machnet

import (
	"encoding/binary"
	"fmt"
	"io"
)

const packetHeaderSize = 16

type Packet struct {
	protocol byte
	flag     byte
	adds     uint16
	stmtID   uint32
	body     []byte
}

func readPacketHeader(reader io.Reader, h *[packetHeaderSize]byte) (byte, byte, uint16, uint32, int, error) {
	if _, err := io.ReadFull(reader, h[:]); err != nil {
		return 0, 0, 0, 0, 0, err
	}
	lenField := binary.BigEndian.Uint32(h[4:8])
	return h[3], byte((lenField >> 30) & 0x3), binary.BigEndian.Uint16(h[1:3]), binary.BigEndian.Uint32(h[8:12]), int(lenField & 0x3fffffff), nil
}

func ensureAppendCapacity(dst []byte, appendLen int) []byte {
	if appendLen == 0 {
		return dst
	}
	oldLen := len(dst)
	newLen := oldLen + appendLen
	if newLen <= cap(dst) {
		return dst[:newLen]
	}
	newCap := cap(dst) * 2
	if newCap < newLen {
		newCap = newLen
	}
	if newCap == 0 {
		newCap = appendLen
	}
	grown := make([]byte, newLen, newCap)
	copy(grown, dst)
	return grown
}

func buildPacket(protocolID byte, stmtID uint32, adds uint16, flag byte, body []byte) []byte {
	ret := make([]byte, packetHeaderSize+len(body))
	ret[0] = 0
	binary.BigEndian.PutUint16(ret[1:3], adds)
	ret[3] = protocolID
	lenWithFlag := (uint32(flag&0x3) << 30) | (uint32(len(body)) & 0x3fffffff)
	binary.BigEndian.PutUint32(ret[4:8], lenWithFlag)
	binary.BigEndian.PutUint32(ret[8:12], stmtID)
	copy(ret[packetHeaderSize:], body)
	return ret
}

func readPacket(reader io.Reader) (Packet, error) {
	var dst Packet
	if err := readPacketInto(reader, &dst); err != nil {
		return Packet{}, err
	}
	return dst, nil
}

func readPacketInto(reader io.Reader, dst *Packet) error {
	var h [packetHeaderSize]byte
	protocol, flag, adds, stmtID, bodyLen, err := readPacketHeader(reader, &h)
	if err != nil {
		return err
	}
	dst.protocol = protocol
	dst.flag = flag
	dst.adds = adds
	dst.stmtID = stmtID
	if bodyLen == 0 {
		dst.body = dst.body[:0]
		return nil
	}
	if cap(dst.body) < bodyLen {
		dst.body = make([]byte, bodyLen)
	} else {
		dst.body = dst.body[:bodyLen]
	}
	if _, err := io.ReadFull(reader, dst.body); err != nil {
		return err
	}
	return nil
}

func writePacket(writer io.Writer, buf []byte) error {
	for len(buf) > 0 {
		n, err := writer.Write(buf)
		if err != nil {
			return err
		}
		buf = buf[n:]
	}
	return nil
}

func readProtocolFrom(reader io.Reader, expected byte) ([]byte, error) {
	var h [packetHeaderSize]byte
	protocol, flag, _, _, bodyLen, err := readPacketHeader(reader, &h)
	if err != nil {
		return nil, err
	}
	if protocol != expected {
		return nil, fmt.Errorf("unexpected protocol %d expected %d", protocol, expected)
	}
	var out []byte
	out = ensureAppendCapacity(out, bodyLen)
	if bodyLen > 0 {
		if _, err := io.ReadFull(reader, out[:bodyLen]); err != nil {
			return nil, err
		}
	}
	if flag == 0 || flag == 3 {
		return out, nil
	}
	for {
		protocol, flag, _, _, bodyLen, err = readPacketHeader(reader, &h)
		if err != nil {
			return nil, err
		}
		if protocol != expected {
			return nil, fmt.Errorf("unexpected protocol %d expected %d", protocol, expected)
		}
		oldLen := len(out)
		out = ensureAppendCapacity(out, bodyLen)
		if bodyLen > 0 {
			if _, err := io.ReadFull(reader, out[oldLen:oldLen+bodyLen]); err != nil {
				return nil, err
			}
		}
		if flag == 0 || flag == 3 {
			return out, nil
		}
	}
}

func readNextProtocolFrom(reader io.Reader) (byte, []byte, error) {
	var h [packetHeaderSize]byte
	protocol, flag, _, _, bodyLen, err := readPacketHeader(reader, &h)
	if err != nil {
		return 0, nil, err
	}
	var out []byte
	out = ensureAppendCapacity(out, bodyLen)
	if bodyLen > 0 {
		if _, err := io.ReadFull(reader, out[:bodyLen]); err != nil {
			return 0, nil, err
		}
	}
	if flag == 0 || flag == 3 {
		return protocol, out, nil
	}
	for {
		nextProtocol, nextFlag, _, _, nextBodyLen, err := readPacketHeader(reader, &h)
		if err != nil {
			return 0, nil, err
		}
		if nextProtocol != protocol {
			return 0, nil, fmt.Errorf("unexpected protocol %d expected %d", nextProtocol, protocol)
		}
		oldLen := len(out)
		out = ensureAppendCapacity(out, nextBodyLen)
		if nextBodyLen > 0 {
			if _, err := io.ReadFull(reader, out[oldLen:oldLen+nextBodyLen]); err != nil {
				return 0, nil, err
			}
		}
		flag = nextFlag
		if flag == 0 || flag == 3 {
			return protocol, out, nil
		}
	}
}
