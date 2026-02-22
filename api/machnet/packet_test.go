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
		var decoded = new(Packet)
		err := decoded.Read(&reader)
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
	var decoded = new(Packet)
	b.ReportAllocs()
	b.SetBytes(int64(len(pkt)))
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		reader.Reset(pkt)
		if err := decoded.Read(&reader); err != nil {
			b.Fatalf("readPacketInto failed: %v", err)
		}
		if len(decoded.body) != bodySize {
			b.Fatalf("unexpected body length: got %d, want %d", len(decoded.body), bodySize)
		}
	}
}

func BenchmarkRead_0(b *testing.B) {
	benchmarkReadPacket(b, 0)
}

func BenchmarkRead_128(b *testing.B) {
	benchmarkReadPacket(b, 128)
}

func BenchmarkRead_4K(b *testing.B) {
	benchmarkReadPacket(b, 4*1024)
}

func BenchmarkRead_64K(b *testing.B) {
	benchmarkReadPacket(b, 64*1024)
}

func BenchmarkRead_Reuse_0(b *testing.B) {
	benchmarkReadPacketIntoReuse(b, 0)
}

func BenchmarkRead_Reuse_128(b *testing.B) {
	benchmarkReadPacketIntoReuse(b, 128)
}

func BenchmarkRead_Reuse_4K(b *testing.B) {
	benchmarkReadPacketIntoReuse(b, 4*1024)
}

func BenchmarkRead_Reuse_64K(b *testing.B) {
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
		actualProtocol, decoded, err := readNextProtocolFrom(&reader)
		if err != nil {
			b.Fatalf("readProtocolFrom failed: %v", err)
		}
		if actualProtocol != protocol {
			b.Fatalf("unexpected protocol: got %d, want %d", actualProtocol, protocol)
		}
		if !bytes.Equal(decoded, expected) {
			b.Fatalf("decoded payload mismatch: got %d bytes, want %d", len(decoded), len(expected))
		}
	}
}

func benchmarkReadNext(b *testing.B, bodySize int, chunkSize int) {
	const protocol = byte(0x33)
	stream, expected := buildFragmentedProtocolStream(protocol, bodySize, chunkSize)

	var reader bytes.Reader
	b.ReportAllocs()
	b.SetBytes(int64(len(stream)))
	b.ResetTimer()

	var decoded Packet
	for i := 0; i < b.N; i++ {
		reader.Reset(stream)
		if err := decoded.Read(&reader); err != nil {
			b.Fatalf("readNext failed: %v", err)
		}
		if decoded.protocol != protocol {
			b.Fatalf("unexpected protocol: got %d, want %d", decoded.protocol, protocol)
		}
		if !bytes.Equal(decoded.body, expected) {
			b.Fatalf("decoded payload mismatch: got %d bytes, want %d", len(decoded.body), len(expected))
		}
	}
}

func BenchmarkReadProtocolFrom_4K_256(b *testing.B) {
	benchmarkReadProtocolFromFragmented(b, 4*1024, 256)
}

func BenchmarkReadProtocolFrom_64K_1K(b *testing.B) {
	benchmarkReadProtocolFromFragmented(b, 64*1024, 1024)
}

func BenchmarkRead_4K_256(b *testing.B) {
	benchmarkReadNext(b, 4*1024, 256)
}

func BenchmarkRead_64K_1K(b *testing.B) {
	benchmarkReadNext(b, 64*1024, 1024)
}
