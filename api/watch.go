package api

import (
	"context"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/machbase/neo-server/api/types"
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
	TagNames     []string // tag table
	MaxRowNum    int      // log table
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
	columnTypes []types.ColumnType
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
	if tableType, err := TableType(ctx, conn, w.TableName); err != nil {
		return err
	} else if tableType != types.TableTypeTag && tableType != types.TableTypeLog {
		return fmt.Errorf("not supported table type")
	} else {
		w.isTagTable = tableType == types.TableTypeTag
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
		w.columnNames = append(w.columnNames, c.Name)
		w.columnTypes = append(w.columnTypes, c.Type)
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
	row := conn.QueryRow(w.ctx, fmt.Sprintf("select recent_row_time from V$%s_STAT where name = ?", w.TableName), tag)
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
			strings.Join(w.columnNames, ","), w.TableName, w.nameColumn, w.basetimeColumn),
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
		if typ == types.ColumnTypeDatetime {
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
		case types.ColumnTypeShort:
			ret[i] = new(int16)
		case types.ColumnTypeUshort:
			ret[i] = new(uint16)
		case types.ColumnTypeInteger:
			ret[i] = new(int32)
		case types.ColumnTypeUinteger:
			ret[i] = new(uint32)
		case types.ColumnTypeLong:
			ret[i] = new(int64)
		case types.ColumnTypeUlong:
			ret[i] = new(uint64)
		case types.ColumnTypeFloat:
			ret[i] = new(float32)
		case types.ColumnTypeDouble:
			ret[i] = new(float64)
		case types.ColumnTypeVarchar:
			ret[i] = new(string)
		case types.ColumnTypeText:
			ret[i] = new(string)
		case types.ColumnTypeIPv4:
			ret[i] = new(net.IP)
		case types.ColumnTypeIPv6:
			ret[i] = new(net.IP)
		case types.ColumnTypeDatetime:
			ret[i] = new(time.Time)
		case types.ColumnTypeBinary:
			ret[i] = new([]byte)
		}
	}
	return ret
}
