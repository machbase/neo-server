package httpd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
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
	case ".dsh":
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
	case ".css":
		return "text/css"
	case ".js":
		return "text/javascript"
	case ".htm", ".html":
		return "text/html"
	}
}

func (svr *httpd) handleFiles(ctx *gin.Context) {
	rsp := &SsfsResponse{Success: false, Reason: "not specified"}
	tick := time.Now()
	path := ctx.Param("path")
	filter := ctx.Query("filter")
	recursive := strBool(ctx.Query("recursive"), false)

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
			if len(ent.Children) == 0 || recursive {
				if recursive {
					err = svr.serverFs.RemoveRecursive(path)
				} else {
					err = svr.serverFs.Remove(path)
				}
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
				if claim, exists := svr.getJwtClaim(ctx); exists {
					consoleInfo := parseConsoleId(ctx)
					topic = fmt.Sprintf("console:%s:%s", claim.Subject, consoleInfo.consoleId)
				}
				cloneReq := &GitCloneReq{}
				err = json.Unmarshal(content, cloneReq)
				if err == nil {
					cloneReq.logTopic = topic
					switch strings.ToLower(cloneReq.Cmd) {
					default:
						entry, err = svr.serverFs.GitClone(path, cloneReq.Url, cloneReq)
					case "pull":
						entry, err = svr.serverFs.GitPull(path, cloneReq.Url, cloneReq)
					}
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
	case http.MethodPut:
		req := RenameReq{}
		if err := ctx.Bind(&req); err != nil {
			rsp.Reason = err.Error()
			rsp.Elapse = time.Since(tick).String()
			ctx.JSON(http.StatusBadRequest, rsp)
			return
		}
		if req.Dest == "" {
			rsp.Reason = "destination is not specified."
			rsp.Elapse = time.Since(tick).String()
			ctx.JSON(http.StatusBadRequest, rsp)
			return
		}
		if err := svr.serverFs.Rename(path, req.Dest); err != nil {
			rsp.Reason = err.Error()
			rsp.Elapse = time.Since(tick).String()
			ctx.JSON(http.StatusInternalServerError, rsp)
			return
		}
		rsp.Success, rsp.Reason = true, "success"
		rsp.Elapse = time.Since(tick).String()
		ctx.JSON(http.StatusOK, rsp)
		return
	}
}

type RenameReq struct {
	Dest string `json:"destination"`
}

type GitCloneReq struct {
	Cmd      string `json:"command"`
	Url      string `json:"url"`
	logTopic string `json:"-"`
}

func (gitClone *GitCloneReq) Write(b []byte) (int, error) {
	if gitClone.logTopic == "" {
		return os.Stdout.Write(b)
	} else {
		taskId := fmt.Sprintf("%p", gitClone)
		lines := bytes.Split(b, []byte{'\n'})
		for _, line := range lines {
			carrageReturns := bytes.Split(line, []byte{'\r'})
			for i := len(carrageReturns) - 1; i >= 0; i-- {
				line = bytes.TrimSpace(carrageReturns[i])
				if len(line) > 0 {
					break
				}
			}
			if len(line) > 0 {
				eventbus.PublishLogTask(gitClone.logTopic, "INFO", taskId, string(line))
			}
		}
		return len(b), nil
	}
}
