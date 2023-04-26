package mqtt

import (
	met "github.com/rcrowley/go-metrics"
)

/*
var bootTime time.Time

func init() {
	bootTime = time.Now()
}

"github.com/machbase/cemlib/metrics"

func GetServerStat(measurePrefix string, tags map[string]string, svr Server) metrics.SourceFunc {
	getStat := func() *metrics.Measurement {
		fields := make(map[string]any)
		fields["uptime"] = int64(time.Since(bootTime).Seconds())
		return &metrics.Measurement{Measurement: fmt.Sprintf("%s.stat", measurePrefix), Tags: tags, Fields: fields}
	}

	getPeers := func() *metrics.Measurement {
		m := svr.Metrics()
		fields := make(map[string]any)
		fields["channels"] = m.ChannelCounter.Snapshot().Count()
		fields["count"] = svr.CountPeers()
		return &metrics.Measurement{Measurement: fmt.Sprintf("%s.peers", measurePrefix), Tags: tags, Fields: fields}
	}

	getAuth := func() *metrics.Measurement {
		m := svr.Metrics()
		fields := make(map[string]any)
		fields["success"] = m.AuthSuccessCounter.Snapshot().Count()
		fields["denied"] = m.AuthDeniedCounter.Snapshot().Count()
		fields["error"] = m.AuthErrorCounter.Snapshot().Count()
		fields["fail"] = m.AuthFailCounter.Snapshot().Count()
		return &metrics.Measurement{Measurement: fmt.Sprintf("%s.auth.result", measurePrefix), Tags: tags, Fields: fields}
	}

	getAllow := func() *metrics.Measurement {
		m := svr.Metrics()
		fields := make(map[string]any)
		fields["allowed"] = m.ConnAllowed.Snapshot().Count()
		fields["denied"] = m.ConnDenied.Snapshot().Count()
		return &metrics.Measurement{Measurement: fmt.Sprintf("%s.allowance", measurePrefix), Tags: tags, Fields: fields}
	}

	getAuthTimer := func() *metrics.Measurement {
		m := svr.Metrics()
		fields := make(map[string]any)
		snapshot := m.AuthTimer.Snapshot()
		fields["count"] = snapshot.Count()
		fields["max"] = snapshot.Max()
		fields["min"] = snapshot.Min()
		fields["mean"] = snapshot.Mean()
		return &metrics.Measurement{Measurement: fmt.Sprintf("%s.auth.time", measurePrefix), Tags: tags, Fields: fields}
	}

	getRecvPubTimer := func() *metrics.Measurement {
		m := svr.Metrics()
		fields := make(map[string]any)
		snapshot := m.RecvPubTimer.Snapshot()
		fields["count"] = snapshot.Count()
		fields["max"] = snapshot.Max()
		fields["min"] = snapshot.Min()
		fields["mean"] = snapshot.Mean()
		return &metrics.Measurement{Measurement: fmt.Sprintf("%s.recv.pub", measurePrefix), Tags: tags, Fields: fields}
	}

	getSendPubTimer := func() *metrics.Measurement {
		m := svr.Metrics()
		fields := make(map[string]any)
		snapshot := m.SendPubTimer.Snapshot()
		fields["count"] = snapshot.Count()
		fields["max"] = snapshot.Max()
		fields["min"] = snapshot.Min()
		fields["mean"] = snapshot.Mean()
		return &metrics.Measurement{Measurement: fmt.Sprintf("%s.send.pub", measurePrefix), Tags: tags, Fields: fields}
	}

	return func() []*metrics.Measurement {
		return []*metrics.Measurement{
			getStat(),
			getPeers(),
			getAuth(),
			getAllow(),
			getAuthTimer(),
			getRecvPubTimer(),
			getSendPubTimer(),
		}
	}
}
*/

type ServerMetrics struct {
	ChannelCounter met.Counter

	AuthTimer    met.Timer
	RecvPubTimer met.Timer
	SendPubTimer met.Timer

	AuthSuccessCounter met.Counter
	AuthDeniedCounter  met.Counter
	AuthErrorCounter   met.Counter
	AuthFailCounter    met.Counter

	ConnAllowed met.Counter
	ConnDenied  met.Counter
}

func NewServerMetrics(svr Server) *ServerMetrics {
	s := &ServerMetrics{}

	//// metric registry report를 통해 reporting하지 않고 GetServerStat()를 통해서 리포트 되도록 하기위해서
	//// registry에 등록하지 않고 독립적인 metric 객체를 생성한다.
	s.ChannelCounter = met.NewCounter()

	s.AuthSuccessCounter = met.NewCounter()
	s.AuthDeniedCounter = met.NewCounter()
	s.AuthErrorCounter = met.NewCounter()
	s.AuthFailCounter = met.NewCounter()

	s.ConnAllowed = met.NewCounter()
	s.ConnDenied = met.NewCounter()

	s.AuthTimer = met.NewTimer()
	s.RecvPubTimer = met.NewTimer()
	s.SendPubTimer = met.NewTimer()

	//// 아래 항목들은 metric registry report를 통해 리포트되도록 한다.
	// r := met.DefaultRegistry
	// s.AuthTimer = met.NewRegisteredTimer("mgwd.auth.time", r)
	// s.RecvPubTimer = met.NewRegisteredTimer("mgwd.recv.pub", r)
	// s.SendPubTimer = met.NewRegisteredTimer("mgwd.send.pub", r)
	return s
}
