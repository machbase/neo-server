package httpd

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// GET /api/shell/:id  - make a copy of the id
func (svr *httpd) handleGetShell(ctx *gin.Context) {
	tick := time.Now()
	shellId := ctx.Param("id")
	shells := svr.webShellProvider.GetAllWebShells()
	for _, s := range shells {
		if s.Id == shellId {
			ctx.JSON(http.StatusOK, gin.H{
				"success": true, "reason": "success",
				"data":   s,
				"elapse": time.Since(tick).String()})
			return
		}
	}
	ctx.JSON(http.StatusNotFound, gin.H{"success": false, "reason": "not found", "elapse": time.Since(tick).String()})
}

// GET /api/shell/:id/copy  - make a copy of the id
func (svr *httpd) handleGetShellCopy(ctx *gin.Context) {

}

// POST /api/shell/:id - update the label, content, icon of the shell by id
func (svr *httpd) handlePostShell(ctx *gin.Context) {

}

// DELETE /api/shell/:id - delete shell by id
func (svr *httpd) handleDeleteShell(ctx *gin.Context) {

}
