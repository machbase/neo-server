package cmd

import (
	"fmt"
	"time"

	"github.com/chzyer/readline"
	"github.com/machbase/neo-server/mods/renderer"
	"github.com/machbase/neo-server/mods/renderer/model"
	"github.com/machbase/neo-server/mods/shell/internal/client"
	"github.com/machbase/neo-server/mods/stream"
	"github.com/machbase/neo-server/mods/util"
	spi "github.com/machbase/neo-spi"
	"github.com/robfig/cron/v3"
)

func init() {
	client.RegisterCmd(&client.Cmd{
		Name:   "chart",
		PcFunc: pcChart,
		Action: doChart,
		Desc:   "Rendering chart from tag table",
		Usage:  helpChart,
	})
}

const helpChart = `  chart [options] <tag_path>...
  arguments:
    tag_path ...   tag path as <table>/<tag>#<column>. ex) mytable/sensor.tag1#column
                   since all tag tables have 'value' column,
                   '#<column>' part can be omitted for default '#value' ex) mytable/sensor
  options:
       --tz                  timezone for handling datetime
    -t,--timeformat          time format [ns|ms|s|<timeformat>] (default:'default')
                             consult "help timeformat"
       --time  <time>        base time, now, last or time string in format "2023-02-03 13:20:30" (default: now)
       --range <duration>    time range of data, from time specified by '--time' (default: 1m)
    -r,--refresh <duration>  refresh period (default: 0)
                             effective only if '--time' is "now" or "last".
                             value format is '[0-9]+(s|m)'  ex) '3s' for 3 seconds, '1m' for 1 minute
                             auto refresh is disabled if value is 0 which is default
    -n,--count <count>       repeat times (default: 0)
                             set 0 for unlimit
    -o,--output <file>       output file (default:'-' stdout)
    -f,--format <format>     output format
                term         terminal chart (default)
                json         json format
                csv          csv format
                html         generate chart page in html format
       --title <title>       title text for html output (default:"Chart")
       --subtitle <title>    sub title text for html output (default:"")
       --width <string>      chart width for html output (default:"1600")
       --height <string>     chart height (default:"900")
`

type ChartCmd struct {
	TagPaths     []string       `arg:"" name:"tags"`
	TimeLocation *time.Location `name:"tz"`
	Timeformat   string         `name:"timeformat"`
	Range        time.Duration  `name:"range" default:"1m"`
	Timestamp    string         `name:"time" default:"now"`
	Refresh      time.Duration  `name:"refresh" short:"r" default:"0"`
	Count        int            `name:"count" short:"n" default:"0"`
	Output       string         `name:"output" short:"o" default:"-"`
	Format       string         `name:"format" short:"f" enum:"term,json,csv,html" default:"term"`
	HtmlTitle    string         `name:"title" default:"Chart"`
	HtmlSubtitle string         `name:"subtitle" default:""`
	HtmlWidth    string         `name:"width" default:"1600"`
	HtmlHeight   string         `name:"height" default:"900"`
	Help         bool           `kong:"-"`
}

func pcChart() readline.PrefixCompleterInterface {
	return readline.PcItem("chart")
}

func doChart(ctx *client.ActionContext) {
	cmd := &ChartCmd{}
	parser, err := client.Kong(cmd, func() error { ctx.Println(helpChart); cmd.Help = true; return nil })
	if err != nil {
		ctx.Println(err.Error())
		return
	}
	_, err = parser.Parse(util.SplitFields(ctx.Line, true))
	if cmd.Help {
		return
	}
	if err != nil {
		ctx.Println(err.Error())
		return
	}

	if cmd.TimeLocation == nil {
		cmd.TimeLocation = ctx.Pref().TimeZone().TimezoneValue()
	}
	if cmd.Timeformat == "" {
		cmd.Timeformat = ctx.Pref().Timeformat().Value()
	}
	cmd.Timeformat = util.StripQuote(cmd.Timeformat)

	if len(cmd.TagPaths) == 0 {
		ctx.Println("at least one tag_path should be specified")
		return
	}

	if len(cmd.Timestamp) == 0 {
		cmd.Timestamp = "now"
	}

	queries, err := renderer.BuildChartQueries(cmd.TagPaths, cmd.Timestamp, cmd.Range, cmd.Timeformat, cmd.TimeLocation)
	if err != nil {
		ctx.Println("ERR", err.Error())
		return
	}

	render := renderer.New(cmd.Format,
		renderer.Title(cmd.HtmlTitle),
		renderer.Subtitle(cmd.HtmlSubtitle),
		renderer.Size(cmd.HtmlWidth, cmd.HtmlHeight),
	)

	if render == nil {
		ctx.Println("ERR", "no renderer found for", cmd.Format)
		return
	}
	switch cmd.Format {
	default:
		// termdash allows cmd.Count only 0 (infinite)
		// it control the termination by itself (enter ESC key)
		cmd.Count = 0
		if cmd.Refresh < time.Second {
			cmd.Refresh = time.Second
		}
	case "csv":
	case "json":
	case "html":
	}

	var scheduler *cron.Cron
	var quitCh = make(chan bool, 1)

	runCount := 0
	runCanceled := false
	runner := func() {
		output, err := stream.NewOutputStream(cmd.Output)
		if err != nil {
			ctx.Println("ERR", err.Error())
			return
		}
		defer output.Close()

		series := []*model.RenderingData{}
		// query
		for _, dq := range queries {
			data, err := dq.Query(ctx.DB)
			if err != nil {
				fmt.Println(err.Error())
				return
			}
			series = append(series, data)
		}
		runCount++

		if err = render.Render(ctx, output, series); err != nil {
			runCanceled = true
			if err != nil && err != spi.ErrUserCancel {
				ctx.Println("ERR", err.Error())
			}
		}
		if runCanceled || cmd.Count > 0 && cmd.Count <= runCount {
			quitCh <- true
		}
	}

	// run first round
	runner()
	// repeat ?
	if cmd.Count != 1 && !runCanceled {
		capture := ctx.NewCaptureUserInterruptCallback("", func(string) bool { return false })
		if ctx.Interactive && cmd.Format != "term" {
			go capture.Start()
			defer capture.Close()
		}

		scheduler = cron.New()
		if _, err := scheduler.AddFunc(fmt.Sprintf("@every %s", cmd.Refresh.String()), runner); err != nil {
			fmt.Println(err.Error())
			return
		}
		go scheduler.Run()

		for !runCanceled {
			select {
			case <-capture.C:
				runCanceled = true
			case <-quitCh:
				runCanceled = true
			}
		}
		scheduler.Stop()
		ctx.Cancel()
	}
}
