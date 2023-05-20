package oscilator

import (
	"math"
	"time"
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

type Composite []*Generator

func (sigs Composite) EvalTime(t time.Time) float64 {
	y := 0.0
	for _, s := range sigs {
		y += s.EvalTime(t)
	}
	return y
}
