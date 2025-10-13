package metric

import "time"

type Deriver interface {
	ID() string
	WindowSize() int
	Derive(values []Value) Value
}

func NewMovingAverage(id string, windowSize int) Deriver {
	return &MovingAverage{id: id, windowSize: windowSize}
}

var _ Deriver = MovingAverage{}

type MovingAverage struct {
	id string
	// Number of points to include in the moving average calculation.
	// Must be less than or equal to the maxCount of the TimeSeries.
	// If greater than maxCount, it will be set to maxCount.
	windowSize int
}

func (ma MovingAverage) ID() string {
	return ma.id
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
	case *MeterValue:
		return ma.DeriveMeter(values)
	case *TimerValue:
		return ma.DeriveTimer(values)
	case *HistogramValue:
		return ma.DeriveHistogram(values)
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

func (ma MovingAverage) DeriveMeter(values []Value) Value {
	var sum float64
	var first float64
	var last float64
	var min float64
	var max float64
	var samples int64
	var validValueCount int

	for _, value := range values {
		if value == nil {
			continue
		}
		val, ok := value.(*MeterValue)
		if !ok {
			continue
		}
		if val.Samples == 0 {
			continue
		}
		validValueCount++
		samples += val.Samples
		sum += val.Sum
		first += val.First
		last += val.Last
		min += val.Min
		max += val.Max
	}
	ret := &MeterValue{
		Samples: samples,
		Sum:     sum,
	}
	if validValueCount > 0 {
		ret.First = first / float64(validValueCount)
		ret.Last = last / float64(validValueCount)
		ret.Min = min / float64(validValueCount)
		ret.Max = max / float64(validValueCount)
	}
	return ret
}

func (ma MovingAverage) DeriveTimer(values []Value) Value {
	var sum time.Duration
	var min time.Duration
	var max time.Duration
	var validValueCount int
	var samples int64
	for _, value := range values {
		if value == nil {
			continue
		}
		val, ok := value.(*TimerValue)
		if !ok {
			continue
		}
		if val.Samples > 0 {
			samples += val.Samples
			sum += val.Sum
			min = min + val.Min
			max = max + val.Max
			validValueCount++
		}
	}
	ret := &TimerValue{
		Samples: samples,
		Sum:     sum,
	}
	if validValueCount > 0 {
		ret.Min = min / time.Duration(validValueCount)
		ret.Max = max / time.Duration(validValueCount)
	}
	return ret
}

func (ma MovingAverage) DeriveHistogram(values []Value) Value {
	var validValues []float64
	var validP []float64
	var validValueCount int
	var samples int64
	for _, value := range values {
		if value == nil {
			continue
		}
		val, ok := value.(*HistogramValue)
		if !ok {
			continue
		}
		if len(validValues) == 0 {
			validValues = make([]float64, len(val.Values))
			validP = make([]float64, len(val.P))
			copy(validP, val.P)
		}
		if val.Samples > 0 {
			samples += val.Samples
			validValueCount++
			for i := range val.Values {
				validValues[i] += val.Values[i]
			}
		}
	}
	ret := &HistogramValue{
		Samples: samples,
		P:       validP,
		Values:  make([]float64, len(validValues)),
	}
	if validValueCount > 0 {
		for i := range ret.Values {
			ret.Values[i] = validValues[i] / float64(validValueCount)
		}
	}
	return ret
}
