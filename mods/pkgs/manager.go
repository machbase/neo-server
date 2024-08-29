package pkgs

import (
	"fmt"
	"io"
	"io/fs"
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
	log         logging.Log
	pkgsDir     string
	roster      *pkgs.Roster
	installEnvs []string
	pkgBackends map[string]*PkgBackend
}

func NewPkgManager(pkgsDir string, envMap map[string]string) (*PkgManager, error) {
	log := logging.GetLog("pkgmgr")
	roster, err := pkgs.NewRoster(pkgsDir, pkgs.WithLogger(log), pkgs.WithSyncWhenInitialized(true))
	if err != nil {
		return nil, err
	}
	// envs
	envs := []string{}
	for k, v := range envMap {
		envs = append(envs, fmt.Sprintf("%s=%s", k, v))
	}
	if b, err := os.Executable(); err == nil {
		b, _ = filepath.Abs(b)
		envs = append(envs, fmt.Sprintf("MACHBASE_NEO=%s", b))
	}

	// expose installed packages to the server side filesystem
	fsmgr := ssfs.Default()
	pkgBackend := make(map[string]*PkgBackend)
	entries, _ := os.ReadDir(filepath.Join(pkgsDir, "dist"))
	for _, ent := range entries {
		if !ent.IsDir() {
			continue
		}
		current := filepath.Join(pkgsDir, "dist", ent.Name(), "current")
		if _, err := os.Stat(current); err == nil {
			path, err := pkgs.Readlink(current)
			if err != nil {
				continue
			}
			pkgName := ent.Name()
			baseName := filepath.Base(path)
			fsmgr.Mount(fmt.Sprintf("/apps/%s", pkgName), filepath.Join(pkgsDir, "dist", pkgName, baseName), true)

			settings, err := LoadPkgBackend(pkgsDir, pkgName, envs)
			if err != nil {
				log.Warnf("failed to load backend settings for %s, %v", pkgName, err)
			}
			if settings != nil {
				pkgBackend[pkgName] = settings
			}
		}
	}
	return &PkgManager{
		log:         log,
		pkgsDir:     pkgsDir,
		roster:      roster,
		installEnvs: envs,
		pkgBackends: pkgBackend,
	}, nil
}

func (pm *PkgManager) Start() {
	for _, ps := range pm.pkgBackends {
		if ps.AutoStart {
			ps.Start()
		}
	}
}

func (pm *PkgManager) Stop() {
	for _, ps := range pm.pkgBackends {
		ps.Stop()
	}
}

// add environment variable which will be used while“ installing/uninstalling packages
func (pm *PkgManager) AddEnv(key, value string) {
	pm.installEnvs = append(pm.installEnvs, fmt.Sprintf("%s=%s", key, value))
}

// if name is empty, it will return all featured packages
func (pm *PkgManager) Search(name string, possible int) (*pkgs.PackageSearchResult, error) {
	return pm.roster.Search(name, possible)
}

func (pm *PkgManager) Install(name string, output io.Writer) (*pkgs.InstallStatus, error) {
	ret := pm.roster.Install([]string{name}, pm.installEnvs)
	if len(ret) == 0 || ret[0].Installed == nil {
		return nil, fmt.Errorf("failed to install %s", name)
	}

	fsmgr := ssfs.Default()
	mntPoint := fmt.Sprintf("/apps/%s", name)
	lst := fsmgr.ListMounts()
	if !slices.Contains(lst, mntPoint) {
		err := fsmgr.Mount(mntPoint, ret[0].Installed.Path, true)
		if err != nil {
			pm.log.Warnf("%s is not mounted, %w", name, err)
		} else {
			pm.log.Info("mounted", name, ret[0].Installed.Path)
		}
	}

	pm.log.Info("installed", name, ret[0].Installed.Version, ret[0].Installed.Path)
	output.Write([]byte(ret[0].Output))
	if backend, _ := LoadPkgBackend(pm.pkgsDir, name, pm.installEnvs); backend != nil {
		pm.pkgBackends[name] = backend
	}
	return ret[0], nil
}

func (pm *PkgManager) Uninstall(name string, output io.Writer) error {
	if pb, ok := pm.pkgBackends[name]; ok && pb != nil {
		pb.Stop()
	}
	err := pm.roster.Uninstall(name, output, pm.installEnvs)
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
	delete(pm.pkgBackends, name)
	pm.log.Info("uninstalled", name)
	return nil
}

type DirFS http.Dir

func (df DirFS) Open(name string) (http.File, error) {
	// prevent reading hidden files
	if strings.HasPrefix(filepath.Base(name), ".") {
		return nil, fs.ErrNotExist
	}
	path := name
	for {
		dir, file := filepath.Split(path)
		if !strings.HasPrefix(dir, "/.") && file == "" {
			break
		}
		if strings.HasPrefix(dir, "/.") || strings.HasPrefix(file, ".") {
			return nil, fs.ErrNotExist
		}
		path = dir
	}
	return http.Dir(df).Open(name)
}

func (pm *PkgManager) HttpAppRouter(r gin.IRouter, tqlHandler gin.HandlerFunc) {
	r.Any("/apps/:name/*path", func(ctx *gin.Context) {
		name := ctx.Param("name")
		path := ctx.Param("path")
		if bp := pm.pkgBackends[name]; bp != nil && bp.HttpProxy != nil && bp.HttpProxy.Match(path) {
			bp.HttpProxy.Handle(ctx)
			return
		}
		if strings.HasSuffix(path, ".tql") {
			path = fmt.Sprintf("/apps/%s/%s", name, path)
			for i := range ctx.Params {
				if ctx.Params[i].Key == "path" {
					ctx.Params[i].Value = path
				}
			}
			tqlHandler(ctx)
		} else if strings.HasPrefix(path, "/.storage/") {
			if ctx.Request.Method == http.MethodGet {
				pm.doReadStoragePublic(ctx)
			} else if ctx.Request.Method == http.MethodPost {
				pm.doWriteStoragePublic(ctx)
			} else {
				ctx.JSON(405, gin.H{"success": false, "reason": "method not allowed"})
			}
		} else {
			ctx.Request.URL.Path = path
			inst, err := pm.roster.InstalledVersion(name)
			if err != nil {
				ctx.JSON(404, gin.H{"success": false, "reason": err.Error()})
				return
			}
			fs := http.FileServer(DirFS(http.Dir(inst.CurrentPath)))
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
	r.GET("/process/:name/:action", pm.doProcess)
	r.POST("/storage/:name/*path", pm.doWriteStorageSys)
	r.GET("/storage/:name/*path", pm.doReadStorageSys)
}

func (pm *PkgManager) doWriteStorageSys(ctx *gin.Context) {
	pm.writeStorage(ctx, true)
}

func (pm *PkgManager) doWriteStoragePublic(ctx *gin.Context) {
	pm.writeStorage(ctx, false)
}

func (pm *PkgManager) doReadStorageSys(ctx *gin.Context) {
	pm.readStorage(ctx, true)
}

func (pm *PkgManager) doReadStoragePublic(ctx *gin.Context) {
	pm.readStorage(ctx, false)
}

func (pm *PkgManager) writeStorage(ctx *gin.Context, isSysUser bool) {
	name := ctx.Param("name")
	path := ctx.Param("path")
	if ctx.Request.Method != http.MethodPost {
		ctx.JSON(405, gin.H{"success": false, "reason": "method not allowed"})
		return
	}
	path = strings.TrimPrefix(path, "/.storage/")
	if !isSysUser {
		// if client is not sys user, prevent writing hidden files
		for _, comp := range strings.Split(path, "/") {
			if strings.HasPrefix(comp, ".") {
				ctx.JSON(404, gin.H{"success": false, "reason": "not found"})
				return
			}
		}
	}
	inst, err := pm.roster.InstalledVersion(name)
	if err != nil || inst.Path == "" {
		ctx.JSON(404, gin.H{"success": false, "reason": err.Error()})
		return
	}
	realPath := filepath.Join(pm.pkgsDir, "dist", name, "storage", filepath.Clean(path))
	dir := filepath.Dir(realPath)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		if err := os.MkdirAll(dir, 0755); err != nil {
			ctx.JSON(500, gin.H{"success": false, "reason": err.Error()})
			return
		}
	}
	f, err := os.OpenFile(realPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		ctx.JSON(500, gin.H{"success": false, "reason": err.Error()})
		return
	}
	defer f.Close()
	_, err = io.Copy(f, ctx.Request.Body)
	if err != nil {
		ctx.JSON(500, gin.H{"success": false, "reason": err.Error()})
		return
	}
	ctx.JSON(200, gin.H{"success": true, "reason": "success"})
}

func (pm *PkgManager) readStorage(ctx *gin.Context, isSysUser bool) {
	name := ctx.Param("name")
	path := ctx.Param("path")

	if ctx.Request.Method != http.MethodGet {
		ctx.JSON(405, gin.H{"success": false, "reason": "method not allowed"})
		return
	}

	path = strings.TrimPrefix(path, "/.storage/")
	if !isSysUser {
		// if client is not sys user, prevent reading hidden files
		for _, comp := range strings.Split(path, "/") {
			if strings.HasPrefix(comp, ".") {
				ctx.JSON(404, gin.H{"success": false, "reason": "not found"})
				return
			}
		}
	}
	inst, err := pm.roster.InstalledVersion(name)
	if err != nil || inst.Path == "" {
		ctx.JSON(404, gin.H{"success": false, "reason": err.Error()})
		return
	}
	realPath := filepath.Join(pm.pkgsDir, "dist", name, "storage", filepath.Clean(path))
	ctx.File(realPath)
}

func (pm *PkgManager) doProcess(c *gin.Context) {
	ts := time.Now()
	name := c.Param("name")
	action := c.Param("action")
	if proc, ok := pm.pkgBackends[name]; ok && proc != nil {
		switch action {
		case "start":
			proc.Start()
		case "stop":
			proc.Stop()
		}
		status := proc.Status()
		c.JSON(200, gin.H{
			"success": true,
			"reason":  "success",
			"elapse":  time.Since(ts),
			"data":    map[string]any{"status": status},
		})
	} else {
		c.JSON(404, gin.H{
			"success": false,
			"reason":  "package not found",
			"elapse":  time.Since(ts),
		})
	}
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
