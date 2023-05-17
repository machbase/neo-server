package cmd

import (
	"fmt"
	"io"
	"math"
	"strings"
	"sync"
	"time"

	"github.com/chzyer/readline"
	"github.com/machbase/neo-server/mods/shell/internal/client"
	"github.com/machbase/neo-server/mods/util"
	spi "github.com/machbase/neo-spi"
)

func init() {
	client.RegisterCmd(&client.Cmd{
		Name:         "fake",
		PcFunc:       pcFake,
		Action:       doFake,
		Desc:         "Generating fake data and writing into the specified table",
		Usage:        helpFake,
		Experimental: false,
	})
}

const helpFake = `  fake [options] [table]
    generates fake data which is (y:value, t:current time)
      y = amplitude0⋅sin(2π⋅frequency0⋅t+phase0) + ... + amplitudeN⋅sin(2π⋅frequencyN⋅t+phaseN) + bias
  arguments:
    table                        table to write data (print to stdout if not specified)
  options:
    -n,--name <tag_name>         tag name (default: 'value')
    -a,--amplitude <float,...>   amplitude (default: 1.0)
    -f,--frequency <float,...>   frequency in Hz (default: 1.0)
    -p,--phase <float,...>       phase (default: 0)
    -b,--bias <float>            bias (default: 0)
    -r,--sampling-rate <float>   sampling rate per sec. (default: 10)
`

/*
ex) machbase-neo shell fake --name sig.1 --amplitude 2.0,1.0 --frequency 0.5,1 --sampling-rate 100
*/

type FakeCmd struct {
	Table        string    `arg:"" optional:""`
	Name         string    `name:"name" short:"n" default:"value"`
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

	var appender spi.Appender
	if len(cmd.Table) > 0 {
		appender, err = ctx.DB.Appender(cmd.Table, spi.AppendTimeformatOption("ns"))
		if err != nil {
			ctx.Printfln("ERR", err.Error())
			return
		}
		defer func() {
			s, f, e := appender.Close()
			if e != nil {
				ctx.Println("Wrote to", cmd.Table, "success:", s, "fail:", f, "ERR", e.Error())
			} else {
				ctx.Println("Wrote to", cmd.Table, "success:", s, "fail:", f)
			}
		}()
	}

	loopQuit := make(chan bool, 1)
	if ctx.Interactive {
		go func() {
			prompt := ""
			if appender != nil {
				prompt = "fake is running ('exit⏎' to stop) > "
			}
			rl, err := readline.NewEx(&readline.Config{
				Prompt:                 prompt,
				DisableAutoSaveHistory: true,
				InterruptPrompt:        "^C",
				Stdin:                  ctx.Stdin,
				Stdout:                 ctx.Stdout,
				Stderr:                 ctx.Stderr,
			})
			if err != nil {
				panic(err)
			}
			defer rl.Close()
			rl.CaptureExitSignal()
			for {
				line, err := rl.Readline()
				if err == readline.ErrInterrupt {
					break
				} else if err == io.EOF {
					break
				}
				line = strings.TrimSpace(line)
				if line == "exit" || line == "quit" {
					break
				}
			}
			loopQuit <- true
		}()
	}

	var stopOnce sync.Once
	gen := NewFakeGenerator(SinComp(sigs), cmd.SamplingRate)
	defer stopOnce.Do(func() { gen.Stop() })

	go func() {
		for v := range gen.C {
			if appender == nil {
				ctx.Printfln("%s,%d,%.3f", cmd.Name, v.T.UnixNano(), v.V)
			} else {
				if err := appender.Append(cmd.Name, v.T.UnixNano(), v.V); err != nil {
					ctx.Println("ERR", err.Error())
				}
			}
		}
	}()

	<-loopQuit
	stopOnce.Do(func() { gen.Stop() })
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
	y := cs.Ampl*math.Sin(2*math.Pi*cs.Freq*ts+cs.Phaz) + cs.Bias /* + addNoise() */
	return y
}
