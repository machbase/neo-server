package fft_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/d5/tengo/v2/require"
	"github.com/machbase/neo-server/mods/nums/fft"
	"github.com/machbase/neo-server/mods/nums/oscillator"
)

func TestFFT(t *testing.T) {
	compo := oscillator.NewComposite([]*oscillator.Generator{
		oscillator.New(10, 1.0),
		oscillator.New(50, 2.0),
	})
	ts := int64(1685714509 * 1000000000)

	times := []time.Time{}
	values := []float64{}
	for i := ts; i < ts+1000000000; /*1s*/ i += 100 * 1000 /* 100us */ {
		t := time.Unix(0, i)
		v := compo.EvalTime(t)

		times = append(times, t)
		values = append(values, v)
	}

	xs, ys := fft.FastFourierTransform(times, values)
	result := []string{}
	for i := range xs {
		if xs[i] >= 60 {
			// equivalent FFT(minHz(0), maxHz(60))
			break
		}
		result = append(result, fmt.Sprintf("%.6f,%.6f", xs[i], ys[i]))
	}
	expect := []string{
		"1.000100,0.000000", "2.000200,0.000000", "3.000300,0.000000", "4.000400,0.000000", "5.000500,0.000000", "6.000600,0.000000", "7.000700,0.000000", "8.000800,0.000000", "9.000900,0.000000", "10.001000,1.000000",
		"11.001100,0.000000", "12.001200,0.000000", "13.001300,0.000000", "14.001400,0.000000", "15.001500,0.000000", "16.001600,0.000000", "17.001700,0.000000", "18.001800,0.000000", "19.001900,0.000000", "20.002000,0.000000",
		"21.002100,0.000001", "22.002200,0.000000", "23.002300,0.000000", "24.002400,0.000000", "25.002500,0.000000", "26.002600,0.000000", "27.002700,0.000000", "28.002800,0.000000", "29.002900,0.000000", "30.003000,0.000000",
		"31.003100,0.000000", "32.003200,0.000000", "33.003300,0.000000", "34.003400,0.000000", "35.003500,0.000000", "36.003600,0.000000", "37.003700,0.000000", "38.003800,0.000000", "39.003900,0.000000", "40.004000,0.000000",
		"41.004100,0.000000", "42.004200,0.000000", "43.004300,0.000000", "44.004400,0.000000", "45.004500,0.000000", "46.004600,0.000000", "47.004700,0.000000", "48.004800,0.000000", "49.004900,0.000000", "50.005001,2.000000",
		"51.005101,0.000000", "52.005201,0.000000", "53.005301,0.000004", "54.005401,0.000000", "55.005501,0.000000", "56.005601,0.000000", "57.005701,0.000000", "58.005801,0.000000", "59.005901,0.000000",
	}
	for i, exp := range expect {
		require.Equal(t, result[i], exp, fmt.Sprintf("expect[%d]", i))
	}
}
