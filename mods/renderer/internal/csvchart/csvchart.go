package csvchart

import (
	"context"
	"fmt"

	"encoding/csv"

	spi "github.com/machbase/neo-spi"
)

type Renderer struct {
}

func NewRenderer() spi.Renderer {
	return &Renderer{}
}

func (r *Renderer) ContentType() string {
	return "text/csv"
}

func (r *Renderer) Render(ctx context.Context, output spi.OutputStream, data []*spi.RenderingData) error {
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
