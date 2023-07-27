package bridge

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	bridgerpc "github.com/machbase/neo-grpc/bridge"
	"github.com/machbase/neo-server/mods/logging"
	cmap "github.com/orcaman/concurrent-map"
)

func NewService(defDir string) Service {
	s := &svr{
		log:     logging.GetLog("bridge"),
		confDir: defDir,
		ctxMap:  cmap.New(),
	}
	return s
}

type Service interface {
	bridgerpc.ManagementServer
	bridgerpc.RuntimeServer

	Start() error
	Stop()
}

type svr struct {
	Service

	log     logging.Log
	confDir string
	ctxMap  cmap.ConcurrentMap
}

func (s *svr) Start() error {
	s.iterateConfigs(func(define *Define) bool {
		if err := Register(define); err == nil {
			s.log.Infof("add bridge %s type=%s", define.Name, define.Type)
		} else {
			s.log.Errorf("fail to add bridge %s type=%s, %s", define.Name, define.Type, err.Error())
		}
		return true
	})
	return nil
}

func (s *svr) Stop() {
	UnregisterAll()
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
			s.log.Warnf("bridge def file", err.Error())
			continue
		}
		def := &Define{}
		if err = json.Unmarshal(content, def); err != nil {
			s.log.Warnf("bridge def format", err.Error())
			continue
		}
		flag := cb(def)
		if !flag {
			break
		}
	}
	return nil
}

func (s *svr) loadConfig(name string) (*Define, error) {
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
	return def, nil
}

func (s *svr) saveConfig(def *Define) error {
	buf, err := json.MarshalIndent(def, "", "\t")
	if err != nil {
		s.log.Warnf("bridge def file", err.Error())
		return err
	}

	path := filepath.Join(s.confDir, fmt.Sprintf("%s.json", def.Name))
	return os.WriteFile(path, buf, 00600)
}

func (s *svr) removeConfig(name string) error {
	path := filepath.Join(s.confDir, fmt.Sprintf("%s.json", name))
	return os.Remove(path)
}
