package cmd

import (
	"compress/gzip"
	"strings"
	"time"

	"github.com/machbase/neo-server/v8/api"
	"github.com/machbase/neo-server/v8/mods/codec"
	"github.com/machbase/neo-server/v8/mods/codec/opts"
	"github.com/machbase/neo-server/v8/mods/shell/internal/action"
	"github.com/machbase/neo-server/v8/mods/stream"
	"github.com/machbase/neo-server/v8/mods/stream/spec"
	"github.com/machbase/neo-server/v8/mods/util"
)

func init() {
	action.RegisterCmd(&action.Cmd{
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
    -p,--precision <int>     set precision of float value to force round`

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

func pcExport() action.PrefixCompleterInterface {
	return action.PcItem("export")
}

func doExport(ctx *action.ActionContext) {
	cmd := &ExportCmd{}
	parser, err := action.Kong(cmd, func() error { ctx.Println(helpExport); cmd.Help = true; return nil })
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

	var lineno int = 0

	tick := time.Now()
	query := &api.Query{
		Begin: func(q *api.Query) {
			cols := q.Columns()
			codec.SetEncoderColumns(encoder, cols)
			encoder.Open()
		},
		Next: func(q *api.Query, nrow int64) bool {
			values, err := q.Columns().MakeBuffer()
			if err != nil {
				ctx.Println("ERR", err.Error())
				return false
			}
			if err = q.Scan(values...); err != nil {
				ctx.Println("ERR", err.Error())
				return false
			}
			if err := encoder.AddRow(values); err != nil {
				ctx.Println("ERR", err.Error())
			}
			lineno++
			if printProgress && lineno%10000 == 0 {
				// update progress message per 10K records
				tps := int(float64(lineno) / time.Since(tick).Seconds())
				ctx.Printf("export %s records (%s/s)\r", util.NumberFormat(lineno), util.NumberFormat(tps))
			}
			return ctx.Ctx.Err() == nil
		},
		End: func(q *api.Query) {
			encoder.Close()
		},
	}

	if err := query.Execute(ctx.Ctx, ctx.Conn, "select * from "+cmd.Table); err != nil {
		ctx.Println("ERR", err.Error())
	}
	if printProgress {
		ctx.Print("\r\n")
		ctx.Printf("export total %s record(s)", util.NumberFormat(lineno))
	}
}
