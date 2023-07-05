package httpd

import (
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/machbase/neo-server/mods/do"
)

type LicenseResponse struct {
	Success bool            `json:"success"`
	Reason  string          `json:"reason"`
	Elapse  string          `json:"elapse"`
	Data    *do.LicenseInfo `json:"data,omitempty"`
}

func (svr *httpd) handleGetLicense(ctx *gin.Context) {
	rsp := &LicenseResponse{Success: false, Reason: "unspecified"}
	tick := time.Now()

	nfo, err := do.GetLicenseInfo(svr.db)
	if err != nil {
		rsp.Reason = err.Error()
		rsp.Elapse = time.Since(tick).String()
		ctx.JSON(http.StatusInternalServerError, rsp)
		return
	}
	rsp.Success, rsp.Reason = true, "success"
	rsp.Data = nfo
	rsp.Elapse = time.Since(tick).String()
	ctx.JSON(http.StatusOK, rsp)
}

func (svr *httpd) handleInstallLicense(ctx *gin.Context) {
	rsp := &LicenseResponse{Success: false, Reason: "unspecified"}
	tick := time.Now()

	file, fileHeader, err := ctx.Request.FormFile("license.dat")
	if err != nil {
		rsp.Reason = err.Error()
		rsp.Elapse = time.Since(tick).String()
		ctx.JSON(http.StatusBadRequest, rsp)
		return
	}
	defer file.Close()

	if fileHeader.Size > 4096 {
		// too big as a license file, user might send wrong file.
		rsp.Reason = "Too large file as a license file."
		rsp.Elapse = time.Since(tick).String()
		ctx.JSON(http.StatusBadRequest, rsp)
		return
	}
	content, err := io.ReadAll(file)
	if err != nil {
		rsp.Reason = err.Error()
		rsp.Elapse = time.Since(tick).String()
		ctx.JSON(http.StatusBadRequest, rsp)
		return
	}
	nfo, err := do.InstallLicenseData(svr.db, svr.licenseFilePath, content)
	if err != nil {
		fmt.Println("ERR", err.Error())
		rsp.Reason = err.Error()
		rsp.Elapse = time.Since(tick).String()
		ctx.JSON(http.StatusInternalServerError, rsp)
		return
	}
	rsp.Success, rsp.Reason = true, "Successfully registered."
	rsp.Data = nfo
	rsp.Elapse = time.Since(tick).String()
	ctx.JSON(http.StatusOK, rsp)
}
