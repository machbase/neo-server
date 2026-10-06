package server

import (
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/machbase/neo-server/v8/mods/embedding"
)

type embedRequest struct {
	Input    json.RawMessage `json:"input"`
	Provider string          `json:"provider"`
}

func (svr *httpd) handleEmbed(ctx *gin.Context) {
	if _, reason := svr.resolveExecUser(ctx); reason != "" {
		ctx.JSON(http.StatusUnauthorized, gin.H{"success": false, "reason": reason})
		return
	}
	var request embedRequest
	if err := json.NewDecoder(http.MaxBytesReader(ctx.Writer, ctx.Request.Body, 1<<20)).Decode(&request); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"success": false, "reason": err.Error()})
		return
	}
	var input string
	var texts []string
	if err := json.Unmarshal(request.Input, &input); err == nil {
		texts = []string{input}
	} else if err := json.Unmarshal(request.Input, &texts); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"success": false, "reason": "input must be a string or string array"})
		return
	}
	results, err := embedding.Default().EmbedMany(ctx.Request.Context(), texts, request.Provider)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"success": false, "reason": err.Error()})
		return
	}
	data := make([]gin.H, len(results))
	for i, result := range results {
		data[i] = gin.H{"index": i, "vector": result.Vector}
	}
	ctx.JSON(http.StatusOK, gin.H{
		"success":    true,
		"model":      results[0].Model,
		"space":      results[0].Space,
		"dimensions": results[0].Dimension,
		"data":       data,
	})
}
