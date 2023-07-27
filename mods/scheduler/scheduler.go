package scheduler

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	schedrpc "github.com/machbase/neo-grpc/schedule"
	logging "github.com/machbase/neo-server/mods/logging"
	"github.com/machbase/neo-server/mods/tql"
	spi "github.com/machbase/neo-spi"
	"github.com/robfig/cron/v3"
)

func NewService(opts ...Option) Service {
	ret := &svr{
		log: logging.GetLog("scheduler"),
	}
	for _, o := range opts {
		o(ret)
	}
	ret.crons = cron.New(
		cron.WithLocation(time.Local),
		cron.WithSeconds(),
		cron.WithLogger(ret),
	)
	return ret
}

type Service interface {
	schedrpc.ManagementServer
	Start() error
	Stop()

	AddEntry(Entry) error
}

type svr struct {
	Service
	log       logging.Log
	confDir   string
	crons     *cron.Cron
	tqlLoader tql.Loader
	db        spi.Database
	verbose   bool
}

type Option func(*svr)

func WithConfigDirPath(path string) Option {
	return func(s *svr) {
		s.confDir = path
	}
}

func WithTqlLoader(ldr tql.Loader) Option {
	return func(s *svr) {
		s.tqlLoader = ldr
	}
}

func WithDatabase(db spi.Database) Option {
	return func(s *svr) {
		s.db = db
	}
}

func WithVerbose(flag bool) Option {
	return func(s *svr) {
		s.verbose = flag
	}
}

func (s *svr) Start() error {
	s.iterateConfigs(func(define *Define) bool {
		if err := Register(s, define); err == nil {
			s.log.Infof("add schedule %s type=%s", define.Name, define.Type)
		} else {
			s.log.Errorf("fail to add schedule %s type=%s, %s", define.Name, define.Type, err.Error())
		}
		return true
	})
	go s.crons.Run()
	s.log.Info("started.")
	return nil
}

func (s *svr) Stop() {
	UnregisterAll()

	ctx := s.crons.Stop()
	<-ctx.Done()
	s.log.Info("closed.")
}

func (s *svr) iterateConfigs(cb func(define *Define) bool) error {
	if cb == nil {
		return nil
	}
	entries, err := os.ReadDir(s.confDir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".json") || entry.IsDir() {
			continue
		}
		content, err := os.ReadFile(filepath.Join(s.confDir, entry.Name()))
		if err != nil {
			s.log.Warnf("schedule def file", err.Error())
			continue
		}
		def := &Define{}
		if err = json.Unmarshal(content, def); err != nil {
			s.log.Warnf("schedule def format", err.Error())
			continue
		}
		def.Name = strings.TrimSuffix(entry.Name(), ".json")
		def.Type = Type(strings.ToLower(string(def.Type)))
		flag := cb(def)
		if !flag {
			break
		}
	}
	return nil
}

func (s *svr) loadConfig(name string) (*Define, error) {
	name = strings.ToUpper(name)
	path := filepath.Join(s.confDir, fmt.Sprintf("%s.json", name))
	content, err := os.ReadFile(path)
	if err != nil {
		s.log.Warnf("bridge def file", err.Error())
		return nil, err
	}
	def := &Define{}
	if err := json.Unmarshal(content, def); err != nil {
		s.log.Warnf("bridge def format", err.Error())
		return nil, err
	}
	def.Name = name
	return def, nil
}

func (s *svr) saveConfig(def *Define) error {
	buf, err := json.MarshalIndent(def, "", "\t")
	if err != nil {
		s.log.Warnf("bridge def file", err.Error())
		return err
	}

	name := strings.ToUpper(def.Name)
	name = strings.ReplaceAll(name, "/", "_")
	name = strings.ReplaceAll(name, "\\", "_")
	name = strings.ReplaceAll(name, "'", "_")
	name = strings.ReplaceAll(name, "$", "_")
	name = strings.ReplaceAll(name, "*", "_")
	name = strings.ReplaceAll(name, "?", "_")
	path := filepath.Join(s.confDir, fmt.Sprintf("%s.json", name))
	return os.WriteFile(path, buf, 00600)
}

func (s *svr) removeConfig(name string) error {
	name = strings.ToUpper(name)
	path := filepath.Join(s.confDir, fmt.Sprintf("%s.json", name))
	return os.Remove(path)
}

func (s *svr) AddEntry(entry Entry) error {
	if entry.AutoStart() {
		if err := entry.Start(); err != nil {
			return err
		}
	}
	return nil
}

// implements cron.Log
func (s *svr) Info(msg string, keysAndValues ...any) {
	if !s.verbose {
		return
	}
	var next time.Time
	var entryId int = -1
	var extra []string
	for i := 0; i < len(keysAndValues)-1; i += 2 {
		switch keysAndValues[i] {
		case "now":
			continue
		case "next":
			next = keysAndValues[i+1].(time.Time)
		case "entry":
			if eid, ok := keysAndValues[i+1].(cron.EntryID); ok {
				entryId = int(eid)
			}
		default:
			extra = append(extra, fmt.Sprintf("%s=%v", keysAndValues[i], keysAndValues[i+1]))
		}
	}
	if entryId == -1 {
		s.log.Debug(msg)
	} else {
		s.log.Debugf("%s entry[%d] next=%s %s", msg, entryId, next, strings.Join(extra, ","))
	}
}

func (s *svr) Error(err error, msg string, keysAndValues ...any) {
	s.log.Error(append([]any{err.Error(), msg}, keysAndValues...)...)
}
