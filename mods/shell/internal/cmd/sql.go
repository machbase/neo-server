package cmd

import (
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"strings"
	"syscall"
	"time"

	"github.com/machbase/neo-server/v8/api"
	"github.com/machbase/neo-server/v8/mods/codec"
	"github.com/machbase/neo-server/v8/mods/codec/opts"
	"github.com/machbase/neo-server/v8/mods/shell/internal/action"
	"github.com/machbase/neo-server/v8/mods/util"
	"golang.org/x/term"
)

func init() {
	action.RegisterCmd(&action.Cmd{
		Name:   "sql",
		PcFunc: pcSql,
		Action: doSql,
		Desc:   "Execute sql query",
		Usage:  strings.ReplaceAll(helpSql, "\t", "    "),
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
	   --[no-]pause         pause for the screen paging
	-T,--timing             print elapsed time
    -p,--precision <int>    set precision of float value to force round`

type SqlCmd struct {
	Output       string         `name:"output" short:"o" default:"-"`
	Heading      bool           `name:"heading" negatable:"" default:"true"`
	Footer       bool           `name:"footer" negatable:"" default:"true"`
	Timing       bool           `name:"timing" short:"T"`
	TimeLocation *time.Location `name:"tz"`
	Format       string         `name:"format" short:"f" default:"box" enum:"box,csv,json"`
	Compress     string         `name:"compress" default:"-" enum:"-,gzip"`
	Delimiter    string         `name:"delimiter" short:"d" default:","`
	Rownum       bool           `name:"rownum" negatable:"" default:"true"`
	Timeformat   string         `name:"timeformat" short:"t"`
	Precision    int            `name:"precision" short:"p" default:"-1"`
	Pause        bool           `name:"pause" default:"false"`
	NoPause      bool           `name:"no-pause" default:"false"`
	BoxStyle     string         `kong:"-"`
	Interactive  bool           `kong:"-"`
	Help         bool           `kong:"-"`
	Query        []string       `arg:"" name:"query" passthrough:""`
}

func pcSql() action.PrefixCompleterInterface {
	return action.PcItem("sql")
}

func doSql(ctx *action.ActionContext) {
	cmd := &SqlCmd{}
	parser, err := action.Kong(cmd, func() error { ctx.Println(helpSql); cmd.Help = true; return nil })
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
	output, err := util.NewOutputStream(outputPath)
	if err != nil {
		ctx.Println("ERR", err.Error())
	}
	if closer, ok := output.(io.Closer); ok {
		defer closer.Close()
	}

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
		output = gw
		cmd.Interactive = false
	} else {
		if outputPath == "-" {
			if cmd.Pause {
				cmd.Interactive = true
			} else if cmd.NoPause {
				cmd.Interactive = false
			} else {
				cmd.Interactive = ctx.Interactive
			}
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
		opts.BoxSeparateColumns(ctx.Interactive), // always column-separate in interactive mode
		opts.BoxDrawBorder(ctx.Interactive),
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

	var beginTime time.Time
	query := &api.Query{
		Begin: func(q *api.Query) {
			beginTime = time.Now()
			cols := q.Columns()
			codec.SetEncoderColumnsTimeLocation(encoder, cols, cmd.TimeLocation)
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
		End: func(q *api.Query) {
			encoder.Close()
			if cmd.Footer {
				ctx.Println(q.UserMessage())
			} else {
				ctx.Println()
			}
			if cmd.Timing {
				ctx.Println("Elapsed Time:", time.Since(beginTime).String())
			}
		},
	}

	conn, err := ctx.BorrowConn()
	if err != nil {
		ctx.Println("ERR", err.Error())
		return
	}

	sqlText := util.StripQuote(strings.Join(cmd.Query, " "))
	if err := query.Execute(ctx.Ctx, conn, sqlText); err != nil {
		ctx.Println("ERR", err.Error())
	}
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
