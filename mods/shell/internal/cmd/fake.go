package cmd

import (
	"fmt"
	"math"
	"time"

	"github.com/chzyer/readline"
	"github.com/machbase/neo-server/mods/shell/internal/client"
	"github.com/machbase/neo-server/mods/util"
)

func init() {
	client.RegisterCmd(&client.Cmd{
		Name:         "fake",
		PcFunc:       pcFake,
		Action:       doFake,
		Desc:         "Generating fake data into the specified table",
		Usage:        helpFake,
		Experimental: true,
	})
}

const helpFake = `  fake [options] <name>
  arguments:
    name                         tag name
  options:
    -a,--amplitude <float,...>   amplitude (default: 1.0)
    -f,--frequency <float,...>   frequency in Hz (default: 1.0)
    -p,--phase <float,...>       phase (default: 0)
    -b,--bias <float>            bias (default: 0)
    -r,--sampling-rate <float>        sampling rate (default: 10)
`

/*
ex) machbase-neo shell fake --frequency 120,60 -p 1.570796 sig.1
*/

type FakeCmd struct {
	Name         string    `arg:"" name:"name"`
	Ampl         []float64 `name:"amplitude" short:"a" default:"1.0"`
	Freq         []float64 `name:"frequency" short:"f" default:"1.0"`
	Phaz         []float64 `name:"phase" short:"p" default:"0"`
	Bias         float64   `name:"bias" short:"b" default:"0"`
	SamplingRate float64   `name:"sampling-rate" short:"r" default:"10"`
	Help         bool      `kong:"-"`
}

func pcFake() readline.PrefixCompleterInterface {
	return readline.PcItem("fake")
}

func doFake(ctx *client.ActionContext) {
	cmd := &FakeCmd{}
	parser, err := client.Kong(cmd, func() error { ctx.Println(helpFake); cmd.Help = true; return nil })
	if err != nil {
		ctx.Println("ERR", err.Error())
		return
	}
	_, err = parser.Parse(util.SplitFields(ctx.Line, false))
	if cmd.Help {
		return
	}
	if err != nil {
		ctx.Println("ERR", err.Error())
		return
	}

	sigs := []*SinSig{}
	for i := 0; i < 10; i++ {
		var freq float64 = 10
		var ampl float64 = 1.0
		var phase float64 = 0
		var exists = false
		if i < len(cmd.Freq) {
			exists, freq = true, cmd.Freq[i]
		}
		if i < len(cmd.Ampl) {
			exists, ampl = true, cmd.Ampl[i]
		}
		if i < len(cmd.Phaz) {
			exists, phase = true, cmd.Phaz[i]
		}
		if !exists {
			break
		}
		cosSig := &SinSig{
			Ampl: ampl,
			Freq: freq,
			Phaz: phase,
			Bias: cmd.Bias,
		}
		sigs = append(sigs, cosSig)
	}
	if len(sigs) == 0 {
		return
	}

	gen := NewFakeGenerator(SinComp(sigs), cmd.SamplingRate)
	defer gen.Stop()
	for v := range gen.C {
		ctx.Printfln("%s,%d,%.3f", cmd.Name, v.T.UnixNano(), v.V)
	}
}

type FakeGenerator struct {
	C            <-chan GenVal
	ch           chan GenVal
	functor      FakeFunctor
	samplingRate float64
	ticker       *time.Ticker
}

type FakeFunctor interface {
	Apply(t time.Time) float64
}

func NewFakeGenerator(s FakeFunctor, samplingRate float64) *FakeGenerator {
	gs := &FakeGenerator{
		functor:      s,
		samplingRate: samplingRate,
	}
	gs.ch = make(chan GenVal)
	gs.C = gs.ch

	go gs.run()
	return gs
}

func (gs *FakeGenerator) run() {
	T := float64(1*time.Second) / gs.samplingRate
	gs.ticker = time.NewTicker(time.Duration(T))

	for t := range gs.ticker.C {
		y := gs.functor.Apply(t)
		gs.ch <- GenVal{T: t, V: y}
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

type GenVal struct {
	T time.Time
	V float64
}

type SinSig struct {
	Ampl float64
	Freq float64
	Phaz float64
	Bias float64
}

type SinComp []*SinSig

func (sigs SinComp) Apply(t time.Time) float64 {
	y := 0.0
	for _, s := range sigs {
		y += s.Apply(t)
	}
	return y
}

func (cs *SinSig) String() string {
	return fmt.Sprintf("y = ampl(%f) x sin( 2π x freq(%f) x time + phase(%f) ) + %f",
		cs.Ampl, cs.Freq, cs.Phaz, cs.Bias)
}

func (cs *SinSig) Apply(t time.Time) float64 {
	ts := float64(t.UnixNano()) / float64(time.Second)
	y := cs.Ampl*math.Sin(2*math.Pi*cs.Freq*ts+cs.Phaz) + cs.Bias
	return y
}
