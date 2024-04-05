package cmd

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/machbase/neo-client/machrpc"
	"github.com/machbase/neo-server/api"
	"github.com/machbase/neo-server/mods/shellV2/internal/action"
	"github.com/machbase/neo-server/mods/util"
	"github.com/rivo/tview"
)

func init() {
	action.RegisterCmd(&action.Cmd{
		Name:   "walk",
		PcFunc: pcWalk,
		Action: doWalk,
		Desc:   "Execute query then walk-through the results",
		Usage:  strings.ReplaceAll(helpWalk, "\t", "    "),

		Deprecated:        true,
		DeprecatedMessage: "Use TQL instead.",
	})
}

const helpWalk = `  walk [options] <sql query>
  options:
        --[no-]rownum        show rownum
        --precision <int>    precision for float values
     -t,--timeformat         time format [ns|ms|s|<timeformat>] (default:'ns')
                             consult "help timeformat"
        --tz                 timezone for handling datetime
                             consult "help tz"`

type WalkCmd struct {
	TimeLocation *time.Location `name:"tz"`
	Timeformat   string         `name:"timeformat" short:"t"`
	Rownum       bool           `name:"rownum" negatable:"" default:"true"`
	Precision    int            `name:"precision" short:"p" default:"-1"`
	Help         bool           `kong:"-"`
	Query        []string       `arg:"" name:"query" passthrough:""`
}

func pcWalk() action.PrefixCompleterInterface {
	return action.PcItem("walk")
}

func doWalk(ctx *action.ActionContext) {
	cmd := &WalkCmd{}
	parser, err := action.Kong(cmd, func() error { ctx.Println(helpWalk); cmd.Help = true; return nil })
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

	sqlText := util.StripQuote(strings.Join(cmd.Query, " "))

	walker, err := NewWalker(ctx.Ctx, ctx.Conn, sqlText, util.GetTimeformat(cmd.Timeformat), cmd.TimeLocation, cmd.Precision)
	if err != nil {
		ctx.Println("ERR", err.Error())
		return
	}
	defer walker.Close()

	app := tview.NewApplication()
	table := tview.NewTable()
	table.SetBorder(true).SetTitle(" ESC to quit, [yellow::bl]R[-::-]eload ").SetTitleAlign(tview.AlignLeft)
	table.SetFixed(1, 1)
	table.SetContent(walker)
	table.SetDoneFunc(func(key tcell.Key) {
		if key == tcell.KeyESC {
			app.Stop()
		}
	})
	table.SetInputCapture(func(evt *tcell.EventKey) *tcell.EventKey {
		if evt.Rune() == 'r' || evt.Rune() == 'R' {
			walker.Reload()
			table.ScrollToBeginning()
			return nil
		}
		return evt
	})
	if err := app.SetRoot(table, true).SetFocus(table).Run(); err != nil {
		ctx.Println("ERR", err.Error())
		return
	}
}

type Walker struct {
	tview.TableContentReadOnly
	sqlText    string
	conn       *machrpc.Conn
	ctx        context.Context
	mutex      sync.Mutex
	rows       *machrpc.Rows
	cols       api.Columns
	values     [][]string
	eof        bool
	fetchSize  int
	tz         *time.Location
	timeformat string
	precision  int
}

func NewWalker(ctx context.Context, conn *machrpc.Conn, sqlText string, timeformat string, tz *time.Location, precision int) (*Walker, error) {
	w := &Walker{
		sqlText:    sqlText,
		conn:       conn,
		ctx:        ctx,
		fetchSize:  400,
		timeformat: timeformat,
		tz:         tz,
		precision:  precision,
	}
	return w, w.Reload()
}

func (w *Walker) Close() {
	w.mutex.Lock()
	defer w.mutex.Unlock()

	if w.rows != nil {
		w.rows.Close()
		w.rows = nil
	}
}

func (w *Walker) Reload() error {
	w.mutex.Lock()
	defer w.mutex.Unlock()

	if w.rows != nil {
		w.rows.Close()
		w.rows = nil
	}

	rows, err := w.conn.Query(w.ctx, w.sqlText)
	if err != nil {
		return err
	}

	cols, err := api.RowsColumns(rows)
	if err != nil {
		rows.Close()
		return err
	}

	values := make([][]string, 1)
	values[0] = make([]string, len(cols)+1)
	values[0][0] = "ROWNUM"
	for i := range cols {
		if cols[i].Type == "datetime" {
			values[0][i+1] = fmt.Sprintf("%s(%s)", cols[i].Name, w.tz.String())
		} else {
			values[0][i+1] = cols[i].Name
		}
	}

	w.rows = rows
	w.cols = cols
	w.values = values
	w.eof = false
	return nil
}

func (w *Walker) GetCell(row, col int) *tview.TableCell {
	w.mutex.Lock()
	defer w.mutex.Unlock()

	if row == 0 {
		return tview.NewTableCell(w.values[row][col]).SetTextColor(tcell.ColorYellow).SetAlign(tview.AlignCenter)
	}

	if row >= len(w.values) {
		w.fetchMore()
	}

	if row < len(w.values) {
		color := tcell.ColorWhite
		if col == 0 {
			color = tcell.ColorYellow
		}
		return tview.NewTableCell(w.values[row][col]).SetTextColor(color)
	} else {
		return nil
	}
}

func (w *Walker) fetchMore() {
	if w.eof {
		return
	}

	buffer := api.MakeBuffer(w.cols)

	count := 0
	nrows := len(w.values)
	for {
		if !w.rows.Next() {
			w.eof = true
			return
		}

		err := w.rows.Scan(buffer...)
		if err != nil {
			w.eof = true
			return
		}

		values := makeValues(buffer, w.tz, w.timeformat, w.precision)
		w.values = append(w.values, append([]string{strconv.Itoa(nrows + count)}, values...))

		count++
		if count >= w.fetchSize {
			return
		}
	}
}

func (w *Walker) GetRowCount() int {
	w.mutex.Lock()
	defer w.mutex.Unlock()

	if w.eof {
		return len(w.values)
	}
	return math.MaxInt32
}

func (w *Walker) GetColumnCount() int {
	return len(w.cols) + 1
}

func makeValues(rec []any, tz *time.Location, timeformat string, precision int) []string {
	cols := make([]string, len(rec))
	for i, r := range rec {
		if r == nil {
			cols[i] = "NULL"
			continue
		}
		switch v := r.(type) {
		case *string:
			cols[i] = *v
		case *time.Time:
			cols[i] = v.In(tz).Format(timeformat)
		case *float64:
			if precision < 0 {
				cols[i] = fmt.Sprintf("%f", *v)
			} else {
				cols[i] = fmt.Sprintf("%.*f", precision, *v)
			}
		case *int:
			cols[i] = fmt.Sprintf("%d", *v)
		case *int32:
			cols[i] = fmt.Sprintf("%d", *v)
		case *int64:
			cols[i] = fmt.Sprintf("%d", *v)
		default:
			cols[i] = fmt.Sprintf("%T", r)
		}
	}
	return cols
}
