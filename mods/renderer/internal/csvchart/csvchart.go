package csvchart

import (
	"context"
	"fmt"

	"encoding/csv"

	"github.com/machbase/neo-server/mods/renderer/model"
	"github.com/machbase/neo-server/mods/stream/spec"
)

type Renderer struct {
}

func NewRenderer() *Renderer {
	return &Renderer{}
}

func (r *Renderer) ContentType() string {
	return "text/csv"
}

func (r *Renderer) Render(ctx context.Context, output spec.OutputStream, data []*model.RenderingData) error {
	w := csv.NewWriter(output)
	for row := range data[0].Values {
		label := data[0].Labels[row]
		varr := []string{label}
		for col := range data {
			varr = append(varr, fmt.Sprintf("%f", data[col].Values[row]))
		}
		w.Write(varr[:])
	}
	w.Flush()
	return nil
}
