package cmd

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/chzyer/readline"
	"github.com/machbase/neo-server/mods/codec"
	"github.com/machbase/neo-server/mods/do"
	"github.com/machbase/neo-server/mods/shell/internal/client"
	"github.com/machbase/neo-server/mods/stream"
	"github.com/machbase/neo-server/mods/util"
	spi "github.com/machbase/neo-spi"
)

func init() {
	client.RegisterCmd(&client.Cmd{
		Name:   "import",
		PcFunc: pcImport,
		Action: doImport,
		Desc:   "Import table",
		Usage:  helpImport,
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
       --eof <string>     specify eof line, use any string matches [a-zA-Z0-9]+ (default: '.')
`

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
	Timeformat    string         `name:"timeformat" short:"t"`
	TimeLocation  *time.Location `name:"tz"`
	Help          bool           `kong:"-"`
}

func pcImport() readline.PrefixCompleterInterface {
	return readline.PcItem("import")
}

func doImport(ctx *client.ActionContext) {
	cmd := &ImportCmd{}
	parser, err := client.Kong(cmd, func() error { ctx.Println(helpImport); cmd.Help = true; return nil })
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

	exists, created, truncated, err := do.ExistsTableOrCreate(ctx.DB, cmd.Table, cmd.CreateTable, cmd.TruncateTable)
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
	if desc0, err := do.Describe(ctx.DB, cmd.Table, false); err != nil {
		ctx.Printfln("ERR fail to get table info '%s', %s", cmd.Table, err.Error())
		return
	} else {
		desc = desc0.(*do.TableDescription)
	}

	if ctx.Interactive {
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

	decoder := codec.NewDecoderBuilder(cmd.InputFormat).
		SetInputStream(in).
		SetColumns(desc.Columns.Columns()).
		SetTimeFormat(cmd.Timeformat).
		SetTimeLocation(cmd.TimeLocation).
		SetCsvDelimieter(cmd.Delimiter).
		Build()

	var appender spi.Appender
	hold := []string{}
	lineno := 0
	for {
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
			if result := ctx.DB.Exec(query, vals...); result.Err() != nil {
				ctx.Println(result.Err().Error())
				break
			}
			hold = hold[:0]
		} else { // append
			if appender == nil {
				appender, err = ctx.DB.Appender(cmd.Table)
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
	}
	if cmd.Method == "insert" {
		ctx.Printfln("import total %d record(s) %sed", lineno, cmd.Method)
	} else if appender != nil {
		succ, fail, err := appender.Close()
		if err != nil {
			ctx.Printfln("import total %d record(s) appended, %d failed %s", succ, fail, err.Error())
		} else {
			if fail == 0 {
				ctx.Printfln("import total %d record(s) appended", succ)
			} else {
				ctx.Printfln("import total %d record(s) appended, %d failed", succ, fail)
			}
		}
	} else {
		ctx.Printfln("import processed no record")
	}
}
