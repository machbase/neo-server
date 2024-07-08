package main

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"time"

	"github.com/machbase/neo-server/booter"
	"github.com/pkg/errors"
)

type Navelcord struct {
	port int
	conn *net.TCPConn
}

func (s *Navelcord) StartNavelCord() error {
	if s.port <= 0 {
		return nil
	}
	if conn, err := net.Dial("tcp", fmt.Sprintf("127.0.0.1:%d", s.port)); err != nil {
		slog.Error("NavelCord failed to connect:", "error", err)
		go func() {
			slog.Error("Shutdown by NavelCord failure.")
			time.Sleep(100 * time.Millisecond)
			booter.NotifySignal()
		}()
		return err
	} else {
		s.conn = conn.(*net.TCPConn)
	}

	go func() {
		ticker := time.NewTicker(1 * time.Second)
		for {
			ts := <-ticker.C
			hb := Heartbeat{Timestamp: ts.Unix()}
			pkt, err := hb.Marshal()
			if err != nil {
				slog.Debug("navelcord", "error", err.Error())
				break
			}
			s.conn.SetWriteDeadline(ts.Add(250 * time.Millisecond))
			s.conn.SetDeadline(ts.Add(250 * time.Millisecond))
			if _, err := s.conn.Write(pkt); err != nil {
				slog.Debug("navelcord", "error", err.Error())
				break
			}
			if err := hb.Unmarshal(s.conn); err != nil {
				slog.Debug("navelcord", "error", err.Error())
				break
			}
		}
		s.conn = nil
		ticker.Stop()

		slog.Info("Shutdown by NavelCord")
		booter.NotifySignal()
	}()
	return nil
}

func (s *Navelcord) StopNavelCord() {
	if s.conn == nil {
		return
	}
	s.conn.Close()
	s.conn = nil
}

const NAVEL_ENV = "NEOSHELL_NAVELCORD"
const NAVEL_STX = 0x4E
const NAVEL_HEARTBEAT = 1

type Heartbeat struct {
	Timestamp int64 `json:"ts"`
	Ack       int64 `json:"ack,omitempty"`
}

func (hb *Heartbeat) Marshal() ([]byte, error) {
	body, err := json.Marshal(hb)
	if err != nil {
		return nil, err
	}
	buf := &bytes.Buffer{}
	buf.Write([]byte{NAVEL_STX, NAVEL_HEARTBEAT})
	hdr := make([]byte, 4)
	binary.BigEndian.PutUint32(hdr, uint32(len(body)))
	buf.Write(hdr)
	buf.Write(body)
	return buf.Bytes(), nil
}

func (hb *Heartbeat) Unmarshal(r io.Reader) error {
	hdr := make([]byte, 2)
	bodyLen := make([]byte, 4)

	n, err := r.Read(hdr)
	if err != nil || n != 2 || hdr[0] != NAVEL_STX || hdr[1] != NAVEL_HEARTBEAT {
		return errors.New("invalid header stx")
	}
	n, err = r.Read(bodyLen)
	if err != nil || n != 4 {
		return errors.New("invalid body length")
	}
	l := binary.BigEndian.Uint32(bodyLen)
	body := make([]byte, l)
	n, err = r.Read(body)
	if err != nil || uint32(n) != l {
		return errors.New("invalid body")
	}
	if err := json.Unmarshal(body, hb); err != nil {
		return fmt.Errorf("invalid format %s", err.Error())
	}
	return nil
}
