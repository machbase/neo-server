package metric

import (
	"errors"
	"expvar"
	"fmt"
	"log/slog"
	"math"
	"strings"
	"sync"
	"time"
)

// InputFunc is a function type that matches the signature of the Collect method.
// Periodically called by the Collector to gather metrics.
type InputFunc func(*Gather) error

// OutputFunc is a function type that processes the collected ProductData.
type OutputFunc func(Product) error

type Gather struct {
	measures []Measure
	ts       time.Time
	noop     bool
}

func (g *Gather) Add(name string, value float64, typ Type) {
	g.measures = append(g.measures, Measure{Name: name, Value: value, Type: typ})
}

func (g *Gather) Filter(filter Filter) {
	var ms []Measure
	for _, f := range g.measures {
		if filter == nil || filter.Match(f.Name) {
			ms = append(ms, f)
		}
	}
	g.measures = ms
}

type Measure struct {
	Name  string
	Value float64
	Type  Type
}

type SeriesInfo struct {
	MeasureName string   `json:"measure_name"`
	MeasureType Type     `json:"measure_type"`
	SeriesID    SeriesID `json:"series_id"`
}

func (si *SeriesInfo) H() map[string]any {
	if si == nil {
		return nil
	}
	return H{
		"name":         si.MeasureName,
		"series_id":    si.SeriesID.ID(),
		"series_title": si.SeriesID.Title(),
		"period":       si.SeriesID.period.String(),
		"max_count":    si.SeriesID.maxCount,
		"unit":         si.MeasureType.Unit(),
		"type":         si.MeasureType.Name(),
	}
}

type Collector struct {
	sync.Mutex

	inputs     []Input                    // registered input
	outputs    []Output                   // registered output
	timeseries map[string]MultiTimeSeries // measurement_name: multi-timeseries

	// only data that match the filter will be stored
	timeseriesFilter Filter

	// periodically collects metrics from inputs
	samplingInterval time.Duration
	closeCh          chan struct{}
	stopWg           sync.WaitGroup

	// event-driven measurements
	recvCh     chan *Gather
	recvChSize int
	// a channel to which measurements can be sent.
	C chan<- *Gather

	// time series configuration
	series       []SeriesID
	expvarPrefix string

	// persistent storage
	storage Storage
}

// NewCollector creates a new Collector with the specified interval.
// The interval determines how often the inputs will be collected.
// The collector will run until Stop() is called.
// It is safe to call Start() multiple times, but Stop() should be called only once
func NewCollector(opts ...CollectorOption) *Collector {
	c := &Collector{
		samplingInterval: 10 * time.Second,
		closeCh:          make(chan struct{}),
		timeseries:       make(map[string]MultiTimeSeries),
	}
	for _, opt := range opts {
		opt(c)
	}
	if c.recvChSize <= 0 {
		c.recvChSize = 100
	}
	c.recvCh = make(chan *Gather, c.recvChSize)
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

func WithSeries(seriesID ...SeriesID) CollectorOption {
	return func(c *Collector) {
		c.series = append(c.series, seriesID...)
	}
}

func WithTimeseriesFilter(filter Filter) CollectorOption {
	return func(c *Collector) {
		c.timeseriesFilter = filter
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

type Input interface {
	Gather(*Gather) error
}

type Output interface {
	Process(Product) error
}

type FilterInput struct {
	Filter Filter
	Input  Input
}

func (fi *FilterInput) Init() error {
	if hasInit, ok := fi.Input.(interface{ Init() error }); ok {
		return hasInit.Init()
	}
	return nil
}

func (fi *FilterInput) Gather(g *Gather) error {
	err := fi.Input.Gather(g)
	if err != nil {
		return err
	}
	g.Filter(fi.Filter)
	return nil
}

func (fi *FilterInput) DeInit() {
	if hasDeInit, ok := fi.Input.(interface{ DeInit() }); ok {
		hasDeInit.DeInit()
	}
}

type FilterOutput struct {
	Filter Filter
	Output Output
}

func (fo *FilterOutput) Init() error {
	if hasInit, ok := fo.Output.(interface{ Init() error }); ok {
		return hasInit.Init()
	}
	return nil
}

func (fo *FilterOutput) Process(p Product) error {
	if fo.Filter != nil && !fo.Filter.Match(p.Name) {
		return nil
	}
	return fo.Output.Process(p)
}

func (fo *FilterOutput) DeInit() {
	if hasDeInit, ok := fo.Output.(interface{ DeInit() }); ok {
		hasDeInit.DeInit()
	}
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

func (c *Collector) AddOutput(outputs ...Output) error {
	var errs MultipleError
	c.Lock()
	defer c.Unlock()
	for _, out := range outputs {
		if hasInit, ok := out.(interface{ Init() error }); ok {
			if err := hasInit.Init(); err != nil {
				errs = append(errs, err)
				continue
			}
		}
		c.outputs = append(c.outputs, out)
	}
	if len(errs) > 0 {
		return errs
	}
	return nil
}

type OutputFuncWrapper struct {
	f OutputFunc
}

func (ow *OutputFuncWrapper) Process(p Product) error {
	return ow.f(p)
}

// AddOutputFunc adds an output function to the collector.
// The output function will be called with the collected Product.
func (c *Collector) AddOutputFunc(output OutputFunc) {
	c.outputs = append(c.outputs, &OutputFuncWrapper{output})
}

func (c *Collector) AddInput(inputs ...Input) error {
	var errs MultipleError
	var initialGathers []*Gather
	c.Lock()
	ts := nowFunc()
	defer func() {
		c.Unlock()
		for _, g := range initialGathers {
			g.ts = ts
			c.receive(g)
		}
	}()
	for _, input := range inputs {
		if hasInit, ok := input.(interface{ Init() error }); ok {
			if err := hasInit.Init(); err != nil {
				errs = append(errs, err)
				continue
			}
		}
		// the first call to get the measurement name
		g := &Gather{}
		if err := input.Gather(g); err != nil {
			errs = append(errs, err)
			continue
		}
		initialGathers = append(initialGathers, g)
		c.inputs = append(c.inputs, input)
	}
	if len(errs) > 0 {
		return errs
	}
	return nil
}

type InputFuncWrapper struct {
	f InputFunc
}

func (iw *InputFuncWrapper) Gather(g *Gather) error {
	return iw.f(g)
}

// AddInputFunc adds an input function to the collector.
func (c *Collector) AddInputFunc(input InputFunc) error {
	return c.AddInput(&InputFuncWrapper{f: input})
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
	// call DeInit() of inputs if exists
	for _, input := range c.inputs {
		if hasDeInit, ok := input.(interface{ DeInit() error }); ok {
			hasDeInit.DeInit()
		}
	}
	// call DeInit() of outputs if exists
	for _, out := range c.outputs {
		if hasDeInit, ok := out.(interface{ DeInit() }); ok {
			hasDeInit.DeInit()
		}
	}
}

func (c *Collector) makePublishName(metricName string) string {
	var prefix string
	if c.expvarPrefix != "" {
		prefix = c.expvarPrefix + ":"
	}
	return fmt.Sprintf("%s%s", prefix, metricName)
}

// Send processes a measurement sent to the collector.
func (c *Collector) Send(measurements ...Measure) {
	g := &Gather{
		measures: measurements,
		ts:       nowFunc(),
	}
	c.recvCh <- g
}

func (c *Collector) runInputs(ts time.Time) {
	// there are chances that recvCh is already closed
	// because of Stop() has been called.
	// so we need to recover from panic.
	defer func() {
		if r := recover(); r != nil {
			slog.Error("Recovered in runInputs", "error", r)
		}
	}()

	for _, input := range c.inputs {
		gather := &Gather{}
		if err := input.Gather(gather); err != nil {
			slog.Error("Error gathering metrics", "error", err)
			continue
		}
		gather.ts = ts
		c.recvCh <- gather
	}
	c.recvCh <- &Gather{noop: true, ts: ts}
}

func (c *Collector) receive(m *Gather) {
	c.Lock()
	defer c.Unlock()

	if m.ts.IsZero() {
		m.ts = nowFunc()
	}

	if m.noop {
		nan := math.NaN()
		for _, mts := range c.timeseries {
			for _, ts := range mts {
				ts.AddTime(m.ts, nan)
			}
		}
		return
	}

	for _, measure := range m.measures {
		if c.timeseriesFilter != nil && !c.timeseriesFilter.Match(measure.Name) {
			continue
		}
		var mts MultiTimeSeries
		if fm, exists := c.timeseries[measure.Name]; exists {
			mts = fm
		} else {
			mts = c.makeMultiTimeSeries(measure)
			c.timeseries[measure.Name] = mts
			publishName := c.makePublishName(measure.Name)
			expvar.Publish(publishName, mts)
		}
		mts.AddTime(m.ts, measure.Value)
	}
}

type Product struct {
	Name        string        `json:"name"`
	Time        time.Time     `json:"ts"`
	Value       Value         `json:"value,omitempty"`
	IsNull      bool          `json:"isNull,omitempty"`
	SeriesID    string        `json:"series_id,omitempty"`
	SeriesTitle string        `json:"series_title,omitempty"`
	Period      time.Duration `json:"period,omitempty"`
	Type        string        `json:"type,omitempty"`
	Unit        Unit          `json:"unit,omitempty"`
}

func (c *Collector) onProduct(prd Product) {
	for _, out := range c.outputs {
		if err := out.Process(prd); err != nil {
			slog.Error("Error processing output", "name", prd.Name, "error", err)
		}
	}
	// Store to storage
	if c.storage != nil {
		for _, series := range c.series {
			if series.ID() == prd.SeriesID {
				if err := c.storage.Store(series, prd, false); err != nil {
					slog.Error("Error storing metric", "name", prd.Name, "error", err)
				}
				break
			}
		}
	}
}

func (c *Collector) makeMultiTimeSeries(measure Measure) MultiTimeSeries {
	mts := make(MultiTimeSeries, len(c.series))
	for i, ser := range c.series {
		var ts = NewTimeSeries(ser.Period(), ser.MaxCount(), measure.Type.Producer(),
			WithListener(c.onProduct),
			WithMeta(SeriesInfo{
				MeasureName: measure.Name,
				MeasureType: measure.Type,
				SeriesID:    ser,
			}),
		)
		if c.storage != nil {
			if err := ts.Restore(c.storage, measure.Name, ser); err != nil {
				slog.Error("Failed to restore time series", "measure", measure.Name, "series", ser.ID(), "error", err)
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
	prefix := ""
	if c.expvarPrefix != "" {
		prefix = c.expvarPrefix + ":"
	}
	for name := range c.timeseries {
		names = append(names, prefix+name)
	}
	return names
}

func (c *Collector) MetricNames() []string {
	c.Lock()
	defer c.Unlock()
	names := make([]string, 0, len(c.inputs))
	for name := range c.timeseries {
		names = append(names, name)
	}
	return names
}

// Timeseries returns the MultiTimeSeries for the specified measurement name.
// If the measurement does not exist, it returns nil.
func (c *Collector) Timeseries(name string) MultiTimeSeries {
	c.Lock()
	defer c.Unlock()
	return c.timeseries[name]
}

func (c *Collector) Series() []SeriesID {
	c.Lock()
	defer c.Unlock()
	ret := make([]SeriesID, len(c.series))
	copy(ret, c.series)
	return ret
}

// Inflight returns the current collecting data for each series of the specified measurement.
// The key of the returned map is the series id.
// If the measurement does not exist, it returns ErrMetricNotFound.
func (c *Collector) Inflight(measureName string) (map[string]Product, error) {
	var mts MultiTimeSeries
	if m, ok := c.timeseries[measureName]; !ok {
		return nil, ErrMetricNotFound
	} else {
		mts = m
	}

	ret := map[string]Product{}
	for idx, n := range c.series {
		seriesID := n.ID()
		nfo, ok := mts[idx].Meta().(SeriesInfo)
		if !ok {
			return nil, fmt.Errorf("metric %s series %s meta is not MeasurementInfo, but %T",
				measureName, seriesID, mts[idx].Meta())
		}
		ts, prd := mts[idx].Last()
		ret[seriesID] = Product{
			Name:        nfo.MeasureName,
			Time:        ts,
			Value:       prd,
			IsNull:      prd == nil,
			SeriesID:    nfo.SeriesID.ID(),
			SeriesTitle: nfo.SeriesID.Title(),
			Period:      nfo.SeriesID.Period(),
			Type:        nfo.MeasureType.Name(),
			Unit:        nfo.MeasureType.Unit(),
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
	for _, mts := range c.timeseries {
		for _, ts := range mts {
			tb, meta := ts.LastBin()
			var prd Product
			ToProduct(&prd, tb, meta)
			id, err := NewSeriesID(prd.SeriesID, prd.Name, prd.Period, ts.maxCount)
			if err != nil {
				slog.Error("Failed to create series ID", "ID", prd.SeriesID, "error", err)
				continue
			}
			if err := c.storage.Store(id, prd, true); err != nil {
				slog.Error("Failed to store time series", "ID", id.ID(), "error", err)
			}
		}
	}
}

var ErrMetricNotFound = errors.New("metric not found")
