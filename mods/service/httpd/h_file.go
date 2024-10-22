package httpd

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gofrs/uuid/v5"
	"github.com/machbase/neo-server/api"
	"github.com/machbase/neo-server/api/types"
	"github.com/machbase/neo-server/mods/service/msg"
	"github.com/machbase/neo-server/mods/util"
)

func (svr *httpd) handleFileQuery(ctx *gin.Context) {
	rsp := &msg.QueryResponse{Success: false, Reason: "not specified"}
	tick := time.Now()

	tableName := ctx.Param("table")
	columnName := ctx.Param("column")
	fileID := ctx.Param("id")
	if len(tableName) == 0 || len(columnName) == 0 || len(fileID) == 0 ||
		strings.ContainsAny(tableName, "; \t\r\n()") ||
		strings.ContainsAny(columnName, "; \t\r\n()") {
		rsp.Reason = "invalid request"
		rsp.Elapse = time.Since(tick).String()
		ctx.JSON(http.StatusBadRequest, rsp)
		return
	}
	uid := uuid.UUID{}
	if err := uid.Parse(fileID); err != nil {
		rsp.Reason = fmt.Sprintf("invalid id, %s", err.Error())
		rsp.Elapse = time.Since(tick).String()
		ctx.JSON(http.StatusBadRequest, rsp)
		return
	}
	uidTs, err := uuid.TimestampFromV6(uid)
	if err != nil {
		rsp.Reason = fmt.Sprintf("bad timestamp id, %s", err.Error())
		rsp.Elapse = time.Since(tick).String()
		ctx.JSON(http.StatusBadRequest, rsp)
		return
	}
	ts, _ := uidTs.Time()

	conn, err := svr.getTrustConnection(ctx)
	if err != nil {
		rsp.Reason = err.Error()
		rsp.Elapse = time.Since(tick).String()
		ctx.JSON(http.StatusUnauthorized, rsp)
		return
	}
	defer conn.Close()

	var sqlText string
	var sqlParams []any
	if tableType, err := api.TableType(ctx, conn, tableName); err != nil {
		rsp.Reason = err.Error()
		rsp.Elapse = time.Since(tick).String()
		ctx.JSON(http.StatusInternalServerError, rsp)
		return
	} else if tableType == types.TableTypeLog {
		sqlText = fmt.Sprintf("SELECT %s FROM %s WHERE _ARRIVAL_TIME BETWEEN ? and ? and %s->'$.ID' = ?",
			columnName, tableName, columnName)
		sqlParams = append(sqlParams, ts.Add(-2*time.Second).UnixNano())
		sqlParams = append(sqlParams, ts.Add(3*time.Second).UnixNano())
		sqlParams = append(sqlParams, fileID)
	} else if tableType == types.TableTypeTag {
		var desc *api.TableDescription
		if desc0, err := api.DescribeTable(ctx, conn, tableName, false); err != nil {
			rsp.Reason = fmt.Sprintf("fail to get table info '%s', %s", tableName, err.Error())
			rsp.Elapse = time.Since(tick).String()
			ctx.JSON(http.StatusInternalServerError, rsp)
			return
		} else {
			desc = desc0
		}
		basetimeColumn := "TIME"
		nameColumn := "NAME"
		tagName := ctx.Query("tag")

		for _, c := range desc.Columns {
			if c.IsBaseTime() {
				basetimeColumn = c.Name
			} else if c.IsTagName() {
				nameColumn = c.Name
			}
		}
		if len(tagName) == 0 || strings.ContainsAny(tagName, "; \t\r\n()") {
			sqlText = fmt.Sprintf("SELECT %s FROM %s WHERE %s BETWEEN ? AND ? AND %s->'$.ID' = ?",
				columnName, tableName, basetimeColumn, columnName)
		} else {
			sqlText = fmt.Sprintf("SELECT %s FROM %s WHERE %s = ? AND %s BETWEEN ? AND ? AND %s->'$.ID' = ?",
				columnName, tableName, nameColumn, basetimeColumn, columnName)
			sqlParams = append(sqlParams, tagName)
		}
		sqlParams = append(sqlParams, ts.Add(-2*time.Second).UnixNano())
		sqlParams = append(sqlParams, ts.Add(3*time.Second).UnixNano())
		sqlParams = append(sqlParams, fileID)
	} else {
		rsp.Reason = fmt.Sprintf("Table '%s' is does not supported for files", tableName)
		rsp.Elapse = time.Since(tick).String()
		ctx.JSON(http.StatusNotFound, rsp)
		return
	}

	row := conn.QueryRow(ctx, sqlText, sqlParams...)
	if row.Err() != nil {
		rsp.Reason = row.Err().Error()
		rsp.Elapse = time.Since(tick).String()
		ctx.JSON(http.StatusInternalServerError, rsp)
		return
	}

	var fileDataStr string
	if err := row.Scan(&fileDataStr); err != nil {
		rsp.Reason = err.Error()
		rsp.Elapse = time.Since(tick).String()
		ctx.JSON(http.StatusInternalServerError, rsp)
		return
	}

	fileData := &msg.UserFileData{}
	if err := json.Unmarshal([]byte(fileDataStr), fileData); err != nil {
		rsp.Reason = err.Error()
		rsp.Elapse = time.Since(tick).String()
		ctx.JSON(http.StatusInternalServerError, rsp)
		return
	}

	if fileData.ContentType != "" {
		ctx.Header("Content-Type", fileData.ContentType)
	} else {
		ctx.Header("Content-Type", "application/octet-stream")
	}
	if fileData.Filename != "" {
		ctx.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", fileData.Filename))
	} else {
		ctx.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", fileData.Id))
	}
	http.ServeFile(ctx.Writer, ctx.Request, filepath.Join(fileData.StoreDir, fileData.Id))
}

func (svr *httpd) handleFileWrite(ctx *gin.Context) {
	rsp := &msg.WriteResponse{Reason: "not specified"}
	tick := time.Now()

	if ctx.ContentType() != "multipart/form-data" {
		rsp.Reason = "content-type must be 'multipart/form-data'"
		rsp.Elapse = time.Since(tick).String()
		ctx.JSON(http.StatusInternalServerError, rsp)
		return
	}

	tableName := ctx.Param("table")
	timeformat := strString(ctx.Query("timeformat"), "ns")
	timeLocation, err := util.ParseTimeLocation(ctx.Query("tz"), time.UTC)
	if err != nil {
		rsp.Reason = err.Error()
		rsp.Elapse = time.Since(tick).String()
		ctx.JSON(http.StatusBadRequest, rsp)
		return
	}

	conn, err := svr.getTrustConnection(ctx)
	if err != nil {
		rsp.Reason = err.Error()
		rsp.Elapse = time.Since(tick).String()
		ctx.JSON(http.StatusUnauthorized, rsp)
		return
	}
	defer conn.Close()

	tableType, err := api.TableType(ctx, conn, tableName)
	if err != nil {
		rsp.Reason = err.Error()
		rsp.Elapse = time.Since(tick).String()
		ctx.JSON(http.StatusInternalServerError, rsp)
		return
	}
	if tableType != types.TableTypeLog && tableType != types.TableTypeTag {
		rsp.Reason = fmt.Sprintf("Table '%s' is does not supported for files", tableName)
		rsp.Elapse = time.Since(tick).String()
		ctx.JSON(http.StatusNotFound, rsp)
		return
	}

	var desc *api.TableDescription
	if desc0, err := api.DescribeTable(ctx, conn, tableName, false); err != nil {
		rsp.Reason = fmt.Sprintf("fail to get table info '%s', %s", tableName, err.Error())
		rsp.Elapse = time.Since(tick).String()
		ctx.JSON(http.StatusInternalServerError, rsp)
		return
	} else {
		desc = desc0
	}

	findColumn := func(name string) *types.Column {
		for _, c := range desc.Columns {
			if strings.EqualFold(c.Name, name) {
				return c
			}
		}
		return nil
	}

	form, err := ctx.MultipartForm()
	if err != nil {
		rsp.Reason = err.Error()
		rsp.Elapse = time.Since(tick).String()
		ctx.JSON(http.StatusBadRequest, rsp)
		return
	}

	ts := time.Now()
	columns := []string{}
	values := []any{}
	for k, v := range form.Value {
		if c := findColumn(k); c != nil {
			columns = append(columns, c.Name)
			val, err := c.Type.DataType().Apply(v[0], timeformat, timeLocation)
			if err != nil {
				rsp.Reason = err.Error()
				rsp.Elapse = time.Since(tick).String()
				ctx.JSON(http.StatusBadRequest, rsp)
				return
			}
			if tableType == types.TableTypeTag && c.IsBaseTime() {
				ts = val.(time.Time)
			}
			values = append(values, val)
		} else {
			rsp.Reason = fmt.Sprintf("column %q not found in the table %q", k, tableName)
			rsp.Elapse = time.Since(tick).String()
			ctx.JSON(http.StatusBadRequest, rsp)
			return
		}
	}

	// request header X-Store-Dir is used as default store directory
	defaultStoreDir := ctx.Request.Header.Get("X-Store-Dir")

	// store file fields
	for k, v := range form.File {
		if len(v) == 0 {
			continue
		}
		mff := &msg.UserFileData{
			Filename: v[0].Filename,
			Size:     v[0].Size,
		}

		// store file to the specified upload directory
		storeDir := defaultStoreDir

		if d := v[0].Header.Get("X-Store-Dir"); d != "" {
			storeDir = d
		}
		if storeDir == "" {
			rsp.Reason = fmt.Sprintf("file %q requires X-Store-Dir header", k)
			rsp.Elapse = time.Since(tick).String()
			ctx.JSON(http.StatusBadRequest, rsp)
			return
		}
		mff.StoreDir = storeDir
		idv6, _ := idGen.NewV6AtTime(ts)
		mff.Id = idv6.String()

		if c := v[0].Header.Get("Content-Type"); c != "" {
			mff.ContentType = c
		}
		mffJson, _ := json.Marshal(mff)
		columns = append(columns, k)
		values = append(values, string(mffJson))

		src, err := v[0].Open()
		if err != nil {
			rsp.Reason = err.Error()
			rsp.Elapse = time.Since(tick).String()
			ctx.JSON(http.StatusInternalServerError, rsp)
			return
		}
		defer src.Close()
		if err := os.MkdirAll(mff.StoreDir, 0755); err != nil {
			rsp.Reason = err.Error()
			rsp.Elapse = time.Since(tick).String()
			ctx.JSON(http.StatusInternalServerError, rsp)
			return
		}
		dst, err := os.OpenFile(filepath.Join(mff.StoreDir, mff.Id), os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
		if err != nil {
			rsp.Reason = err.Error()
			rsp.Elapse = time.Since(tick).String()
			ctx.JSON(http.StatusInternalServerError, rsp)
			return
		}
		defer dst.Close()
		if _, err := io.Copy(dst, src); err != nil {
			rsp.Reason = err.Error()
			rsp.Elapse = time.Since(tick).String()
			ctx.JSON(http.StatusInternalServerError, rsp)
			return
		}
		if rsp.Data == nil {
			rsp.Data = &msg.WriteResponseData{}
			rsp.Data.Files = make(map[string]*msg.UserFileData)
		}
		rsp.Data.Files[strings.ToUpper(k)] = mff
	}
	holders := make([]string, len(columns))
	for i := range holders {
		holders[i] = "?"
	}

	insertQuery := fmt.Sprintf("INSERT INTO %s(%s) VALUES(%s)", tableName, strings.Join(columns, ","), strings.Join(holders, ","))

	if result := conn.Exec(ctx, insertQuery, values...); result.Err() != nil {
		rsp.Reason = result.Err().Error()
		rsp.Elapse = time.Since(tick).String()
		ctx.JSON(http.StatusInternalServerError, rsp)
		return
	}

	rsp.Success, rsp.Reason = true, fmt.Sprintf("success, %d record(s) inserted", 1)
	rsp.Elapse = time.Since(tick).String()
	ctx.JSON(http.StatusOK, rsp)
}

var idGen = uuid.NewGen()
