package tql

import (
	"errors"
	"math"
	"testing"
	"time"

	"github.com/machbase/neo-server/v8/mods/nums"
	"github.com/stretchr/testify/require"
	"gonum.org/v1/gonum/stat"
)

type capturePredictor struct {
	fitErr     error
	fittedX    []float64
	fittedY    []float64
	predictArg float64
	ret        float64
}

func (p *capturePredictor) Fit(xs, ys []float64) error {
	p.fittedX = append([]float64(nil), xs...)
	p.fittedY = append([]float64(nil), ys...)
	return p.fitErr
}

func (p *capturePredictor) Predict(x float64) float64 {
	p.predictArg = x
	return p.ret
}

func TestAggregateHelpers(t *testing.T) {
	node := &Node{}

	t.Run("covariance aggregate", func(t *testing.T) {
		agg := node.fmCovariance(1.5, 2.5, Weight(3), "cov").(*GroupAggregate)
		require.Equal(t, "covariance", agg.Type)
		require.Equal(t, "cov", agg.Name)
		require.Equal(t, []any{1.5, 2.5, Weight(3)}, agg.Value)
	})

	t.Run("quantile interpolated aggregate", func(t *testing.T) {
		agg := node.fmQuantileInterpolated(10, 0.9).(*GroupAggregate)
		require.Equal(t, "quantile", agg.Type)
		require.Equal(t, "QUANTILE", agg.Name)
		require.Equal(t, 10.0, agg.Value)
		require.Equal(t, 0.9, agg.Percentile)
		require.Equal(t, stat.LinInterp, agg.Cumulant)
	})
}

func TestMapLowPass(t *testing.T) {
	t.Run("invalid alpha", func(t *testing.T) {
		node := &Node{}
		node.SetInflight(NewRecord("k", []any{0.0}))

		ret, err := node.fmMapLowPass(0, 10.0, 1.0)
		require.Nil(t, ret)
		require.EqualError(t, err, "MAP_LOWPASS() should have 0 < alpha < 1 ")
	})

	t.Run("stateful smoothing", func(t *testing.T) {
		node := &Node{}
		node.SetInflight(NewRecord("k", []any{0.0}))

		ret, err := node.fmMapLowPass(0, 10.0, 0.25)
		require.NoError(t, err)
		require.Equal(t, []any{10.0}, ret.(*Record).Value())

		node.SetInflight(NewRecord("k", []any{0.0}))
		ret, err = node.fmMapLowPass(0, 14.0, 0.25)
		require.NoError(t, err)
		require.Equal(t, []any{11.0}, ret.(*Record).Value())
	})
}

func TestGeoDistance(t *testing.T) {
	node := &Node{}
	seoul := nums.NewLatLon(37.5665, 126.9780)
	busan := nums.NewLatLon(35.1796, 129.0756)

	node.SetInflight(NewRecord("k", []any{99.0}))
	ret, err := node.fmGeoDistance(0, seoul)
	require.NoError(t, err)
	values := ret.(*Record).Value().([]any)
	require.Len(t, values, 1)
	require.Zero(t, values[0])

	node.SetInflight(NewRecord("k", []any{99.0}))
	ret, err = node.fmGeoDistance(0, busan)
	require.NoError(t, err)
	values = ret.(*Record).Value().([]any)
	require.Len(t, values, 1)
	require.InDelta(t, busan.Distance(seoul), values[0].(float64), 0.001)

	node.SetInflight(NewRecord("k", []any{99.0}))
	ret, err = node.fmGeoDistance(0, "not-a-location")
	require.NoError(t, err)
	values = ret.(*Record).Value().([]any)
	require.Len(t, values, 1)
	require.Zero(t, values[0])
}

func TestGroupFillerPredict(t *testing.T) {
	t.Run("fit keeps latest samples", func(t *testing.T) {
		predictor := &capturePredictor{}
		filler := &GroupFillerPredict{predictor: predictor, fallback: "fallback"}

		filler.Fit("bad", 1.0)
		filler.Fit(1.0, "bad")
		require.Empty(t, filler.xs)
		require.Empty(t, filler.ys)

		for i := 0; i < 105; i++ {
			filler.Fit(i, i*2)
		}
		require.Len(t, filler.xs, 100)
		require.Len(t, filler.ys, 100)
		require.Equal(t, 5.0, filler.xs[0])
		require.Equal(t, 10.0, filler.ys[0])
		require.Equal(t, 104.0, filler.xs[len(filler.xs)-1])
		require.Equal(t, 208.0, filler.ys[len(filler.ys)-1])
	})

	t.Run("predict returns fallback when not ready", func(t *testing.T) {
		plain := &GroupFillerPredict{fallback: "fallback"}
		require.Equal(t, "fallback", plain.Predict(10.0))

		withPredictor := &GroupFillerPredict{fallback: "fallback", predictor: &capturePredictor{ret: 9}}
		withPredictor.Fit(1.0, 2.0)
		require.Equal(t, "fallback", withPredictor.Predict(10.0))

		withPredictor.Fit(2.0, 4.0)
		require.Equal(t, "fallback", withPredictor.Predict("bad"))

		failedFit := &GroupFillerPredict{fallback: "fallback", predictor: &capturePredictor{fitErr: errors.New("fit failed")}}
		failedFit.Fit(1.0, 2.0)
		failedFit.Fit(2.0, 4.0)
		require.Equal(t, "fallback", failedFit.Predict(3.0))
	})

	t.Run("predict with linear regression", func(t *testing.T) {
		filler := &GroupFillerPredict{fallback: -1.0, useLinearRegression: true}
		filler.Fit(0.0, 1.0)
		filler.Fit(1.0, 3.0)
		filler.Fit(2.0, 5.0)

		ret := filler.Predict(3.0)
		require.InDelta(t, 7.0, ret.(float64), 1e-9)
	})

	t.Run("predict with external predictor", func(t *testing.T) {
		base := time.Unix(0, 100)
		next := base.Add(10 * time.Nanosecond)
		predictor := &capturePredictor{ret: 123.5}
		filler := &GroupFillerPredict{fallback: -1.0, predictor: predictor}

		filler.Fit(base, 10)
		filler.Fit(next, 20)

		ret := filler.Predict(next)
		require.Equal(t, 123.5, ret)
		require.Equal(t, []float64{float64(base.UnixNano()), float64(next.UnixNano())}, predictor.fittedX)
		require.Equal(t, []float64{10, 20}, predictor.fittedY)
		require.Equal(t, float64(next.UnixNano()), predictor.predictArg)
	})
}

func TestGroupFillersAndUnbox(t *testing.T) {
	t.Run("null value filler", func(t *testing.T) {
		filler := &GroupFillerNullValue{alt: "fallback"}
		filler.Fit("ignored", 123)
		require.Equal(t, "fallback", filler.Predict("anything"))
	})

	t.Run("time window filler", func(t *testing.T) {
		filler := &GroupFillerTimeWindow{}
		tick := time.Unix(0, 42)
		filler.Fit("ignored", 123)
		require.Equal(t, tick, filler.Predict(tick))
	})

	t.Run("unbox scalar types", func(t *testing.T) {
		filler := &GroupFillerPredict{}
		floatVal := 1.25
		intVal := 7
		int32Val := int32(8)
		int64Val := int64(9)
		timeVal := time.Unix(0, 100)

		cases := []struct {
			name  string
			input any
			want  float64
			ok    bool
		}{
			{name: "nil", input: nil, want: 0, ok: false},
			{name: "float64 pointer", input: &floatVal, want: 1.25, ok: true},
			{name: "time pointer", input: &timeVal, want: float64(timeVal.UnixNano()), ok: true},
			{name: "int pointer", input: &intVal, want: 7, ok: true},
			{name: "int32", input: int32Val, want: 8, ok: true},
			{name: "int32 pointer", input: &int32Val, want: 8, ok: true},
			{name: "int64", input: int64Val, want: 9, ok: true},
			{name: "int64 pointer", input: &int64Val, want: 9, ok: true},
			{name: "unsupported", input: "bad", want: 0, ok: false},
		}

		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				got, ok := filler.unbox(tc.input)
				require.Equal(t, tc.ok, ok)
				require.Equal(t, tc.want, got)
			})
		}
	})
}

func TestThrottle(t *testing.T) {
	t.Run("nil inflight still initializes throttle", func(t *testing.T) {
		node := &Node{}
		require.Nil(t, node.fmThrottle(1000))

		value, ok := node.GetValue("throttle")
		require.True(t, ok)
		throttle := value.(*Throttle)
		require.Equal(t, time.Millisecond, throttle.minDuration)
	})

	t.Run("returns inflight without waiting when interval passed", func(t *testing.T) {
		rec := NewRecord("k", 1)
		node := &Node{}
		node.SetInflight(rec)
		previous := time.Now().Add(-10 * time.Millisecond)
		node.SetValue("throttle", &Throttle{minDuration: time.Millisecond, last: previous})

		ret := node.fmThrottle(1000)
		require.Same(t, rec, ret)

		value, _ := node.GetValue("throttle")
		throttle := value.(*Throttle)
		require.True(t, throttle.last.After(previous))
	})

	t.Run("returns inflight after short wait when interval not passed", func(t *testing.T) {
		rec := NewRecord("k", 1)
		node := &Node{}
		node.SetInflight(rec)
		previous := time.Now()
		node.SetValue("throttle", &Throttle{minDuration: time.Millisecond, last: previous})

		ret := node.fmThrottle(1000)
		require.Same(t, rec, ret)

		value, _ := node.GetValue("throttle")
		throttle := value.(*Throttle)
		require.True(t, throttle.last.After(previous))
	})
}

func TestGroupColumnRelation(t *testing.T) {
	t.Run("append and result", func(t *testing.T) {
		covariance := &GroupColumnRelation{name: "covariance"}
		require.NoError(t, covariance.Append([]any{1.0, 2.0}))
		require.NoError(t, covariance.Append([]any{2.0, 4.0, Weight(2)}))
		require.Equal(t, stat.Covariance(covariance.x, covariance.wv.Values(), covariance.wv.Weights()), covariance.Result())

		correlation := &GroupColumnRelation{name: "correlation"}
		require.NoError(t, correlation.Append([]any{1.0, 2.0}))
		require.NoError(t, correlation.Append([]any{2.0, 4.0}))
		require.InDelta(t, 1.0, correlation.Result().(float64), 1e-12)

		lrs := &GroupColumnRelation{name: "lrs"}
		require.NoError(t, lrs.Append([]any{1.0, 3.0}))
		require.NoError(t, lrs.Append([]any{2.0, 5.0}))
		require.NoError(t, lrs.Append([]any{3.0, 7.0}))
		require.InDelta(t, 2.0, lrs.Result().(float64), 1e-12)
	})

	t.Run("invalid append leaves no result", func(t *testing.T) {
		relation := &GroupColumnRelation{name: "covariance"}
		require.NoError(t, relation.Append("bad"))
		require.NoError(t, relation.Append([]any{"bad", 1.0}))
		require.NoError(t, relation.Append([]any{1.0, "bad"}))
		require.Nil(t, relation.Result())

		relation = &GroupColumnRelation{name: "unknown"}
		require.NoError(t, relation.Append([]any{1.0, 2.0}))
		require.Nil(t, relation.Result())
	})
}

func TestGroupColumnMoment(t *testing.T) {
	t.Run("append and result", func(t *testing.T) {
		moment := &GroupColumnMoment{name: "moment", moment: 2}
		require.NoError(t, moment.Append(2.0))
		require.NoError(t, moment.Append([]any{4.0, Weight(2)}))
		require.Equal(t, stat.Moment(2, moment.wv.Values(), moment.wv.Weights()), moment.Result())
	})

	t.Run("empty and invalid", func(t *testing.T) {
		empty := &GroupColumnMoment{name: "moment", moment: 2}
		require.Nil(t, empty.Result())

		unknown := &GroupColumnMoment{name: "unknown", moment: 2}
		require.NoError(t, unknown.Append(1.0))
		require.Nil(t, unknown.Result())

		invalid := &GroupColumnMoment{name: "moment", moment: 2}
		err := invalid.Append("bad")
		require.Error(t, err)
	})
}

func TestGroupColumnContainer(t *testing.T) {
	t.Run("calculations and reset", func(t *testing.T) {
		cdf := &GroupColumnContainer{name: "cdf", quantile: 2.5, cumulant: stat.Empirical}
		require.NoError(t, cdf.Append(1.0))
		require.NoError(t, cdf.Append(2.0))
		require.NoError(t, cdf.Append(4.0))
		require.Equal(t, stat.CDF(2.5, stat.Empirical, []float64{1, 2, 4}, []float64{1, 1, 1}), cdf.Result())
		require.Empty(t, cdf.wv)

		quantile := &GroupColumnContainer{name: "quantile", percentile: 0.5, cumulant: stat.Empirical}
		require.NoError(t, quantile.Append(1.0))
		require.NoError(t, quantile.Append(2.0))
		require.NoError(t, quantile.Append(4.0))
		require.Equal(t, stat.Quantile(0.5, stat.Empirical, []float64{1, 2, 4}, []float64{1, 1, 1}), quantile.Result())

		mean := &GroupColumnContainer{name: "mean"}
		require.NoError(t, mean.Append(1.0))
		require.NoError(t, mean.Append([]any{3.0, Weight(2)}))
		meanWant, _ := stat.MeanStdDev(mean.wv.Values(), mean.wv.Weights())
		require.Equal(t, meanWant, mean.Result())

		variance := &GroupColumnContainer{name: "variance"}
		require.NoError(t, variance.Append(1.0))
		require.NoError(t, variance.Append(3.0))
		_, varianceWant := stat.MeanVariance(variance.wv.Values(), variance.wv.Weights())
		require.Equal(t, varianceWant, variance.Result())

		stddev := &GroupColumnContainer{name: "stddev"}
		require.NoError(t, stddev.Append(1.0))
		require.NoError(t, stddev.Append(3.0))
		_, stddevWant := stat.MeanStdDev(stddev.wv.Values(), stddev.wv.Weights())
		require.Equal(t, stddevWant, stddev.Result())

		stderr := &GroupColumnContainer{name: "stderr"}
		require.NoError(t, stderr.Append(1.0))
		require.NoError(t, stderr.Append(3.0))
		_, stderrStd := stat.MeanStdDev(stderr.wv.Values(), stderr.wv.Weights())
		require.Equal(t, stat.StdErr(stderrStd, float64(len(stderr.wv))), stderr.Result())

		entropy := &GroupColumnContainer{name: "entropy"}
		require.NoError(t, entropy.Append(1.0))
		require.NoError(t, entropy.Append(1.0))
		require.NoError(t, entropy.Append(2.0))
		require.Equal(t, stat.Entropy(entropy.wv.Values()), entropy.Result())

		mode := &GroupColumnContainer{name: "mode"}
		require.NoError(t, mode.Append(1.0))
		require.NoError(t, mode.Append(2.0))
		require.NoError(t, mode.Append(2.0))
		modeWant, _ := stat.Mode(mode.wv.Values(), mode.wv.Weights())
		require.Equal(t, modeWant, mode.Result())
	})

	t.Run("empty unknown nan and invalid", func(t *testing.T) {
		empty := &GroupColumnContainer{name: "mean"}
		require.Nil(t, empty.Result())

		unknown := &GroupColumnContainer{name: "unknown"}
		require.NoError(t, unknown.Append(1.0))
		require.Nil(t, unknown.Result())

		nanValue := &GroupColumnContainer{name: "entropy"}
		require.NoError(t, nanValue.Append(0.0))
		require.NoError(t, nanValue.Append(0.0))
		require.Equal(t, 0.0, nanValue.Result())

		invalid := &GroupColumnContainer{name: "mean"}
		require.Error(t, invalid.Append("bad"))
	})
}

func TestGroupColumnCounter(t *testing.T) {
	t.Run("count avg rss rms and reset", func(t *testing.T) {
		count := &GroupColumnCounter{name: "count"}
		require.NoError(t, count.Append("ignored"))
		require.NoError(t, count.Append(nil))
		require.Equal(t, 2.0, count.Result())
		require.Zero(t, count.count)
		require.Zero(t, count.value)

		avg := &GroupColumnCounter{name: "avg"}
		require.NoError(t, avg.Append(2.0))
		require.NoError(t, avg.Append(4.0))
		require.Equal(t, 3.0, avg.Result())

		rss := &GroupColumnCounter{name: "rss"}
		require.NoError(t, rss.Append(3.0))
		require.NoError(t, rss.Append(4.0))
		require.Equal(t, 5.0, rss.Result())

		rms := &GroupColumnCounter{name: "rms"}
		require.NoError(t, rms.Append(3.0))
		require.NoError(t, rms.Append(4.0))
		require.Equal(t, math.Sqrt(12.5), rms.Result())
	})

	t.Run("empty unknown and invalid", func(t *testing.T) {
		empty := &GroupColumnCounter{name: "avg"}
		require.Nil(t, empty.Result())

		unknown := &GroupColumnCounter{name: "unknown"}
		require.NoError(t, unknown.Append(3.0))
		require.Nil(t, unknown.Result())

		invalid := &GroupColumnCounter{name: "avg"}
		require.Error(t, invalid.Append("bad"))
	})
}
