package cmd

import (
	"compress/gzip"
	"strings"
	"time"

	"github.com/machbase/neo-server/mods/codec"
	"github.com/machbase/neo-server/mods/codec/opts"
	"github.com/machbase/neo-server/mods/do"
	"github.com/machbase/neo-server/mods/shell/internal/client"
	"github.com/machbase/neo-server/mods/stream"
	"github.com/machbase/neo-server/mods/stream/spec"
	"github.com/machbase/neo-server/mods/util"
	"github.com/machbase/neo-server/mods/util/readline"
	spi "github.com/machbase/neo-spi"
)

func init() {
	client.RegisterCmd(&client.Cmd{
		Name:   "export",
		PcFunc: pcExport,
		Action: doExport,
		Desc:   "Export table",
		Usage:  strings.ReplaceAll(helpExport, "\t", "    "),
	})
}

const helpExport = `  export [options] <table>
  arguments:
    table                    table name to read
  options:
    -o,--output <file>       output file (default:'-' stdout)
    -f,--format <format>     output format
                csv          csv format (default)
                json         json format
       --compress <method>   compression method [gzip] (default is not compressed)
       --[no-]heading        print header message (default:false)
       --[no-]footer         print footer message (default:false)
    -d,--delimiter           csv delimiter (default:',')
       --tz                  timezone for handling datetime
    -t,--timeformat          time format [ns|ms|s|<timeformat>] (default:'ns')
                             consult "help timeformat"
       --precision <int>     set precision of float value to force round
`

type ExportCmd struct {
	Table   string `arg:"" name:"table"`
	Output  string `name:"output" short:"o" default:"-"`
	Heading bool   `name:"heading" negatable:""`
	Footer  bool   `name:"footer" negatable:""`

	TimeLocation *time.Location `name:"tz"`
	Format       string         `name:"format" short:"f" default:"csv" enum:"box,csv,json"`
	Compress     string         `name:"compress" default:"-" enum:"-,gzip"`
	Delimiter    string         `name:"delimiter" short:"d" default:","`
	Timeformat   string         `name:"timeformat" short:"t"`
	Precision    int            `name:"precision" short:"p" default:"-1"`
	Help         bool           `kong:"-"`
}

func pcExport() readline.PrefixCompleterInterface {
	return readline.PcItem("export")
}

func doExport(ctx *client.ActionContext) {
	cmd := &ExportCmd{}
	parser, err := client.Kong(cmd, func() error { ctx.Println(helpExport); cmd.Help = true; return nil })
	if err != nil {
		ctx.Println("ERR", err.Error())
		return
	}
	_, err = parser.Parse(util.SplitFields(ctx.Line, false))
	if cmd.Help {
		return
	}
	if err != nil {
		ctx.Println("ERR", err.Error())
		return
	}

	if cmd.TimeLocation == nil {
		cmd.TimeLocation = ctx.Pref().TimeZone().TimezoneValue()
	}
	if cmd.Timeformat == "" {
		cmd.Timeformat = ctx.Pref().Timeformat().Value()
	}
	cmd.Timeformat = util.StripQuote(cmd.Timeformat)

	if len(cmd.Table) == 0 {
		ctx.Println("ERR", "no table is specified")
		return
	}

	var outputPath = util.StripQuote(cmd.Output)
	var output spec.OutputStream
	output, err = stream.NewOutputStream(outputPath)
	if err != nil {
		ctx.Println("ERR", err.Error())
		return
	}
	defer output.Close()

	if cmd.Compress == "gzip" {
		gw := gzip.NewWriter(output)
		defer func() {
			if gw != nil {
				err := gw.Close()
				if err != nil {
					ctx.Println("ERR", err.Error())
				}
			}
		}()
		output = &stream.WriterOutputStream{Writer: gw}
	}

	encoder := codec.NewEncoder(cmd.Format,
		opts.OutputStream(output),
		opts.Timeformat(cmd.Timeformat),
		opts.Precision(cmd.Precision),
		opts.Rownum(false),
		opts.Heading(cmd.Heading),
		opts.TimeLocation(cmd.TimeLocation),
		opts.Delimiter(cmd.Delimiter),
		opts.BoxStyle("light"),
		opts.BoxSeparateColumns(true),
		opts.BoxDrawBorder(true),
	)

	printProgress := false
	capture := ctx.NewCaptureUserInterrupt("")
	if ctx.IsUserShellInteractiveMode() && cmd.Output != "-" {
		printProgress = true
		go capture.Start()
	}
	doneC := make(chan bool)

	var alive bool = true
	var lineno int = 0
	go func() {
		defer capture.Close()
		tick := time.Now()
		queryCtx := &do.QueryContext{
			Conn: ctx.Conn,
			Ctx:  ctx.Ctx,
			OnFetchStart: func(cols spi.Columns) {
				codec.SetEncoderColumns(encoder, cols)
				encoder.Open()
			},
			OnFetch: func(nrow int64, values []any) bool {
				err := encoder.AddRow(values)
				if err != nil {
					ctx.Println("ERR", err.Error())
				}
				lineno++
				if printProgress && lineno%10000 == 0 {
					// update progress message per 10K records
					tps := int(float64(lineno) / time.Since(tick).Seconds())
					ctx.Printf("export %s records (%s/s)\r", util.NumberFormat(lineno), util.NumberFormat(tps))
				}
				return alive
			},
			OnFetchEnd: func() {
				encoder.Close()
			},
		}

		if _, err := do.Query(queryCtx, "select * from "+cmd.Table); err != nil {
			ctx.Println("ERR", err.Error())
		}
		if printProgress {
			ctx.Print("\r\n")
			ctx.Printf("export total %s record(s)", util.NumberFormat(lineno))
		}
		doneC <- true
	}()

	select {
	case <-capture.C:
	case <-doneC:
	}
	alive = false
}
