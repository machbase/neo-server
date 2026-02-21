package machnet

import (
	"encoding/binary"
	"fmt"
	"io"
)

const packetHeaderSize = 16

type packet struct {
	protocol byte
	flag     byte
	adds     uint16
	stmtID   uint32
	body     []byte
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

func readPacket(reader io.Reader) (packet, error) {
	var h [packetHeaderSize]byte
	if _, err := io.ReadFull(reader, h[:]); err != nil {
		return packet{}, err
	}
	lenField := binary.BigEndian.Uint32(h[4:8])
	bodyLen := int(lenField & 0x3fffffff)
	body := make([]byte, bodyLen)
	if bodyLen > 0 {
		if _, err := io.ReadFull(reader, body); err != nil {
			return packet{}, err
		}
	}
	return packet{
		protocol: h[3],
		flag:     byte((lenField >> 30) & 0x3),
		adds:     binary.BigEndian.Uint16(h[1:3]),
		stmtID:   binary.BigEndian.Uint32(h[8:12]),
		body:     body,
	}, nil
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
	chunks := make([][]byte, 0, 2)
	total := 0
	for {
		pkt, err := readPacket(reader)
		if err != nil {
			return nil, err
		}
		if pkt.protocol != expected {
			return nil, fmt.Errorf("unexpected protocol %d expected %d", pkt.protocol, expected)
		}
		chunks = append(chunks, pkt.body)
		total += len(pkt.body)
		if pkt.flag == 0 || pkt.flag == 3 {
			if len(chunks) == 1 {
				return chunks[0], nil
			}
			out := make([]byte, total)
			off := 0
			for _, c := range chunks {
				copy(out[off:], c)
				off += len(c)
			}
			return out, nil
		}
	}
}

func readNextProtocolFrom(reader io.Reader) (byte, []byte, error) {
	first, err := readPacket(reader)
	if err != nil {
		return 0, nil, err
	}
	protocol := first.protocol
	chunks := make([][]byte, 0, 2)
	chunks = append(chunks, first.body)
	total := len(first.body)
	flag := first.flag
	for flag != 0 && flag != 3 {
		pkt, err := readPacket(reader)
		if err != nil {
			return 0, nil, err
		}
		if pkt.protocol != protocol {
			return 0, nil, fmt.Errorf("unexpected protocol %d expected %d", pkt.protocol, protocol)
		}
		chunks = append(chunks, pkt.body)
		total += len(pkt.body)
		flag = pkt.flag
	}
	if len(chunks) == 1 {
		return protocol, chunks[0], nil
	}
	out := make([]byte, total)
	off := 0
	for _, c := range chunks {
		copy(out[off:], c)
		off += len(c)
	}
	return protocol, out, nil
}
