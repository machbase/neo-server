package httpd

import (
	"fmt"
	"math"
	"math/cmplx"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/machbase/neo-server/mods/renderer"
	"github.com/machbase/neo-server/mods/stream"
	spi "github.com/machbase/neo-spi"
	"gonum.org/v1/gonum/dsp/fourier"
	"gonum.org/v1/gonum/dsp/window"
)

type ChartRequest struct {
	TagPaths     []string      `json:"tags"`
	Timeformat   string        `json:"timeformat,omitempty"`
	TimeLocation string        `json:"tz,omitempty"`
	Range        time.Duration `json:"range,omitempty"`
	Timestamp    string        `json:"time,omitempty"`
	Transform    string        `json:"transform,omitempty"`
	Window       string        `json:"window,omitempty"`
	Format       string        `json:"format,omitempty"`
	Title        string        `json:"title,omitempty"`
	Subtitle     string        `json:"subtitle,omitempty"`
	Width        string        `json:"width,omitempty"`
	Height       string        `json:"height,omitempty"`
}

func (svr *httpd) handleChart(ctx *gin.Context) {
	var err error
	req := &ChartRequest{
		Timeformat:   "default", // do not change this default value; already documented as is.
		TimeLocation: "UTC",
		Range:        1 * time.Minute,
		Timestamp:    "now",
		Transform:    "",
		Format:       "html",
		Title:        "Chart",
		Subtitle:     "",
		Width:        "1600",
		Height:       "900",
	}

	if ctx.Request.Method == http.MethodPost {
		contentType := ctx.ContentType()
		if contentType == "application/json" {
			if err = ctx.Bind(req); err != nil {
				ctx.String(http.StatusBadRequest, err.Error())
				return
			}
		} else {
			ctx.String(http.StatusBadRequest, fmt.Sprintf("unsupported content-type: %s", contentType))
			return
		}
	} else if ctx.Request.Method == http.MethodGet {
		req.TagPaths = ctx.QueryArray("tags")
		req.Timeformat = strString(ctx.Query("timeformat"), req.Timeformat)
		req.TimeLocation = strString(ctx.Query("tz"), req.TimeLocation)
		req.Range = strDuration(ctx.Query("range"), req.Range)
		req.Timestamp = strString(ctx.Query("time"), req.Timestamp)
		req.Transform = strString(ctx.Query("transform"), req.Transform)
		req.Window = strString(ctx.Query("window"), req.Window)
		req.Format = strString(ctx.Query("format"), req.Format)
		req.Title = strString(ctx.Query("title"), req.Title)
		req.Subtitle = strString(ctx.Query("subtitle"), req.Subtitle)
		req.Width = strString(ctx.Query("width"), req.Width)
		req.Height = strString(ctx.Query("height"), req.Height)
	}

	if len(req.TagPaths) == 0 {
		ctx.String(http.StatusBadRequest, "no 'tags' is specified")
		return
	}

	var timeLocation = strTimeLocation(req.TimeLocation, time.UTC)
	var output = &stream.WriterOutputStream{Writer: ctx.Writer}

	queries, err := renderer.BuildChartQueries(req.TagPaths, req.Timestamp, req.Range, req.Timeformat, timeLocation)
	if err != nil {
		ctx.String(http.StatusInternalServerError, err.Error())
		return
	}
	series := []*spi.RenderingData{}
	for _, dq := range queries {
		data, err := dq.Query(svr.db)
		if err != nil {
			ctx.String(http.StatusInternalServerError, err.Error())
			return
		}
		series = append(series, data)
	}

	if req.Transform == "fft" {
		series[0] = transformFFT(series[0], req.Range, req.Window)
	}

	rndr := renderer.NewChartRendererBuilder(req.Format).
		SetTitle(req.Title).
		SetSubtitle(req.Subtitle).
		SetSize(req.Width, req.Height).
		Build()
	if rndr == nil {
		svr.log.Warnf("chart request has no renderer %+v", req)
		ctx.String(http.StatusInternalServerError, "no renderer")
		return
	}
	ctx.Writer.Header().Set("Content-type", rndr.ContentType())
	if err = rndr.Render(ctx, output, series); err != nil {
		ctx.String(http.StatusInternalServerError, err.Error())
		return
	}
}

func transformFFT(series *spi.RenderingData, periodDuration time.Duration, windowType string) *spi.RenderingData {
	lenSamples := len(series.Values)
	period := float64(lenSamples) / (float64(periodDuration) / float64(time.Second))
	fft := fourier.NewFFT(lenSamples)
	vals := series.Values
	amplifier := func(v float64) float64 { return v }

	switch strings.ToLower(windowType) {
	case "hamming":
		vals = window.Hamming(vals)
		// The magnitude of all bins has been decreased by β.
		// Generally in an analysis amplification may be omitted, but to
		// make a comparable data, the result should be amplified by -β
		// of the window function — +5.37 dB for the Hamming window.
		//  -β = 20 log_10(amplifier).
		// amplifier := math.Pow(10, 5.37/20.0)
		amplifier = func(v float64) float64 {
			return v * math.Pow(10, 5.37/float64(lenSamples))
		}
	default:
		amplifier = func(v float64) float64 {
			return v * 2.0 / float64(lenSamples)
		}
	}

	coeff := fft.Coefficients(nil, vals)

	trans := &spi.RenderingData{}
	trans.Name = fmt.Sprintf("FFT-%s", series.Name)
	for i, c := range coeff {
		hz := fft.Freq(i) * period
		if hz == 0 {
			continue
		}
		magnitude := cmplx.Abs(c)
		amplitude := amplifier(magnitude)
		// phase = cmplx.Phase(c)
		trans.Labels = append(trans.Labels, fmt.Sprintf("%f", hz))
		trans.Values = append(trans.Values, amplitude)
	}
	return trans
}
