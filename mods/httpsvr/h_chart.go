package httpsvr

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/machbase/neo-shell/renderer"
	"github.com/machbase/neo-shell/stream"
	spi "github.com/machbase/neo-spi"
)

type ChartRequest struct {
	TagPaths     []string      `json:"tags"`
	Timeformat   string        `json:"timeformat,omitempty"`
	TimeLocation string        `json:"tz,omitempty"`
	Range        time.Duration `json:"range,omitempty"`
	Timestamp    string        `json:"time,omitempty"`
	Format       string        `json:"format,omitempty"`
	Title        string        `json:"title,omitempty"`
	Subtitle     string        `json:"subtitle,omitempty"`
	Width        string        `json:"width,omitempty"`
	Height       string        `json:"height,omitempty"`
}

func (svr *Server) handleChart(ctx *gin.Context) {
	var err error
	req := &ChartRequest{
		Timeformat:   "default",
		TimeLocation: "UTC",
		Range:        1 * time.Minute,
		Timestamp:    "now",
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
	rndr := renderer.NewChartRendererBuilder(req.Format).
		SetTitle(req.Title).
		SetSubtitle(req.Subtitle).
		SetSize(req.Width, req.Height).
		Build()

	ctx.Writer.Header().Set("Content-type", rndr.ContentType())
	if err = rndr.Render(ctx, output, series); err != nil {
		ctx.String(http.StatusInternalServerError, err.Error())
		return
	}
}
