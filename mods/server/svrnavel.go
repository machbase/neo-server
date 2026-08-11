package server

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
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
		s.setNavelConn(conn.(*net.TCPConn))
	}

	go func() {
		ticker := time.NewTicker(navelHeartbeatInterval)
		detector := newPhiAccrualDetector(time.Now())
		consecutiveFailures := 0
		degradedSince := time.Time{}
		shouldNotify := false
		for {
			ts := <-ticker.C
			conn := s.getNavelConn()
			if conn == nil {
				break
			}

			hb := Heartbeat{Timestamp: ts.Unix()}
			pkt, err := hb.Marshal()
			if err != nil {
				s.log.Trace("navelcord", err.Error())
				break
			}

			_ = conn.SetWriteDeadline(time.Now().Add(navelWriteTimeout))
			if _, err := conn.Write(pkt); err != nil {
				if isConnectionClosedError(err) {
					break
				}
				if isHardNavelError(err) {
					s.log.Trace("navelcord", "hard write failure:", err.Error())
					shouldNotify = true
					break
				}
				consecutiveFailures++
				if degradedSince.IsZero() {
					degradedSince = time.Now()
				}
				s.log.Trace("navelcord", "transient write timeout:", err.Error())
				if shouldShutdownByPhi(detector.Phi(time.Now()), consecutiveFailures, degradedSince, time.Now()) {
					shouldNotify = true
					break
				}
				continue
			}

			_ = conn.SetReadDeadline(time.Now().Add(navelReadTimeout))
			if err := hb.Unmarshal(conn); err != nil {
				if isConnectionClosedError(err) {
					break
				}
				if isHardNavelError(err) {
					s.log.Trace("navelcord", "hard read failure:", err.Error())
					shouldNotify = true
					break
				}
				consecutiveFailures++
				if degradedSince.IsZero() {
					degradedSince = time.Now()
				}
				s.log.Trace("navelcord", "transient read timeout:", err.Error())
				if shouldShutdownByPhi(detector.Phi(time.Now()), consecutiveFailures, degradedSince, time.Now()) {
					shouldNotify = true
					break
				}
				continue
			}

			now := time.Now()
			detector.Observe(now)
			consecutiveFailures = 0
			degradedSince = time.Time{}
		}
		s.setNavelConn(nil)
		ticker.Stop()

		if shouldNotify {
			s.log.Info("Shutdown by NavelCord")
			booter.NotifySignal()
		}
	}()
	util.AddShutdownHook(func() { s.StopNavelCord() })
}

func (s *Server) StopNavelCord() {
	conn := s.getNavelConn()
	if conn == nil {
		return
	}
	conn.Close()
	s.setNavelConn(nil)
}

func (s *Server) getNavelConn() *net.TCPConn {
	s.navelLock.RLock()
	defer s.navelLock.RUnlock()
	return s.navel
}

func (s *Server) setNavelConn(conn *net.TCPConn) {
	s.navelLock.Lock()
	defer s.navelLock.Unlock()
	s.navel = conn
}

// Navel detection tuning presets (recommended).
//
// Ultra-fast detection (highest sensitivity, more false-positive risk on busy hosts):
//
//	heartbeatInterval=500ms, writeTimeout=300ms, readTimeout=1s,
//	phiThreshold=7.0, failureThreshold=2, minDegradedDuration=3s
//
// Fast detection:
//
//	heartbeatInterval=1s, writeTimeout=400ms, readTimeout=1500ms,
//	phiThreshold=7.5, failureThreshold=2, minDegradedDuration=6s
//
// Normal detection (current defaults, balanced):
//
//	heartbeatInterval=1s, writeTimeout=500ms, readTimeout=2s,
//	phiThreshold=8.0, failureThreshold=3, minDegradedDuration=10s
//
// Slow detection (lowest false-positive risk, slower peer-down confirmation):
//
//	heartbeatInterval=2s, writeTimeout=800ms, readTimeout=3s,
//	phiThreshold=9.0, failureThreshold=4, minDegradedDuration=20s
//
// Notes:
// - Keep readTimeout >= heartbeatInterval.
// - Increase phiThreshold/failureThreshold first if overloaded hosts trigger false positives.
// - Decrease minDegradedDuration first if you need faster confirmation without aggressive timeouts.
const navelHeartbeatInterval = 1 * time.Second
const navelWriteTimeout = 500 * time.Millisecond
const navelReadTimeout = 2 * time.Second
const navelPhiThreshold = 8.0
const navelMinSamples = 8
const navelFailureThreshold = 3
const navelMinDegradedDuration = 10 * time.Second

func shouldShutdownByPhi(phi float64, failures int, degradedSince, now time.Time) bool {
	if failures < navelFailureThreshold {
		return false
	}
	if degradedSince.IsZero() || now.Sub(degradedSince) < navelMinDegradedDuration {
		return false
	}
	return phi >= navelPhiThreshold
}

func isConnectionClosedError(err error) bool {
	if err == nil {
		return false
	}
	return errors.Is(err, net.ErrClosed)
}

func isHardNavelError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, io.EOF) {
		return true
	}
	if ne := (&net.OpError{}); errors.As(err, &ne) {
		if ne.Timeout() {
			return false
		}
	}
	var nerr net.Error
	if errors.As(err, &nerr) {
		if nerr.Timeout() {
			return false
		}
	}
	return true
}

const NAVEL_ENV = "NEOSHELL_NAVELCORD"
const NAVEL_STX = 0x4E
const NAVEL_HEARTBEAT = 1

type Heartbeat struct {
	Timestamp int64 `json:"ts"`
	Ack       int64 `json:"ack,omitempty"`
}

type phiAccrualDetector struct {
	last time.Time
	ints []float64
	idx  int
	cnt  int
}

func newPhiAccrualDetector(now time.Time) *phiAccrualDetector {
	return &phiAccrualDetector{last: now, ints: make([]float64, 32)}
}

func (d *phiAccrualDetector) Observe(now time.Time) {
	if d.last.IsZero() {
		d.last = now
		return
	}
	interval := now.Sub(d.last).Seconds()
	if interval <= 0 {
		return
	}
	d.ints[d.idx] = interval
	d.idx = (d.idx + 1) % len(d.ints)
	if d.cnt < len(d.ints) {
		d.cnt++
	}
	d.last = now
}

func (d *phiAccrualDetector) Phi(now time.Time) float64 {
	if d.last.IsZero() {
		return 0
	}
	if d.cnt < navelMinSamples {
		return 0
	}
	elapsed := now.Sub(d.last).Seconds()
	if elapsed <= 0 {
		return 0
	}

	mean, stddev := d.meanStddev()
	if stddev < 0.05 {
		stddev = 0.05
	}
	z := (elapsed - mean) / stddev
	cdf := 0.5 * (1 + math.Erf(z/math.Sqrt2))
	if cdf >= 1.0 {
		return 12
	}
	prob := 1 - cdf
	if prob <= 1e-12 {
		return 12
	}
	return -math.Log10(prob)
}

func (d *phiAccrualDetector) meanStddev() (float64, float64) {
	if d.cnt == 0 {
		return 0, 0
	}
	var sum float64
	for i := 0; i < d.cnt; i++ {
		sum += d.ints[i]
	}
	mean := sum / float64(d.cnt)
	var variance float64
	for i := 0; i < d.cnt; i++ {
		delta := d.ints[i] - mean
		variance += delta * delta
	}
	variance /= float64(d.cnt)
	return mean, math.Sqrt(variance)
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

	if _, err := io.ReadFull(r, hdr); err != nil {
		return errors.New("invalid header stx")
	}
	if hdr[0] != NAVEL_STX || hdr[1] != NAVEL_HEARTBEAT {
		return errors.New("invalid header stx")
	}
	if _, err := io.ReadFull(r, bodyLen); err != nil {
		return errors.New("invalid body length")
	}
	l := binary.BigEndian.Uint32(bodyLen)
	body := make([]byte, l)
	if _, err := io.ReadFull(r, body); err != nil {
		return errors.New("invalid body")
	}
	if err := json.Unmarshal(body, hb); err != nil {
		return fmt.Errorf("invalid format %s", err.Error())
	}
	return nil
}
