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

	if path == "/" {
		references := &WebReferenceGroup{Label: "REFERENCES"}
		references.Items = append(references.Items, ReferenceItem{Type: "url", Title: "machbase-neo docs", Addr: "https://machbase.com/neo", Target: "_blank"})
		references.Items = append(references.Items, ReferenceItem{Type: "url", Title: "machbase sql reference", Addr: "https://machbase.com/dbms/sql-ref/", Target: "_docs_machbase"})
		references.Items = append(references.Items, ReferenceItem{Type: "url", Title: "https://machbase.com", Addr: "https://machbase.com/", Target: "_home_machbase"})
		references.Items = append(references.Items, ReferenceItem{Type: "url", Title: "Tutorials", Addr: "https://github.com/machbase/neo-tutorials", Target: "_blank"})
		references.Items = append(references.Items, ReferenceItem{Type: "url", Title: "Demo web app", Addr: "https://github.com/machbase/neo-apps"})

		sdk := &WebReferenceGroup{Label: "SDK"}
		sdk.Items = append(sdk.Items, ReferenceItem{Type: "url", Title: "SDK Download", Addr: "https://machbase.com/home/download/", Target: "_home_machbase"})
		sdk.Items = append(sdk.Items, ReferenceItem{Type: "url", Title: ".NET Connector", Addr: "https://machbase.com/dbms/sdk/dotnet/", Target: "_docs_machbase"})
		sdk.Items = append(sdk.Items, ReferenceItem{Type: "url", Title: "JDBC Driver", Addr: "https://machbase.com/dbms/sdk/jdbc/", Target: "_docs_machbase"})
		sdk.Items = append(sdk.Items, ReferenceItem{Type: "url", Title: "ODBC", Addr: "https://machbase.com/dbms/sdk/cli-odbc/", Target: "_docs_machbase"})
		sdk.Items = append(sdk.Items, ReferenceItem{Type: "url", Title: "ODBC Example", Addr: "https://machbase.com/dbms/sdk/cli-odbc-example/", Target: "_docs_machbase"})

		cheatsheets := &WebReferenceGroup{Label: "CHEAT SHEETS"}
		cheatsheets.Items = append(cheatsheets.Items, ReferenceItem{Type: "wrk", Title: "markdown example", Addr: "./tutorials/sample_markdown.wrk"})
		cheatsheets.Items = append(cheatsheets.Items, ReferenceItem{Type: "wrk", Title: "mermaid example", Addr: "./tutorials/sample_mermaid.wrk"})
		cheatsheets.Items = append(cheatsheets.Items, ReferenceItem{Type: "wrk", Title: "pikchr example", Addr: "./tutorials/sample_pikchr.wrk"})

		rsp.Data.Refs = []*WebReferenceGroup{references, sdk, cheatsheets}
		rsp.Success, rsp.Reason = true, "success"
		rsp.Elapse = time.Since(tick).String()
		ctx.JSON(http.StatusOK, rsp)
	} else {
		rsp.Reason = fmt.Sprintf("'%s' not found", path)
		rsp.Elapse = time.Since(tick).String()
		ctx.JSON(http.StatusNotFound, rsp)
	}
}
