package viz

const Version1 = 1

const (
	DomainKindTime     = "time"
	DomainKindNumeric  = "numeric"
	DomainKindCategory = "category"
)

const (
	TimeformatRFC3339 = "rfc3339"
	TimeformatSecond  = "s"
	TimeformatMilli   = "ms"
	TimeformatMicro   = "us"
	TimeformatNano    = "ns"
)

const (
	AxisTypeTime     = "time"
	AxisTypeLinear   = "linear"
	AxisTypeLog      = "log"
	AxisTypeCategory = "category"
)

const (
	RepresentationRawPoint              = "raw-point"
	RepresentationTimeBucketValue       = "time-bucket-value"
	RepresentationTimeBucketBand        = "time-bucket-band"
	RepresentationDistributionHistogram = "distribution-histogram"
	RepresentationDistributionBoxplot   = "distribution-boxplot"
	RepresentationEventPoint            = "event-point"
	RepresentationEventRange            = "event-range"
)

const (
	SourceKindRaw     = "raw"
	SourceKindRollup  = "rollup"
	SourceKindSampled = "sampled"
	SourceKindDerived = "derived"
)

const (
	AnnotationKindPoint = "point"
	AnnotationKindLine  = "line"
	AnnotationKindRange = "range"
)

type Spec struct {
	Version     int          `json:"version"`
	Domain      Domain       `json:"domain,omitempty"`
	Axes        Axes         `json:"axes,omitempty"`
	Series      []Series     `json:"series,omitempty"`
	Annotations []Annotation `json:"annotations,omitempty"`
	View        View         `json:"view,omitempty"`
	Meta        Meta         `json:"meta,omitempty"`
}

type Domain struct {
	Kind       string   `json:"kind,omitempty"`
	From       any      `json:"from,omitempty"`
	To         any      `json:"to,omitempty"`
	Timeformat string   `json:"timeformat,omitempty"`
	TZ         string   `json:"tz,omitempty"`
	Categories []string `json:"categories,omitempty"`
}

type Axes struct {
	X Axis   `json:"x,omitempty"`
	Y []Axis `json:"y,omitempty"`
}

type Axis struct {
	ID     string  `json:"id,omitempty"`
	Type   string  `json:"type,omitempty"`
	Unit   string  `json:"unit,omitempty"`
	Label  string  `json:"label,omitempty"`
	TZ     string  `json:"tz,omitempty"`
	Extent *Extent `json:"extent,omitempty"`
}

type Extent struct {
	Min any `json:"min,omitempty"`
	Max any `json:"max,omitempty"`
}

type Series struct {
	ID             string         `json:"id,omitempty"`
	Name           string         `json:"name,omitempty"`
	Axis           string         `json:"axis,omitempty"`
	Representation Representation `json:"representation,omitempty"`
	Source         Source         `json:"source,omitempty"`
	Data           []any          `json:"data,omitempty"`
	Quality        Quality        `json:"quality,omitempty"`
	Style          map[string]any `json:"style,omitempty"`
	Extra          map[string]any `json:"extra,omitempty"`
}

type Representation struct {
	Kind          string   `json:"kind,omitempty"`
	BucketWidth   string   `json:"bucketWidth,omitempty"`
	Aggregation   string   `json:"aggregation,omitempty"`
	Fields        []string `json:"fields,omitempty"`
	OutlierFields []string `json:"outlierFields,omitempty"`
}

type Source struct {
	Kind        string `json:"kind,omitempty"`
	Table       string `json:"table,omitempty"`
	Query       string `json:"query,omitempty"`
	Resolution  string `json:"resolution,omitempty"`
	DerivedFrom string `json:"derivedFrom,omitempty"`
}

type Quality struct {
	Sampled          bool    `json:"sampled,omitempty"`
	Coverage         float64 `json:"coverage,omitempty"`
	RowCount         int     `json:"rowCount,omitempty"`
	EstimatedPoints  int64   `json:"estimatedPoints,omitempty"`
	DownsamplePolicy string  `json:"downsamplePolicy,omitempty"`
}

type Annotation struct {
	Kind  string         `json:"kind,omitempty"`
	Axis  string         `json:"axis,omitempty"`
	At    any            `json:"at,omitempty"`
	From  any            `json:"from,omitempty"`
	To    any            `json:"to,omitempty"`
	Value any            `json:"value,omitempty"`
	Label string         `json:"label,omitempty"`
	Style map[string]any `json:"style,omitempty"`
}

type View struct {
	DefaultZoom       []float64 `json:"defaultZoom,omitempty"`
	PreferredRenderer string    `json:"preferredRenderer,omitempty"`
}

type Meta struct {
	Producer    string `json:"producer,omitempty"`
	GeneratedAt string `json:"generatedAt,omitempty"`
	LODGroup    string `json:"lodGroup,omitempty"`
}

func (spec *Spec) Normalize() *Spec {
	if spec == nil {
		spec = &Spec{}
	}
	if spec.Version == 0 {
		spec.Version = Version1
	}
	if spec.Series == nil {
		spec.Series = []Series{}
	}
	if spec.Annotations == nil {
		spec.Annotations = []Annotation{}
	}
	if spec.Axes.Y == nil {
		spec.Axes.Y = []Axis{}
	}
	return spec
}
