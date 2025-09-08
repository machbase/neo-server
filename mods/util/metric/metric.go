package metric

import (
	"errors"
	"expvar"
	"fmt"
	"math"
	"strings"
	"sync"
	"time"
)

// InputFunc is a function type that matches the signature of the Collect method.
// Periodically called by the Collector to gather metrics.
type InputFunc func(Gather)

// OutputFunc is a function type that processes the collected ProductData.
type OutputFunc func(Product)

type Gather interface {
	AddMeasurement(Measurement)
	AddError(error)
	Err() error
}

type Measurement struct {
	Name   string
	Fields []Field // name -value pairs and producer function
	// fields not exposed
	ts   time.Time // time when the measurement was taken
	noop bool
}

func (m *Measurement) AddField(f ...Field) {
	m.Fields = append(m.Fields, f...)
}

type Field struct {
	Name  string
	Value float64
	Type  Type
}

// CounterType supports samples count, value (sum)
func CounterType(u Unit) Type {
	return Type{
		p: func() Producer { return NewCounter() },
		s: "counter",
		u: u,
	}
}

// GaugeType supports samples count, sum, value (last)
func GaugeType(u Unit) Type {
	return Type{
		p: func() Producer { return NewGauge() },
		s: "gauge",
		u: u,
	}
}

// MeterType supports samples count, sum, first, last, min, max
func MeterType(u Unit) Type {
	return Type{
		p: func() Producer { return NewMeter() },
		s: "meter",
		u: u,
	}
}

// OdometerType supports samples count, quantiles
func HistogramType(u Unit) Type {
	return HistogramTypePercentiles(u, 100, 0.5, 0.90, 0.99)
}

func OdometerType(u Unit) Type {
	return Type{
		p: func() Producer { return NewOdometer() },
		s: "odometer",
		u: u,
	}
}

func HistogramTypePercentiles(u Unit, maxBin int, ps ...float64) Type {
	return Type{
		p: func() Producer { return NewHistogram(maxBin, ps...) },
		s: "histogram",
		u: u,
	}
}

// TimerType supports samples count, total, min, max
func TimerType(u Unit) Type {
	return Type{
		p: func() Producer { return NewTimer() },
		s: "timer",
		u: u,
	}
}

type FieldInfo struct {
	Measure string
	Name    string
	Series  string
	Period  time.Duration
	Type    string
	Unit    Unit
}

type InputWrapper struct {
	input          InputFunc
	measureName    string
	mtsFields      map[string]MultiTimeSeries
	publishedNames map[string]string
}

type Collector struct {
	sync.Mutex

	// registered input functions
	inputs  map[string]*InputWrapper
	outputs []OutputFunc

	// periodically collects metrics from inputs
	samplingInterval time.Duration
	closeCh          chan struct{}
	stopWg           sync.WaitGroup

	// event-driven measurements
	recvCh     chan Measurement
	recvChSize int
	// a channel to which measurements can be sent.
	C chan<- Measurement

	// time series configuration
	series       []CollectorSeries
	expvarPrefix string

	// persistent storage
	storage Storage
}

type CollectorSeries struct {
	Name     string
	Period   time.Duration
	MaxCount int
}

// NewCollector creates a new Collector with the specified interval.
// The interval determines how often the inputs will be collected.
// The collector will run until Stop() is called.
// It is safe to call Start() multiple times, but Stop() should be called only once
func NewCollector(opts ...CollectorOption) *Collector {
	c := &Collector{
		samplingInterval: 10 * time.Second,
		closeCh:          make(chan struct{}),
		inputs:           make(map[string]*InputWrapper),
	}
	for _, opt := range opts {
		opt(c)
	}
	if c.recvChSize <= 0 {
		c.recvChSize = 100
	}
	c.recvCh = make(chan Measurement, c.recvChSize)
	c.C = c.recvCh
	return c
}

type CollectorOption func(c *Collector)

// WithSamplingInterval sets the collection interval for the collector.
// Default is 10 seconds.
func WithSamplingInterval(interval time.Duration) CollectorOption {
	return func(c *Collector) {
		c.samplingInterval = interval
	}
}

func WithSeries(name string, period time.Duration, maxCount int) CollectorOption {
	return func(c *Collector) {
		c.series = append(c.series, CollectorSeries{Name: name, Period: period, MaxCount: maxCount})
	}
}

// WithPrefix sets the prefix for all published expvar metrics.
func WithPrefix(prefix string) CollectorOption {
	return func(c *Collector) {
		c.expvarPrefix = prefix
	}
}

// WithInputBuffer sets the size of the input buffer channel.
func WithInputBuffer(size int) CollectorOption {
	return func(c *Collector) {
		c.recvChSize = size
	}
}

func WithStorage(store Storage) CollectorOption {
	return func(c *Collector) {
		c.storage = store
	}
}

// AddOutputFunc adds an output function to the collector.
// The output function will be called with the collected Product.
func (c *Collector) AddOutputFunc(output OutputFunc) {
	c.Lock()
	defer c.Unlock()
	c.outputs = append(c.outputs, output)
}

type Input interface {
	Init() error
	Gather(Gather)
}

type MultipleError []error

var _ error = MultipleError{}

func (me MultipleError) Error() string {
	var sb strings.Builder
	for i, err := range me {
		if i > 0 {
			sb.WriteString("; ")
		}
		sb.WriteString(err.Error())
	}
	return sb.String()
}

func (c *Collector) AddInput(gs ...Input) error {
	var errs MultipleError = nil
	for _, g := range gs {
		if err := c.AddInputFunc(g.Gather); err != nil {
			errs = append(errs, err)
		}
	}
	return errs
}

// AddInputFunc adds an input function to the collector.
func (c *Collector) AddInputFunc(input InputFunc) error {
	// initial call to get the measurement name
	g := &Gatherer{}
	input(g)
	if err := g.Err(); err != nil {
		return err
	}

	c.Lock()
	defer func() {
		c.Unlock()
		ts := nowFunc()
		for _, m := range g.M {
			m.ts = ts
			c.receive(m)
		}
	}()

	for _, m := range g.M {
		if _, exists := c.inputs[m.Name]; exists {
			return fmt.Errorf("input with name %q already registered", m.Name)
		}
		iw := &InputWrapper{
			measureName:    m.Name,
			input:          input,
			mtsFields:      make(map[string]MultiTimeSeries),
			publishedNames: make(map[string]string),
		}
		c.inputs[iw.measureName] = iw
	}
	return nil
}

func (c *Collector) Start() {
	ticker := time.NewTicker(c.samplingInterval)
	c.stopWg.Add(1)
	go func() {
		defer c.stopWg.Done()
		for {
			select {
			case ts := <-ticker.C:
				go c.runInputs(ts)
			case m := <-c.recvCh:
				c.receive(m)
			case <-c.closeCh:
				ticker.Stop()
				// derain the recvCh
				for {
					select {
					case m := <-c.recvCh:
						c.receive(m)
					default:
						return
					}
				}
			}
		}
	}()
}

func (c *Collector) Stop() {
	close(c.closeCh)
	c.stopWg.Wait()
	close(c.recvCh)
	c.syncStorage()
}

func (c *Collector) makePublishName(measureName, fieldName string) string {
	var prefix string
	if c.expvarPrefix != "" {
		prefix = c.expvarPrefix + ":"
	}
	return fmt.Sprintf("%s%s:%s", prefix, measureName, fieldName)
}

// Send processes a measurement sent to the collector.
func (c *Collector) Send(m Measurement) {
	c.recvCh <- m
}

type Gatherer struct {
	C    *Collector
	M    []Measurement
	errs MultipleError
}

func (g *Gatherer) AddMeasurement(m Measurement) {
	g.M = append(g.M, m)
}

func (g *Gatherer) AddError(err error) {
	g.errs = append(g.errs, err)
}

func (g *Gatherer) Err() error {
	if len(g.errs) == 0 {
		return nil
	} else if len(g.errs) == 1 {
		return g.errs[0]
	}
	return g.errs
}

var _ Gather = &Gatherer{}

func (c *Collector) runInputs(ts time.Time) {
	for name, iw := range c.inputs {
		if iw.input == nil {
			measure := Measurement{
				Name:   name,
				Fields: nil,
				ts:     ts,
				noop:   true,
			}
			c.recvCh <- measure
			continue
		} else {
			gather := &Gatherer{}
			iw.input(gather)
			if err := gather.Err(); err != nil {
				fmt.Printf("Error measuring: %v\n", err)
				continue
			}
			for _, measure := range gather.M {
				measure.ts = ts
				c.recvCh <- measure
			}
		}
	}
}

func (c *Collector) receive(m Measurement) {
	c.Lock()
	defer c.Unlock()

	if m.noop {
		if input, ok := c.inputs[m.Name]; ok {
			for _, mts := range input.mtsFields {
				for _, ts := range mts {
					ts.AddTime(m.ts, math.NaN())
				}
			}
		}
		return
	}
	if m.ts.IsZero() {
		m.ts = nowFunc()
	}

	input, ok := c.inputs[m.Name]
	if !ok && len(m.Fields) != 0 {
		input = &InputWrapper{
			measureName:    m.Name,
			mtsFields:      make(map[string]MultiTimeSeries),
			publishedNames: make(map[string]string),
		}
		c.inputs[m.Name] = input
	}
	for _, field := range m.Fields {
		var mts MultiTimeSeries
		if fm, exists := input.mtsFields[field.Name]; exists {
			mts = fm
		} else {
			mts = c.makeMultiTimeSeries(m.Name, field)
			publishName := c.makePublishName(m.Name, field.Name)
			input.mtsFields[field.Name] = mts
			input.publishedNames[field.Name] = publishName
			expvar.Publish(publishName, mts)
		}
		mts.AddTime(m.ts, field.Value)
	}
}

type Product struct {
	Time    time.Time     `json:"ts"`
	Value   Value         `json:"value,omitempty"`
	IsNull  bool          `json:"isNull,omitempty"`
	Measure string        `json:"measure"`
	Field   string        `json:"field"`
	Series  string        `json:"series"`
	Period  time.Duration `json:"period"`
	Type    string        `json:"type"`
	Unit    Unit          `json:"unit"`
}

func (c *Collector) onProduct(tb TimeBin, meta any) {
	if len(c.outputs) == 0 {
		return
	}

	field, ok := meta.(FieldInfo)
	if !ok {
		return
	}

	data := Product{
		Time:    tb.Time,
		Measure: field.Measure,
		Field:   field.Name,
		Series:  field.Series,
		Period:  field.Period,
		Unit:    field.Unit,
		Type:    field.Type,
		IsNull:  tb.IsNull,
		Value:   tb.Value,
	}
	for _, lsnr := range c.outputs {
		lsnr(data)
	}
}

func (c *Collector) makeMultiTimeSeries(measureName string, field Field) MultiTimeSeries {
	mts := make(MultiTimeSeries, len(c.series))
	for i, ser := range c.series {
		var ts = NewTimeSeries(ser.Period, ser.MaxCount, field.Type.Producer())
		ts.SetListener(c.onProduct)
		ts.SetMeta(FieldInfo{
			Measure: measureName,
			Name:    field.Name,
			Series:  ser.Name,
			Period:  ser.Period,
			Type:    field.Type.String(),
			Unit:    field.Type.Unit(),
		})
		if c.storage != nil {
			seriesName := cleanPath(ts.interval.String())
			if data, err := c.storage.Load(measureName, field.Name, seriesName); err != nil {
				fmt.Printf("Failed to load time series for %s %s %s: %v\n", measureName, field.Name, ser.Name, err)
			} else if data != nil {
				// if file is not exists, data will be nil
				ts.data = data.data
			}
		}
		mts[i] = ts
	}
	return mts
}

func (c *Collector) SamplingInterval() time.Duration {
	return c.samplingInterval
}

// PublishNames returns a list of all published metric names in the collector.
func (c *Collector) PublishNames() []string {
	c.Lock()
	defer c.Unlock()
	names := make([]string, 0, len(c.inputs))
	for _, iw := range c.inputs {
		for _, publishedName := range iw.publishedNames {
			names = append(names, publishedName)
		}
	}
	return names
}

func (c *Collector) MetricNames() []string {
	c.Lock()
	defer c.Unlock()
	names := make([]string, 0, len(c.inputs))
	for _, iw := range c.inputs {
		for _, publishedName := range iw.publishedNames {
			if c.expvarPrefix != "" {
				names = append(names, strings.TrimPrefix(publishedName, c.expvarPrefix+":"))
			} else {
				names = append(names, publishedName)
			}
		}
	}
	return names
}

func (c *Collector) Series() []CollectorSeries {
	c.Lock()
	defer c.Unlock()
	ret := make([]CollectorSeries, len(c.series))
	copy(ret, c.series)
	return ret
}

// Inflight returns the current collecting data for each series of the specified measure and field.
// The key of the returned map is the series name.
// If the measure or field does not exist, it returns ErrMetricNotFound.
func (c *Collector) Inflight(measure string, field string) (map[string]Product, error) {
	return c.InflightName(c.makePublishName(measure, field))
}

// InflightName returns the current collecting data for each series of the specified published metric name.
// The key of the returned map is the series name.
// If the metric does not exist, it returns ErrMetricNotFound.
func (c *Collector) InflightName(metricName string) (map[string]Product, error) {
	var mts MultiTimeSeries
	if ev := expvar.Get(metricName); ev != nil {
		if m, ok := ev.(MultiTimeSeries); !ok {
			return nil, fmt.Errorf("metric %s is not a Metric, but %T", metricName, ev)
		} else {
			mts = m
		}
	}
	if mts == nil {
		return nil, ErrMetricNotFound
	}

	ret := map[string]Product{}
	for idx, n := range c.series {
		seriesName := n.Name
		nfo, ok := mts[idx].Meta().(FieldInfo)
		if !ok {
			return nil, fmt.Errorf("metric %s series %s meta is not FieldInfo, but %T",
				metricName, seriesName, mts[idx].Meta())
		}
		ts, prd := mts[idx].Last()
		ret[seriesName] = Product{
			Time:    ts,
			Value:   prd,
			IsNull:  prd == nil,
			Measure: nfo.Measure,
			Field:   nfo.Name,
			Series:  nfo.Series,
			Type:    nfo.Type,
			Unit:    nfo.Unit,
		}
	}
	return ret, nil
}

func (c *Collector) syncStorage() {
	if c.storage == nil {
		return
	}
	c.Lock()
	defer c.Unlock()
	for measureName, iw := range c.inputs {
		for fieldName, mts := range iw.mtsFields {
			for _, ts := range mts {
				seriesName := cleanPath(ts.interval.String())
				err := c.storage.Store(measureName, fieldName, seriesName, ts)
				if err != nil {
					fmt.Printf("Failed to store time series for %s %s %s: %v\n", measureName, fieldName, seriesName, err)
				}
			}
		}
	}
}

var ErrMetricNotFound = errors.New("metric not found")
