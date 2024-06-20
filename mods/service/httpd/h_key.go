package httpd

import (
	"archive/zip"
	"bytes"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/machbase/neo-server/api/mgmt"
)

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
