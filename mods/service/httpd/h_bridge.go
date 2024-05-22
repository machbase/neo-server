package httpd

import (
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	bridgerpc "github.com/machbase/neo-server/api/bridge"
	"github.com/machbase/neo-server/mods/bridge"
)

func (svr *httpd) handleListBridge(ctx *gin.Context) {
	tick := time.Now()
	rsp := gin.H{"success": false, "reason": "not specified"}

	listRsp, err := svr.bridgeMgmtImpl.ListBridge(ctx, &bridgerpc.ListBridgeRequest{})
	if err != nil {
		rsp["reason"] = err.Error()
		rsp["elapse"] = time.Since(tick).String()
		ctx.JSON(http.StatusInternalServerError, rsp)
		return
	}
	if !listRsp.Success {
		rsp["reason"] = listRsp.Reason
		rsp["elapse"] = time.Since(tick).String()
		ctx.JSON(http.StatusInternalServerError, rsp)
		return
	}

	sort.Slice(listRsp.Bridges, func(i, j int) bool { return listRsp.Bridges[i].Name < listRsp.Bridges[j].Name })

	rsp["success"] = true
	rsp["reason"] = "success"
	rsp["data"] = listRsp.Bridges
	rsp["elapse"] = time.Since(tick).String()
	ctx.JSON(http.StatusOK, rsp)
}

func (svr *httpd) handleAddBridge(ctx *gin.Context) {
	tick := time.Now()
	rsp := gin.H{"success": false, "reason": "not specified"}
	req := struct {
		Name string `json:"name"`
		Type string `json:"type"`
		Path string `json:"path"`
	}{}

	err := ctx.ShouldBind(&req)
	if err != nil {
		rsp["reason"] = err.Error()
		rsp["elapse"] = time.Since(tick).String()
		ctx.JSON(http.StatusBadRequest, rsp)
		return
	}

	getRsp, err := svr.bridgeMgmtImpl.GetBridge(ctx, &bridgerpc.GetBridgeRequest{
		Name: req.Name,
	})
	if err != nil {
		rsp["reason"] = err.Error()
		rsp["elapse"] = time.Since(tick).String()
		ctx.JSON(http.StatusInternalServerError, rsp)
		return
	}
	if getRsp.Success {
		rsp["reason"] = fmt.Sprintf("'%s' is duplicate bridge name.", req.Name)
		rsp["elapse"] = time.Since(tick).String()
		ctx.JSON(http.StatusBadRequest, rsp)
		return
	}

	addRsp, err := svr.bridgeMgmtImpl.AddBridge(ctx, &bridgerpc.AddBridgeRequest{
		Name: strings.ToLower(req.Name), Type: strings.ToLower(req.Type), Path: req.Path,
	})
	if err != nil {
		rsp["reason"] = err.Error()
		rsp["elapse"] = time.Since(tick).String()
		ctx.JSON(http.StatusInternalServerError, rsp)
		return
	}
	if !addRsp.Success {
		rsp["reason"] = addRsp.Reason
		rsp["elapse"] = time.Since(tick).String()
		ctx.JSON(http.StatusInternalServerError, rsp)
		return
	}

	rsp["success"] = true
	rsp["reason"] = "success"
	rsp["elapse"] = time.Since(tick).String()
	ctx.JSON(http.StatusOK, rsp)
}

func (svr *httpd) handleDeleteBridge(ctx *gin.Context) {
	tick := time.Now()
	rsp := gin.H{"success": false, "reason": "not specified"}

	name := ctx.Param("name")
	if name == "" {
		rsp["reason"] = "no name specified"
		rsp["elapse"] = time.Since(tick).String()
		ctx.JSON(http.StatusBadRequest, rsp)
		return
	}

	delRsp, err := svr.bridgeMgmtImpl.DelBridge(ctx, &bridgerpc.DelBridgeRequest{
		Name: strings.ToLower(name),
	})
	if err != nil {
		rsp["reason"] = err.Error()
		rsp["elapse"] = time.Since(tick).String()
		ctx.JSON(http.StatusInternalServerError, rsp)
		return
	}
	if !delRsp.Success {
		rsp["reason"] = delRsp.Reason
		rsp["elapse"] = time.Since(tick).String()
		ctx.JSON(http.StatusInternalServerError, rsp)
		return
	}

	rsp["success"] = true
	rsp["reason"] = "success"
	rsp["elapse"] = time.Since(tick).String()
	ctx.JSON(http.StatusOK, rsp)
}

type stateRequest struct {
	State   string `json:"state"`
	Command string `json:"command"`
	Name    string
}

func (svr *httpd) handleStateBridge(ctx *gin.Context) {
	tick := time.Now()
	rsp := gin.H{"success": false, "reason": "not specified"}

	name := ctx.Param("name")
	if name == "" {
		rsp["reason"] = "no name specified"
		rsp["elapse"] = time.Since(tick).String()
		ctx.JSON(http.StatusBadRequest, rsp)
		return
	}

	req := &stateRequest{Name: name}
	err := ctx.ShouldBind(req)
	if err != nil {
		rsp["reason"] = err.Error()
		rsp["elapse"] = time.Since(tick).String()
		ctx.JSON(http.StatusBadRequest, rsp)
		return
	}

	switch strings.ToLower(req.State) {
	case "exec":
		execBridge(svr, ctx, req)
	case "query":
		queryBridge(svr, ctx, req)
	case "test":
		testBridge(svr, ctx, req)
	default:
		rsp["reason"] = fmt.Sprintf("invalid state '%s'", req.State)
		rsp["elapse"] = time.Since(tick).String()
		ctx.JSON(http.StatusBadRequest, rsp)
		return
	}
}

func execBridge(svr *httpd, ctx *gin.Context, req *stateRequest) {
	tick := time.Now()
	rsp := gin.H{"success": false, "reason": "not specified"}

	// bridge type find
	getRsp, err := svr.bridgeMgmtImpl.GetBridge(ctx, &bridgerpc.GetBridgeRequest{
		Name: req.Name,
	})
	if err != nil {
		rsp["reason"] = err.Error()
		rsp["elapse"] = time.Since(tick).String()
		ctx.JSON(http.StatusInternalServerError, rsp)
		return
	}
	if !getRsp.Success {
		rsp["reason"] = getRsp.Reason
		rsp["elapse"] = time.Since(tick).String()
		ctx.JSON(http.StatusInternalServerError, rsp)
		return
	}

	brType := getRsp.Bridge.Type
	if brType == "" {
		rsp["reason"] = "bridge type is empty"
		rsp["elapse"] = time.Since(tick).String()
		ctx.JSON(http.StatusBadRequest, rsp)
		return
	}

	switch brType {
	case "python":
		cmd := &bridgerpc.ExecRequest_Invoke{Invoke: &bridgerpc.InvokeRequest{}}
		cmd.Invoke.Args = []string{req.Command}
		execRsp, err := svr.bridgeRuntimeImpl.Exec(ctx, &bridgerpc.ExecRequest{Name: req.Name, Command: cmd})
		result := execRsp.GetInvokeResult()
		if result != nil && len(result.Stdout) > 0 {
			rsp["stdout"] = string(result.Stdout)
		}
		if result != nil && len(result.Stderr) > 0 {
			rsp["stderr"] = string(result.Stderr)
		}
		if err != nil {
			rsp["reason"] = err.Error()
			rsp["elapse"] = time.Since(tick).String()
			ctx.JSON(http.StatusInternalServerError, rsp)
			return
		}
		if !execRsp.Success {
			rsp["reason"] = execRsp.Reason
			rsp["elapse"] = time.Since(tick).String()
			ctx.JSON(http.StatusInternalServerError, rsp)
			return
		}
	default:
		cmd := &bridgerpc.ExecRequest_SqlExec{SqlExec: &bridgerpc.SqlRequest{}}
		cmd.SqlExec.SqlText = req.Command
		execRsp, err := svr.bridgeRuntimeImpl.Exec(ctx, &bridgerpc.ExecRequest{Name: req.Name, Command: cmd})
		if err != nil {
			rsp["reason"] = err.Error()
			rsp["elapse"] = time.Since(tick).String()
			ctx.JSON(http.StatusInternalServerError, rsp)
			return
		}
		if !execRsp.Success {
			rsp["reason"] = execRsp.Reason
			rsp["elapse"] = time.Since(tick).String()
			ctx.JSON(http.StatusInternalServerError, rsp)
			return
		}
		result := execRsp.GetSqlExecResult()
		if result == nil {
			rsp["reason"] = "exec result is empty"
			rsp["elapse"] = time.Since(tick).String()
			ctx.JSON(http.StatusInternalServerError, rsp)
			return
		}
	}

	rsp["success"] = true
	rsp["reason"] = "success"
	rsp["elapse"] = time.Since(tick).String()
	ctx.JSON(http.StatusOK, rsp)
}

func queryBridge(svr *httpd, ctx *gin.Context, req *stateRequest) {
	tick := time.Now()
	rsp := gin.H{"success": false, "reason": "not specified"}

	if req.Command == "" {
		rsp["reason"] = "no command specified"
		rsp["elapse"] = time.Since(tick).String()
		ctx.JSON(http.StatusBadRequest, rsp)
		return
	}

	cmd := &bridgerpc.ExecRequest_SqlQuery{SqlQuery: &bridgerpc.SqlRequest{}}
	cmd.SqlQuery.SqlText = req.Command

	execRsp, err := svr.bridgeRuntimeImpl.Exec(ctx, &bridgerpc.ExecRequest{Name: req.Name, Command: cmd})
	if err != nil {
		rsp["reason"] = err.Error()
		rsp["elapse"] = time.Since(tick).String()
		ctx.JSON(http.StatusInternalServerError, rsp)
		return
	}
	if !execRsp.Success {
		rsp["reason"] = execRsp.Reason
		rsp["elapse"] = time.Since(tick).String()
		ctx.JSON(http.StatusInternalServerError, rsp)
		return
	}

	result := execRsp.GetSqlQueryResult()
	defer svr.bridgeRuntimeImpl.SqlQueryResultClose(ctx, result)

	if execRsp.Result != nil && len(result.Fields) == 0 {
		rsp["success"] = true
		rsp["reason"] = "0 rows"
		ctx.JSON(http.StatusOK, rsp)
		return
	}

	column := []string{}
	for _, col := range result.Fields {
		column = append(column, col.Name)
	}

	rows := [][]any{}
	rownum := 0
	for {
		fetch, err0 := svr.bridgeRuntimeImpl.SqlQueryResultFetch(ctx, result)
		if err0 != nil {
			err = err0
			break
		}
		if !fetch.Success {
			err = fmt.Errorf("fetch failed; %s", fetch.Reason)
			break
		}
		if fetch.HasNoRows {
			break
		}
		rownum++
		vals, err0 := bridge.ConvertFromDatum(fetch.Values...)
		if err0 != nil {
			err = err0
			break
		}
		rows = append(rows, vals)
	}

	if err != nil {
		rsp["reason"] = err.Error()
		rsp["elapse"] = time.Since(tick).String()
		ctx.JSON(http.StatusInternalServerError, rsp)
		return
	}

	rsp["success"] = true
	rsp["reason"] = "success"
	rsp["data"] = map[string]interface{}{
		"column": column,
		"rows":   rows,
	}
	rsp["elapse"] = time.Since(tick).String()
	ctx.JSON(http.StatusOK, rsp)
}

func testBridge(svr *httpd, ctx *gin.Context, req *stateRequest) {
	tick := time.Now()
	rsp := gin.H{"success": false, "reason": "not specified"}

	testRsp, err := svr.bridgeMgmtImpl.TestBridge(ctx, &bridgerpc.TestBridgeRequest{
		Name: req.Name,
	})
	if err != nil {
		rsp["reason"] = err.Error()
		rsp["elapse"] = time.Since(tick).String()
		ctx.JSON(http.StatusInternalServerError, rsp)
		return
	}
	if !testRsp.Success {
		rsp["reason"] = testRsp.Reason
		rsp["elapse"] = time.Since(tick).String()
		ctx.JSON(http.StatusInternalServerError, rsp)
		return
	}

	rsp["success"] = true
	rsp["reason"] = "success"
	rsp["elapse"] = time.Since(tick).String()
	ctx.JSON(http.StatusOK, rsp)
}
