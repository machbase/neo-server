package server

import (
	"bytes"
	"encoding/binary"
	"net"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestHeartbeatMarshalUnmarshalRoundTrip(t *testing.T) {
	original := &Heartbeat{Timestamp: 1234, Ack: 5678}
	pkt, err := original.Marshal()
	require.NoError(t, err)

	decoded := &Heartbeat{}
	err = decoded.Unmarshal(bytes.NewReader(pkt))
	require.NoError(t, err)
	require.Equal(t, *original, *decoded)
}

func TestHeartbeatUnmarshalRejectsInvalidHeader(t *testing.T) {
	hb := &Heartbeat{}
	err := hb.Unmarshal(bytes.NewReader([]byte{0x00, NAVEL_HEARTBEAT, 0x00, 0x00, 0x00, 0x00}))
	require.EqualError(t, err, "invalid header stx")
}

func TestHeartbeatUnmarshalRejectsInvalidBodyLength(t *testing.T) {
	hb := &Heartbeat{}
	err := hb.Unmarshal(bytes.NewReader([]byte{NAVEL_STX, NAVEL_HEARTBEAT, 0x00, 0x00, 0x00}))
	require.EqualError(t, err, "invalid body length")
}

func TestHeartbeatUnmarshalRejectsInvalidBody(t *testing.T) {
	buf := &bytes.Buffer{}
	buf.Write([]byte{NAVEL_STX, NAVEL_HEARTBEAT})
	hdr := make([]byte, 4)
	binary.BigEndian.PutUint32(hdr, 5)
	buf.Write(hdr)
	buf.WriteString("{}")

	hb := &Heartbeat{}
	err := hb.Unmarshal(bytes.NewReader(buf.Bytes()))
	require.EqualError(t, err, "invalid body")
}

func TestHeartbeatUnmarshalRejectsInvalidJSON(t *testing.T) {
	buf := &bytes.Buffer{}
	buf.Write([]byte{NAVEL_STX, NAVEL_HEARTBEAT})
	body := []byte("{nope}")
	hdr := make([]byte, 4)
	binary.BigEndian.PutUint32(hdr, uint32(len(body)))
	buf.Write(hdr)
	buf.Write(body)

	hb := &Heartbeat{}
	err := hb.Unmarshal(bytes.NewReader(buf.Bytes()))
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid format")
}

func TestServerStartNavelCordWithoutConfig(t *testing.T) {
	s := &Server{}
	s.StartNavelCord()
	require.Nil(t, s.navel)
}

func TestServerStopNavelCord(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer listener.Close()

	accepted := make(chan net.Conn, 1)
	go func() {
		conn, err := listener.Accept()
		if err == nil {
			accepted <- conn
		}
	}()

	client, err := net.Dial("tcp", listener.Addr().String())
	require.NoError(t, err)
	defer client.Close()

	serverConn := <-accepted
	defer serverConn.Close()

	s := &Server{navel: client.(*net.TCPConn)}
	s.StopNavelCord()
	require.Nil(t, s.navel)

	_, err = client.Write([]byte("ping"))
	require.Error(t, err)
}
