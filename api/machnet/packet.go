package machnet

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"time"
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

func readPacketNoDeadline(reader io.Reader) (packet, error) {
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

func readPacket(conn net.Conn, timeout time.Duration) (packet, error) {
	if timeout > 0 {
		_ = conn.SetReadDeadline(time.Now().Add(timeout))
		defer conn.SetReadDeadline(time.Time{})
	}
	return readPacketNoDeadline(conn)
}

func writeAllNoDeadline(writer io.Writer, buf []byte) error {
	for len(buf) > 0 {
		n, err := writer.Write(buf)
		if err != nil {
			return err
		}
		buf = buf[n:]
	}
	return nil
}

func writeAll(conn net.Conn, buf []byte, timeout time.Duration) error {
	if timeout > 0 {
		_ = conn.SetWriteDeadline(time.Now().Add(timeout))
		defer conn.SetWriteDeadline(time.Time{})
	}
	return writeAllNoDeadline(conn, buf)
}

func readProtocolFrom(reader io.Reader, conn net.Conn, expected byte, timeout time.Duration) ([]byte, error) {
	if timeout > 0 {
		_ = conn.SetReadDeadline(time.Now().Add(timeout))
		defer conn.SetReadDeadline(time.Time{})
	}
	chunks := make([][]byte, 0, 2)
	total := 0
	for {
		pkt, err := readPacketNoDeadline(reader)
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

func readProtocol(conn net.Conn, expected byte, timeout time.Duration) ([]byte, error) {
	return readProtocolFrom(conn, conn, expected, timeout)
}

func readNextProtocolFrom(reader io.Reader, conn net.Conn, timeout time.Duration) (byte, []byte, error) {
	if timeout > 0 {
		_ = conn.SetReadDeadline(time.Now().Add(timeout))
		defer conn.SetReadDeadline(time.Time{})
	}
	first, err := readPacketNoDeadline(reader)
	if err != nil {
		return 0, nil, err
	}
	protocol := first.protocol
	chunks := make([][]byte, 0, 2)
	chunks = append(chunks, first.body)
	total := len(first.body)
	flag := first.flag
	for flag != 0 && flag != 3 {
		pkt, err := readPacketNoDeadline(reader)
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
