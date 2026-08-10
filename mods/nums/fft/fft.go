package fft

import (
	"math/cmplx"
	"sync"
	"time"

	"gonum.org/v1/gonum/dsp/fourier"
)

const maxFFTInputSamples = 1 << 20

type fftRuntime struct {
	transform *fourier.FFT
	coeff     []complex128
	hz        []float64
	ampl      []float64
}

var fftPool = sync.Pool{New: func() any {
	return &fftRuntime{}
}}

func FastFourierTransform(times []time.Time, values []float64) ([]float64, []float64) {
	lenSamples := len(times)
	if lenSamples == 0 || lenSamples != len(values) {
		return nil, nil
	}
	if lenSamples < 2 {
		return nil, nil
	}
	if lenSamples > maxFFTInputSamples {
		return nil, nil
	}

	samplesDuration := times[lenSamples-1].Sub(times[0])
	if samplesDuration <= 0 {
		return nil, nil
	}

	period := float64(lenSamples) / (float64(samplesDuration) / float64(time.Second))
	runtime := fftPool.Get().(*fftRuntime)
	if runtime.transform == nil || runtime.transform.Len() != lenSamples {
		runtime.transform = fourier.NewFFT(lenSamples)
	} else {
		runtime.transform.Reset(lenSamples)
	}

	coeffLen := lenSamples/2 + 1
	if cap(runtime.coeff) < coeffLen {
		runtime.coeff = make([]complex128, coeffLen)
	} else {
		runtime.coeff = runtime.coeff[:coeffLen]
	}
	coeff := runtime.transform.Coefficients(runtime.coeff, values)

	if cap(runtime.hz) < coeffLen {
		runtime.hz = make([]float64, 0, coeffLen)
	} else {
		runtime.hz = runtime.hz[:0]
	}
	if cap(runtime.ampl) < coeffLen {
		runtime.ampl = make([]float64, 0, coeffLen)
	} else {
		runtime.ampl = runtime.ampl[:0]
	}

	amplifier := func(v float64) float64 {
		return v * 2.0 / float64(lenSamples)
	}
	for i, c := range coeff {
		hz := runtime.transform.Freq(i) * period
		if hz == 0 {
			continue
		}
		magnitude := cmplx.Abs(c)
		amplitude := amplifier(magnitude)
		runtime.hz = append(runtime.hz, hz)
		runtime.ampl = append(runtime.ampl, amplitude)
	}

	retHz := append([]float64(nil), runtime.hz...)
	retAmpl := append([]float64(nil), runtime.ampl...)
	fftPool.Put(runtime)
	return retHz, retAmpl
}
