package jschart

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"html/template"

	spi "github.com/machbase/neo-spi"
)

type ChartJsModel struct {
	Type    string         `json:"type"`
	Data    ChartJsData    `json:"data"`
	Options ChartJsOptions `json:"options"`
}

type ChartJsData struct {
	Labels   []string         `json:"labels"`
	Datasets []ChartJsDataset `json:"datasets"`
}

type ChartJsDataset struct {
	Label           string    `json:"label"`
	Data            []float64 `json:"data"`
	BorderWidth     int       `json:"borderWidth"`
	BorderColor     string    `json:"borderColor"`
	BackgroundColor string    `json:"backgroundColor"`
}

type ChartJsOptions struct {
	Scales ChartJsScalesOption `json:"scales"`
}

type ChartJsScalesOption struct {
	Y ChartJsScale `json:"y"`
}

type ChartJsScale struct {
	BeginAtZero bool `json:"beginAtZero"`
}

func convertChartJsModel(data []*spi.RenderingData) (*ChartJsModel, error) {
	pl := []string{"rgb(44,142,229)", "rgb(251,72,113)", "rgb(63,180,179)", "rgb(252,141,50)", "rgb(133,71,255)", "rgb(253,195,69)", "rgb(189,192,197)"}

	cm := &ChartJsModel{}
	cm.Type = "line"
	cm.Data = ChartJsData{}
	cm.Data.Labels = data[0].Labels
	cm.Data.Datasets = []ChartJsDataset{}
	for idx, series := range data {
		cm.Data.Datasets = append(cm.Data.Datasets, ChartJsDataset{
			Label:           series.Name,
			Data:            series.Values,
			BorderWidth:     1,
			BorderColor:     pl[idx%len(pl)],
			BackgroundColor: "rgba(0,0,0,0)",
		})
	}
	cm.Options = ChartJsOptions{}
	cm.Options.Scales = ChartJsScalesOption{
		Y: ChartJsScale{BeginAtZero: false},
	}
	return cm, nil
}

///////////////////////////////////////////////
// JSON Renderer

type JsonRenderer struct {
}

func NewJsonRenderer() spi.Renderer {
	return &JsonRenderer{}
}

func (r *JsonRenderer) ContentType() string {
	return "application/json"
}

func (r *JsonRenderer) Render(ctx context.Context, output spi.OutputStream, data []*spi.RenderingData) error {
	model, err := convertChartJsModel(data)
	if err != nil {
		return err
	}
	buf, err := json.Marshal(model)
	if err != nil {
		return err
	}
	output.Write(buf)
	return nil
}

///////////////////////////////////////////////
// HTML Renderer

func NewHtmlRenderer(opts HtmlOptions) spi.Renderer {
	return &HtmlRenderer{
		Options: opts,
	}
}

func (r *HtmlRenderer) ContentType() string {
	return "text/html"
}

//go:embed chartjs.html
var chartHtmlTemplate string

type ChartHtmlVars struct {
	HtmlOptions
	ChartData template.JS
}

type HtmlOptions struct {
	Title    string
	Subtitle string
	Width    string
	Height   string
}

type HtmlRenderer struct {
	Options HtmlOptions
}

func (r *HtmlRenderer) Render(ctx context.Context, output spi.OutputStream, data []*spi.RenderingData) error {
	tmpl, err := template.New("chart_template").Parse(chartHtmlTemplate)
	if err != nil {
		fmt.Println(err.Error())
		return err
	}

	model, err := convertChartJsModel(data)
	if err != nil {
		return err
	}
	dataJson, err := json.Marshal(model)
	if err != nil {
		fmt.Println(err.Error())
		return err
	}

	buff := &bytes.Buffer{}
	vars := &ChartHtmlVars{HtmlOptions: HtmlOptions(r.Options)}
	vars.ChartData = template.JS(string(dataJson))
	err = tmpl.Execute(buff, vars)
	if err != nil {
		fmt.Println(err.Error())
		return err
	}

	output.Write(buff.Bytes())
	return nil
}
