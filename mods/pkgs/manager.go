package pkgs

import (
	"fmt"
	"io"
	"net/http"
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
}

func NewPkgManager(pkgsDir string) (*PkgManager, error) {
	roster, err := pkgs.NewRoster(pkgsDir)
	if err != nil {
		return nil, err
	}
	return &PkgManager{
		log:    logging.GetLog("pkgmgr"),
		roster: roster,
	}, nil
}

func (pm *PkgManager) Sync() error {
	return pm.roster.Sync()
}

// if name is empty, it will return all featured packages
func (pm *PkgManager) Search(name string, possible int) (*pkgs.PackageSearchResult, error) {
	if name == "" {
		prj, err := pm.roster.RootMeta()
		if err != nil {
			return nil, err
		}
		ret := &pkgs.PackageSearchResult{}
		for _, pkg := range prj.Featured {
			if meta, err := pm.roster.LoadPackageMeta(pkg); err != nil {
				pm.log.Error("failed to load package meta", pkg, err)
			} else {
				cache, err := pm.roster.LoadPackageCache(pkg, meta, false)
				if err != nil {
					pm.log.Error("failed to load package cache", pkg, err)
				} else {
					ret.Possibles = append(ret.Possibles, cache)
				}
			}
			if possible > 0 && len(ret.Possibles) >= possible {
				break
			}
		}
		return ret, nil
	} else {
		return pm.roster.SearchPackage(name, possible)
	}
}

func (pm *PkgManager) Install(name string, output io.Writer) (*pkgs.PackageCache, error) {
	err := pm.roster.Install(name, output)
	if err != nil {
		return nil, err
	}
	cache, err := pm.roster.ReadCache(name)
	if err != nil {
		return nil, err
	}

	pm.log.Info("installed", name, cache.InstalledVersion, cache.InstalledPath)
	return cache, nil
}

func (pm *PkgManager) Uninstall(name string, output io.Writer) (*pkgs.PackageCache, error) {
	err := pm.roster.Uninstall(name, output)
	if err != nil {
		return nil, err
	}
	cache, err := pm.roster.ReadCache(name)
	if err != nil {
		return nil, err
	}
	pm.log.Info("uninstalled", name)
	return cache, nil
}

func (pm *PkgManager) HttpAppRouter(r gin.IRouter) {
	r.Any("/apps/:name/*path", func(ctx *gin.Context) {
		name := ctx.Param("name")
		path := ctx.Param("path")
		ctx.Request.URL.Path = path
		fs := http.FileServer(http.Dir(pm.roster.InstallDir(name)))
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
	r.GET("/sync", pm.doSync)
	r.GET("/install/:name", pm.doInstall)
	r.GET("/uninstall/:name", pm.doUninstall)
}

// doSearch is a handler for /search
// it requires a query parameter "name"
// if the name is empty, it will return 400
// if the name is not found, it will return 404
// if the name is found, it will return 200
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

func (pm *PkgManager) doSync(c *gin.Context) {
	ts := time.Now()
	err := pm.Sync()
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
			"log":     output.String(),
		})
		return
	}
	c.JSON(200, gin.H{
		"success": true,
		"reason":  "success",
		"elapse":  fmt.Sprintf("%v", time.Since(ts)),
		"data":    cache,
		"log":     output.String(),
	})
}

func (pm *PkgManager) doUninstall(c *gin.Context) {
	ts := time.Now()
	name := c.Param("name")

	output := &strings.Builder{}
	cache, err := pm.Uninstall(name, output)
	if err != nil {
		c.JSON(500, gin.H{
			"success": false,
			"reason":  err.Error(),
			"elapse":  fmt.Sprintf("%v", time.Since(ts)),
			"log":     output.String(),
		})
		return
	}
	c.JSON(200, gin.H{
		"success": true,
		"reason":  "success",
		"elapse":  fmt.Sprintf("%v", time.Since(ts)),
		"data":    cache,
		"log":     output.String(),
	})
}
