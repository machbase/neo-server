package metric

import (
	"errors"
	"expvar"
	"fmt"
	"sync"
	"time"
)

// InputFunc is a function type that matches the signature of the Collect method.
// Periodically called by the Collector to gather metrics.
type InputFunc func() (Measurement, error)

// OutputFunc is a function type that processes the collected ProductData.
type OutputFunc func(Product)

type Measurement struct {
	Name   string
	Fields []Field // name -value pairs and producer function
	// fields not exposed
	ts     time.Time // time when the measurement was taken
	doSync bool      // whether to sync storage after this measurement
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

func HistogramType(u Unit) Type {
	return HistogramTypePercentiles(u, 100, 0.5, 0.90, 0.99)
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
	interval time.Duration
	closeCh  chan struct{}
	stopWg   sync.WaitGroup

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
	name     string
	period   time.Duration
	maxCount int
}

// NewCollector creates a new Collector with the specified interval.
// The interval determines how often the inputs will be collected.
// The collector will run until Stop() is called.
// It is safe to call Start() multiple times, but Stop() should be called only once
func NewCollector(opts ...CollectorOption) *Collector {
	c := &Collector{
		interval: 10 * time.Second,
		closeCh:  make(chan struct{}),
		inputs:   make(map[string]*InputWrapper),
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

func WithInterval(interval time.Duration) CollectorOption {
	return func(c *Collector) {
		c.interval = interval
	}
}

func WithSeries(name string, period time.Duration, maxCount int) CollectorOption {
	return func(c *Collector) {
		c.series = append(c.series, CollectorSeries{name: name, period: period, maxCount: maxCount})
	}
}

func WithPrefix(prefix string) CollectorOption {
	return func(c *Collector) {
		c.expvarPrefix = prefix
	}
}

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

func (c *Collector) AddOutputFunc(output OutputFunc) {
	c.Lock()
	defer c.Unlock()
	c.outputs = append(c.outputs, output)
}

func (c *Collector) AddInputFunc(input InputFunc) error {
	// initial call to get the measurement name
	m, err := input()
	if err != nil {
		return err
	}

	c.Lock()
	defer func() {
		c.Unlock()
		c.receive(m)
	}()

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
	return nil
}

func (c *Collector) Start() {
	ticker := time.NewTicker(c.interval)
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

func (c *Collector) runInputs(ts time.Time) {
	for _, iw := range c.inputs {
		if iw.input == nil {
			continue
		}
		measure, err := iw.input()
		if err != nil {
			fmt.Printf("Error measuring: %v\n", err)
			continue
		}
		measure.ts = ts
		c.recvCh <- measure
	}
	// trigger storage sync
	c.recvCh <- Measurement{doSync: true}
}

func (c *Collector) receive(m Measurement) {
	c.Lock()
	defer c.Unlock()

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
	if m.doSync {
		c.syncStorage()
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
		var ts = NewTimeSeries(ser.period, ser.maxCount, field.Type.Producer())
		ts.SetListener(c.onProduct)
		ts.SetMeta(FieldInfo{
			Measure: measureName,
			Name:    field.Name,
			Series:  ser.name,
			Period:  ser.period,
			Type:    field.Type.String(),
			Unit:    field.Type.Unit(),
		})
		if c.storage != nil {
			seriesName := cleanPath(ts.interval.String())
			if data, err := c.storage.Load(measureName, field.Name, seriesName); err != nil {
				fmt.Printf("Failed to load time series for %s %s %s: %v\n", measureName, field.Name, ser.name, err)
			} else if data != nil {
				// if file is not exists, data will be nil
				ts.data = data.data
			}
		}
		mts[i] = ts
	}
	return mts
}

// Names returns a list of all published metric names in the collector.
func (c *Collector) Names() []string {
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
		seriesName := n.name
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
