package httpd

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	mach "github.com/machbase/neo-engine"
	"github.com/machbase/neo-server/api"
	"github.com/machbase/neo-server/mods/do"
	"github.com/machbase/neo-server/mods/service/msg"
	"github.com/machbase/neo-server/mods/util/glob"
)

// Get a list of existing tables
//
// @Summary     Get table list
// @Description Get table list
// @Param       name         query string false "table name prefix or glob pattern"
// @Param       showall      query boolean false "show all hidden tables"
// @Success     200  {object}  msg.QueryResponse
// @Failure     500 {object}  msg.QueryResponse
// @Router      /web/api/tables [get]
func (svr *httpd) handleTables(ctx *gin.Context) {
	tick := time.Now()
	nameFilter := strings.ToUpper(ctx.Query("name"))
	nameFilterGlob := false
	showAll := strBool(ctx.Query("showall"), false)

	if nameFilter != "" {
		nameFilterGlob = glob.IsGlob(nameFilter)
	}

	rsp := &msg.QueryResponse{Success: true, Reason: "success"}
	data := &msg.QueryData{
		Columns: []string{"ROWNUM", "DB", "USER", "NAME", "TYPE"},
		Types: []string{
			mach.ColumnBufferTypeInt32,  // rownum
			mach.ColumnBufferTypeString, // db
			mach.ColumnBufferTypeString, // user
			mach.ColumnBufferTypeString, // name
			mach.ColumnBufferTypeString, // type
		},
	}

	conn, err := svr.getUserConnection(ctx)
	if err != nil {
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}
	defer conn.Close()

	rownum := 0
	do.Tables(ctx, conn, func(ti *do.TableInfo, err error) bool {
		if err != nil {
			rsp.Success, rsp.Reason = false, err.Error()
			return false
		}
		if nameFilter != "" {
			if nameFilterGlob {
				matched, err := glob.Match(nameFilter, ti.Name)
				if err != nil {
					rsp.Success, rsp.Reason = false, err.Error()
					return false
				}
				if !matched {
					return true
				}
			} else if !strings.HasPrefix(ti.Name, nameFilter) {
				return true
			}
		}
		if !showAll {
			if strings.HasPrefix(ti.Name, "_") {
				return true
			}
		}
		rownum++
		data.Rows = append(data.Rows, []any{
			rownum,
			ti.Database,
			ti.User,
			ti.Name,
			do.TableTypeDescription(api.TableType(ti.Type), ti.Flag),
		})
		return true
	})

	rsp.Elapse = time.Since(tick).String()
	if rsp.Success {
		rsp.Data = data
		ctx.JSON(http.StatusOK, rsp)
	} else {
		ctx.JSON(http.StatusInternalServerError, rsp)
	}
}

// Get tag names of the given table
//
// @Summary     Get tag list of the table
// @Description Get tag list of the table
// @Param       table         path string true "table name"
// @Param       name          query string false "tag name filter"
// @Success     200  {object}  msg.QueryResponse
// @Failure     500 {object}  msg.QueryResponse
// @Router      /web/api/tables/:table/tags [get]
func (svr *httpd) handleTags(ctx *gin.Context) {
	tick := time.Now()
	table := strings.ToUpper(ctx.Param("table"))
	nameFilter := strings.ToUpper(ctx.Query("name"))

	rsp := &msg.QueryResponse{Success: true, Reason: "success"}
	data := &msg.QueryData{
		Columns: []string{"ROWNUM", "NAME"},
		Types: []string{
			mach.ColumnBufferTypeInt32,  // rownum
			mach.ColumnBufferTypeString, // name
		},
		Rows: [][]any{},
	}
	rownum := 0

	conn, err := svr.getUserConnection(ctx)
	if err != nil {
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}
	defer conn.Close()

	do.Tags(ctx, conn, table, func(name string, err error) bool {
		if err != nil {
			rsp.Success, rsp.Reason = false, err.Error()
			return false
		}
		if nameFilter != "" && !strings.HasPrefix(name, nameFilter) {
			return true
		}
		rownum++
		data.Rows = append(data.Rows, []any{
			rownum,
			name,
		})
		return true
	})

	rsp.Elapse = time.Since(tick).String()
	if rsp.Success {
		rsp.Data = data
		ctx.JSON(http.StatusOK, rsp)
	} else {
		ctx.JSON(http.StatusInternalServerError, rsp)
	}
}

// Get tag stat
//
// @Summary     Get tag stat
// @Description Get tag stat
// @Param       table         path string true "table name"
// @Param       tag           path string true "tag name"
// @Param       timeformat    query string false "timeformat (ns, us, ms, s, timeformat)"
// @Param       tz            query string false "timezone"
// @Success     200  {object}  msg.QueryResponse
// @Failure     500 {object}  msg.QueryResponse
// @Router      /web/api/tables/:table/tags/:tag/stat [get]
func (svr *httpd) handleTagStat(ctx *gin.Context) {
	tick := time.Now()
	rsp := &msg.QueryResponse{Success: true, Reason: "success"}
	table := strings.ToUpper(ctx.Param("table"))
	tag := ctx.Param("tag")
	timeformat := strString(ctx.Query("timeformat"), "ns")
	timeLocation := strTimeLocation(ctx.Query("tz"), time.UTC)

	conn, err := svr.getUserConnection(ctx)
	if err != nil {
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}
	defer conn.Close()

	nfo, err := do.TagStat(ctx, conn, table, tag)
	if err != nil {
		rsp.Success, rsp.Reason = false, err.Error()
		rsp.Elapse = time.Since(tick).String()
		ctx.JSON(http.StatusInternalServerError, rsp)
		return
	}

	data := &msg.QueryData{
		Columns: []string{
			"ROWNUM", "NAME", "ROW_COUNT", "MIN_TIME", "MAX_TIME",
			"MIN_VALUE", "MIN_VALUE_TIME", "MAX_VALUE", "MAX_VALUE_TIME", "RECENT_ROW_TIME"},
		Types: []string{
			mach.ColumnBufferTypeInt32,    // rownum
			mach.ColumnBufferTypeString,   // name
			mach.ColumnBufferTypeInt64,    // row_count
			mach.ColumnBufferTypeDatetime, // min_time
			mach.ColumnBufferTypeDatetime, // max_time
			mach.ColumnBufferTypeDouble,   // min_value
			mach.ColumnBufferTypeDatetime, // min_value_time
			mach.ColumnBufferTypeDouble,   // max_value
			mach.ColumnBufferTypeDatetime, // max_value_time
			mach.ColumnBufferTypeDatetime, // recent_row_time
		},
		Rows: [][]any{},
	}

	timeToJson := func(v time.Time) any {
		switch timeformat {
		case "ns":
			return v.UnixNano()
		case "ms":
			return v.UnixMilli()
		case "us":
			return v.UnixMicro()
		case "s":
			return v.Unix()
		default:
			return v.In(timeLocation).Format(timeformat)
		}
	}

	vs := []any{1, nfo.Name, nfo.RowCount}
	if nfo.MinTime.IsZero() {
		vs = append(vs, nil)
	} else {
		vs = append(vs, timeToJson(nfo.MinTime))
	}
	if nfo.MaxTime.IsZero() {
		vs = append(vs, nil)
	} else {
		vs = append(vs, timeToJson(nfo.MaxTime))
	}
	if nfo.MinValueTime.IsZero() {
		vs = append(vs, nil, nil)
	} else {
		vs = append(vs, nfo.MinValue, timeToJson(nfo.MinValueTime))
	}
	if nfo.MaxValueTime.IsZero() {
		vs = append(vs, nil, nil)
	} else {
		vs = append(vs, nfo.MaxValue, timeToJson(nfo.MaxValueTime))
	}
	if nfo.RecentRowTime.IsZero() {
		vs = append(vs, nil)
	} else {
		vs = append(vs, timeToJson(nfo.RecentRowTime))
	}
	data.Rows = append(data.Rows, vs)

	rsp.Elapse = time.Since(tick).String()
	rsp.Data = data
	ctx.JSON(http.StatusOK, rsp)
}
