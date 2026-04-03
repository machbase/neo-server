package advn

import (
	"bytes"
	"encoding/json"
)

type encodedDomain struct {
	Kind       string   `json:"kind,omitempty"`
	From       any      `json:"from,omitempty"`
	To         any      `json:"to,omitempty"`
	Timeformat string   `json:"timeformat,omitempty"`
	TZ         string   `json:"tz,omitempty"`
	Categories []string `json:"categories,omitempty"`
}

func (spec Spec) MarshalJSON() ([]byte, error) {
	type encodedSpec struct {
		Version     int          `json:"version"`
		Domain      *Domain      `json:"domain,omitempty"`
		Axes        *Axes        `json:"axes,omitempty"`
		Series      []Series     `json:"series,omitempty"`
		Annotations []Annotation `json:"annotations,omitempty"`
		View        *View        `json:"view,omitempty"`
		Meta        *Meta        `json:"meta,omitempty"`
	}
	ret := encodedSpec{
		Version:     spec.Version,
		Series:      spec.Series,
		Annotations: spec.Annotations,
	}
	if !spec.Domain.IsZero() {
		ret.Domain = &spec.Domain
	}
	if !spec.Axes.IsZero() {
		ret.Axes = &spec.Axes
	}
	if !spec.View.IsZero() {
		ret.View = &spec.View
	}
	if !spec.Meta.IsZero() {
		ret.Meta = &spec.Meta
	}
	return json.Marshal(ret)
}

func (series Series) MarshalJSON() ([]byte, error) {
	type encodedSeries struct {
		ID             string         `json:"id,omitempty"`
		Name           string         `json:"name,omitempty"`
		Axis           string         `json:"axis,omitempty"`
		Representation Representation `json:"representation,omitempty"`
		Source         *Source        `json:"source,omitempty"`
		Data           []any          `json:"data,omitempty"`
		Quality        *Quality       `json:"quality,omitempty"`
		Style          map[string]any `json:"style,omitempty"`
		Extra          map[string]any `json:"extra,omitempty"`
	}
	ret := encodedSeries{
		ID:             series.ID,
		Name:           series.Name,
		Axis:           series.Axis,
		Representation: series.Representation,
		Data:           series.Data,
		Style:          series.Style,
		Extra:          series.Extra,
	}
	if !series.Source.IsZero() {
		ret.Source = &series.Source
	}
	if !series.Quality.IsZero() {
		ret.Quality = &series.Quality
	}
	return json.Marshal(ret)
}

func (domain Domain) MarshalJSON() ([]byte, error) {
	ret := encodedDomain{
		Kind:       domain.Kind,
		From:       domain.From,
		To:         domain.To,
		Timeformat: canonicalTimeformat(domain.Timeformat, ""),
		TZ:         domain.TZ,
		Categories: domain.Categories,
	}
	if ret.Timeformat == "" {
		ret.Timeformat = domain.Timeformat
	}
	return json.Marshal(ret)
}

func (domain *Domain) UnmarshalJSON(data []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	var input encodedDomain
	if err := decoder.Decode(&input); err != nil {
		return err
	}
	*domain = Domain{
		Kind:       input.Kind,
		From:       input.From,
		To:         input.To,
		Timeformat: input.Timeformat,
		TZ:         input.TZ,
		Categories: input.Categories,
	}
	return nil
}

func (domain Domain) IsZero() bool {
	return domain.Kind == "" && domain.From == nil && domain.To == nil && domain.Timeformat == "" && domain.TZ == "" && len(domain.Categories) == 0
}

func (axes Axes) IsZero() bool {
	return axes.X.IsZero() && len(axes.Y) == 0
}

func (axis Axis) IsZero() bool {
	return axis.ID == "" && axis.Type == "" && axis.Unit == "" && axis.Label == "" && axis.TZ == "" && (axis.Extent == nil || axis.Extent.IsZero())
}

func (extent Extent) IsZero() bool {
	return extent.Min == nil && extent.Max == nil
}

func (source Source) IsZero() bool {
	return source.Kind == "" && source.Table == "" && source.Query == "" && source.Resolution == "" && source.DerivedFrom == ""
}

func (quality Quality) IsZero() bool {
	return !quality.Sampled && quality.Coverage == 0 && quality.RowCount == 0 && quality.EstimatedPoints == 0 && quality.DownsamplePolicy == ""
}

func (view View) IsZero() bool {
	return len(view.DefaultZoom) == 0 && view.PreferredRenderer == ""
}

func (meta Meta) IsZero() bool {
	return meta.Producer == "" && meta.GeneratedAt == "" && meta.LODGroup == ""
}

func Parse(data []byte) (*Spec, error) {
	ret := &Spec{}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	if err := decoder.Decode(ret); err != nil {
		return nil, err
	}
	ret.Normalize()
	if err := ret.Validate(); err != nil {
		return nil, err
	}
	return ret, nil
}

func ParseString(text string) (*Spec, error) {
	return Parse([]byte(text))
}

func Marshal(spec *Spec) ([]byte, error) {
	if spec == nil {
		spec = (&Spec{}).Normalize()
	} else {
		spec.Normalize()
	}
	if err := spec.Validate(); err != nil {
		return nil, err
	}
	return json.Marshal(spec)
}

func MarshalIndent(spec *Spec, prefix, indent string) ([]byte, error) {
	if spec == nil {
		spec = (&Spec{}).Normalize()
	} else {
		spec.Normalize()
	}
	if err := spec.Validate(); err != nil {
		return nil, err
	}
	return json.MarshalIndent(spec, prefix, indent)
}
