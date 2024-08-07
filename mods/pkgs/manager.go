package pkgs

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v4"
	"github.com/machbase/neo-pkgdev/pkgs"
	"github.com/machbase/neo-server/mods/logging"
)

type PkgManager struct {
	log    logging.Log
	roster *pkgs.Roster
	envs   []string
}

func NewPkgManager(pkgsDir string) (*PkgManager, error) {
	roster, err := pkgs.NewRoster(pkgsDir)
	if err != nil {
		return nil, err
	}
	envs := []string{}
	if b, err := os.Executable(); err == nil {
		b, _ = filepath.Abs(b)
		envs = append(envs, fmt.Sprintf("MACHBASE_NEO=%s", b))
	}
	return &PkgManager{
		log:    logging.GetLog("pkgmgr"),
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
	if name == "" {
		prj, err := pm.roster.FeaturedPackages()
		if err != nil {
			return nil, err
		}
		ret := &pkgs.PackageSearchResult{}
		for _, pkg := range prj.Featured {
			cache, err := pm.roster.LoadPackageCache(pkg)
			if err != nil {
				pm.log.Error("failed to load package cache", pkg, err)
			} else {
				ret.Possibles = append(ret.Possibles, cache)
			}
		}
		return ret, nil
	} else {
		return pm.roster.SearchPackage(name, possible)
	}
}

func (pm *PkgManager) Install(name string, output io.Writer) (*pkgs.InstallStatus, error) {
	ret := pm.roster.Install([]string{name}, pm.envs)
	if len(ret) == 0 {
		return nil, fmt.Errorf("failed to install %s", name)
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
	pm.log.Info("uninstalled", name)
	return nil
}

func (pm *PkgManager) HttpAppRouter(r gin.IRouter) {
	r.Any("/apps/:name/*path", func(ctx *gin.Context) {
		name := ctx.Param("name")
		path := ctx.Param("path")
		ctx.Request.URL.Path = path
		inst, err := pm.roster.InstalledVersion(name)
		if err != nil {
			ctx.JSON(404, gin.H{"success": false, "reason": err.Error()})
			return
		}
		fs := http.FileServer(http.Dir(inst.CurrentPath))
		fs.ServeHTTP(ctx.Writer, ctx.Request)
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
