package machnet

import (
	"bytes"
	"testing"
)

func buildFragmentedProtocolStream(protocol byte, bodySize int, chunkSize int) ([]byte, []byte) {
	body := bytes.Repeat([]byte{0x7a}, bodySize)
	if bodySize == 0 {
		return buildPacket(protocol, 42, 0, 0, nil), body
	}
	if chunkSize <= 0 {
		chunkSize = bodySize
	}

	stream := make([]byte, 0, bodySize+((bodySize/chunkSize)+1)*packetHeaderSize)
	for offset := 0; offset < bodySize; offset += chunkSize {
		end := offset + chunkSize
		if end > bodySize {
			end = bodySize
		}
		flag := byte(1)
		if end == bodySize {
			flag = 0
		}
		stream = append(stream, buildPacket(protocol, 42, 0, flag, body[offset:end])...)
	}
	return stream, body
}

func benchmarkReadPacket(b *testing.B, bodySize int) {
	body := bytes.Repeat([]byte{0x7a}, bodySize)
	pkt := buildPacket(0x11, 42, 0, 0, body)

	var reader bytes.Reader
	b.ReportAllocs()
	b.SetBytes(int64(len(pkt)))
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		reader.Reset(pkt)
		decoded, err := readPacket(&reader)
		if err != nil {
			b.Fatalf("readPacket failed: %v", err)
		}
		if len(decoded.body) != bodySize {
			b.Fatalf("unexpected body length: got %d, want %d", len(decoded.body), bodySize)
		}
	}
}

func benchmarkReadPacketIntoReuse(b *testing.B, bodySize int) {
	body := bytes.Repeat([]byte{0x7a}, bodySize)
	pkt := buildPacket(0x11, 42, 0, 0, body)

	var reader bytes.Reader
	var decoded Packet
	b.ReportAllocs()
	b.SetBytes(int64(len(pkt)))
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		reader.Reset(pkt)
		if err := readPacketInto(&reader, &decoded); err != nil {
			b.Fatalf("readPacketInto failed: %v", err)
		}
		if len(decoded.body) != bodySize {
			b.Fatalf("unexpected body length: got %d, want %d", len(decoded.body), bodySize)
		}
	}
}

func BenchmarkReadPacketBody0(b *testing.B) {
	benchmarkReadPacket(b, 0)
}

func BenchmarkReadPacketBody128(b *testing.B) {
	benchmarkReadPacket(b, 128)
}

func BenchmarkReadPacketBody4K(b *testing.B) {
	benchmarkReadPacket(b, 4*1024)
}

func BenchmarkReadPacketBody64K(b *testing.B) {
	benchmarkReadPacket(b, 64*1024)
}

func BenchmarkReadPacketIntoReuseBody0(b *testing.B) {
	benchmarkReadPacketIntoReuse(b, 0)
}

func BenchmarkReadPacketIntoReuseBody128(b *testing.B) {
	benchmarkReadPacketIntoReuse(b, 128)
}

func BenchmarkReadPacketIntoReuseBody4K(b *testing.B) {
	benchmarkReadPacketIntoReuse(b, 4*1024)
}

func BenchmarkReadPacketIntoReuseBody64K(b *testing.B) {
	benchmarkReadPacketIntoReuse(b, 64*1024)
}

func benchmarkReadProtocolFromFragmented(b *testing.B, bodySize int, chunkSize int) {
	const protocol = byte(0x33)
	stream, expected := buildFragmentedProtocolStream(protocol, bodySize, chunkSize)

	var reader bytes.Reader
	b.ReportAllocs()
	b.SetBytes(int64(len(stream)))
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		reader.Reset(stream)
		decoded, err := readProtocolFrom(&reader, protocol)
		if err != nil {
			b.Fatalf("readProtocolFrom failed: %v", err)
		}
		if !bytes.Equal(decoded, expected) {
			b.Fatalf("decoded payload mismatch: got %d bytes, want %d", len(decoded), len(expected))
		}
	}
}

func benchmarkReadNextProtocolFromFragmented(b *testing.B, bodySize int, chunkSize int) {
	const protocol = byte(0x33)
	stream, expected := buildFragmentedProtocolStream(protocol, bodySize, chunkSize)

	var reader bytes.Reader
	b.ReportAllocs()
	b.SetBytes(int64(len(stream)))
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		reader.Reset(stream)
		actualProtocol, decoded, err := readNextProtocolFrom(&reader)
		if err != nil {
			b.Fatalf("readNextProtocolFrom failed: %v", err)
		}
		if actualProtocol != protocol {
			b.Fatalf("unexpected protocol: got %d, want %d", actualProtocol, protocol)
		}
		if !bytes.Equal(decoded, expected) {
			b.Fatalf("decoded payload mismatch: got %d bytes, want %d", len(decoded), len(expected))
		}
	}
}

func BenchmarkReadProtocolFromFragmented4K_256(b *testing.B) {
	benchmarkReadProtocolFromFragmented(b, 4*1024, 256)
}

func BenchmarkReadProtocolFromFragmented64K_1K(b *testing.B) {
	benchmarkReadProtocolFromFragmented(b, 64*1024, 1024)
}

func BenchmarkReadNextProtocolFromFragmented4K_256(b *testing.B) {
	benchmarkReadNextProtocolFromFragmented(b, 4*1024, 256)
}

func BenchmarkReadNextProtocolFromFragmented64K_1K(b *testing.B) {
	benchmarkReadNextProtocolFromFragmented(b, 64*1024, 1024)
}
