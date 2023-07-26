package scheduler

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	logging "github.com/machbase/neo-server/mods/logging"
	"github.com/robfig/cron/v3"
)

func NewService(confDir string) Service {
	ret := &svr{
		log:     logging.GetLog("scheduler"),
		confDir: confDir,
	}
	ret.crons = cron.New(
		cron.WithLocation(time.Local),
		cron.WithSeconds(),
		cron.WithLogger(ret),
	)
	return ret
}

type Service interface {
	Start() error
	Stop()

	AddEntry(Entry) error
}

type svr struct {
	Service
	log     logging.Log
	confDir string
	crons   *cron.Cron
}

func (s *svr) Start() error {
	s.iterateConfigs(func(define *Define) bool {
		if err := Register(define); err == nil {
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
		def.Id = strings.TrimSuffix(entry.Name(), ".json")
		flag := cb(def)
		if !flag {
			break
		}
	}
	return nil
}

func (s *svr) loadConfig(id string) (*Define, error) {
	path := filepath.Join(s.confDir, fmt.Sprintf("%s.json", id))
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
	def.Id = id
	return def, nil
}

func (s *svr) saveConfig(def *Define) error {
	buf, err := json.MarshalIndent(def, "", "\t")
	if err != nil {
		s.log.Warnf("bridge def file", err.Error())
		return err
	}

	path := filepath.Join(s.confDir, fmt.Sprintf("%s.json", def.Id))
	return os.WriteFile(path, buf, 00600)
}

func (s *svr) removeConfig(id string) error {
	path := filepath.Join(s.confDir, fmt.Sprintf("%s.json", id))
	return os.Remove(path)
}

func (s *svr) AddEntry(entry Entry) error {
	if entry.AutoStart() {
		if err := entry.Start(s); err != nil {
			return err
		}
	}
	return nil
}

// implements cron.Log
func (s *svr) Info(msg string, keysAndValues ...any) {
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
		s.log.Trace(msg)
	} else {
		s.log.Tracef("%s entry[%d] next=%s %s", msg, entryId, next, strings.Join(extra, ","))
	}
}

func (s *svr) Error(err error, msg string, keysAndValues ...any) {
	s.log.Error(append([]any{err.Error(), msg}, keysAndValues...)...)
}
