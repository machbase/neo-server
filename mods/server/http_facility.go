package server

import (
	"archive/zip"
	"bytes"
	"fmt"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	bridgerpc "github.com/machbase/neo-server/v8/api/bridge"
	"github.com/machbase/neo-server/v8/api/mgmt"
	"github.com/machbase/neo-server/v8/api/schedule"
	"github.com/machbase/neo-server/v8/mods/bridge"
	"github.com/machbase/neo-server/v8/mods/model"
	"github.com/machbase/neo-server/v8/mods/util"
)

func (svr *httpd) handleTimer(ctx *gin.Context) {
	tick := time.Now()
	rsp := gin.H{"success": false, "reason": "not specified"}

	name := ctx.Param("name")
	getRsp, err := svr.schedMgmtImpl.GetSchedule(ctx, &schedule.GetScheduleRequest{
		Name: name,
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

	rsp["success"] = true
	rsp["reason"] = "success"
	rsp["data"] = getRsp.Schedule
	rsp["elapse"] = time.Since(tick).String()
	ctx.JSON(http.StatusOK, rsp)
}

func (svr *httpd) handleTimers(ctx *gin.Context) {
	tick := time.Now()
	rsp := gin.H{"success": false, "reason": "not specified"}

	listRsp, err := svr.schedMgmtImpl.ListSchedule(ctx, &schedule.ListScheduleRequest{})
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

	list := []*schedule.Schedule{}
	for _, c := range listRsp.Schedules {
		typ := strings.ToUpper(c.Type)
		if typ != "TIMER" {
			continue
		}
		list = append(list, c)
	}

	rsp["success"] = true
	rsp["reason"] = "success"
	rsp["data"] = list
	rsp["elapse"] = time.Since(tick).String()
	ctx.JSON(http.StatusOK, rsp)
}

func (svr *httpd) handleTimersAdd(ctx *gin.Context) {
	tick := time.Now()
	rsp := gin.H{"success": false, "reason": "not specified"}
	req := struct {
		Name      string `json:"name"`
		AutoStart bool   `json:"autoStart"`
		Schedule  string `json:"schedule"`
		Path      string `json:"path"`
	}{}

	err := ctx.ShouldBind(&req)
	if err != nil {
		rsp["reason"] = err.Error()
		rsp["elapse"] = time.Since(tick).String()
		ctx.JSON(http.StatusBadRequest, rsp)
		return
	}

	getRsp, err := svr.schedMgmtImpl.GetSchedule(ctx, &schedule.GetScheduleRequest{Name: req.Name})
	if err != nil {
		rsp["reason"] = err.Error()
		rsp["elapse"] = time.Since(tick).String()
		ctx.JSON(http.StatusInternalServerError, rsp)
		return
	}
	if getRsp.Success {
		rsp["reason"] = fmt.Sprintf("'%s' is duplicate name.", req.Name)
		rsp["elapse"] = time.Since(tick).String()
		ctx.JSON(http.StatusBadRequest, rsp)
		return
	}

	addRsp, err := svr.schedMgmtImpl.AddSchedule(ctx, &schedule.AddScheduleRequest{
		Name:      strings.ToLower(req.Name),
		Type:      "timer",
		AutoStart: req.AutoStart,
		Schedule:  req.Schedule,
		Task:      req.Path,
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

func (svr *httpd) handleTimersState(ctx *gin.Context) {
	tick := time.Now()
	rsp := gin.H{"success": false, "reason": "not specified"}

	name := ctx.Param("name")
	if name == "" {
		rsp["reason"] = "no name specified"
		rsp["elapse"] = time.Since(tick).String()
		ctx.JSON(http.StatusBadRequest, rsp)
		return
	}

	req := struct {
		State string `json:"state"`
	}{}
	err := ctx.ShouldBind(&req)
	if err != nil {
		rsp["reason"] = err.Error()
		rsp["elapse"] = time.Since(tick).String()
		ctx.JSON(http.StatusBadRequest, rsp)
		return
	}

	switch strings.ToUpper(req.State) {
	case "START":
		startRsp, err := svr.schedMgmtImpl.StartSchedule(ctx, &schedule.StartScheduleRequest{
			Name: name,
		})
		if err != nil {
			rsp["reason"] = err.Error()
			rsp["elapse"] = time.Since(tick).String()
			ctx.JSON(http.StatusInternalServerError, rsp)
			return
		}
		if !startRsp.Success {
			rsp["reason"] = startRsp.Reason
			rsp["elapse"] = time.Since(tick).String()
			ctx.JSON(http.StatusInternalServerError, rsp)
			return
		}
	case "STOP":
		stopRsp, err := svr.schedMgmtImpl.StopSchedule(ctx, &schedule.StopScheduleRequest{
			Name: name,
		})
		if err != nil {
			rsp["reason"] = err.Error()
			rsp["elapse"] = time.Since(tick).String()
			ctx.JSON(http.StatusInternalServerError, rsp)
			return
		}
		if !stopRsp.Success {
			rsp["reason"] = stopRsp.Reason
			rsp["elapse"] = time.Since(tick).String()
			ctx.JSON(http.StatusInternalServerError, rsp)
			return
		}
	default:
		rsp["reason"] = fmt.Sprintf("no state specified: '%s'", req.State)
		rsp["elapse"] = time.Since(tick).String()
		ctx.JSON(http.StatusBadRequest, rsp)
		return
	}

	rsp["success"] = true
	rsp["reason"] = "success"
	rsp["elapse"] = time.Since(tick).String()
	ctx.JSON(http.StatusOK, rsp)
}

func (svr *httpd) handleTimersUpdate(ctx *gin.Context) {
	tick := time.Now()
	rsp := gin.H{"success": false, "reason": "not specified"}
	req := struct {
		AutoStart bool   `json:"autoStart"`
		Schedule  string `json:"schedule"`
		Path      string `json:"path"`
	}{}

	name := ctx.Param("name")
	if name == "" {
		rsp["reason"] = "no name specified"
		rsp["elapse"] = time.Since(tick).String()
		ctx.JSON(http.StatusBadRequest, rsp)
		return
	}

	err := ctx.ShouldBind(&req)
	if err != nil {
		rsp["reason"] = err.Error()
		rsp["elapse"] = time.Since(tick).String()
		ctx.JSON(http.StatusBadRequest, rsp)
		return
	}

	getRsp, err := svr.schedMgmtImpl.GetSchedule(ctx, &schedule.GetScheduleRequest{
		Name: name,
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

	updateRsp, err := svr.schedMgmtImpl.UpdateSchedule(ctx, &schedule.UpdateScheduleRequest{
		Name:      name,
		AutoStart: req.AutoStart,
		Schedule:  req.Schedule,
		Task:      req.Path,
	})
	if err != nil {
		rsp["reason"] = err.Error()
		rsp["elapse"] = time.Since(tick).String()
		ctx.JSON(http.StatusInternalServerError, rsp)
		return
	}
	if !updateRsp.Success {
		rsp["reason"] = updateRsp.Reason
		rsp["elapse"] = time.Since(tick).String()
		ctx.JSON(http.StatusInternalServerError, rsp)
		return
	}

	rsp["success"] = true
	rsp["reason"] = "success"
	rsp["elapse"] = time.Since(tick).String()
	ctx.JSON(http.StatusOK, rsp)
}

func (svr *httpd) handleTimersDel(ctx *gin.Context) {
	tick := time.Now()
	rsp := gin.H{"success": false, "reason": "not specified"}

	name := ctx.Param("name")
	if name == "" {
		rsp["reason"] = "no name specified"
		rsp["elapse"] = time.Since(tick).String()
		ctx.JSON(http.StatusBadRequest, rsp)
		return
	}

	getRsp, err := svr.schedMgmtImpl.GetSchedule(ctx, &schedule.GetScheduleRequest{
		Name: name,
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
	if getRsp.Schedule.State == "RUNNING" {
		stopRsp, err := svr.schedMgmtImpl.StopSchedule(ctx, &schedule.StopScheduleRequest{
			Name: name,
		})
		if err != nil {
			rsp["reason"] = err.Error()
			rsp["elapse"] = time.Since(tick).String()
			ctx.JSON(http.StatusInternalServerError, rsp)
			return
		}
		if !stopRsp.Success {
			rsp["reason"] = stopRsp.Reason
			rsp["elapse"] = time.Since(tick).String()
			ctx.JSON(http.StatusInternalServerError, rsp)
			return
		}
	}

	deleteRsp, err := svr.schedMgmtImpl.DelSchedule(ctx, &schedule.DelScheduleRequest{
		Name: name,
	})
	if err != nil {
		rsp["reason"] = err.Error()
		rsp["elapse"] = time.Since(tick).String()
		ctx.JSON(http.StatusInternalServerError, rsp)
		return
	}
	if !deleteRsp.Success {
		rsp["reason"] = deleteRsp.Reason
		rsp["elapse"] = time.Since(tick).String()
		ctx.JSON(http.StatusInternalServerError, rsp)
		return
	}

	rsp["success"] = true
	rsp["reason"] = "success"
	rsp["elapse"] = time.Since(tick).String()
	ctx.JSON(http.StatusOK, rsp)
}

// GET /api/shell/:id  - get the id
func (svr *httpd) handleGetShell(ctx *gin.Context) {
	tick := time.Now()
	rsp := gin.H{"success": false, "reason": "not specified"}
	shellId := ctx.Param("id")
	if shellId == "" {
		rsp["reason"] = "no id specified"
		rsp["elapse"] = time.Since(tick).String()
		ctx.JSON(http.StatusBadRequest, rsp)
		return
	}
	shell, err := svr.webShellProvider.GetShell(shellId)
	if err != nil {
		rsp["reason"] = err.Error()
		rsp["elapse"] = time.Since(tick).String()
		ctx.JSON(http.StatusInternalServerError, rsp)
		return
	}
	if shell == nil {
		rsp["reason"] = "not found"
		rsp["elapse"] = time.Since(tick).String()
		ctx.JSON(http.StatusNotFound, rsp)
		return
	} else {
		rsp["success"] = true
		rsp["reason"] = "success"
		rsp["data"] = shell
		rsp["elapse"] = time.Since(tick).String()
		ctx.JSON(http.StatusOK, rsp)
	}
}

// GET /api/shell/:id/copy  - make a copy of the id
func (svr *httpd) handleGetShellCopy(ctx *gin.Context) {
	tick := time.Now()
	rsp := gin.H{"success": false, "reason": "not specified"}
	shellId := ctx.Param("id")
	if shellId == "" {
		rsp["reason"] = "no id specified"
		rsp["elapse"] = time.Since(tick).String()
		ctx.JSON(http.StatusBadRequest, rsp)
		return
	}
	shell, err := svr.webShellProvider.CopyShell(shellId)
	if err != nil {
		rsp["reason"] = err.Error()
		rsp["elapse"] = time.Since(tick).String()
		ctx.JSON(http.StatusInternalServerError, rsp)
		return
	}
	if shell == nil {
		rsp["reason"] = "not found"
		rsp["elapse"] = time.Since(tick).String()
		ctx.JSON(http.StatusNotFound, rsp)
		return
	} else {
		rsp["success"] = true
		rsp["reason"] = "success"
		rsp["data"] = shell
		rsp["elapse"] = time.Since(tick).String()
		ctx.JSON(http.StatusOK, rsp)
	}
}

// POST /api/shell/:id - update the label, content, icon of the shell by id
func (svr *httpd) handlePostShell(ctx *gin.Context) {
	tick := time.Now()
	rsp := gin.H{"success": false, "reason": "not specified"}
	shellId := ctx.Param("id")
	if shellId == "" {
		rsp["reason"] = "no id specified"
		rsp["elapse"] = time.Since(tick).String()
		svr.log.Debug("update shell def, no id specified")
		ctx.JSON(http.StatusBadRequest, rsp)
		return
	}
	shell := &model.ShellDefinition{}
	err := ctx.Bind(shell)
	if err != nil {
		rsp["reason"] = err.Error()
		rsp["elapse"] = time.Since(tick).String()
		svr.log.Debug("update shell def, invalid request", err.Error())
		ctx.JSON(http.StatusBadRequest, rsp)
		return
	}
	if err := svr.webShellProvider.SaveShell(shell); err != nil {
		rsp["reason"] = err.Error()
		rsp["elapse"] = time.Since(tick).String()
		svr.log.Debug("update shell def, internal err", err.Error())
		ctx.JSON(http.StatusInternalServerError, rsp)
		return
	} else {
		rsp["success"] = true
		rsp["reason"] = "success"
		rsp["data"] = shell
		rsp["elapse"] = time.Since(tick).String()
		ctx.JSON(http.StatusOK, rsp)
	}
}

// DELETE /api/shell/:id - delete shell by id
func (svr *httpd) handleDeleteShell(ctx *gin.Context) {
	tick := time.Now()
	rsp := gin.H{"success": false, "reason": "not specified"}
	shellId := ctx.Param("id")
	if shellId == "" {
		rsp["reason"] = "no id specified"
		rsp["elapse"] = time.Since(tick).String()
		ctx.JSON(http.StatusBadRequest, rsp)
		return
	}
	err := svr.webShellProvider.RemoveShell(shellId)
	if err != nil {
		rsp["reason"] = err.Error()
		rsp["elapse"] = time.Since(tick).String()
		ctx.JSON(http.StatusInternalServerError, rsp)
		return
	} else {
		rsp["success"] = true
		rsp["reason"] = "success"
		rsp["elapse"] = time.Since(tick).String()
		ctx.JSON(http.StatusOK, rsp)
	}
}

type KeyInfo struct {
	Idx       int    `json:"idx"`
	Id        string `json:"id"`
	NotBefore int64  `json:"notBefore"`
	NotAfter  int64  `json:"notAfter"`
}

func (svr *httpd) handleKeys(ctx *gin.Context) {
	tick := time.Now()
	rsp := gin.H{"success": false, "reason": "not specified"}

	mgmtRsp, err := svr.mgmtImpl.ListKey(ctx, &mgmt.ListKeyRequest{})
	if err != nil {
		rsp["reason"] = err.Error()
		rsp["elapse"] = time.Since(tick).String()
		ctx.JSON(http.StatusInternalServerError, rsp)
		return
	}
	if !mgmtRsp.Success {
		rsp["reason"] = mgmtRsp.Reason
		rsp["elapse"] = time.Since(tick).String()
		ctx.JSON(http.StatusInternalServerError, rsp)
		return
	}

	infoList := make([]KeyInfo, 0, len(mgmtRsp.Keys))
	for i, k := range mgmtRsp.Keys {
		info := KeyInfo{
			Idx:       i,
			Id:        k.Id,
			NotBefore: k.NotBefore,
			NotAfter:  k.NotAfter,
		}
		infoList = append(infoList, info)
	}

	rsp["success"] = true
	rsp["reason"] = "success"
	rsp["data"] = infoList
	rsp["elapse"] = time.Since(tick).String()
	ctx.JSON(http.StatusOK, rsp)
}

func (svr *httpd) handleKeysGen(ctx *gin.Context) {
	tick := time.Now()
	rsp := gin.H{"success": false, "reason": "not specified"}
	req := struct {
		Name      string `json:"name"`
		NotBefore int64  `json:"notBefore"`
		NotAfter  int64  `json:"notAfter"`
	}{}

	err := ctx.ShouldBind(&req)
	if err != nil {
		rsp["reason"] = err.Error()
		rsp["elapse"] = time.Since(tick).String()
		ctx.JSON(http.StatusBadRequest, rsp)
		return
	}

	// name duplicate
	listRsp, err := svr.mgmtImpl.ListKey(ctx, &mgmt.ListKeyRequest{})
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
	for _, key := range listRsp.Keys {
		if key.Id == req.Name {
			rsp["reason"] = fmt.Sprintf("'%s' is duplicate id.", req.Name)
			rsp["elapse"] = time.Since(tick).String()
			ctx.JSON(http.StatusBadRequest, rsp)
			return
		}
	}

	name := strings.ToLower(req.Name)
	pass, _ := regexp.MatchString("[a-z][a-z0-9_.@-]+", name)
	if !pass {
		rsp["reason"] = "id contains invalid letter, use only alphnum and _.@-"
		rsp["elapse"] = time.Since(tick).String()
		ctx.JSON(http.StatusBadRequest, rsp)
		return
	}

	if req.NotBefore == 0 {
		// req.NotBefore = time.Now().UnixNano() // client certificate: asn1: structure error: cannot represent time as GeneralizedTime 에러 발생
		req.NotBefore = time.Now().Unix()
	}
	if req.NotAfter <= req.NotBefore {
		req.NotAfter = time.Now().Add(10 * time.Hour * 24 * 365).Unix() // 10 years
	}

	genRsp, err := svr.mgmtImpl.GenKey(ctx, &mgmt.GenKeyRequest{
		Id:        name,
		Type:      "ec",
		NotBefore: req.NotBefore,
		NotAfter:  req.NotAfter,
	})
	if err != nil {
		rsp["reason"] = err.Error()
		rsp["elapse"] = time.Since(tick).String()
		ctx.JSON(http.StatusInternalServerError, rsp)
		return
	}
	if !genRsp.Success {
		rsp["reason"] = genRsp.Reason
		rsp["elapse"] = time.Since(tick).String()
		ctx.JSON(http.StatusInternalServerError, rsp)
		return
	}

	serverRsp, err := svr.mgmtImpl.ServerKey(ctx, &mgmt.ServerKeyRequest{})
	if err != nil {
		rsp["reason"] = err.Error()
		rsp["elapse"] = time.Since(tick).String()
		ctx.JSON(http.StatusInternalServerError, rsp)
		return
	}
	if !serverRsp.Success {
		rsp["reason"] = genRsp.Reason
		rsp["elapse"] = time.Since(tick).String()
		ctx.JSON(http.StatusInternalServerError, rsp)
		return
	}

	buf := new(bytes.Buffer)
	zipWriter := zip.NewWriter(buf)

	var files = []struct {
		Name, Body string
	}{
		{"server.pem", serverRsp.Certificate},
		{req.Name + "_cert.pem", genRsp.Certificate},
		{req.Name + "_key.pem", genRsp.Key},
		{req.Name + "_token.txt", genRsp.Token},
	}

	for _, file := range files {
		f, err := zipWriter.CreateHeader(&zip.FileHeader{
			Name:     file.Name,
			Modified: time.Now().UTC(),
		})
		if err != nil {
			rsp["reason"] = err.Error()
			rsp["elapse"] = time.Since(tick).String()
			ctx.JSON(http.StatusInternalServerError, rsp)
			return
		}
		_, err = f.Write([]byte(file.Body))
		if err != nil {
			rsp["reason"] = err.Error()
			rsp["elapse"] = time.Since(tick).String()
			ctx.JSON(http.StatusInternalServerError, rsp)
			return
		}
	}

	err = zipWriter.Close()
	if err != nil {
		rsp["reason"] = err.Error()
		rsp["elapse"] = time.Since(tick).String()
		ctx.JSON(http.StatusInternalServerError, rsp)
		return
	}

	rsp["success"] = true
	rsp["reason"] = "success"
	rsp["serverKey"] = serverRsp.Certificate
	rsp["certificate"] = genRsp.Certificate
	rsp["privateKey"] = genRsp.Key
	rsp["token"] = genRsp.Token
	rsp["zip"] = buf.Bytes()
	rsp["elapse"] = time.Since(tick).String()

	ctx.JSON(http.StatusOK, rsp)
}

func (svr *httpd) handleKeysDel(ctx *gin.Context) {
	tick := time.Now()
	rsp := gin.H{"success": false, "reason": "not specified"}
	keyId := ctx.Param("id")
	if keyId == "" {
		rsp["reason"] = "no id specified"
		rsp["elapse"] = time.Since(tick).String()
		ctx.JSON(http.StatusBadRequest, rsp)
		return
	}

	mgmtRsp, err := svr.mgmtImpl.DelKey(ctx, &mgmt.DelKeyRequest{
		Id: keyId,
	})
	if err != nil {
		rsp["reason"] = err.Error()
		rsp["elapse"] = time.Since(tick).String()
		ctx.JSON(http.StatusInternalServerError, rsp)
		return
	}
	if !mgmtRsp.Success {
		rsp["reason"] = mgmtRsp.Reason
		rsp["elapse"] = time.Since(tick).String()
		ctx.JSON(http.StatusInternalServerError, rsp)
		return
	}

	rsp["success"] = true
	rsp["reason"] = "success"
	rsp["elapse"] = time.Since(tick).String()
	ctx.JSON(http.StatusOK, rsp)
}

func (svr *httpd) handleSshKeys(ctx *gin.Context) {
	tick := time.Now()
	rsp := gin.H{"success": false, "reason": "not specified"}

	mgmtRsp, err := svr.mgmtImpl.ListSshKey(ctx, &mgmt.ListSshKeyRequest{})
	if err != nil {
		rsp["reason"] = err.Error()
		rsp["elapse"] = time.Since(tick).String()
		ctx.JSON(http.StatusInternalServerError, rsp)
		return
	}
	if !mgmtRsp.Success {
		rsp["reason"] = mgmtRsp.Reason
		rsp["elapse"] = time.Since(tick).String()
		ctx.JSON(http.StatusInternalServerError, rsp)
		return
	}

	rsp["success"] = true
	rsp["reason"] = "success"
	rsp["data"] = mgmtRsp.SshKeys
	rsp["elapse"] = time.Since(tick).String()
	ctx.JSON(http.StatusOK, rsp)
}

func (svr *httpd) handleSshKeysAdd(ctx *gin.Context) {
	tick := time.Now()
	rsp := gin.H{"success": false, "reason": "not specified"}

	req := struct {
		Key string `json:"key"`
	}{}

	err := ctx.ShouldBind(&req)
	if err != nil {
		rsp["reason"] = err.Error()
		rsp["elapse"] = time.Since(tick).String()
		ctx.JSON(http.StatusBadRequest, rsp)
		return
	}

	//parser
	fields := util.SplitFields(req.Key, false)
	if len(fields) < 2 {
		rsp["reason"] = "invalid key format"
		rsp["elapse"] = time.Since(tick).String()
		ctx.JSON(http.StatusBadRequest, rsp)
		return
	}

	// 중복검사

	mgmtRsp, err := svr.mgmtImpl.AddSshKey(ctx, &mgmt.AddSshKeyRequest{
		KeyType: fields[0], Key: fields[1], Comment: fields[2],
	})
	if err != nil {
		rsp["reason"] = err.Error()
		rsp["elapse"] = time.Since(tick).String()
		ctx.JSON(http.StatusInternalServerError, rsp)
		return
	}
	if !mgmtRsp.Success {
		rsp["reason"] = mgmtRsp.Reason
		rsp["elapse"] = time.Since(tick).String()
		ctx.JSON(http.StatusInternalServerError, rsp)
		return
	}

	rsp["success"] = true
	rsp["reason"] = "success"
	rsp["elapse"] = time.Since(tick).String()
	ctx.JSON(http.StatusOK, rsp)
}

func (svr *httpd) handleSshKeysDel(ctx *gin.Context) {
	tick := time.Now()
	rsp := gin.H{"success": false, "reason": "not specified"}

	fingerPrint := ctx.Param("fingerprint")

	mgmtRsp, err := svr.mgmtImpl.DelSshKey(ctx, &mgmt.DelSshKeyRequest{
		Fingerprint: fingerPrint,
	})
	if err != nil {
		rsp["reason"] = err.Error()
		rsp["elapse"] = time.Since(tick).String()
		ctx.JSON(http.StatusInternalServerError, rsp)
		return
	}
	if !mgmtRsp.Success {
		rsp["reason"] = mgmtRsp.Reason
		rsp["elapse"] = time.Since(tick).String()
		ctx.JSON(http.StatusInternalServerError, rsp)
		return
	}

	rsp["success"] = true
	rsp["reason"] = "success"
	rsp["elapse"] = time.Since(tick).String()
	ctx.JSON(http.StatusOK, rsp)
}

func (svr *httpd) handleSubscriber(ctx *gin.Context) {
	tick := time.Now()
	rsp := gin.H{"success": false, "reason": "not specified"}

	name := ctx.Param("name")
	getRsp, err := svr.schedMgmtImpl.GetSchedule(ctx, &schedule.GetScheduleRequest{Name: name})
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

	rsp["success"] = true
	rsp["reason"] = "success"
	rsp["data"] = getRsp.Schedule
	rsp["elapse"] = time.Since(tick).String()
	ctx.JSON(http.StatusOK, rsp)
}

func (svr *httpd) handleSubscribers(ctx *gin.Context) {
	tick := time.Now()
	rsp := gin.H{"success": false, "reason": "not specified"}

	schedRsp, err := svr.schedMgmtImpl.ListSchedule(ctx, &schedule.ListScheduleRequest{})
	if err != nil {
		rsp["reason"] = err.Error()
		rsp["elapse"] = time.Since(tick).String()
		ctx.JSON(http.StatusInternalServerError, rsp)
		return
	}
	if !schedRsp.Success {
		rsp["reason"] = schedRsp.Reason
		rsp["elapse"] = time.Since(tick).String()
		ctx.JSON(http.StatusInternalServerError, rsp)
		return
	}

	lst := []*schedule.Schedule{}
	for _, c := range schedRsp.Schedules {
		typ := strings.ToUpper(c.Type)
		if typ != "SUBSCRIBER" {
			continue
		}
		lst = append(lst, c)
	}

	if len(lst) > 0 {
		sort.Slice(lst, func(i, j int) bool { return lst[i].Name < lst[j].Name })
	}

	rsp["success"] = true
	rsp["reason"] = "success"
	rsp["data"] = lst
	rsp["elapse"] = time.Since(tick).String()
	ctx.JSON(http.StatusOK, rsp)
}

func (svr *httpd) handleSubscribersAdd(ctx *gin.Context) {
	tick := time.Now()
	rsp := gin.H{"success": false, "reason": "not specified"}

	req := struct {
		Name      string `json:"name"`
		AutoStart bool   `json:"autoStart"`
		Bridge    string `json:"bridge"`
		Topic     string `json:"topic"`
		Task      string `json:"task"`
		QoS       int    `json:"QoS"`
		Queue     string `json:"quque"`
		Stream    string `json:"stream"`
	}{}

	err := ctx.ShouldBind(&req)
	if err != nil {
		rsp["reason"] = err.Error()
		rsp["elapse"] = time.Since(tick).String()
		ctx.JSON(http.StatusBadRequest, rsp)
		return
	}

	getRsp, err := svr.schedMgmtImpl.GetSchedule(ctx, &schedule.GetScheduleRequest{Name: req.Name})
	if err != nil {
		rsp["reason"] = err.Error()
		rsp["elapse"] = time.Since(tick).String()
		ctx.JSON(http.StatusInternalServerError, rsp)
		return
	}
	if getRsp.Success {
		rsp["reason"] = "duplicate name"
		rsp["elapse"] = time.Since(tick).String()
		ctx.JSON(http.StatusInternalServerError, rsp)
		return
	}

	bridgeRsp, err := svr.bridgeMgmtImpl.GetBridge(ctx, &bridgerpc.GetBridgeRequest{Name: req.Bridge})
	if err != nil {
		rsp["reason"] = err.Error()
		rsp["elapse"] = time.Since(tick).String()
		ctx.JSON(http.StatusInternalServerError, rsp)
		return
	}
	if !bridgeRsp.Success {
		rsp["reason"] = bridgeRsp.Reason
		rsp["elapse"] = time.Since(tick).String()
		ctx.JSON(http.StatusInternalServerError, rsp)
		return
	}
	bridgeType := bridgeRsp.Bridge.Type

	addSchedReq := &schedule.AddScheduleRequest{
		Name:      strings.ToLower(req.Name),
		Type:      "subscriber",
		AutoStart: req.AutoStart,
		Bridge:    req.Bridge,
		Task:      req.Task,
	}

	switch bridgeType {
	case "mqtt":
		addSchedReq.Opt = &schedule.AddScheduleRequest_Mqtt{
			Mqtt: &schedule.MqttOption{
				Topic: req.Topic,
				QoS:   int32(req.QoS),
			},
		}
	case "nats":
		addSchedReq.Opt = &schedule.AddScheduleRequest_Nats{
			Nats: &schedule.NatsOption{
				Subject:    req.Topic,
				QueueName:  req.Queue,
				StreamName: req.Stream,
			},
		}
	default:
		rsp["reason"] = fmt.Sprintf("unknown birdge type %q", bridgeType)
		rsp["elapse"] = time.Since(tick).String()
		ctx.JSON(http.StatusInternalServerError, rsp)
		return
	}

	schedRsp, err := svr.schedMgmtImpl.AddSchedule(ctx, addSchedReq)
	if err != nil {
		rsp["reason"] = err.Error()
		rsp["elapse"] = time.Since(tick).String()
		ctx.JSON(http.StatusInternalServerError, rsp)
		return
	}
	if !schedRsp.Success {
		rsp["reason"] = schedRsp.Reason
		rsp["elapse"] = time.Since(tick).String()
		ctx.JSON(http.StatusInternalServerError, rsp)
		return
	}

	rsp["success"] = true
	rsp["reason"] = "success"
	rsp["elapse"] = time.Since(tick).String()
	ctx.JSON(http.StatusOK, rsp)
}

func (svr *httpd) handleSubscribersDel(ctx *gin.Context) {
	tick := time.Now()
	rsp := gin.H{"success": false, "reason": "not specified"}
	name := ctx.Param("name")

	schedRsp, err := svr.schedMgmtImpl.DelSchedule(ctx, &schedule.DelScheduleRequest{
		Name: name,
	})
	if err != nil {
		rsp["reason"] = err.Error()
		rsp["elapse"] = time.Since(tick).String()
		ctx.JSON(http.StatusInternalServerError, rsp)
		return
	}
	if !schedRsp.Success {
		rsp["reason"] = schedRsp.Reason
		rsp["elapse"] = time.Since(tick).String()
		ctx.JSON(http.StatusInternalServerError, rsp)
		return
	}

	rsp["success"] = true
	rsp["reason"] = "success"
	rsp["elapse"] = time.Since(tick).String()
	ctx.JSON(http.StatusOK, rsp)
}

func (svr *httpd) handleSubscribersState(ctx *gin.Context) {
	tick := time.Now()
	rsp := gin.H{"success": false, "reason": "not specified"}
	name := ctx.Param("name")

	req := struct {
		State string `json:"state"`
	}{}

	err := ctx.ShouldBind(&req)
	if err != nil {
		rsp["reason"] = err.Error()
		rsp["elapse"] = time.Since(tick).String()
		ctx.JSON(http.StatusBadRequest, rsp)
		return
	}

	switch strings.ToUpper(req.State) {
	case "START":
		schedRsp, err := svr.schedMgmtImpl.StartSchedule(ctx, &schedule.StartScheduleRequest{
			Name: name,
		})
		if err != nil {
			rsp["reason"] = err.Error()
			rsp["elapse"] = time.Since(tick).String()
			ctx.JSON(http.StatusInternalServerError, rsp)
			return
		}
		if !schedRsp.Success {
			rsp["reason"] = schedRsp.Reason
			rsp["elapse"] = time.Since(tick).String()
			ctx.JSON(http.StatusInternalServerError, rsp)
			return
		}
	case "STOP":
		schedRsp, err := svr.schedMgmtImpl.StopSchedule(ctx, &schedule.StopScheduleRequest{
			Name: name,
		})
		if err != nil {
			rsp["reason"] = err.Error()
			rsp["elapse"] = time.Since(tick).String()
			ctx.JSON(http.StatusInternalServerError, rsp)
			return
		}
		if !schedRsp.Success {
			rsp["reason"] = schedRsp.Reason
			rsp["elapse"] = time.Since(tick).String()
			ctx.JSON(http.StatusInternalServerError, rsp)
			return
		}
	default:
		rsp["reason"] = "invalid state"
		rsp["elapse"] = time.Since(tick).String()
		ctx.JSON(http.StatusBadRequest, rsp)
		return
	}

	rsp["success"] = true
	rsp["reason"] = "success"
	rsp["elapse"] = time.Since(tick).String()
	ctx.JSON(http.StatusOK, rsp)
}

func (svr *httpd) handleBridges(ctx *gin.Context) {
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

func (svr *httpd) handleBridgesAdd(ctx *gin.Context) {
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

func (svr *httpd) handleBridgesDel(ctx *gin.Context) {
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

func (svr *httpd) handleBridgeState(ctx *gin.Context) {
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
