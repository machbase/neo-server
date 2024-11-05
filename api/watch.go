package api

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/machbase/neo-server/mods/util"
)

type WatchData = map[string]any

type WatcherConfig struct {
	ConnProvider func() (Conn, error)
	Timeformat   string
	Timezone     *time.Location
	Parallelism  int
	ChanSize     int
	TableName    string
	TagNames     []string // required in watching tag table
	MaxRowNum    int      // affects on watching log table
}

type Watcher struct {
	WatcherConfig
	initialized bool
	ctx         context.Context
	parallelCh  chan bool
	out         chan any
	C           <-chan any
	// table info
	isTagTable  bool
	columnNames []string
	columns     Columns
	timeformat  *util.TimeFormatter
	// tag table
	nameColumn      string
	basetimeColumn  string
	tagLastTime     map[string]time.Time
	tagLastTimeLock sync.Mutex
	// log table
	lastArrivalTime time.Time
}

func NewWatcher(ctx context.Context, conf WatcherConfig) (*Watcher, error) {
	w := &Watcher{WatcherConfig: conf}
	w.timeformat = util.NewTimeFormatter(util.Timeformat(w.Timeformat), util.TimeLocation(w.Timezone))
	if err := w.init(ctx); err != nil {
		return nil, err
	}
	return w, nil
}

func (w *Watcher) String() string {
	return fmt.Sprintf("Watcher {table:%s, tags:%v, parallelism:%d}", w.TableName, w.TagNames, w.Parallelism)
}

func (w *Watcher) handleData(obj WatchData) {
	w.out <- obj
}

func (w *Watcher) handleError(err error) {
	w.out <- err
}

func (w *Watcher) Close() {
	if w.parallelCh != nil {
		// drain the channel, waiting for all goroutines to finish
		for i := 0; i < w.Parallelism; i++ {
			<-w.parallelCh
		}
		close(w.parallelCh)
	}
	if w.out != nil {
		close(w.out)
	}
}

func (w *Watcher) init(ctx context.Context) error {
	if w.initialized {
		return nil
	}
	w.initialized = true
	w.ctx = ctx
	if w.ChanSize <= 0 {
		w.ChanSize = 100
	}
	w.out = make(chan any, w.ChanSize)
	w.C = w.out

	conn, err := w.ConnProvider()
	if err != nil {
		return err
	}
	if tableType, err := QueryTableType(ctx, conn, w.TableName); err != nil {
		return err
	} else if tableType != TableTypeTag && tableType != TableTypeLog {
		return fmt.Errorf("not supported table type")
	} else {
		w.isTagTable = tableType == TableTypeTag
	}

	if w.isTagTable {
		if len(w.TagNames) == 0 {
			return fmt.Errorf("table '%s' is TAG table, no tag specified", w.TableName)
		}
		if w.Parallelism <= 0 || w.Parallelism > len(w.TagNames) {
			w.Parallelism = len(w.TagNames)
		}
		w.parallelCh = make(chan bool, w.Parallelism)
		for i := 0; i < w.Parallelism; i++ {
			w.parallelCh <- true
		}
	} else {
		if w.MaxRowNum <= 0 {
			w.MaxRowNum = 20
		} else if w.MaxRowNum > 100 {
			w.MaxRowNum = 100
		}
		w.Parallelism = 1 // log table does not support parallelism
		w.parallelCh = make(chan bool, w.Parallelism)
		for i := 0; i < w.Parallelism; i++ {
			w.parallelCh <- true
		}
	}

	var desc *TableDescription
	if desc0, err := DescribeTable(ctx, conn, w.TableName, false); err != nil {
		return fmt.Errorf("fail to get table info '%s', %s", w.TableName, err.Error())
	} else {
		desc = desc0
	}
	for _, c := range desc.Columns {
		if w.isTagTable {
			if c.IsBaseTime() {
				w.basetimeColumn = c.Name
			} else if c.IsTagName() {
				w.nameColumn = c.Name
			}
		}
		w.columns = append(w.columns, c)
		w.columnNames = append(w.columnNames, c.Name)
	}
	if w.isTagTable {
		if len(w.nameColumn) == 0 {
			return fmt.Errorf("fail to get tag name column '%s'", w.TableName)
		}
		if len(w.basetimeColumn) == 0 {
			return fmt.Errorf("fail to get basetime column '%s'", w.TableName)
		}
		w.tagLastTime = map[string]time.Time{}
	}
	return nil
}

func (w *Watcher) Execute() {
	if w.isTagTable {
		for _, tag := range w.TagNames {
			go w.executeTag(tag)
		}
	} else {
		if len(w.parallelCh) == 0 {
			// previous execution is not finished
			return
		}
		go w.executeLog()
	}
}

func (w *Watcher) executeTag(tag string) {
	if w.parallelCh != nil {
		if _, isOpen := <-w.parallelCh; !isOpen {
			return
		}
		defer func() {
			w.parallelCh <- true
		}()
	}
	conn, err := w.ConnProvider()
	if err != nil {
		w.handleError(err)
		return
	}
	defer conn.Close()
	row := conn.QueryRow(w.ctx, fmt.Sprintf("select recent_row_time from V$%s_STAT where name = ?", strings.ToUpper(w.TableName)), tag)
	if err := row.Err(); err != nil {
		// ignore, no such tag
		return
	}
	recentTime := time.Time{}
	if err := row.Scan(&recentTime); err != nil {
		w.handleError(err)
		return
	}
	w.tagLastTimeLock.Lock()
	if lt, ok := w.tagLastTime[tag]; ok && !recentTime.After(lt) {
		w.tagLastTimeLock.Unlock()
		// no change
		return
	}
	w.tagLastTime[tag] = recentTime
	w.tagLastTimeLock.Unlock()

	row = conn.QueryRow(w.ctx,
		fmt.Sprintf("select %s from %s where %s = ? and %s = ?",
			strings.Join(w.columnNames, ","), strings.ToUpper(w.TableName), w.nameColumn, w.basetimeColumn),
		tag, recentTime)
	if err := row.Err(); err != nil {
		w.handleError(row.Err())
		return
	}
	values, err := w.columns.MakeBuffer()
	if err != nil {
		w.handleError(err)
		return
	}
	if err := row.Scan(values...); err != nil {
		w.handleError(err)
		return
	}
	obj := WatchData{}
	for i, col := range w.columns {
		name := col.Name
		typ := col.Type
		if typ == ColumnTypeDatetime {
			if v, ok := values[i].(*time.Time); ok {
				obj[name] = w.timeformat.FormatEpoch(*v)
				continue
			}
		}
		obj[name] = Unbox(values[i])
	}
	w.handleData(obj)
}

func (w *Watcher) executeLog() {
	if w.parallelCh != nil {
		<-w.parallelCh
		defer func() {
			w.parallelCh <- true
		}()
	}
	conn, err := w.ConnProvider()
	if err != nil {
		w.handleError(err)
		return
	}
	defer conn.Close()
	if w.lastArrivalTime.IsZero() {
		if row := conn.QueryRow(w.ctx, fmt.Sprintf("select max(_ARRIVAL_TIME) from %s", w.TableName)); row.Err() != nil {
			w.handleError(row.Err())
			return
		} else {
			if err := row.Scan(&w.lastArrivalTime); err != nil {
				w.handleError(err)
				return
			}
		}
	}
	columns := "_ARRIVAL_TIME," + strings.Join(w.columnNames, ",")
	rows, err := conn.Query(w.ctx,
		fmt.Sprintf(`select /*+ SCAN_FORWARD(%s) */ %s from %s where _ARRIVAL_TIME > ?`, w.TableName, columns, w.TableName),
		w.lastArrivalTime.UnixNano(),
	)
	if err != nil {
		w.handleError(err)
		return
	}
	rowNum := 0
	for rows.Next() {
		rowNum++
		if rowNum > w.MaxRowNum {
			w.handleError(fmt.Errorf("too many changes, omit the rest"))
			w.lastArrivalTime = time.Time{}
			return
		}
		values, err := w.columns.MakeBuffer()
		if err != nil {
			w.handleError(err)
			return
		}
		values = append([]any{new(time.Time)}, values...) // for _ARRIVAL_TIME
		if err := rows.Scan(values...); err != nil {
			w.handleError(err)
			return
		}
		arrivalTime := values[0].(*time.Time)
		values = values[1:]
		obj := WatchData{}
		for i, n := range w.columnNames {
			obj[n] = Unbox(values[i])
		}
		w.handleData(obj)
		w.lastArrivalTime = *arrivalTime
	}
}
