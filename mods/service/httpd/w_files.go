package httpd

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v4"
	"github.com/machbase/neo-server/mods/service/eventbus"
	"github.com/machbase/neo-server/mods/util/ssfs"
)

type SsfsResponse struct {
	Success bool        `json:"success"`
	Reason  string      `json:"reason"`
	Elapse  string      `json:"elapse"`
	Data    *ssfs.Entry `json:"data,omitempty"`
}

func isFsFile(path string) bool {
	return contentTypeOfFile(path) != ""
}

// returns supproted content-type of the given file path (name),
// if the name is an unsupported file type, it returns empty string
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
	// image files
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
	// text files
	case ".txt":
		return "text/plain"
	case ".json":
		return "application/json"
	case ".csv":
		return "text/csv"
	case ".md", ".markdown":
		return "text/markdown"
	}
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
				var topic string
				if vals, exists := ctx.Get("jwt-claim"); exists {
					claim := vals.(*jwt.RegisteredClaims)
					consoleInfo := parseConsoleId(ctx)
					topic = fmt.Sprintf("console:%s:%s", claim.Subject, consoleInfo.consoleId)
				}
				cloneReq := &GitCloneReq{logTopic: topic}
				err = json.Unmarshal(content, cloneReq)
				if err == nil {
					entry, err = svr.serverFs.GitClone(path, cloneReq.Url, cloneReq)
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
	Url      string `json:"url"`
	logTopic string `json:"-"`
}

func (gitClone *GitCloneReq) Write(b []byte) (int, error) {
	if gitClone.logTopic == "" {
		return os.Stdout.Write(b)
	} else {
		l := len(b)
		eventbus.PublishLog(gitClone.logTopic, "INFO", string(b))
		return l, nil
	}
}
