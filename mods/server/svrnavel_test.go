package server

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type mockNetError struct {
	err     string
	timeout bool
}

func (e mockNetError) Error() string {
	return e.err
}

func (e mockNetError) Timeout() bool {
	return e.timeout
}

func (e mockNetError) Temporary() bool {
	return false
}

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

func TestShouldShutdownByPhi(t *testing.T) {
	now := time.Now()

	t.Run("insufficient failures", func(t *testing.T) {
		require.False(t, shouldShutdownByPhi(navelPhiThreshold+1, navelFailureThreshold-1, now.Add(-navelMinDegradedDuration), now))
	})

	t.Run("insufficient degraded duration", func(t *testing.T) {
		require.False(t, shouldShutdownByPhi(navelPhiThreshold+1, navelFailureThreshold, now.Add(-(navelMinDegradedDuration-time.Second)), now))
	})

	t.Run("insufficient phi", func(t *testing.T) {
		require.False(t, shouldShutdownByPhi(navelPhiThreshold-0.1, navelFailureThreshold, now.Add(-navelMinDegradedDuration), now))
	})

	t.Run("meets all thresholds", func(t *testing.T) {
		require.True(t, shouldShutdownByPhi(navelPhiThreshold, navelFailureThreshold, now.Add(-navelMinDegradedDuration), now))
	})
}

func TestIsHardNavelError(t *testing.T) {
	t.Run("nil", func(t *testing.T) {
		require.False(t, isHardNavelError(nil))
	})

	t.Run("eof is hard", func(t *testing.T) {
		require.True(t, isHardNavelError(io.EOF))
	})

	t.Run("timeout net error is transient", func(t *testing.T) {
		require.False(t, isHardNavelError(mockNetError{err: "i/o timeout", timeout: true}))
	})

	t.Run("non-timeout net error is hard", func(t *testing.T) {
		require.True(t, isHardNavelError(mockNetError{err: "connection reset", timeout: false}))
	})

	t.Run("wrapped timeout op error is transient", func(t *testing.T) {
		err := &net.OpError{Op: "read", Net: "tcp", Err: mockNetError{err: "i/o timeout", timeout: true}}
		require.False(t, isHardNavelError(err))
	})
}

func TestIsConnectionClosedError(t *testing.T) {
	require.False(t, isConnectionClosedError(nil))
	require.True(t, isConnectionClosedError(net.ErrClosed))
	require.True(t, isConnectionClosedError(errors.Join(net.ErrClosed, errors.New("extra"))))
	require.False(t, isConnectionClosedError(errors.New("other")))
}

func TestPhiAccrualDetector(t *testing.T) {
	base := time.Unix(1700000000, 0)
	d := newPhiAccrualDetector(base)

	t.Run("insufficient samples returns zero", func(t *testing.T) {
		for i := 1; i < navelMinSamples; i++ {
			d.Observe(base.Add(time.Duration(i) * time.Second))
		}
		require.Equal(t, 0.0, d.Phi(base.Add(30*time.Second)))
	})

	t.Run("large delay raises phi", func(t *testing.T) {
		d2 := newPhiAccrualDetector(base)
		for i := 1; i <= navelMinSamples+4; i++ {
			d2.Observe(base.Add(time.Duration(i) * time.Second))
		}
		phi := d2.Phi(base.Add(25 * time.Second))
		require.Greater(t, phi, navelPhiThreshold)
	})

	t.Run("non-positive elapsed returns zero", func(t *testing.T) {
		require.Equal(t, 0.0, d.Phi(base))
	})
}
