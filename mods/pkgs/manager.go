package pkgs

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v4"
	"github.com/machbase/neo-pkgdev/pkgs"
	"github.com/machbase/neo-server/mods/logging"
	"github.com/machbase/neo-server/mods/util/ssfs"
)

type PkgManager struct {
	log    logging.Log
	roster *pkgs.Roster
	envs   []string
}

func NewPkgManager(pkgsDir string, envMap map[string]string) (*PkgManager, error) {
	log := logging.GetLog("pkgmgr")
	roster, err := pkgs.NewRoster(pkgsDir, pkgs.WithLogger(log))
	if err != nil {
		return nil, err
	}
	fsmgr := ssfs.Default()
	entries, _ := os.ReadDir(filepath.Join(pkgsDir, "dist"))
	for _, ent := range entries {
		if !ent.IsDir() {
			continue
		}
		current := filepath.Join(pkgsDir, "dist", ent.Name(), "current")
		if _, err := os.Stat(current); err == nil {
			if _, err := os.Readlink(current); err == nil {
				fsmgr.Mount(fmt.Sprintf("/apps/%s", ent.Name()), filepath.Join(pkgsDir, "dist", ent.Name(), "current"), true)
			}
		}
	}
	envs := []string{}
	for k, v := range envMap {
		envs = append(envs, fmt.Sprintf("%s=%s", k, v))
	}
	if b, err := os.Executable(); err == nil {
		b, _ = filepath.Abs(b)
		envs = append(envs, fmt.Sprintf("MACHBASE_NEO=%s", b))
	}
	return &PkgManager{
		log:    log,
		roster: roster,
		envs:   envs,
	}, nil
}

// add environment variable which will be used while“ installing/uninstalling packages
func (pm *PkgManager) AddEnv(key, value string) {
	pm.envs = append(pm.envs, fmt.Sprintf("%s=%s", key, value))
}

// if name is empty, it will return all featured packages
func (pm *PkgManager) Search(name string, possible int) (*pkgs.PackageSearchResult, error) {
	return pm.roster.Search(name, possible)
}

func (pm *PkgManager) Install(name string, output io.Writer) (*pkgs.InstallStatus, error) {
	ret := pm.roster.Install([]string{name}, pm.envs)
	if len(ret) == 0 || ret[0].Installed == nil {
		return nil, fmt.Errorf("failed to install %s", name)
	}

	fsmgr := ssfs.Default()
	mntPoint := fmt.Sprintf("/apps/%s", name)
	lst := fsmgr.ListMounts()
	if !slices.Contains(lst, mntPoint) {
		err := fsmgr.Mount(mntPoint, ret[0].Installed.CurrentPath, true)
		if err != nil {
			pm.log.Warnf("%s is not mounted, %w", name, err)
		} else {
			pm.log.Info("mounted", name, ret[0].Installed.CurrentPath)
		}
	}

	pm.log.Info("installed", name, ret[0].Installed.Version, ret[0].Installed.Path)
	output.Write([]byte(ret[0].Output))
	return ret[0], nil
}

func (pm *PkgManager) Uninstall(name string, output io.Writer) error {
	err := pm.roster.Uninstall(name, output, pm.envs)
	if err != nil {
		return err
	}
	fsmgr := ssfs.Default()
	err = fsmgr.Unmount(fmt.Sprintf("/apps/%s", name))
	if err != nil {
		pm.log.Warnf("%s is not unmounted, %w", name, err)
	} else {
		pm.log.Info("unmounted", name)
	}
	pm.log.Info("uninstalled", name)
	return nil
}

func (pm *PkgManager) HttpAppRouter(r gin.IRouter, tqlHandler gin.HandlerFunc) {
	r.Any("/apps/:name/*path", func(ctx *gin.Context) {
		name := ctx.Param("name")
		path := ctx.Param("path")
		if strings.HasSuffix(path, ".tql") {
			path = fmt.Sprintf("/apps/%s/%s", name, path)
			for i := range ctx.Params {
				if ctx.Params[i].Key == "path" {
					ctx.Params[i].Value = path
				}
			}
			tqlHandler(ctx)
		} else {
			ctx.Request.URL.Path = path
			inst, err := pm.roster.InstalledVersion(name)
			if err != nil {
				ctx.JSON(404, gin.H{"success": false, "reason": err.Error()})
				return
			}
			fs := http.FileServer(http.Dir(inst.CurrentPath))
			fs.ServeHTTP(ctx.Writer, ctx.Request)
		}
	})
}

func (pm *PkgManager) HttpPkgRouter(r gin.IRouter) {
	r.GET("/search", pm.doSearch)
	r.Use(func(ctx *gin.Context) {
		// allow only SYS user
		obj, ok := ctx.Get("jwt-claim")
		if !ok {
			ctx.JSON(401, gin.H{"success": false, "reason": "unauthorized"})
			ctx.Abort()
			return
		}
		if token, ok := obj.(*jwt.RegisteredClaims); !ok {
			ctx.JSON(401, gin.H{"success": false, "reason": "unauthorized"})
			ctx.Abort()
			return
		} else {
			if strings.ToLower(token.Subject) != "sys" {
				ctx.JSON(401, gin.H{"success": false, "reason": "unauthorized"})
				ctx.Abort()
				return
			}
		}
	})
	r.GET("/update", pm.doUpdate)
	r.GET("/install/:name", pm.doInstall)
	r.GET("/uninstall/:name", pm.doUninstall)
}

// doSearch is a handler for /search
// if the name is empty, it will featured packages in possibles
// if there is an error, it will return 500
// if the possibles query parameter is 0, it will return only exact match
// if the possibles query parameter is > 0, it will return similar package names
func (pm *PkgManager) doSearch(c *gin.Context) {
	ts := time.Now()
	name := c.Query("name")
	possibles := c.Query("possibles")
	nposs, _ := strconv.ParseInt(possibles, 10, 32)
	result, err := pm.Search(name, int(nposs))
	if err != nil {
		c.JSON(500, gin.H{
			"success": false,
			"reason":  err.Error(),
			"elapse":  time.Since(ts),
		})
		return
	}
	c.JSON(200, gin.H{
		"success": true,
		"reason":  "success",
		"elapse":  fmt.Sprintf("%v", time.Since(ts)),
		"data":    result,
	})
}

func (pm *PkgManager) doUpdate(c *gin.Context) {
	ts := time.Now()
	stat, err := pm.roster.Update()
	if err != nil {
		c.JSON(500, gin.H{
			"success": false,
			"reason":  err.Error(),
			"elapse":  fmt.Sprintf("%v", time.Since(ts)),
		})
		return
	}
	c.JSON(200, gin.H{
		"success": true,
		"reason":  "success",
		"elapse":  fmt.Sprintf("%v", time.Since(ts)),
		"data":    stat,
	})
}

func (pm *PkgManager) doInstall(c *gin.Context) {
	ts := time.Now()
	name := c.Param("name")
	output := &strings.Builder{}
	cache, err := pm.Install(name, output)
	if err != nil {
		c.JSON(500, gin.H{
			"success": false,
			"reason":  err.Error(),
			"elapse":  fmt.Sprintf("%v", time.Since(ts)),
			"data":    map[string]any{"log": output.String()},
		})
		return
	}
	c.JSON(200, gin.H{
		"success": true,
		"reason":  "success",
		"elapse":  fmt.Sprintf("%v", time.Since(ts)),
		"data": map[string]any{
			"info": cache,
			"log":  output.String(),
		},
	})
}

func (pm *PkgManager) doUninstall(c *gin.Context) {
	ts := time.Now()
	name := c.Param("name")

	output := &strings.Builder{}
	err := pm.Uninstall(name, output)
	if err != nil {
		c.JSON(500, gin.H{
			"success": false,
			"reason":  err.Error(),
			"elapse":  fmt.Sprintf("%v", time.Since(ts)),
			"data": map[string]any{
				"log": output.String(),
			},
		})
		return
	}
	c.JSON(200, gin.H{
		"success": true,
		"reason":  "success",
		"elapse":  fmt.Sprintf("%v", time.Since(ts)),
		"data": map[string]any{
			"log": output.String(),
		},
	})
}
