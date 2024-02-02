package httpd

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// GET /api/keys
// GET /api/keys/:id
func (svr *httpd) handleGetKeys(ctx *gin.Context) {
	ctx.String(http.StatusOK, "")
}

// POST /api/keys
func (svr *httpd) handlePostKeys(ctx *gin.Context) {
	ctx.String(http.StatusOK, "")
}

// DELETE /api/keys/:id
func (svr *httpd) handleDeleteKeys(ctx *gin.Context) {
	ctx.String(http.StatusOK, "")
}
