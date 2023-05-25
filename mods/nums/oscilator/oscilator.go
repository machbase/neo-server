package oscilator

import (
	"math"
	"time"

	"github.com/machbase/neo-server/mods/nums/opensimplex"
)

type Generator struct {
	Amplitude float64
	Frequency float64
	Phase     float64
	Bias      float64
	Functor   func(float64) float64
}

func New(frequency float64, amplitude float64) *Generator {
	return &Generator{
		Amplitude: amplitude,
		Frequency: frequency,
		Functor:   math.Sin,
	}
}
func (g *Generator) Eval(x float64) float64 {
	if g.Functor == nil {
		g.Functor = math.Sin
	}
	return g.Amplitude*g.Functor(2*math.Pi*g.Frequency*x+g.Phase) + g.Bias
}

func (g *Generator) EvalTime(t time.Time) float64 {
	x := float64(t.UnixNano()) / float64(time.Second)
	return g.Eval(x)
}

type Composite interface {
	EvalTime(t time.Time) float64
}

func NewComposite(s []*Generator) Composite {
	return &composite{
		sigs: s,
	}
}

func NewCompositeWithNoise(s []*Generator, noiseMaxAmplitude float64) Composite {
	var noise *opensimplex.Generator
	if noiseMaxAmplitude != 0 {
		noise = opensimplex.New(time.Now().UnixNano())
	}
	return &composite{
		sigs:  s,
		noise: noise,
	}
}

type composite struct {
	sigs  []*Generator
	noise *opensimplex.Generator

	noiseMaxAmplitude float64
}

func (c *composite) EvalTime(t time.Time) float64 {
	y := 0.0
	for _, s := range c.sigs {
		y += s.EvalTime(t)
	}
	if c.noise != nil {
		y += c.noiseMaxAmplitude + c.noise.Eval(float64(t.Nanosecond()))
	}
	return y
}
