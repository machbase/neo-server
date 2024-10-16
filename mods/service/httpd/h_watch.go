package httpd

import (
	"encoding/json"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/machbase/neo-server/api"
	"github.com/machbase/neo-server/mods/do"
	"github.com/machbase/neo-server/mods/util"
)

func (svr *httpd) handleWatchQuery(ctx *gin.Context) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Watcher panic", r)
		}
	}()
	watch := &Watcher{}

	watch.tableName = ctx.Param("table")
	for _, arr := range ctx.QueryArray("tag") {
		watch.tagNames = append(watch.tagNames, strings.Split(arr, ",")...)
	}
	var period time.Duration
	if p, err := time.ParseDuration(ctx.Query("period")); err == nil {
		period = p
	}
	if period < 1*time.Second {
		period = 1 * time.Second
	}
	var keepAliveTime time.Duration
	if p, err := time.ParseDuration(ctx.Query("keep-alive")); err == nil {
		keepAliveTime = p
	}
	if keepAliveTime == 0 {
		keepAliveTime = 30 * time.Second
	}

	timeformat := strString(ctx.Query("timeformat"), "ns")
	tz := time.UTC
	if timezone := ctx.Query("tz"); timezone != "" {
		tz, _ = util.ParseTimeLocation(timezone, time.UTC)
	}
	watch.timeformat = util.NewTimeFormatter(util.Timeformat(timeformat), util.TimeLocation(tz))

	ctx.Writer.Header().Set("Content-Type", "text/event-stream")
	ctx.Writer.Header().Set("Cache-Control", "no-cache")
	ctx.Writer.Header().Set("Connection", "keep-alive")

	svr.log.Infof("Watcher add %s, period %v", watch.String(), period)

	conn, err := svr.getTrustConnection(ctx)
	if err != nil {
		svr.log.Debug("Watcher error", err.Error())
		WatchSend(ctx.Writer, &WatchEvent{Type: "error", Reason: err.Error()})
		return
	}
	defer conn.Close()

	if tableType, err := do.TableType(ctx, conn, watch.tableName); err != nil {
		svr.log.Debug("Watcher error", err.Error())
		WatchSend(ctx.Writer, &WatchEvent{Type: "error", Reason: err.Error()})
		return
	} else if tableType != api.TagTableType && tableType != api.LogTableType {
		svr.log.Debug("Watcher unsupported table type %s", watch.tableName)
		WatchSend(ctx.Writer, &WatchEvent{Type: "error", Reason: "not supported table type"})
		return
	} else {
		watch.isTagTable = tableType == api.TagTableType
	}

	if watch.isTagTable {
		if len(watch.tagNames) == 0 {
			WatchSend(ctx.Writer, &WatchEvent{Type: "error", Reason: fmt.Sprintf("table '%s' is TAG table, specify tag", watch.tableName)})
			return
		}
	}

	var desc *do.TableDescription
	if desc0, err := do.Describe(ctx, conn, watch.tableName, false); err != nil {
		WatchSend(ctx.Writer, &WatchEvent{Type: "error", Reason: fmt.Sprintf("fail to get table info '%s', %s", watch.tableName, err.Error())})
		return
	} else {
		desc = desc0.(*do.TableDescription)
	}
	for _, c := range desc.Columns {
		if watch.isTagTable {
			if c.IsBaseTime() {
				watch.basetimeColumn = c.Name
			} else if c.IsTagName() {
				watch.nameColumn = c.Name
			}
		}
		watch.columnNames = append(watch.columnNames, c.Name)
		watch.columnTypes = append(watch.columnTypes, c.Type)
	}
	if watch.isTagTable {
		if len(watch.nameColumn) == 0 {
			WatchSend(ctx.Writer, &WatchEvent{Type: "error", Reason: fmt.Sprintf("fail to get tag name column '%s'", watch.tableName)})
			return
		}
		if len(watch.basetimeColumn) == 0 {
			WatchSend(ctx.Writer, &WatchEvent{Type: "error", Reason: fmt.Sprintf("fail to get basetime column '%s'", watch.tableName)})
			return
		}
	}

	periodTick := time.NewTicker(period)
	defer periodTick.Stop()
	keepAliveTick := time.NewTicker(keepAliveTime)
	defer keepAliveTick.Stop()

	lastTime := time.Now()
	for {
		select {
		case <-keepAliveTick.C:
			if time.Since(lastTime) >= keepAliveTime {
				WatchSend(ctx.Writer, &WatchKeepAlive{Message: "keep-alive"})
				lastTime = time.Now()
			}
		case <-periodTick.C:
			results := watch.Execute(ctx, conn)
			if len(results) == 0 {
				continue
			}
			for _, r := range results {
				for cIdx, typ := range watch.columnTypes {
					if typ == api.DatetimeColumnType {
						col := watch.columnNames[cIdx]
						if v, ok := r[col].(*time.Time); ok {
							r[col] = watch.timeformat.FormatEpoch(*v)
						}
					}
				}
				WatchSend(ctx.Writer, r)
			}
			lastTime = time.Now()
		case <-ctx.Writer.CloseNotify():
			svr.log.Infof("Watcher close %s", watch.String())
			return
		}
	}
}

func WatchSend[M WatchMessage](w gin.ResponseWriter, m M) {
	var msg any = m
	switch m := msg.(type) {
	case *WatchKeepAlive:
		w.Write([]byte(fmt.Sprintf(": %s", m.Message)))
	case *WatchEvent:
		b, _ := json.Marshal(m)
		w.Write([]byte("event: "))
		w.Write(b)
	case WatchData:
		b, _ := json.Marshal(m)
		w.Write([]byte("data: "))
		w.Write(b)
	}
	w.Write([]byte("\n\n"))
	w.Flush()
}

type WatchMessage interface {
	*WatchEvent | *WatchKeepAlive | WatchData
}

type WatchKeepAlive struct {
	Message string
}

type WatchEvent struct {
	Type   string `json:"type,omitempty"`
	Reason string `json:"reason,omitempty"`
}

type WatchData = map[string]any

type Watcher struct {
	isTagTable  bool
	tableName   string
	columnNames []string
	columnTypes []api.ColumnType
	timeformat  *util.TimeFormatter
	// tag table
	nameColumn     string
	basetimeColumn string
	tagNames       []string
	tagLastTime    map[string]time.Time
	// log table
	lastArrivalTime time.Time
}

func (w *Watcher) String() string {
	return fmt.Sprintf("Watcher{table:%s, tags:%v}", w.tableName, w.tagNames)
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

func (w *Watcher) Execute(ctx *gin.Context, conn api.Conn) []WatchData {
	if w.isTagTable {
		return w.executeTag(ctx, conn)
	} else {
		return w.executeLog(ctx, conn)
	}
}

func (w *Watcher) executeTag(ctx *gin.Context, conn api.Conn) []WatchData {
	ret := []WatchData{}
	if w.tagLastTime == nil {
		w.tagLastTime = map[string]time.Time{}
	}
	for _, tag := range w.tagNames {
		row := conn.QueryRow(ctx, fmt.Sprintf("select recent_row_time from V$%s_STAT where name = ?", w.tableName), tag)
		if row.Err() != nil {
			ret = append(ret, WatchData{"error": row.Err().Error()})
			break
		}
		recentTime := time.Time{}
		if err := row.Scan(&recentTime); err != nil {
			ret = append(ret, WatchData{"error": err.Error()})
			break
		}
		if lt, ok := w.tagLastTime[tag]; ok && !recentTime.After(lt) {
			continue
		}
		w.tagLastTime[tag] = recentTime
		row = conn.QueryRow(ctx, fmt.Sprintf("select %s from %s where %s = ? and %s = ?", strings.Join(w.columnNames, ","), w.tableName, w.nameColumn, w.basetimeColumn), tag, recentTime)
		if row.Err() != nil {
			ret = append(ret, WatchData{"error": row.Err().Error()})
			break
		}
		var values = w.makeBuffer()
		if err := row.Scan(values...); err != nil {
			ret = append(ret, WatchData{"error": row.Err().Error()})
			break
		}
		obj := WatchData{}
		for i, n := range w.columnNames {
			obj[n] = values[i]
		}
		ret = append(ret, obj)
	}
	return ret
}

func (w *Watcher) executeLog(ctx *gin.Context, conn api.Conn) []WatchData {
	ret := []WatchData{}

	if w.lastArrivalTime.IsZero() {
		if row := conn.QueryRow(ctx, fmt.Sprintf("select max(_ARRIVAL_TIME) from %s", w.tableName)); row.Err() != nil {
			ret = append(ret, WatchData{"error": row.Err().Error()})
			return ret
		} else {
			if err := row.Scan(&w.lastArrivalTime); err != nil {
				ret = append(ret, WatchData{"error": err.Error()})
				return ret
			}
		}
	}
	columns := "_ARRIVAL_TIME," + strings.Join(w.columnNames, ",")
	rows, err := conn.Query(ctx,
		fmt.Sprintf(`select /*+ SCAN_FORWARD(%s) */ %s from %s where _ARRIVAL_TIME > ?`, w.tableName, columns, w.tableName),
		w.lastArrivalTime.UnixNano(),
	)
	if err != nil {
		ret = append(ret, WatchData{"error": err.Error()})
		return ret
	}
	rowNum := 0
	for rows.Next() {
		rowNum++
		if rowNum > 200 {
			ret = append(ret, WatchData{"error": "too many rows, omit the rest"})
			w.lastArrivalTime = time.Time{}
			break
		}
		var values = w.makeBuffer()
		values = append([]any{new(time.Time)}, values...) // for _ARRIVAL_TIME
		if err := rows.Scan(values...); err != nil {
			ret = append(ret, WatchData{"error": err.Error()})
			return ret
		}
		arrivalTime := values[0].(*time.Time)
		values = values[1:]
		obj := WatchData{}
		for i, n := range w.columnNames {
			obj[n] = values[i]
		}
		ret = append(ret, obj)
		fmt.Println("WatchData", arrivalTime, obj)
		w.lastArrivalTime = *arrivalTime
	}
	return ret
}
