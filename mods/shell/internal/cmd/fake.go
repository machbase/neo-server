package cmd

import (
	"math"
	"strings"

	"github.com/machbase/neo-server/v8/api"
	"github.com/machbase/neo-server/v8/mods/nums"
	"github.com/machbase/neo-server/v8/mods/nums/oscillator"
	"github.com/machbase/neo-server/v8/mods/shell/internal/action"
	"github.com/machbase/neo-server/v8/mods/util"
)

func init() {
	action.RegisterCmd(&action.Cmd{
		Name:              "fake",
		PcFunc:            pcFake,
		Action:            doFake,
		Desc:              "Generating fake data and writing into the specified table",
		Usage:             strings.ReplaceAll(helpFake, "\t", "    "),
		Experimental:      false,
		Deprecated:        true,
		DeprecatedMessage: "Use TQL 'FAKE()' instead",
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
    -r,--sampling-rate <int>     sampling rate per sec. (default: 10)
    -z,--noise <float>           possible max amplitude of noise (default: 0 no-noise)`

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
	SamplingRate int       `name:"sampling-rate" short:"r" default:"10"`
	Noise        float64   `name:"noise" short:"z" default:"0"`
	Help         bool      `kong:"-"`
}

func pcFake() action.PrefixCompleterInterface {
	return action.PcItem("fake")
}

func doFake(ctx *action.ActionContext) {
	cmd := &FakeCmd{}
	parser, err := action.Kong(cmd, func() error { ctx.Println(helpFake); cmd.Help = true; return nil })
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

	sigs := []*oscillator.Generator{}
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
		s := &oscillator.Generator{
			Amplitude: ampl,
			Frequency: freq,
			Phase:     phase,
			Bias:      cmd.Bias,
			Functor:   math.Sin,
		}
		sigs = append(sigs, s)
	}
	if len(sigs) == 0 {
		return
	}

	eval := oscillator.NewCompositeWithNoise(sigs, cmd.Noise).EvalTime

	var appender api.Appender
	if len(cmd.Table) > 0 {
		conn, err := ctx.BorrowConn()
		if err != nil {
			ctx.Printfln("ERR", err.Error())
			return
		}
		appender, err = conn.Appender(ctx.Ctx, cmd.Table)
		if err != nil {
			ctx.Printfln("ERR", err.Error())
			return
		}
		defer func() {
			s, f, e := appender.Close()
			if e != nil {
				ctx.Println("Wrote to", strings.ToUpper(cmd.Table)+"/"+cmd.Name, "success:", s, "fail:", f, "ERR", e.Error())
			} else {
				ctx.Println("Wrote to", strings.ToUpper(cmd.Table)+"/"+cmd.Name, "success:", s, "fail:", f)
			}
		}()
	}

	gen := nums.NewFakeGenerator(eval, cmd.SamplingRate)
	for v := range gen.C {
		if appender == nil {
			ctx.Printfln("%s,%d,%f", cmd.Name, v.T, v.V)
		} else {
			if err := appender.Append(cmd.Name, v.T, v.V); err != nil {
				ctx.Println("ERR", err.Error())
			}
		}
		if ctx.Ctx.Err() != nil {
			break
		}
	}
	gen.Stop()
}
