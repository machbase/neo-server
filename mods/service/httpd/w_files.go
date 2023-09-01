package httpd

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/machbase/neo-server/mods/util/ssfs"
)

type SsfsResponse struct {
	Success bool        `json:"success"`
	Reason  string      `json:"reason"`
	Elapse  string      `json:"elapse"`
	Data    *ssfs.Entry `json:"data,omitempty"`
}

func isFsFile(path string) bool {
	return strings.HasSuffix(path, ".tql") ||
		strings.HasSuffix(path, ".sql") ||
		strings.HasSuffix(path, ".taz") ||
		strings.HasSuffix(path, ".wrk")
}

// returns supproted content-type of the given file path (name),
// if the name is an unsupported file type, it returns empty string
func contentTypeOfFileFallback(name string, fallback string) string {
	ret := contentTypeOfFile(name)
	if ret == "" {
		ret = fallback
	}
	return ret
}

func contentTypeOfFile(name string) string {
	ext := filepath.Ext(name)
	switch strings.ToLower(ext) {
	default:
		return ""
	case ".sql":
		return "text/plain"
	case ".tql":
		return "text/plain"
	case ".taz":
		return "application/json"
	case ".wrk":
		return "application/json"
	case ".apng":
		return "image/apng"
	case ".avif":
		return "image/avif"
	case ".gif":
		return "image/gif"
	case ".jpeg", ".jpg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".svg":
		return "image/svg+xml"
	case ".webp":
		return "image/webp"
	case ".bmp":
		return "image/bmp"
	case ".ico":
		return "image/x-icon"
	case ".tiff":
		return "image/tiff"
	}
}

var ignores = map[string]bool{
	".git":          true,
	"machbase_home": true,
	"node_modules":  true,
	".pnp":          true,
	".DS_Store":     true,
}

func (svr *httpd) handleFiles(ctx *gin.Context) {
	rsp := &SsfsResponse{Success: false, Reason: "not specified"}
	tick := time.Now()
	path := ctx.Param("path")
	filter := ctx.Query("filter")

	switch ctx.Request.Method {
	case http.MethodGet:
		var ent *ssfs.Entry
		var err error
		if isFsFile(filter) {
			ent, err = svr.serverFs.GetGlob(path, filter)
		} else {
			ent, err = svr.serverFs.GetFilter(path, func(se *ssfs.SubEntry) bool {
				base := filepath.Base(path)
				if ignores[base] {
					return false
				}
				if se.IsDir {
					return true
				}
				return contentTypeOfFile(se.Name) != ""
			})
		}
		if err != nil {
			rsp.Reason = err.Error()
			rsp.Elapse = time.Since(tick).String()
			ctx.JSON(http.StatusNotFound, rsp)
			return
		}
		if ent != nil {
			if ent.IsDir {
				rsp.Success, rsp.Reason = true, "success"
				rsp.Elapse = time.Since(tick).String()
				rsp.Data = ent
				ctx.JSON(http.StatusOK, rsp)
				return
			}
			if contentType := contentTypeOfFile(ent.Name); contentType != "" {
				ctx.Data(http.StatusOK, contentType, ent.Content)
				return
			}
		}
		rsp.Reason = fmt.Sprintf("not found: %s", path)
		rsp.Elapse = time.Since(tick).String()
		ctx.JSON(http.StatusNotFound, rsp)
		return
	case http.MethodDelete:
		ent, err := svr.serverFs.Get(path)
		if err != nil {
			rsp.Reason = err.Error()
			rsp.Elapse = time.Since(tick).String()
			ctx.JSON(http.StatusNotFound, rsp)
			return
		}
		if ent.IsDir {
			if len(ent.Children) == 0 {
				err = svr.serverFs.Remove(path)
				if err != nil {
					rsp.Reason = err.Error()
					rsp.Elapse = time.Since(tick).String()
					ctx.JSON(http.StatusInternalServerError, rsp)
					return
				} else {
					rsp.Success, rsp.Reason = true, "success"
					rsp.Elapse = time.Since(tick).String()
					ctx.JSON(http.StatusOK, rsp)
					return
				}
			} else {
				rsp.Reason = "directory is not empty"
				rsp.Elapse = time.Since(tick).String()
				ctx.JSON(http.StatusUnprocessableEntity, rsp)
				return
			}
		} else if isFsFile(path) {
			err = svr.serverFs.Remove(path)
			if err != nil {
				rsp.Reason = err.Error()
				rsp.Elapse = time.Since(tick).String()
				ctx.JSON(http.StatusInternalServerError, rsp)
				return
			} else {
				rsp.Success, rsp.Reason = true, "success"
				rsp.Elapse = time.Since(tick).String()
				ctx.JSON(http.StatusOK, rsp)
				return
			}
		} else {
			rsp.Reason = fmt.Sprintf("not found: %s", path)
			rsp.Elapse = time.Since(tick).String()
			ctx.JSON(http.StatusNotFound, rsp)
			return
		}
	case http.MethodPost:
		if isFsFile(path) {
			content, err := io.ReadAll(ctx.Request.Body)
			if err != nil {
				rsp.Reason = err.Error()
				rsp.Elapse = time.Since(tick).String()
				ctx.JSON(http.StatusInternalServerError, rsp)
				return
			}
			if ctx.ContentType() == "application/json" {
				var text string
				if err := json.Unmarshal(content, &text); err != nil {
					rsp.Reason = err.Error()
					rsp.Elapse = time.Since(tick).String()
					ctx.JSON(http.StatusBadRequest, rsp)
					return
				}
				content = []byte(text)
			}
			err = svr.serverFs.Set(path, content)
			if err == nil {
				rsp.Success, rsp.Reason = true, "success"
				rsp.Elapse = time.Since(tick).String()
				ctx.JSON(http.StatusOK, rsp)
				return
			} else {
				rsp.Reason = err.Error()
				rsp.Elapse = time.Since(tick).String()
				ctx.JSON(http.StatusInternalServerError, rsp)
				return
			}
		} else {
			content, err := io.ReadAll(ctx.Request.Body)
			if err != nil {
				rsp.Reason = err.Error()
				rsp.Elapse = time.Since(tick).String()
				ctx.JSON(http.StatusInternalServerError, rsp)
				return
			}
			var entry *ssfs.Entry
			if len(content) > 0 && ctx.ContentType() == "application/json" {
				cloneReq := GitCloneReq{}
				err = json.Unmarshal(content, &cloneReq)
				if err == nil {
					entry, err = svr.serverFs.GitClone(path, cloneReq.Url)
				}
			} else {
				entry, err = svr.serverFs.MkDir(path)
			}
			if err == nil {
				rsp.Success, rsp.Reason = true, "success"
				rsp.Elapse = time.Since(tick).String()
				rsp.Data = entry
				ctx.JSON(http.StatusOK, rsp)
				return
			} else {
				rsp.Reason = err.Error()
				rsp.Elapse = time.Since(tick).String()
				ctx.JSON(http.StatusInternalServerError, rsp)
				return
			}
		}
	}
}

type GitCloneReq struct {
	Url string `json:"url"`
}
