package httpd

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

type RefsResponse struct {
	Success bool   `json:"success"`
	Reason  string `json:"reason"`
	Elapse  string `json:"elapse"`
	Data    struct {
		Refs []*WebReferenceGroup `json:"refs,omitempty"`
	} `json:"data"`
}

func (svr *httpd) handleRefs(ctx *gin.Context) {
	rsp := &RefsResponse{Reason: "unspecified"}
	tick := time.Now()
	path := ctx.Param("path")

	if path == "" {
		references := WebReferenceGroup{Label: "References"}
		references.Items = append(references.Items, ReferenceItem{Type: "url", Title: "machbase-neo docs", Addr: "https://neo.machbase.com/", Target: "_blank"})
		references.Items = append(references.Items, ReferenceItem{Type: "url", Title: "machbase sql reference", Addr: "https://docs.machbase.com/en/", Target: "_blank"})
		references.Items = append(references.Items, ReferenceItem{Type: "url", Title: "https://machbase.com", Addr: "https://machbase.com/", Target: "_blank"})

		references.Items = append(references.Items, ReferenceItem{Type: "wrk", Title: "markdown cheatsheet", Addr: "./tutorials/sample_markdown.wrk"})
		references.Items = append(references.Items, ReferenceItem{Type: "wrk", Title: "mermaid cheatsheet", Addr: "./tutorials/sample_mermaid.wrk"})
		references.Items = append(references.Items, ReferenceItem{Type: "wrk", Title: "pikchr cheatsheet", Addr: "./tutorials/sample_pikchr.wrk"})
		rsp.Data.Refs = []*WebReferenceGroup{&references}
		rsp.Success, rsp.Reason = true, "success"
		rsp.Elapse = time.Since(tick).String()
		ctx.JSON(http.StatusOK, rsp)
	} else {
		rsp.Reason = fmt.Sprintf("'%s' not found", path)
		rsp.Elapse = time.Since(tick).String()
		ctx.JSON(http.StatusNotFound, rsp)
	}
}
