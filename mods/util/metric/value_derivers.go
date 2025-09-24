package metric

type Deriver interface {
	WindowSize() int
	Derive(values []Value) Value
}

func NewMovingAverage(windowSize int) Deriver {
	return &MovingAverage{windowSize: windowSize}
}

var _ Deriver = MovingAverage{}

type MovingAverage struct {
	// Number of points to include in the moving average calculation.
	// Must be less than or equal to the maxCount of the TimeSeries.
	// If greater than maxCount, it will be set to maxCount.
	windowSize int
}

func (ma MovingAverage) WindowSize() int {
	return ma.windowSize
}

func (ma MovingAverage) Derive(values []Value) Value {
	switch values[len(values)-1].(type) {
	case *CounterValue:
		return ma.DeriveCounter(values)
	case *GaugeValue:
		return ma.DeriveGauge(values)
	default:
		return values[len(values)-1]
	}
}

func (ma MovingAverage) DeriveCounter(values []Value) Value {
	var sum float64
	var samples int64

	for _, value := range values {
		if value == nil {
			continue
		}
		val, ok := value.(*CounterValue)
		if !ok {
			continue
		}
		if val.Samples > 0 {
			samples += val.Samples
			sum += val.Value * float64(val.Samples)
		}
	}
	ret := &CounterValue{
		Samples: samples,
	}
	if samples > 0 {
		ret.Value = sum / float64(samples)
	}
	return ret
}

func (ma MovingAverage) DeriveGauge(values []Value) Value {
	var sum float64
	var lastValueSum float64
	var lastValueCount int
	var samples int64
	for _, value := range values {
		if value == nil {
			continue
		}
		val, ok := value.(*GaugeValue)
		if !ok {
			continue
		}
		if val.Samples > 0 {
			samples += val.Samples
			sum += val.Sum
			lastValueSum += val.Value
			lastValueCount++
		}
	}
	ret := &GaugeValue{
		Samples: samples,
		Sum:     sum,
	}
	if lastValueCount > 0 {
		ret.Value = lastValueSum / float64(lastValueCount)
	}
	return ret
}
