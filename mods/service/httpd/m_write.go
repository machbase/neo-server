package httpd

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/machbase/neo-server/mods/util/ymds"
	spi "github.com/machbase/neo-spi"
)

type lakeReq interface {
	lakeRequest()
	count() int
	reset()
	next() ([]any, error)
}

var _ lakeReq = &lakeDefaultReq{}
var _ lakeReq = &lakeStandardReq{}

type lakeDefaultValue struct {
	Tag string
	Ts  int64
	Val float64
}

type lakeDefaultReq struct {
	Values []*lakeDefaultValue `json:"values"`
	cursor int                 `json:"-"`
}

func (r *lakeDefaultReq) lakeRequest() {}
func (r *lakeDefaultReq) count() int   { return len(r.Values) }
func (r *lakeDefaultReq) reset()       { r.cursor = 0 }

func (r *lakeDefaultReq) next() ([]any, error) {
	if r.cursor >= len(r.Values) {
		return nil, io.EOF
	}
	r.cursor++
	v := r.Values[r.cursor-1]
	return []any{v.Tag, v.Ts, v.Val}, nil
}

type lakeStandardValue [2]any

type lakeStandardReq struct {
	Tag        string              `json:"tag_name"`
	Dateformat string              `json:"date_format"`
	Values     []lakeStandardValue `json:"values"`
	cursor     int                 `json:"-"`
	timeParser *ymds.Parser        `json:"-"`
}

func (r *lakeStandardReq) lakeRequest() {}
func (r *lakeStandardReq) count() int   { return len(r.Values) }
func (r *lakeStandardReq) reset()       { r.cursor = 0 }

func (r *lakeStandardReq) next() ([]any, error) {
	if r.cursor >= len(r.Values) {
		return nil, io.EOF
	}
	var ts time.Time
	var val float64
	var err error

	switch tv := r.Values[r.cursor][0].(type) {
	case string:
		ts, err = r.timeParser.Parse(tv)
		if err != nil {
			return nil, fmt.Errorf("values[%d] has wrong timeformat %q, format:%q", r.cursor, tv, r.Dateformat)
		}
	case int64:
		ts = time.Unix(0, tv)
	default:
		return nil, fmt.Errorf("values[%d] has wrong time in %T (%v)", r.cursor, tv, tv)
	}
	switch vs := r.Values[r.cursor][1].(type) {
	case float64:
		val = vs
	case int:
		val = float64(vs)
	default:
		return nil, fmt.Errorf("values[%d] has wrong value in %T (%v)", r.cursor, vs, vs)
	}
	r.cursor++
	return []any{r.Tag, ts, val}, nil
}

type lakeRsp struct {
	Success bool        `json:"success"`
	Reason  string      `json:"reason"`
	Data    interface{} `json:"data,omitempty"`
}

const TableName = "TAG"

func (svr *httpd) handleLakePostValues(ctx *gin.Context) {
	rsp := lakeRsp{Success: false}

	// 기존 lake에서는 cli를 통해서 db 사용
	dataType := ctx.Param("type")

	var req lakeReq
	var err error

	switch dataType {
	case "standard":
		stdReq := lakeStandardReq{}
		err = ctx.Bind(&stdReq)
		if stdReq.Dateformat == "" {
			stdReq.Dateformat = `YYYY-MM-DD HH24:MI:SS mmm:uuu:nnn`
		}
		stdReq.timeParser = ymds.NewParser(stdReq.Dateformat).WithLocation(time.Local)
		req = &stdReq
	default:
		defReq := lakeDefaultReq{}
		err = ctx.Bind(&defReq)
		req = &defReq
	}

	if err != nil {
		rsp.Reason = err.Error()
		ctx.JSON(http.StatusPreconditionFailed, rsp)
		return
	}

	if req.count() == 0 {
		rsp.Reason = "values is empty"
		ctx.JSON(http.StatusPreconditionFailed, rsp)
		return
	}

	// TODO: 1. change it to svr.getUserConnection()
	// TODO: 2. Appender should take care of multiple session and terminated at the end.
	conn, err := svr.getTrustConnection(ctx)
	if err != nil {
		rsp.Reason = err.Error()
		ctx.JSON(http.StatusUnauthorized, rsp)
		return
	}
	defer conn.Close()

	appender, err := conn.Appender(ctx, "TAG")
	if err != nil {
		svr.log.Error("appender error: ", err)
		return
	}
	defer appender.Close()

	defer func() {
		succ, fail, err := appender.Close()
		data := map[string]any{
			"success": succ,
			"fail":    fail,
		}
		if err != nil {
			data["close"] = err.Error()
		}
		rsp.Data = data
		if rsp.Success {
			ctx.JSON(http.StatusOK, rsp)
		} else {
			ctx.JSON(http.StatusInternalServerError, rsp)
		}
	}()

	for {
		data, err := req.next()
		if err != nil {
			if err == io.EOF {
				break
			} else {
				rsp.Reason = err.Error()
				return
			}
		}
		err = appender.Append(data...)
		if err != nil {
			rsp.Reason = err.Error()
			return
		}
	}

	rsp.Success = true
	rsp.Reason = "success"
}

type Query struct {
	Sql string `json:"query"`
}

func (svr *httpd) handleLakeExecQuery(ctx *gin.Context) {
	rsp := ResSet{Status: "fail"}
	query := Query{}

	svr.log.Trace("start ExecQuery()")

	err := ctx.Bind(&query)
	if err != nil {
		svr.log.Info("data bind error: ", err.Error())
		rsp.Data = map[string]interface{}{"title": "data is wrong. check data."}
		ctx.JSON(http.StatusPreconditionFailed, rsp)
		return
	}
	svr.log.Debugf("request data : %+v", query)

	conn, err := svr.getTrustConnection(ctx)
	if err != nil {
		rsp.Message = err.Error()
		ctx.JSON(http.StatusBadRequest, rsp)
		return
	}
	defer conn.Close()

	result, err := svr.getExec(ctx, conn, query.Sql)
	if err != nil {
		svr.log.Info("get data error : ", err.Error())
		rsp.Message = err.Error()
		ctx.JSON(http.StatusFailedDependency, rsp)
		return
	}

	rsp.Status = "success"
	rsp.Data = result

	ctx.JSON(http.StatusOK, rsp)
}

type ExecResult struct {
	Columns      []MachbaseColumn         `json:"columns"`
	Data         []map[string]interface{} `json:"data"`
	ErrorCode    int                      `json:"error_code"`
	ErrorMessage string                   `json:"error_message"`
}

type ExecData struct {
	Name  string  `json:"name"`
	Time  string  `json:"time"`
	Value float64 `json:"value"`
}

func (svr *httpd) getExec(ctx context.Context, conn spi.Conn, sqlText string) (*ExecResult, error) {
	result := &ExecResult{}
	rows, err := conn.Query(ctx, sqlText)
	if err != nil {
		return result, err
	}

	cols, err := rows.Columns()
	if err != nil {
		return result, err
	}
	colsLen := len(cols.Names())
	colsList := make([]MachbaseColumn, colsLen)

	wg := sync.WaitGroup{}
	wg.Add(1)

	go func() {
		defer wg.Done()

		for idx, col := range cols {
			colsList[idx].Name = col.Name
			// colsList[idx].Type = col.Type
			colsList[idx].Type = ColumnTypeConvert(col.Type)
			colsList[idx].Length = col.Length
		}
	}()

	for rows.Next() { // scale 적용을 어떻게 할 건가, 컬럼 여러개일때 value 컬럼을 찾아서 처리가 가능한가? ( rows.columns 으로 순서 확인 가능? )
		buffer := cols.MakeBuffer()
		err = rows.Scan(buffer...)
		if err != nil {
			svr.log.Warn("scan error : ", err.Error())
			return result, err
		}

		mv := map[string]any{}
		mv["name"] = buffer[0]
		mv["time"] = buffer[1]
		mv["value"] = buffer[2]
		result.Data = append(result.Data, mv)
	}

	wg.Wait()

	result.Columns = colsList

	return result, nil
}
