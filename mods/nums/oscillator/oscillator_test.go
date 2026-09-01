package oscillator

import (
	"math"
	"testing"
	"time"
)

func TestGenerator(t *testing.T) {
	generator := New(0.25, 2)
	if generator.Frequency != 0.25 || generator.Amplitude != 2 {
		t.Fatalf("New() returned %+v", generator)
	}
	if got := generator.Eval(1); math.Abs(got-2) > 1e-9 {
		t.Fatalf("Eval(1) was %f, want 2", got)
	}
	generator.Functor = nil
	generator.Phase = math.Pi / 2
	generator.Bias = 3
	if got := generator.Eval(0); math.Abs(got-5) > 1e-9 {
		t.Fatalf("Eval(0) with phase and bias was %f, want 5", got)
	}
	if got := generator.EvalTime(time.Unix(0, 0)); math.Abs(got-5) > 1e-9 {
		t.Fatalf("EvalTime(epoch) was %f, want 5", got)
	}
}

func TestComposite(t *testing.T) {
	first := New(0, 1)
	first.Bias = 2
	second := New(0, 1)
	second.Bias = 3
	now := time.Unix(1, 0)
	if got := NewComposite([]*Generator{first, second}).EvalTime(now); math.Abs(got-5) > 1e-9 {
		t.Fatalf("composite value was %f, want 5", got)
	}
	if got := NewCompositeWithNoise([]*Generator{first}, 0).EvalTime(now); math.Abs(got-2) > 1e-9 {
		t.Fatalf("zero-noise composite value was %f, want 2", got)
	}
	if got := NewCompositeWithNoise(nil, 1).EvalTime(now); math.IsNaN(got) || math.IsInf(got, 0) {
		t.Fatalf("noise composite returned non-finite value %f", got)
	}
}
