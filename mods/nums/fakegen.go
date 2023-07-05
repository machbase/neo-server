package nums

import (
	"time"
)

type FakeGenerator struct {
	C            <-chan FakeVal
	ch           chan FakeVal
	functor      FakeFunctor
	samplingRate int
	ticker       *time.Ticker

	batchThreshold time.Duration
}

type FakeFunctor func(t time.Time) float64

func NewFakeGenerator(s FakeFunctor, samplingRate int) *FakeGenerator {
	gs := &FakeGenerator{
		functor:        s,
		samplingRate:   samplingRate,
		batchThreshold: 5 * time.Microsecond,
	}
	gs.ch = make(chan FakeVal, 100)
	gs.C = gs.ch

	go gs.run()
	return gs
}

func (gs *FakeGenerator) run() {
	T := int(1*time.Second) / gs.samplingRate
	if time.Duration(T) >= gs.batchThreshold {
		gs.ticker = time.NewTicker(time.Duration(T))
		for t := range gs.ticker.C {
			y := gs.functor(t)
			gs.ch <- FakeVal{T: t.UnixNano(), V: y}
		}
	} else {
		samplesPerTick := 1
		tick := LCM(int64(T), int64(gs.batchThreshold))
		gs.ticker = time.NewTicker(time.Duration(tick))
		samplesPerTick = int(tick / int64(T))
		// fmt.Println("samplingRate  ", gs.samplingRate)
		// fmt.Println("T             ", time.Duration(T).String())
		// fmt.Println("tick          ", time.Duration(tick).String())
		// fmt.Println("samplesPerTick", samplesPerTick)
		for t := range gs.ticker.C {
			for i := 0; i < samplesPerTick; i++ {
				t = t.Add(time.Duration(i * T))
				y := gs.functor(t)
				gs.ch <- FakeVal{T: t.UnixNano(), V: y}
			}
		}
	}
}

func (gs *FakeGenerator) Stop() {
	if gs.ticker != nil {
		gs.ticker.Stop()
	}

	if gs.ch != nil {
		close(gs.ch)
	}
}

type FakeVal struct {
	// unix nano seconds
	T int64
	// value in float64
	V float64
}
