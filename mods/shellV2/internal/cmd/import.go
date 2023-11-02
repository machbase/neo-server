package cmd

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/machbase/neo-server/mods/codec"
	"github.com/machbase/neo-server/mods/codec/opts"
	"github.com/machbase/neo-server/mods/do"
	"github.com/machbase/neo-server/mods/shellV2/internal/action"
	"github.com/machbase/neo-server/mods/stream"
	"github.com/machbase/neo-server/mods/util"
	spi "github.com/machbase/neo-spi"
)

func init() {
	action.RegisterCmd(&action.Cmd{
		Name:   "import",
		PcFunc: pcImport,
		Action: doImport,
		Desc:   "Import table",
		Usage:  strings.ReplaceAll(helpImport, "\t", "    "),
	})
}

const helpImport = `  import [options] <table>
  arguments:
    table                 table name to write
  options:
    -i,--input <file>     input file, (default: '-' stdin)
    -f,--format <fmt>     file format [csv] (default:'csv')
       --compress <alg>   input data is compressed in <alg> (support:gzip)
       --no-header        there is no header, do not skip first line (default)
       --header           first line is header, skip it
       --method           write method [insert|append] (default:'insert')
       --create-table     create table if it doesn't exist (default:false)
       --truncate-table   truncate table ahead importing new data (default:false)
    -d,--delimiter        csv delimiter (default:',')
       --tz               timezone for handling datetime
    -t,--timeformat       time format [ns|ms|s|<timeformat>] (default:'ns')
                          consult "help timeformat"
       --eof <string>     specify eof line, use any string matches [a-zA-Z0-9]+ (default: '.')`

type ImportCmd struct {
	Table         string         `arg:"" name:"table"`
	Input         string         `name:"input" short:"i" default:"-"`
	Compress      string         `name:"compress" short:"z" default:"-" enum:"-,gzip"`
	HasHeader     bool           `name:"header" negatable:""`
	EofMark       string         `name:"eof" default:"."`
	InputFormat   string         `name:"format" short:"f" default:"csv" enum:"csv"`
	Method        string         `name:"method" default:"insert" enum:"insert,append"`
	CreateTable   bool           `name:"create-table" default:"false"`
	TruncateTable bool           `name:"truncate-table" default:"false"`
	Delimiter     string         `name:"delimiter" short:"d" default:","`
	Timeformat    string         `name:"timeformat" short:"t" default:"ns"`
	TimeLocation  *time.Location `name:"tz"`
	Help          bool           `kong:"-"`
}

func pcImport() action.PrefixCompleterInterface {
	return action.PcItem("import")
}

func doImport(ctx *action.ActionContext) {
	cmd := &ImportCmd{}
	parser, err := action.Kong(cmd, func() error { ctx.Println(helpImport); cmd.Help = true; return nil })
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

	in, err := stream.NewInputStream(cmd.Input)
	if err != nil {
		ctx.Println(err.Error())
		return
	}
	defer in.Close()

	exists, created, truncated, err := do.ExistsTableOrCreate(ctx.Ctx, ctx.Conn, cmd.Table, cmd.CreateTable, cmd.TruncateTable)
	if err != nil {
		ctx.Println("ERR", err.Error())
		return
	}
	if !exists {
		ctx.Printfln("Table '%s' does not exist", cmd.Table)
		return
	}
	if created {
		ctx.Printfln("Table '%s' created", cmd.Table)
	}
	if truncated {
		ctx.Printfln("Table '%s' truncated", cmd.Table)
	}

	var desc *do.TableDescription
	if desc0, err := do.Describe(ctx.Ctx, ctx.Conn, cmd.Table, false); err != nil {
		ctx.Printfln("ERR fail to get table info '%s', %s", cmd.Table, err.Error())
		return
	} else {
		desc = desc0.(*do.TableDescription)
	}

	if ctx.IsUserShellInteractiveMode() && cmd.Input == "-" {
		ctx.Printfln("# Enter %s⏎ to quit", cmd.EofMark)
		colNames := desc.Columns.Columns().Names()
		ctx.Println("#", strings.Join(colNames, cmd.Delimiter))

		buff := []byte{}
		for {
			bufferedIn := bufio.NewReader(in)
			bs, _, err := bufferedIn.ReadLine()
			if err != nil {
				break
			}
			if string(bs) == cmd.EofMark {
				break
			}
			buff = append(buff, bs...)
		}
		in = &stream.ReaderInputStream{Reader: bytes.NewReader(buff)}
	} else {
		if cmd.Compress == "gzip" {
			gr, err := gzip.NewReader(in)
			if err != nil {
				ctx.Println("ERR", err.Error())
				return
			}
			in = &stream.ReaderInputStream{Reader: gr}
			defer gr.Close()
		}
	}

	cols := desc.Columns.Columns()
	decoder := codec.NewDecoder(cmd.InputFormat,
		opts.InputStream(in),
		opts.Timeformat(cmd.Timeformat),
		opts.TimeLocation(cmd.TimeLocation),
		opts.TableName(cmd.Table),
		opts.Columns(cols.Names()...),
		opts.ColumnTypes(cols.Types()...),
		opts.Delimiter(cmd.Delimiter),
		opts.Heading(cmd.HasHeader),
	)

	var appender spi.Appender
	var lineno int = 0

	hold := []string{}
	tick := time.Now()
	for ctx.Ctx.Err() == nil {
		vals, err := decoder.NextRow()
		if err != nil {
			if err != io.EOF {
				ctx.Println("ERR", err.Error())
			}
			break
		}
		lineno++

		if len(vals) != len(desc.Columns) {
			ctx.Printfln("line %d contains %d columns, but expected %d", lineno, len(vals), len(desc.Columns))
			break
		}
		if cmd.Method == "insert" {
			for i := 0; i < len(desc.Columns); i++ {
				hold = append(hold, "?")
			}
			query := fmt.Sprintf("insert into %s values(%s)", cmd.Table, strings.Join(hold, ","))
			if result := ctx.Conn.Exec(ctx.Ctx, query, vals...); result.Err() != nil {
				ctx.Println(result.Err().Error())
				break
			}
			hold = hold[:0]
		} else { // append
			if appender == nil {
				appender, err = ctx.Conn.Appender(ctx.Ctx, cmd.Table)
				if err != nil {
					ctx.Println("ERR", err.Error())
					break
				}
			}
			err = appender.Append(vals...)
			if err != nil {
				ctx.Println("ERR", err.Error())
				break
			}
		}
		if appender != nil && lineno%500000 == 0 {
			// update progress message per 500K records
			tps := int(float64(lineno) / time.Since(tick).Seconds())
			ctx.Printf("%s %s records (%s/s)\r", cmd.Method, util.NumberFormat(lineno), util.NumberFormat(tps))
		} else if appender == nil && lineno%100 == 0 {
			// progress message per 100 records
			tps := int(float64(lineno) / time.Since(tick).Seconds())
			ctx.Printf("%s %s records (%s/s)\r", cmd.Method, util.NumberFormat(lineno), util.NumberFormat(tps))
		}
	}

	ctx.Print("\r\n")
	if cmd.Method == "insert" {
		ctx.Printf("import total %s record(s) %sed\n", util.NumberFormat(lineno), cmd.Method)
	} else if appender != nil {
		succ, fail, err := appender.Close()
		if err != nil {
			ctx.Printf("import total %s record(s) appended, %s failed %s\n", util.NumberFormat(succ), util.NumberFormat(fail), err.Error())
		} else if fail > 0 {
			ctx.Printf("import total %s record(s) appended, %s failed\n", util.NumberFormat(succ), util.NumberFormat(fail))
		} else {
			ctx.Printf("import total %s record(s) appended\n", util.NumberFormat(succ))
		}
	} else {
		ctx.Print("import processed no record\n")
	}
}
