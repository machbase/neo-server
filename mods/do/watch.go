package do

import (
	"context"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/machbase/neo-server/api"
	"github.com/machbase/neo-server/mods/util"
)

type WatchData = map[string]any

type Watcher struct {
	initialized  bool
	ctx          context.Context
	connProvider func() (api.Conn, error)
	parallelism  int
	parallelCh   chan bool
	chanSize     int
	out          chan any
	C            <-chan any
	// table info
	isTagTable  bool
	tableName   string
	columnNames []string
	columnTypes []api.ColumnType
	timeformat  *util.TimeFormatter
	// tag table
	nameColumn      string
	basetimeColumn  string
	tagNames        []string
	tagLastTime     map[string]time.Time
	tagLastTimeLock sync.Mutex
	// log table
	lastArrivalTime time.Time
	maxRowNum       int
}

type WatchOption func(*Watcher)

func NewWatcher(ctx context.Context, provider func() (api.Conn, error), opts ...WatchOption) (*Watcher, error) {
	w := &Watcher{connProvider: provider}
	for _, opt := range opts {
		opt(w)
	}
	if err := w.init(ctx); err != nil {
		return nil, err
	}
	return w, nil
}

func WithTableName(name string) WatchOption {
	return func(w *Watcher) {
		w.tableName = name
	}
}

func WithTagNames(names ...string) WatchOption {
	return func(w *Watcher) {
		w.tagNames = append(w.tagNames, names...)
	}
}

func WithTimeformat(timeformat string, tz *time.Location) WatchOption {
	return func(w *Watcher) {
		w.timeformat = util.NewTimeFormatter(util.Timeformat(timeformat), util.TimeLocation(tz))
	}
}

func WithParallelism(n int) WatchOption {
	return func(w *Watcher) {
		w.parallelism = n
	}
}

func WithChanSize(n int) WatchOption {
	return func(w *Watcher) {
		w.chanSize = n
	}
}

func WithMaxRowNum(n int) WatchOption {
	return func(w *Watcher) {
		w.maxRowNum = n
	}
}

func (w *Watcher) String() string {
	return fmt.Sprintf("Watcher {table:%s, tags:%v, parallelism:%d}", w.tableName, w.tagNames, w.parallelism)
}

func (w *Watcher) handleData(obj WatchData) {
	w.out <- obj
}

func (w *Watcher) handleError(err error) {
	w.out <- err
}

func (w *Watcher) Close() {
	if w.parallelCh != nil {
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
	if w.chanSize <= 0 {
		w.chanSize = 100
	}
	w.out = make(chan any, w.chanSize)
	w.C = w.out

	conn, err := w.connProvider()
	if err != nil {
		return err
	}
	if tableType, err := TableType(ctx, conn, w.tableName); err != nil {
		return err
	} else if tableType != api.TagTableType && tableType != api.LogTableType {
		return fmt.Errorf("not supported table type")
	} else {
		w.isTagTable = tableType == api.TagTableType
	}

	if w.isTagTable {
		if len(w.tagNames) == 0 {
			return fmt.Errorf("table '%s' is TAG table, no tag specified", w.tableName)
		}
		if w.parallelism <= 0 || w.parallelism > len(w.tagNames) {
			w.parallelism = len(w.tagNames)
		}
		w.parallelCh = make(chan bool, w.parallelism)
		for i := 0; i < w.parallelism; i++ {
			w.parallelCh <- true
		}
	} else {
		if w.maxRowNum <= 0 {
			w.maxRowNum = 20
		} else if w.maxRowNum > 100 {
			w.maxRowNum = 100
		}
		w.parallelism = 1 // log table does not support parallelism
		w.parallelCh = make(chan bool, w.parallelism)
		for i := 0; i < w.parallelism; i++ {
			w.parallelCh <- true
		}
	}

	var desc *TableDescription
	if desc0, err := Describe(ctx, conn, w.tableName, false); err != nil {
		return fmt.Errorf("fail to get table info '%s', %s", w.tableName, err.Error())
	} else {
		desc = desc0.(*TableDescription)
	}
	for _, c := range desc.Columns {
		if w.isTagTable {
			if c.IsBaseTime() {
				w.basetimeColumn = c.Name
			} else if c.IsTagName() {
				w.nameColumn = c.Name
			}
		}
		w.columnNames = append(w.columnNames, c.Name)
		w.columnTypes = append(w.columnTypes, c.Type)
	}
	if w.isTagTable {
		if len(w.nameColumn) == 0 {
			return fmt.Errorf("fail to get tag name column '%s'", w.tableName)
		}
		if len(w.basetimeColumn) == 0 {
			return fmt.Errorf("fail to get basetime column '%s'", w.tableName)
		}
		w.tagLastTime = map[string]time.Time{}
	}
	return nil
}

func (w *Watcher) Execute() {
	if w.isTagTable {
		for _, tag := range w.tagNames {
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
		<-w.parallelCh
		defer func() {
			w.parallelCh <- true
		}()
	}
	conn, err := w.connProvider()
	if err != nil {
		w.handleError(err)
		return
	}
	defer conn.Close()
	row := conn.QueryRow(w.ctx, fmt.Sprintf("select recent_row_time from V$%s_STAT where name = ?", w.tableName), tag)
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
			strings.Join(w.columnNames, ","), w.tableName, w.nameColumn, w.basetimeColumn),
		tag, recentTime)
	if err := row.Err(); err != nil {
		w.handleError(row.Err())
		return
	}
	if len(row.Values()) == 0 {
		return
	}
	var values = w.makeBuffer()
	if err := row.Scan(values...); err != nil {
		w.handleError(err)
		return
	}
	obj := WatchData{}
	for i := range w.columnNames {
		name := w.columnNames[i]
		typ := w.columnTypes[i]
		if typ == api.DatetimeColumnType {
			if v, ok := values[i].(*time.Time); ok {
				obj[name] = w.timeformat.FormatEpoch(*v)
				continue
			}
		}
		obj[name] = values[i]
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
	conn, err := w.connProvider()
	if err != nil {
		w.handleError(err)
		return
	}
	defer conn.Close()
	if w.lastArrivalTime.IsZero() {
		if row := conn.QueryRow(w.ctx, fmt.Sprintf("select max(_ARRIVAL_TIME) from %s", w.tableName)); row.Err() != nil {
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
		fmt.Sprintf(`select /*+ SCAN_FORWARD(%s) */ %s from %s where _ARRIVAL_TIME > ?`, w.tableName, columns, w.tableName),
		w.lastArrivalTime.UnixNano(),
	)
	if err != nil {
		w.handleError(err)
		return
	}
	rowNum := 0
	for rows.Next() {
		rowNum++
		if rowNum > w.maxRowNum {
			w.handleError(fmt.Errorf("too many changes, omit the rest"))
			w.lastArrivalTime = time.Time{}
			return
		}
		var values = w.makeBuffer()
		values = append([]any{new(time.Time)}, values...) // for _ARRIVAL_TIME
		if err := rows.Scan(values...); err != nil {
			w.handleError(err)
			return
		}
		arrivalTime := values[0].(*time.Time)
		values = values[1:]
		obj := WatchData{}
		for i, n := range w.columnNames {
			obj[n] = values[i]
		}
		w.handleData(obj)
		w.lastArrivalTime = *arrivalTime
	}
}

func (w *Watcher) makeBuffer() []any {
	ret := make([]any, len(w.columnTypes))
	for i, t := range w.columnTypes {
		switch t {
		case api.Int16ColumnType:
			ret[i] = new(int16)
		case api.Uint16ColumnType:
			ret[i] = new(uint16)
		case api.Int32ColumnType:
			ret[i] = new(int32)
		case api.Uint32ColumnType:
			ret[i] = new(uint32)
		case api.Int64ColumnType:
			ret[i] = new(int64)
		case api.Uint64ColumnType:
			ret[i] = new(uint64)
		case api.Float32ColumnType:
			ret[i] = new(float32)
		case api.Float64ColumnType:
			ret[i] = new(float64)
		case api.VarcharColumnType:
			ret[i] = new(string)
		case api.TextColumnType:
			ret[i] = new(string)
		case api.IpV4ColumnType:
			ret[i] = new(net.IP)
		case api.IpV6ColumnType:
			ret[i] = new(net.IP)
		case api.DatetimeColumnType:
			ret[i] = new(time.Time)
		case api.BinaryColumnType:
			ret[i] = new([]byte)
		}
	}
	return ret
}
