package server

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"time"

	"github.com/machbase/neo-server/v8/booter"
	"github.com/machbase/neo-server/v8/mods/util"
)

func (s *Server) StartNavelCord() {
	if s.NavelCord == nil {
		return
	}
	if conn, err := net.Dial("tcp", fmt.Sprintf("127.0.0.1:%d", s.NavelCord.Port)); err != nil {
		s.log.Error("NavelCord failed to connect:", err)
		go func() {
			s.log.Error("Shutdown by NavelCord failure.")
			time.Sleep(100 * time.Millisecond)
			booter.NotifySignal()
		}()
		return
	} else {
		s.navel = conn.(*net.TCPConn)
	}

	go func() {
		ticker := time.NewTicker(1 * time.Second)
		for {
			ts := <-ticker.C
			hb := Heartbeat{Timestamp: ts.Unix()}
			pkt, err := hb.Marshal()
			if err != nil {
				s.log.Trace("navelcord", err.Error())
				break
			}
			s.navel.SetWriteDeadline(ts.Add(250 * time.Millisecond))
			s.navel.SetDeadline(ts.Add(250 * time.Millisecond))
			if _, err := s.navel.Write(pkt); err != nil {
				s.log.Trace("navelcord", err.Error())
				break
			}
			if err := hb.Unmarshal(s.navel); err != nil {
				s.log.Trace("navelcord", err.Error())
				break
			}
		}
		s.navel = nil
		ticker.Stop()

		s.log.Info("Shutdown by NavelCord")
		booter.NotifySignal()
	}()
	util.AddShutdownHook(func() { s.StopNavelCord() })
}

func (s *Server) StopNavelCord() {
	if s.navel == nil {
		return
	}
	s.navel.Close()
	s.navel = nil
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
