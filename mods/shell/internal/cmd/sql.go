package cmd

import (
	"compress/gzip"
	"fmt"
	"os"
	"strings"
	"syscall"
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
	"golang.org/x/term"
)

func init() {
	client.RegisterCmd(&client.Cmd{
		Name:   "sql",
		PcFunc: pcSql,
		Action: doSql,
		Desc:   "Execute sql query",
		Usage:  helpSql,
	})
}

const helpSql string = `  sql [options] <query>
  arguments:
    query                   sql query to execute
  options:
    -o,--output <file>      output file (default:'-' stdout)
    -f,--format <format>    output format
                box         box format (default)
                csv         csv format
                json        json format
       --compress <method>  compression method [gzip] (default is not compressed)
    -d,--delimiter          csv delimiter (default:',')
       --[no-]rownum        include rownum as first column (default:true)
    -t,--timeformat         time format [ns|ms|s|<timeformat>] (default:'default')
                            consult "help timeformat"
       --tz                 timezone for handling datetime
                            consult "help tz"
       --[no-]heading       print header
       --[no-]footer        print footer message
    -p,--precision <int>    set precision of float value to force round
`

type SqlCmd struct {
	Output       string         `name:"output" short:"o" default:"-"`
	Heading      bool           `name:"heading" negatable:"" default:"true"`
	Footer       bool           `name:"footer" negatable:"" default:"true"`
	TimeLocation *time.Location `name:"tz"`
	Format       string         `name:"format" short:"f" default:"box" enum:"box,csv,json"`
	Compress     string         `name:"compress" default:"-" enum:"-,gzip"`
	Delimiter    string         `name:"delimiter" short:"d" default:","`
	Rownum       bool           `name:"rownum" negatable:"" default:"true"`
	Timeformat   string         `name:"timeformat" short:"t"`
	Precision    int            `name:"precision" short:"p" default:"-1"`
	BoxStyle     string         `kong:"-"`
	Interactive  bool           `kong:"-"`
	Help         bool           `kong:"-"`
	Query        []string       `arg:"" name:"query" passthrough:""`
}

func pcSql() readline.PrefixCompleterInterface {
	return readline.PcItem("sql",
		readline.PcItemDynamic(client.SqlHistory),
	)
}

func doSql(ctx *client.ActionContext) {
	cmd := &SqlCmd{}
	parser, err := client.Kong(cmd, func() error { ctx.Println(helpSql); cmd.Help = true; return nil })
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
	if cmd.BoxStyle == "" {
		cmd.BoxStyle = ctx.Pref().BoxStyle().Value()
	}
	var outputPath = util.StripQuote(cmd.Output)
	var output spec.OutputStream
	output, err = stream.NewOutputStream(outputPath)
	if err != nil {
		ctx.Println("ERR", err.Error())
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
		cmd.Interactive = false
	} else {
		if outputPath == "-" {
			cmd.Interactive = ctx.Interactive
		} else {
			cmd.Interactive = false
		}
	}

	encoder := codec.NewEncoder(cmd.Format,
		opts.OutputStream(output),
		opts.Timeformat(cmd.Timeformat),
		opts.Precision(cmd.Precision),
		opts.Rownum(cmd.Rownum),
		opts.Heading(cmd.Heading),
		opts.TimeLocation(cmd.TimeLocation),
		opts.Delimiter(cmd.Delimiter),
		opts.BoxStyle(cmd.BoxStyle),
		opts.BoxSeparateColumns(cmd.Interactive),
		opts.BoxDrawBorder(cmd.Interactive),
	)

	headerHeight := 0
	switch cmd.Format {
	default: // "box"
		headerHeight = 4
	case "csv":
		headerHeight = 1
		cmd.Footer = false
	case "json":
		headerHeight = 0
		cmd.Footer = false
	}

	windowHeight := 0
	if cmd.Interactive && term.IsTerminal(int(syscall.Stdout)) {
		if _, height, err := term.GetSize(int(syscall.Stdout)); err == nil {
			windowHeight = height
		}
	}
	pageHeight := windowHeight - 1
	if cmd.Heading {
		pageHeight -= headerHeight
	}
	nextPauseRow := int64(pageHeight)

	queryCtx := &do.QueryContext{
		Conn: ctx.Conn,
		Ctx:  ctx.Ctx,
		OnFetchStart: func(cols spi.Columns) {
			codec.SetEncoderColumnsTimeLocation(encoder, cols, cmd.TimeLocation)
			encoder.Open()
		},
		OnFetch: func(nrow int64, values []any) bool {
			err := encoder.AddRow(values)
			if err != nil {
				ctx.Println("ERR", err.Error())
			}
			if nextPauseRow > 0 && nextPauseRow == nrow {
				nextPauseRow += int64(pageHeight)
				encoder.Flush(cmd.Heading)
				if !pauseForMore() {
					return false
				}
			}
			if nextPauseRow <= 0 && nrow%1000 == 0 {
				encoder.Flush(false)
			}
			return true
		},
		OnFetchEnd: func() {
			encoder.Close()
		},
	}

	sqlText := util.StripQuote(strings.Join(cmd.Query, " "))
	msg, err := do.Query(queryCtx, sqlText)
	if err != nil {
		ctx.Println("ERR", err.Error())
	} else {
		if cmd.Footer {
			ctx.Println(msg)
		} else {
			ctx.Println()
		}
	}
	client.AddSqlHistory(sqlText)
}

func pauseForMore() bool {
	fmt.Fprintf(os.Stdout, ":")
	// switch stdin into 'raw' mode
	if oldState, err := term.MakeRaw(int(os.Stdin.Fd())); err == nil {
		b := make([]byte, 3)
		if _, err = os.Stdin.Read(b); err == nil {
			term.Restore(int(os.Stdin.Fd()), oldState)
			// remove ':' prompt'd line
			// erase line
			fmt.Fprintf(os.Stdout, "%s%s", "\x1b", "[2K")
			// cursor backward
			fmt.Fprintf(os.Stdout, "%s%s", "\x1b", "[1D")
			switch b[0] {
			case 'q', 'Q':
				return false
			default:
				return true
			}
		}
	}
	return true
}
