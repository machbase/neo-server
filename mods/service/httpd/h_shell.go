package httpd

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// GET /api/shell/:id  - make a copy of the id
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
	shell, err := svr.webShellProvider.GetWebShell(shellId)
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
	shell, err := svr.webShellProvider.CopyWebShell(shellId)
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
	shell := &WebShell{}
	err := ctx.Bind(shell)
	if err != nil {
		rsp["reason"] = err.Error()
		rsp["elapse"] = time.Since(tick).String()
		svr.log.Debug("update shell def, invalid request", err.Error())
		ctx.JSON(http.StatusBadRequest, rsp)
		return
	}
	if err := svr.webShellProvider.UpdateWebShell(shell); err != nil {
		rsp["reason"] = err.Error()
		rsp["elapse"] = time.Since(tick).String()
		svr.log.Debug("update shell def, internal err", err.Error())
		ctx.JSON(http.StatusInternalServerError, rsp)
		return
	} else {
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
	err := svr.webShellProvider.RemoveWebShell(shellId)
	if err != nil {
		rsp["reason"] = err.Error()
		rsp["elapse"] = time.Since(tick).String()
		ctx.JSON(http.StatusInternalServerError, rsp)
		return
	} else {
		rsp["reason"] = "success"
		rsp["elapse"] = time.Since(tick).String()
		ctx.JSON(http.StatusOK, rsp)
	}
}
