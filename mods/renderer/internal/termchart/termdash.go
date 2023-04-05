package termchart

import (
	"context"

	spi "github.com/machbase/neo-spi"
	"github.com/mum4k/termdash"
	"github.com/mum4k/termdash/cell"
	"github.com/mum4k/termdash/container"
	"github.com/mum4k/termdash/keyboard"
	"github.com/mum4k/termdash/linestyle"
	"github.com/mum4k/termdash/terminal/tcell"
	"github.com/mum4k/termdash/terminal/terminalapi"
	"github.com/mum4k/termdash/widgets/linechart"
)

type Renderer struct {
	term       *tcell.Terminal
	controller *termdash.Controller
	lchart     *linechart.LineChart
	quitCh     chan bool
	err        error
}

func NewRenderer() spi.Renderer {
	return &Renderer{}
}

func (r *Renderer) ContentType() string {
	return "application/octet-stream"
}

func (r *Renderer) Render(ctx context.Context, output spi.OutputStream, data []*spi.RenderingData) error {
	if r.err != nil {
		return r.err
	}

	if r.term == nil {
		// make terminal interface
		r.term, r.err = tcell.New()
		if r.err != nil {
			return r.err
		}

		// line chart
		r.lchart, r.err = linechart.New(
			linechart.AxesCellOpts(cell.FgColor(cell.ColorRed)),
			linechart.YLabelCellOpts(cell.FgColor(cell.ColorGreen)),
			linechart.XLabelCellOpts(cell.FgColor(cell.ColorCyan)),
		)
		if r.err != nil {
			return r.err
		}

		// terminal container
		cont, err := container.New(
			r.term,
			container.Border(linestyle.Light),
			container.BorderTitle("ESC to quit"),
			container.PlaceWidget(r.lchart),
		)
		if err != nil {
			return err
		}

		r.quitCh = make(chan bool, 1)

		quitter := func(k *terminalapi.Keyboard) {
			if k.Key == keyboard.KeyEsc {
				r.err = spi.ErrUserCancel
				close(r.quitCh)
			}
		}

		termOpts := []termdash.Option{
			termdash.KeyboardSubscriber(quitter),
		}

		controller, err := termdash.NewController(r.term, cont, termOpts...)
		if err != nil {
			return err
		}
		r.controller = controller

		go func() {
			select {
			case <-r.quitCh:
			case <-ctx.Done():
				close(r.quitCh)
			}
			r.controller.Close()
			r.term.Close()
		}()
	}

	for _, series := range data {
		xlabels := make(map[int]string)
		for i, n := range series.Labels {
			xlabels[i] = n
		}
		err := r.lchart.Series(
			series.Name,
			series.Values,
			linechart.SeriesCellOpts(cell.FgColor(cell.ColorNumber(33))),
			linechart.SeriesXLabels(xlabels),
		)
		if err != nil {
			return err
		}
	}

	return r.controller.Redraw()
}
