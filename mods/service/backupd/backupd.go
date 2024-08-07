package backupd

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/machbase/neo-server/api"
	"github.com/machbase/neo-server/mods/logging"
)

func NewService(opts ...Option) Service {
	ret := &service{
		log: logging.GetLog("backupd"),
	}
	for _, opt := range opts {
		opt(ret)
	}
	return ret
}

type Service interface {
	Start() error
	Stop()

	HttpRouter(r gin.IRouter)
	HttpHandler(ctx *gin.Context)
}

type Option func(s *service)

type service struct {
	log     logging.Log
	baseDir string
	db      api.Database
	cutset  string
	backup  backup
	mutex   sync.Mutex
}

var _ = Service((*service)(nil))

func WithBaseDir(baseDir string) Option {
	return func(s *service) {
		// baseDir, err := filepath.Abs(baseDir)
		s.baseDir = baseDir
	}
}

func WithDatabase(db api.Database) Option {
	return func(s *service) {
		s.db = db
	}
}

func (s *service) Start() error {
	s.log.Infof("backupd started at %s", s.baseDir)
	if runtime.GOOS == "windows" {
		s.cutset = "\\"
	} else {
		s.cutset = "/"
	}

	return nil
}

func (s *service) Stop() {
	s.log.Infof("backupd stop")
}

func (s *service) HttpRouter(r gin.IRouter) {
	// /web/api/backup/archives
	r.GET("/archives", s.handleArchives) // returns backup list
	// r.GET("/archives/:id", s.HttpHandler) // returns the backup detail
	r.POST("/archive", s.handleArchive)             // backup
	r.GET("/archive/status", s.handleArchiveStatus) // returns the backup detail

	// /web/api/backup/mounts
	r.GET("/mounts", s.handleMounts)           // returns mount list
	r.POST("/mounts/:name", s.handleMount)     // mount archive dir to the database
	r.DELETE("/mounts/:name", s.handleUnmount) // unmount
}

func (s *service) HttpHandler(ctx *gin.Context) {
	s.log.Info("backup api request", ctx.Request.Method, ctx.Request.RequestURI)
	ctx.JSON(200, gin.H{
		"message": "pong",
	})
}

func (s *service) handleArchiveStatus(ctx *gin.Context) {
	tick := time.Now()
	rsp := gin.H{"success": false, "reason": "not specified"}

	status := s.backup
	if !status.IsRunning {
		if status.err != nil {
			rsp["reason"] = status.Message
			rsp["elapse"] = time.Since(tick).String()
			ctx.JSON(http.StatusInternalServerError, rsp)
			return
		}
	}

	rsp["success"] = true
	rsp["reason"] = "success"
	rsp["data"] = status.Info
	rsp["elapse"] = time.Since(tick).String()
	ctx.JSON(http.StatusOK, rsp)
}

type backup struct {
	IsRunning bool
	Message   string
	err       error
	Info      Archive
}

type Archive struct {
	Type      string `json:"type" binding:"required"` // database, table
	TableName string `json:"tableName"`
	Duration  struct {
		Type  string `json:"type" binding:"required"` // full, incremetal, time
		After string `json:"after"`
		From  string `json:"from"`
		To    string `json:"to"`
	} `json:"duration"`
	Path string `json:"path" binding:"required"`
}

func (s *service) handleArchive(ctx *gin.Context) {
	tick := time.Now()
	rsp := gin.H{"success": false, "reason": "not specified"}

	originArchive := Archive{}
	if err := ctx.ShouldBind(&originArchive); err != nil {
		rsp["reason"] = err.Error()
		rsp["elapse"] = time.Since(tick).String()
		ctx.JSON(http.StatusBadRequest, rsp)
		return
	}
	copyArchive := originArchive

	if s.backup.IsRunning {
		rsp["reason"] = "backup is running."
		rsp["elapse"] = time.Since(tick).String()
		ctx.JSON(http.StatusOK, rsp)
		return
	}

	s.log.Info("before path: ", copyArchive.Path)
	s.log.Info("before duration.path: ", copyArchive.Duration.After)

	if !filepath.IsAbs(copyArchive.Path) {
		copyArchive.Path = filepath.Join(s.baseDir, copyArchive.Path)
	}

	if runtime.GOOS == "windows" {
		copyArchive.Path = strings.ReplaceAll(copyArchive.Path, "/", "\\")
		copyArchive.Path = strings.ReplaceAll(copyArchive.Path, "\\", "\\\\")
	}

	s.log.Info("path: ", copyArchive.Path)
	s.log.Info("duration.path: ", copyArchive.Duration.After)

	if _, err := os.Stat(copyArchive.Path); !os.IsNotExist(err) {
		rsp["reason"] = fmt.Sprintf("backup exist %q", copyArchive.Path)
		rsp["elapse"] = time.Since(tick).String()
		ctx.JSON(http.StatusBadRequest, rsp)
		return
	}

	var backupTarget string
	switch strings.ToLower(copyArchive.Type) {
	case "database":
		backupTarget = "DATABASE"
	case "table":
		if copyArchive.TableName == "" {
			rsp["reason"] = "table name is empty"
			rsp["elapse"] = time.Since(tick).String()
			ctx.JSON(http.StatusBadRequest, rsp)
			return
		}
		backupTarget = fmt.Sprintf("TABLE %s", copyArchive.TableName)
	default:
		rsp["reason"] = fmt.Sprintf("invalid backup %q", copyArchive.Type)
		rsp["elapse"] = time.Since(tick).String()
		ctx.JSON(http.StatusBadRequest, rsp)
		return
	}

	var sqlText string
	switch strings.ToLower(copyArchive.Duration.Type) {
	case "full":
		sqlText = fmt.Sprintf("BACKUP %s INTO DISK = '%s'", backupTarget, copyArchive.Path)
	case "incremental":
		if !filepath.IsAbs(copyArchive.Duration.After) {
			copyArchive.Duration.After = filepath.Join(s.baseDir, copyArchive.Duration.After)
		}
		if runtime.GOOS == "windows" {
			copyArchive.Duration.After = strings.ReplaceAll(copyArchive.Duration.After, "/", "\\")
			copyArchive.Duration.After = strings.ReplaceAll(copyArchive.Duration.After, "\\", "\\\\")
		}
		sqlText = fmt.Sprintf("BACKUP %s AFTER '%s' INTO DISK = '%s'", backupTarget, copyArchive.Duration.After, copyArchive.Path) //after add
	case "time":
		fromSql := "0"
		if copyArchive.Duration.From != "" {
			fromSql = copyArchive.Duration.From
		}
		toSql := "sysdate"
		if copyArchive.Duration.To != "" {
			toSql = fmt.Sprintf("FROM_UNIXTIME(%s)", copyArchive.Duration.To)
		}
		sqlText = fmt.Sprintf(`
		BACKUP %s FROM FROM_UNIXTIME(%s)
				  TO %s
				  INTO DISK = '%s'`,
			backupTarget, fromSql, toSql, copyArchive.Path,
		)
		sqlText = strings.TrimSpace(sqlText)
	default:
		rsp["reason"] = fmt.Sprintf("invalid backup type %q", copyArchive.Duration.Type)
		rsp["elapse"] = time.Since(tick).String()
		ctx.JSON(http.StatusBadRequest, rsp)
		return
	}

	s.log.Info("sqlText: ", sqlText)

	conn, err := s.db.Connect(ctx, api.WithTrustUser("sys"))
	if err != nil {
		rsp["reason"] = err.Error()
		rsp["elapse"] = time.Since(tick).String()
		ctx.JSON(http.StatusInternalServerError, rsp)
		return
	}

	s.backupManager(conn, originArchive, sqlText)

	rsp["success"] = true
	rsp["reason"] = "success"
	rsp["elapse"] = time.Since(tick).String()
	ctx.JSON(http.StatusOK, rsp)
}

func (s *service) backupManager(conn api.Conn, archive Archive, sqlText string) {
	go func() {
		defer conn.Close()

		if isLock := s.mutex.TryLock(); isLock {
			defer s.mutex.Unlock()
			s.backup.IsRunning = true
			s.backup.Info = archive

			if result := conn.Exec(context.Background(), sqlText); result.Err() != nil {
				s.backup.err = result.Err()
				s.backup.Message = result.Message()
			} else {
				s.backup.err = nil
				s.backup.Info = Archive{}
				s.backup.Message = ""
			}
			s.backup.IsRunning = false
		} else {
			s.backup.IsRunning = true
		}
	}()
}

type ArchiveInfo struct {
	Path      string `json:"path"`
	IsMount   bool   `json:"isMount"`
	MountName string `json:"mountName,omitempty"`
}

func (s *service) handleArchives(ctx *gin.Context) {
	tick := time.Now()
	rsp := gin.H{"success": false, "reason": "not specified"}

	dirs, err := os.ReadDir(s.baseDir)
	if err != nil {
		rsp["reason"] = err.Error()
		rsp["elapse"] = time.Since(tick).String()
		ctx.JSON(http.StatusInternalServerError, rsp)
		return
	}

	// mount status check
	conn, err := s.db.Connect(ctx, api.WithTrustUser("sys"))
	if err != nil {
		rsp["reason"] = err.Error()
		rsp["elapse"] = time.Since(tick).String()
		ctx.JSON(http.StatusBadRequest, rsp)
		return
	}
	defer conn.Close()

	rows, err := conn.Query(ctx, "SELECT PATH, MOUNTDB FROM V$STORAGE_MOUNT_DATABASES")
	if err != nil {
		rsp["reason"] = err.Error()
		rsp["elapse"] = time.Since(tick).String()
		ctx.JSON(http.StatusInternalServerError, rsp)
		return
	}
	defer rows.Close()

	mountMap := map[string]string{}
	for rows.Next() {
		var path, name string
		err = rows.Scan(&path, &name)
		if err != nil {
			break
		}
		if runtime.GOOS == "windows" {
			path = strings.ReplaceAll(path, "/", "\\")
		}
		mountMap[path] = name
	}
	if err != nil {
		rsp["reason"] = err.Error()
		rsp["elapse"] = time.Since(tick).String()
		ctx.JSON(http.StatusInternalServerError, rsp)
		return
	}

	archiveInfos := []ArchiveInfo{}
	for _, dir := range dirs {
		if !dir.IsDir() {
			continue
		}
		// baseDir check
		if dir.Name() == "SYSTEM_TABLESPACE" || dir.Name() == "TAG_TABLESPACE" {
			continue
		}
		name := filepath.Join(s.baseDir, dir.Name())
		subDir, err := os.ReadDir(name)
		if err != nil {
			rsp["reason"] = err.Error()
			rsp["elapse"] = time.Since(tick).String()
			ctx.JSON(http.StatusInternalServerError, rsp)
			return
		}
		for _, file := range subDir {
			if file.IsDir() {
				continue
			}
			if file.Name() == "backup.dat" {
				archiveInfo := ArchiveInfo{Path: dir.Name()}
				key := filepath.Join(s.baseDir, dir.Name())
				if val, ok := mountMap[key]; ok {
					archiveInfo.IsMount = true
					archiveInfo.MountName = val
				}
				archiveInfos = append(archiveInfos, archiveInfo)
				break
			}
		}
	}

	rsp["success"] = true
	rsp["reason"] = "success"
	rsp["data"] = archiveInfos
	rsp["elapse"] = time.Since(tick).String()
	ctx.JSON(http.StatusOK, rsp)
}

func (s *service) handleMount(ctx *gin.Context) {
	tick := time.Now()
	rsp := gin.H{"success": false, "reason": "not specified"}

	name := ctx.Param("name")
	if name == "" {
		rsp["reason"] = "invalid mount name"
		rsp["elapse"] = time.Since(tick).String()
		ctx.JSON(http.StatusBadRequest, rsp)
		return
	}

	mount := struct {
		Path string `json:"path" binding:"required"`
	}{}

	if err := ctx.ShouldBind(&mount); err != nil {
		rsp["reason"] = err.Error()
		rsp["elapse"] = time.Since(tick).String()
		ctx.JSON(http.StatusBadRequest, rsp)
		return
	}

	var sqlText string
	if filepath.IsAbs(mount.Path) {
		sqlText = fmt.Sprintf("MOUNT DATABASE '%s' TO %s", mount.Path, name)
	} else {
		baseMountPath := filepath.Join(s.baseDir, mount.Path)
		sqlText = fmt.Sprintf("MOUNT DATABASE '%s' TO %s", baseMountPath, name)
	}

	if runtime.GOOS == "windows" { // windows
		sqlText = strings.ReplaceAll(sqlText, "\\", "\\\\")
	}

	conn, err := s.db.Connect(ctx, api.WithTrustUser("sys"))
	if err != nil {
		rsp["reason"] = err.Error()
		rsp["elapse"] = time.Since(tick).String()
		ctx.JSON(http.StatusBadRequest, rsp)
		return
	}
	defer conn.Close()

	result := conn.Exec(ctx, sqlText)
	if result.Err() != nil {
		rsp["reason"] = result.Message()
		rsp["elapse"] = time.Since(tick).String()
		ctx.JSON(http.StatusInternalServerError, rsp)
		return
	}

	rsp["success"] = true
	rsp["reason"] = "success"
	rsp["elapse"] = time.Since(tick).String()
	ctx.JSON(http.StatusOK, rsp)
}

func (s *service) handleUnmount(ctx *gin.Context) {
	tick := time.Now()
	rsp := gin.H{"success": false, "reason": "not specified"}

	name := ctx.Param("name")
	if name == "" {
		rsp["reason"] = "invalid mount name"
		rsp["elapse"] = time.Since(tick).String()
		ctx.JSON(http.StatusBadRequest, rsp)
		return
	}

	conn, err := s.db.Connect(ctx, api.WithTrustUser("sys"))
	if err != nil {
		rsp["reason"] = err.Error()
		rsp["elapse"] = time.Since(tick).String()
		ctx.JSON(http.StatusBadRequest, rsp)
		return
	}
	defer conn.Close()

	sqlText := fmt.Sprintf("UNMOUNT DATABASE %s", name)
	result := conn.Exec(ctx, sqlText)
	if result.Err() != nil {
		rsp["reason"] = result.Message()
		rsp["elapse"] = time.Since(tick).String()
		ctx.JSON(http.StatusInternalServerError, rsp)
		return
	}

	rsp["success"] = true
	rsp["reason"] = "success"
	rsp["elapse"] = time.Since(tick).String()
	ctx.JSON(http.StatusOK, rsp)
}

type StorageMount struct {
	Name            string `json:"name"`
	Path            string `json:"path"`
	BackupTBSID     int64  `json:"tbsid"`
	BackupSCN       int64  `json:"scn"`
	MountDB         string `json:"mountdb"`
	DBBeginTime     string `json:"dbBeginTime"`
	DBEndTime       string `json:"dbEndTime"`
	BackupBeginTime string `json:"backupBeginTime"`
	BackupEndTime   string `json:"backupEndTime"`
	Flag            int    `json:"flag"`
}

func (s *service) handleMounts(ctx *gin.Context) {
	tick := time.Now()
	rsp := gin.H{"success": false, "reason": "not specified"}

	conn, err := s.db.Connect(ctx, api.WithTrustUser("sys"))
	if err != nil {
		rsp["reason"] = err.Error()
		rsp["elapse"] = time.Since(tick).String()
		ctx.JSON(http.StatusBadRequest, rsp)
		return
	}
	defer conn.Close()

	rows, err := conn.Query(ctx, "SELECT * FROM V$STORAGE_MOUNT_DATABASES")
	if err != nil {
		rsp["reason"] = err.Error()
		rsp["elapse"] = time.Since(tick).String()
		ctx.JSON(http.StatusInternalServerError, rsp)
		return
	}
	defer rows.Close()

	mounts := []StorageMount{}
	for rows.Next() {
		mount := StorageMount{}
		err = rows.Scan(&mount.Name, &mount.Path, &mount.BackupTBSID, &mount.BackupSCN, &mount.MountDB, &mount.DBBeginTime, &mount.DBEndTime, &mount.BackupBeginTime, &mount.BackupEndTime, &mount.Flag)
		if err != nil {
			break
		}

		if strings.HasPrefix(mount.Path, s.baseDir) {
			mount.Path = strings.TrimPrefix(mount.Path, s.baseDir+s.cutset)
		}

		mounts = append(mounts, mount)
	}
	if err != nil {
		rsp["reason"] = err.Error()
		rsp["elapse"] = time.Since(tick).String()
		ctx.JSON(http.StatusInternalServerError, rsp)
		return
	}

	rsp["success"] = true
	rsp["reason"] = "success"
	rsp["data"] = mounts
	rsp["elapse"] = time.Since(tick).String()
	ctx.JSON(http.StatusOK, rsp)
}
