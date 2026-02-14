package server

import (
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/machbase/neo-server/v8/jsh/engine"
	"github.com/machbase/neo-server/v8/jsh/native"
	"github.com/machbase/neo-server/v8/jsh/root"
)

func (svr *httpd) handlePublic(ctx *gin.Context) {
	tick := time.Now()
	path := ctx.Param("path")
	// Remove leading slash and prevent directory traversal higher than /public/
	path = strings.TrimPrefix(path, "/")
	if strings.Contains(path, "..") {
		handleError(ctx, http.StatusBadRequest, "invalid path", tick)
		return
	}
	path = "/public/" + path
	if strings.Contains(path, "/cgi-bin/") {
		if !strings.HasSuffix(path, ".js") {
			path = path + ".js"
		}
		if ent, err := svr.serverFs.Get(path); err != nil || ent.IsDir {
			handleError(ctx, http.StatusNotFound, "not found", tick)
			return
		}
		toks := strings.SplitN(path, "/cgi-bin/", 2)
		appPath := toks[0]
		appRealPath, err := svr.serverFs.FindRealPath(appPath)
		mountPoint := "/work" + appPath
		cgiPath := "/cgi-bin/" + toks[1]
		if err != nil {
			handleError(ctx, http.StatusInternalServerError, "app path error: "+err.Error(), tick)
			return
		}
		code := strings.Join([]string{
			"const process = require('process');",
			"try {",
			fmt.Sprintf("const result = process.exec('%s%s');", mountPoint, cgiPath),
			"result && console.println(JSON.stringify(result, null, 2));",
			"} catch (err) {",
			"console.println(JSON.stringify({ error: err.message }, null, 2));",
			"}",
		}, "\n")
		// fmt.Println("Mount "+mountPoint, "->", appRealPath.AbsPath, "\ncode:\n", code)
		fsTabs := []engine.FSTab{root.RootFSTab(), {MountPoint: mountPoint, Source: appRealPath.AbsPath}}
		env := contextToCGIEnv(ctx, path) // custom env
		env["HOME"] = "/work"
		env["PWD"] = mountPoint
		env["QUERY"] = ctx.Request.URL.Query()
		conf := engine.Config{
			Name:   path,
			Code:   code,
			FSTabs: fsTabs,
			Env:    env,
			Reader: ctx.Request.Body,
			Writer: ctx.Writer,
			ExecBuilder: func(code string, args []string, env map[string]any) (*exec.Cmd, error) {
				self, err := os.Executable()
				if err != nil {
					return nil, err
				}
				conf := engine.Config{
					Code:   code,
					Args:   args,
					FSTabs: fsTabs,
					Env:    env,
				}
				secretBox, err := engine.NewSecretBox(conf)
				if err != nil {
					return nil, err
				}
				execCmd := exec.Command(self, "jsh", "-S", secretBox.FilePath(), args[0])
				return execCmd, nil
			},
		}
		jr, err := engine.New(conf)
		if err != nil {
			handleError(ctx, http.StatusInternalServerError, "engine error: "+err.Error(), tick)
			return
		}
		native.Enable(jr)
		if err := jr.Run(); err != nil {
			handleError(ctx, http.StatusInternalServerError, "engine run error: "+err.Error(), tick)
		}
		return
	} else if ctx.Request.Method == http.MethodGet {
		ent, err := svr.serverFs.Get(path)
		if err != nil {
			handleError(ctx, http.StatusNotFound, "not found", tick)
			return
		}
		if ent.IsDir {
			path, _ = url.JoinPath(path, "index.html")
			ent, err = svr.serverFs.Get(path)
			if err != nil || ent.IsDir {
				handleError(ctx, http.StatusNotFound, "not found", tick)
				return
			}
		} else {
			// Redirect to path without "index.html" suffix if it exists
			// e.g. redirect "/public/foo/index.html" to "/public/foo/"
			if strings.HasSuffix(path, "/index.html") {
				ctx.Redirect(http.StatusFound, strings.TrimSuffix(path, "index.html"))
				return
			}
		}
		// Serve the file content with correct Content-Type
		contentType := contentTypeOfFile(ent.Name)
		if ent, err := svr.serverFs.Get(path); err == nil && !ent.IsDir {
			ctx.Header("Content-Type", contentType)
			ctx.Writer.Write(ent.Content)
			return
		}
	}
	handleError(ctx, http.StatusNotFound, "not found", tick)
}

func contextToCGIEnv(ctx *gin.Context, scriptName string) map[string]any {
	m := map[string]any{
		// CGI standard env
		"AUTH_TYPE":                "",
		"CONTENT_ENCODING":         ctx.Request.Header.Get("Content-Encoding"),
		"CONTENT_LENGTH":           ctx.Request.Header.Get("Content-Length"),
		"CONTENT_TYPE":             ctx.Request.Header.Get("Content-Type"),
		"GATEWAY_INTERFACE":        "CGI/1.1",
		"HTTP_ACCEPT":              ctx.Request.Header.Get("Accept"),
		"HTTP_ACCEPT_CHARSET":      ctx.Request.Header.Get("Accept-Charset"),
		"HTTP_ACCEPT_ENCODING":     ctx.Request.Header.Get("Accept-Encoding"),
		"HTTP_ACCEPT_LANGUAGE":     ctx.Request.Header.Get("Accept-Language"),
		"HTTP_COOKIE":              ctx.Request.Header.Get("Cookie"),
		"HTTP_FORWARDED":           ctx.Request.Header.Get("Forwarded"),
		"HTTP_HOST":                ctx.Request.Header.Get("Host"),
		"HTTP_PROXY_AUTHORIZATION": ctx.Request.Header.Get("Proxy-Authorization"),
		"HTTP_USER_AGENT":          ctx.Request.Header.Get("User-Agent"),
		"PATH_INFO":                ctx.Request.URL.Path,
		"PATH_TRANSLATED":          ctx.Request.URL.Path,
		"QUERY_STRING":             ctx.Request.URL.RawQuery,
		"REMOTE_ADDR":              ctx.ClientIP(),
		"REMOTE_HOST":              ctx.ClientIP(),
		"REMOTE_USER":              "",
		"REQUEST_METHOD":           ctx.Request.Method,
		"SCRIPT_NAME":              scriptName,
		"SERVER_PROTOCOL":          ctx.Request.Proto,
		"SERVER_SOFTWARE":          "machbase-neo",
	}
	return m
}
