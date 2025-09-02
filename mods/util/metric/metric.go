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

func HistogramType(u Unit, maxBin int, ps ...float64) Type {
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
	inputs map[string]*InputWrapper

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
	lsnr     func(TimeBin, any)
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

func WithCollectInterval(interval time.Duration) CollectorOption {
	return func(c *Collector) {
		c.interval = interval
	}
}

func WithSeries(name string, period time.Duration, maxCount int) CollectorOption {
	return WithSeriesListener(name, period, maxCount, nil)
}

type ProductData struct {
	Time    time.Time `json:"ts"`
	Value   Product   `json:"value,omitempty"`
	IsNull  bool      `json:"isNull,omitempty"`
	Measure string    `json:"measure"`
	Field   string    `json:"field"`
	Series  string    `json:"series"`
	Type    string    `json:"type"`
	Unit    Unit      `json:"unit"`
}

func WithSeriesListener(name string, period time.Duration, maxCount int, lsnr func(ProductData)) CollectorOption {
	return func(c *Collector) {
		var productLsnr func(tb TimeBin, meta any)
		if lsnr != nil {
			productLsnr = func(tb TimeBin, meta any) {
				field, ok := meta.(FieldInfo)
				if !ok {
					return
				}
				data := ProductData{
					Time:    tb.Time,
					Measure: field.Measure,
					Field:   field.Name,
					Series:  field.Series,
					Unit:    field.Unit,
					Type:    field.Type,
					IsNull:  tb.IsNull,
					Value:   tb.Value,
				}
				lsnr(data)
			}
		}
		c.series = append(c.series, CollectorSeries{name: name, period: period, maxCount: maxCount, lsnr: productLsnr})
	}
}

func WithExpvarPrefix(prefix string) CollectorOption {
	return func(c *Collector) {
		c.expvarPrefix = prefix
	}
}

func WithReceiverSize(size int) CollectorOption {
	return func(c *Collector) {
		c.recvChSize = size
	}
}

func WithStorage(store Storage) CollectorOption {
	return func(c *Collector) {
		c.storage = store
	}
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

func (c *Collector) makeMultiTimeSeries(measureName string, field Field) MultiTimeSeries {
	mts := make(MultiTimeSeries, len(c.series))
	for i, ser := range c.series {
		var ts = NewTimeSeries(ser.period, ser.maxCount, field.Type.Producer())
		ts.SetListener(ser.lsnr)
		ts.SetMeta(FieldInfo{
			Measure: measureName,
			Name:    field.Name,
			Series:  ser.name,
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
func (c *Collector) Inflight(measure string, field string) (map[string]ProductData, error) {
	return c.InflightName(c.makePublishName(measure, field))
}

// InflightName returns the current collecting data for each series of the specified published metric name.
// The key of the returned map is the series name.
// If the metric does not exist, it returns ErrMetricNotFound.
func (c *Collector) InflightName(metricName string) (map[string]ProductData, error) {
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

	ret := map[string]ProductData{}
	for idx, n := range c.series {
		seriesName := n.name
		nfo, ok := mts[idx].Meta().(FieldInfo)
		if !ok {
			return nil, fmt.Errorf("metric %s series %s meta is not FieldInfo, but %T",
				metricName, seriesName, mts[idx].Meta())
		}
		ts, prd := mts[idx].Last()
		ret[seriesName] = ProductData{
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

type Output interface {
	Export(name string, data *Snapshot) error
}

type OutputWrapper struct {
	output Output
	filter func(string) bool
}

type Exporter struct {
	sync.Mutex
	ows       []OutputWrapper
	metrics   []string
	interval  time.Duration
	closeCh   chan struct{}
	latestErr error
}

func NewExporter(interval time.Duration, metrics []string) *Exporter {
	return &Exporter{
		interval: interval,
		metrics:  metrics,
		closeCh:  make(chan struct{}),
	}
}

func (s *Exporter) AddOutput(output Output, filter any) {
	s.Lock()
	defer s.Unlock()
	ow := OutputWrapper{
		output: output,
		filter: func(string) bool { return true }, // Default filter allows all metrics
	}
	s.ows = append(s.ows, ow)
}

func (s *Exporter) Start() {
	ticker := time.NewTicker(s.interval)
	go func() {
		for {
			select {
			case <-s.closeCh:
				ticker.Stop()
				return
			case <-ticker.C:
				if err := s.exportAll(0); err != nil {
					s.latestErr = err
				}
			}
		}
	}()
}

func (s *Exporter) Stop() {
	if s.closeCh == nil {
		return
	}
	close(s.closeCh)
	s.closeCh = nil
}

// Err returns the latest error encountered during export.
// If no error has occurred, it returns nil.
func (s *Exporter) Err() error {
	return s.latestErr
}

func (s *Exporter) exportAll(tsIdx int) error {
	for _, metricName := range s.metrics {
		if err := s.Export(metricName, tsIdx); err != nil {
			return err
		}
	}
	return nil
}

func (s *Exporter) Export(metricName string, tsIdx int) error {
	var ss *Snapshot
	var name string
	var data *Snapshot
	for _, ow := range s.ows {
		if !ow.filter(metricName) {
			continue
		}
		if ss == nil {
			var err error
			ss, err = snapshot(metricName, tsIdx)
			if err != nil {
				return err
			}
			if ss == nil || len(ss.Values) == 0 {
				// If the metric is nil or has no values, skip
				break
			}
			name = fmt.Sprintf("%s:%d", metricName, tsIdx)
			data = ss
		}
		if err := ow.output.Export(name, data); err != nil {
			return err
		}
	}
	return nil
}

func snapshot(metricName string, idx int) (*Snapshot, error) {
	if ev := expvar.Get(metricName); ev != nil {
		mts, ok := ev.(MultiTimeSeries)
		if !ok {
			return nil, fmt.Errorf("metric %s is not a Metric, but %T", metricName, ev)
		}
		if idx < 0 || idx >= len(mts) {
			return nil, fmt.Errorf("index %d out of range for metric %s with %d time series",
				idx, metricName, len(mts))
		}
		return mts[idx].Snapshot(), nil
	}
	return nil, ErrMetricNotFound
}

var ErrMetricNotFound = errors.New("metric not found")
