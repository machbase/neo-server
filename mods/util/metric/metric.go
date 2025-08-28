package metric

import (
	"errors"
	"expvar"
	"fmt"
	"sync"
	"time"
)

// Input represents a metric input source that provides measurements via its Collect method.
// Periodically called by the Collector to gather metrics.
type Input interface {
	Collect() (Measurement, error)
}

// InputFunc is a function type that matches the signature of the Collect method.
// Periodically called by the Collector to gather metrics.
type InputFunc func() (Measurement, error)

type Measurement struct {
	Name   string
	Fields []Field // name -value pairs and producer function
}

func (m *Measurement) AddField(f ...Field) {
	m.Fields = append(m.Fields, f...)
}

type Field struct {
	Name  string
	Value float64
	Type  Type
}

func CounterType(u Unit) Type {
	return Type{
		p: func() Producer { return NewCounter() },
		s: "counter",
		u: u,
	}
}

func GaugeType(u Unit) Type {
	return Type{
		p: func() Producer { return NewGauge() },
		s: "gauge",
		u: u,
	}
}
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

type EmitterWrapper struct {
	measureName    string
	mtsFields      map[string]MultiTimeSeries
	publishedNames map[string]string
}

type Collector struct {
	sync.Mutex
	// periodically collects metrics from inputs
	inputs   []*InputWrapper
	interval time.Duration
	closeCh  chan struct{}
	stopWg   sync.WaitGroup

	// event-driven emitters
	emitters   map[string]*EmitterWrapper
	recvCh     chan Measurement
	recvChSize int

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
		emitters: make(map[string]*EmitterWrapper),
	}
	for _, opt := range opts {
		opt(c)
	}
	if c.recvChSize <= 0 {
		c.recvChSize = 100
	}
	c.recvCh = make(chan Measurement, c.recvChSize)
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

type ProducedData struct {
	Time    time.Time `json:"ts"`
	Value   Product   `json:"value,omitempty"`
	IsNull  bool      `json:"isNull,omitempty"`
	Measure string    `json:"measure"`
	Field   string    `json:"field"`
	Series  string    `json:"series"`
	Type    string    `json:"type"`
	Unit    Unit      `json:"unit"`
}

func WithSeriesListener(name string, period time.Duration, maxCount int, lsnr func(ProducedData)) CollectorOption {
	return func(c *Collector) {
		var productLsnr func(tb TimeBin, meta any)
		if lsnr != nil {
			productLsnr = func(tb TimeBin, meta any) {
				field, ok := meta.(FieldInfo)
				if !ok {
					return
				}
				data := ProducedData{
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

func (c *Collector) AddInput(input Input) {
	c.AddInputFunc(input.Collect)
}

func (c *Collector) AddInputFunc(input InputFunc) {
	c.Lock()
	defer c.Unlock()
	iw := &InputWrapper{
		input:          input,
		mtsFields:      make(map[string]MultiTimeSeries),
		publishedNames: make(map[string]string),
	}
	c.inputs = append(c.inputs, iw)
}

func (c *Collector) Start() {
	ticker := time.NewTicker(c.interval)
	c.stopWg.Add(1)
	go func() {
		defer c.stopWg.Done()
		c.runInputs(nowFunc()) // initial run
		for {
			select {
			case <-ticker.C:
				tm := nowFunc()
				c.runInputs(tm)
				c.syncStorage()
			case m := <-c.recvCh:
				c.receive(m)
			case <-c.closeCh:
				ticker.Stop()
				return
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

func (c *Collector) runInputs(tm time.Time) {
	c.Lock()
	defer c.Unlock()

	for _, iw := range c.inputs {
		measure, err := iw.input()
		if err != nil {
			fmt.Printf("Error measuring: %v\n", err)
			continue
		}
		if iw.measureName == "" {
			iw.measureName = measure.Name
		}
		for _, field := range measure.Fields {
			var mts MultiTimeSeries
			if fm, exists := iw.mtsFields[field.Name]; exists {
				mts = fm
			} else {
				mts = c.makeMultiTimeSeries(measure.Name, field)
				iw.mtsFields[field.Name] = mts

				publishName := c.makePublishName(measure.Name, field.Name)
				expvar.Publish(publishName, mts)
				iw.publishedNames[field.Name] = publishName
			}
			mts.AddTime(tm, field.Value)
		}
	}
}

func (c *Collector) makePublishName(measureName, fieldName string) string {
	var prefix string
	if c.expvarPrefix != "" {
		prefix = c.expvarPrefix + ":"
	}
	return fmt.Sprintf("%s%s:%s", prefix, measureName, fieldName)
}

// EventChannel returns a channel to which measurements can be sent.
// This channel is used by emitters to send measurements to the collector.
func (c *Collector) EventChannel() chan<- Measurement {
	return c.recvCh
}

// SendEvent processes a measurement sent to the collector.
func (c *Collector) SendEvent(m Measurement) {
	c.recvCh <- m
}

func (c *Collector) receive(m Measurement) {
	c.Lock()
	defer c.Unlock()

	now := nowFunc()

	emit, ok := c.emitters[m.Name]
	if !ok {
		emit = &EmitterWrapper{
			measureName:    m.Name,
			mtsFields:      make(map[string]MultiTimeSeries),
			publishedNames: make(map[string]string),
		}
		c.emitters[m.Name] = emit
	}
	for _, field := range m.Fields {
		var mts MultiTimeSeries
		if fm, exists := emit.mtsFields[field.Name]; exists {
			mts = fm
		} else {
			mts = c.makeMultiTimeSeries(m.Name, field)
			emit.mtsFields[field.Name] = mts

			publishName := c.makePublishName(m.Name, field.Name)
			expvar.Publish(publishName, MultiTimeSeries(mts))
			emit.publishedNames[field.Name] = publishName
		}
		mts.AddTime(now, field.Value)
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

func (c *Collector) Snapshot(metricName string, seriesName string) (*Snapshot, error) {
	idx := -1
	for i, n := range c.series {
		if n.name == seriesName {
			idx = i
			break
		}
	}
	if idx < 0 {
		return nil, ErrMetricNotFound
	}
	return snapshot(metricName, 0)
}

func (c *Collector) syncStorage() {
	if c.storage == nil {
		return
	}
	func() {
		c.Lock()
		defer c.Unlock()
		for _, iw := range c.inputs {
			for fieldName, mts := range iw.mtsFields {
				for _, ts := range mts {
					seriesName := cleanPath(ts.interval.String())
					err := c.storage.Store(iw.measureName, fieldName, seriesName, ts)
					if err != nil {
						fmt.Printf("Failed to store time series for %s %s %s: %v\n", iw.measureName, fieldName, seriesName, err)
					}
				}
			}
		}
	}()

	func() {
		c.Lock()
		defer c.Unlock()
		for measureName, emit := range c.emitters {
			for fieldName, mts := range emit.mtsFields {
				for _, ts := range mts {
					seriesName := cleanPath(ts.interval.String())
					err := c.storage.Store(measureName, fieldName, seriesName, ts)
					if err != nil {
						fmt.Printf("Failed to store time series for %s %s %s: %v\n", measureName, fieldName, seriesName, err)
					}
				}
			}
		}
	}()
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
