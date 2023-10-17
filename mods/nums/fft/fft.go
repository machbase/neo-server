package fft

import (
	"math/cmplx"
	"time"

	"gonum.org/v1/gonum/dsp/fourier"
)

func FastFourierTransform(times []time.Time, values []float64) ([]float64, []float64) {
	lenSamples := len(times)
	samplesDuration := times[lenSamples-1].Sub(times[0])
	period := float64(lenSamples) / (float64(samplesDuration) / float64(time.Second))
	fft := fourier.NewFFT(lenSamples)
	amplifier := func(v float64) float64 {
		return v * 2.0 / float64(lenSamples)
	}
	coeff := fft.Coefficients(nil, values)
	retHz := []float64{}
	retAmpl := []float64{}
	for i, c := range coeff {
		hz := fft.Freq(i) * period
		if hz == 0 {
			continue
		}
		// if hz < minHz {
		// 	continue
		// }
		// if hz > maxHz {
		// 	continue
		// }
		magnitude := cmplx.Abs(c)
		amplitude := amplifier(magnitude)
		// phase = cmplx.Phase(c)
		retHz = append(retHz, hz)
		retAmpl = append(retAmpl, amplitude)
	}
	return retHz, retAmpl
}
