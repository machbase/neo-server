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
// @Router      /db/tables [get]
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
		Rows: [][]any{},
	}

	rownum := 1
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
		data.Rows = append(data.Rows, []any{
			rownum,
			ti.Database,
			ti.User,
			ti.Name,
			do.TableTypeDescription(spi.TableType(ti.Type), ti.Flag),
		})
		rownum++
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
