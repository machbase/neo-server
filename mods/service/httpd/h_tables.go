package httpd

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	mach "github.com/machbase/neo-engine"
	"github.com/machbase/neo-server/mods/do"
	"github.com/machbase/neo-server/mods/service/msg"
	spi "github.com/machbase/neo-spi"
)

// Get a list of existing tables
//
// @Summary     Get table list
// @Description Get table list
// @Param       name         query string false "table name prefix"
// @Param       showall      query boolean false "show all hidden tables"
// @Success     200  {object}  msg.QueryResponse
// @Failure     500 {object}  msg.QueryResponse
// @Router      /web/api/tables [get]
func (svr *httpd) handleTables(ctx *gin.Context) {
	tick := time.Now()
	nameFilter := strings.ToUpper(ctx.Query("name"))
	showAll := strBool(ctx.Query("showall"), false)

	rsp := &msg.QueryResponse{Success: true, Reason: "success"}
	data := &msg.QueryData{
		Columns: []string{"ROWNUM", "DB", "USER", "NAME", "TYPE"},
		Types: []string{
			mach.ColumnTypeNameInt32,  // rownum
			mach.ColumnTypeNameString, // db
			mach.ColumnTypeNameString, // user
			mach.ColumnTypeNameString, // name
			mach.ColumnTypeNameString, // type
		},
	}

	rownum := 0
	do.Tables(svr.db, func(ti *do.TableInfo, err error) bool {
		if err != nil {
			rsp.Success, rsp.Reason = false, err.Error()
			return false
		}
		if nameFilter != "" && !strings.HasPrefix(ti.Name, nameFilter) {
			return true
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
			do.TableTypeDescription(spi.TableType(ti.Type), ti.Flag),
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
			mach.ColumnTypeNameInt32,  // rownum
			mach.ColumnTypeNameString, // name
		},
		Rows: [][]any{},
	}
	rownum := 0
	do.Tags(svr.db, table, func(name string, err error) bool {
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
// @Success     200  {object}  msg.QueryResponse
// @Failure     500 {object}  msg.QueryResponse
// @Router      /web/api/tables/:table/tags/:tag/stat [get]
func (svr *httpd) handleTagStat(ctx *gin.Context) {
	tick := time.Now()
	rsp := &msg.QueryResponse{Success: true, Reason: "success"}
	table := strings.ToUpper(ctx.Param("table"))
	tag := ctx.Param("tag")

	nfo, err := do.TagStat(svr.db, table, tag)
	if err != nil {
		rsp.Success, rsp.Reason = false, err.Error()
		ctx.JSON(http.StatusInternalServerError, rsp)
		return
	}

	data := &msg.QueryData{
		Columns: []string{
			"ROWNUM", "NAME", "ROW_COUNT", "MIN_TIME", "MAX_TIME",
			"MIN_VALUE", "MIN_VALUE_TIME", "MAX_VALUE", "MAX_VALUE_TIME", "RECENT_ROW_TIME"},
		Types: []string{
			mach.ColumnTypeNameInt32,    // rownum
			mach.ColumnTypeNameString,   // name
			mach.ColumnTypeNameInt64,    // row_count
			mach.ColumnTypeNameDatetime, // min_time
			mach.ColumnTypeNameDatetime, // max_time
			mach.ColumnTypeNameDouble,   // min_value
			mach.ColumnTypeNameDatetime, // min_value_time
			mach.ColumnTypeNameDouble,   // max_value
			mach.ColumnTypeNameDatetime, // max_value_time
			mach.ColumnTypeNameDatetime, // recent_row_time
		},
		Rows: [][]any{},
	}
	data.Rows = append(data.Rows, []any{
		1,
		nfo.Name, nfo.RowCount,
		nfo.MinTime, nfo.MaxTime,
		nfo.MinValue, nfo.MinValueTime,
		nfo.MaxValue, nfo.MaxValueTime,
		nfo.RecentRowTime,
	})

	rsp.Elapse = time.Since(tick).String()
	rsp.Data = data
	ctx.JSON(http.StatusOK, rsp)
}
