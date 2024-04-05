package httpd

import (
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/machbase/neo-server/api/mgmt"
)

func (svr *httpd) handleGetKeys(ctx *gin.Context) {
	tick := time.Now()
	rsp := gin.H{"success": false, "reason": "not specified"}

	id := ctx.Query("id")
	mgmtRsp, err := svr.mgmtImpl.ListKey(ctx, &mgmt.ListKeyRequest{})
	if err != nil {
		rsp["reason"] = err.Error()
		ctx.JSON(http.StatusInternalServerError, rsp)
		return
	}
	if !mgmtRsp.Success {
		rsp["reason"] = mgmtRsp.Reason
		ctx.JSON(http.StatusInternalServerError, rsp)
		return
	}

	keyList := make([][]string, 0, len(mgmtRsp.Keys))
	for i, k := range mgmtRsp.Keys {
		if id != "" && k.Id != id {
			continue
		}
		nb := time.Unix(k.NotBefore, 0).UTC()
		na := time.Unix(k.NotAfter, 0).UTC()
		key := []string{fmt.Sprintf("%d", i), k.Id, nb.String(), na.String()}
		keyList = append(keyList, key)
	}

	rsp["success"] = true
	rsp["reason"] = "success"
	rsp["list"] = keyList
	rsp["elapse"] = time.Since(tick).String()
	ctx.JSON(http.StatusOK, rsp)
}

func (svr *httpd) handleGenKey(ctx *gin.Context) {
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
		ctx.JSON(http.StatusBadRequest, rsp)
		return
	}

	name := strings.ToLower(req.Name)
	pass, _ := regexp.MatchString("[a-z][a-z0-9_.@-]+", name)
	if !pass {
		rsp["reason"] = "id contains invalid letter, use only alphnum and _.@-"
		ctx.JSON(http.StatusBadRequest, rsp)
		return
	}

	if req.NotBefore == 0 {
		req.NotBefore = time.Now().UnixNano()
	}
	if req.NotAfter <= req.NotBefore {
		req.NotAfter = time.Now().Add(10 * time.Hour * 24 * 365).Unix() // 10 years
	}

	mgmtRsp, err := svr.mgmtImpl.GenKey(ctx, &mgmt.GenKeyRequest{
		Id:        req.Name,
		Type:      "ec",
		NotBefore: time.Now().Unix(),
		NotAfter:  req.NotAfter,
	})
	if err != nil {
		rsp["reason"] = err.Error()
		ctx.JSON(http.StatusInternalServerError, rsp)
		return
	}
	if !mgmtRsp.Success {
		rsp["reason"] = mgmtRsp.Reason
		ctx.JSON(http.StatusInternalServerError, rsp)
		return
	}

	rsp["success"] = true
	rsp["reason"] = "success"
	rsp["certificate"] = mgmtRsp.Certificate
	rsp["privateKey"] = mgmtRsp.Key
	rsp["token"] = mgmtRsp.Token
	rsp["elapse"] = time.Since(tick).String()

	ctx.JSON(http.StatusOK, rsp)
}

func (svr *httpd) handleDeleteKey(ctx *gin.Context) {
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
