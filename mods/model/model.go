package model

import (
	"context"
	"database/sql"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sync"

	"github.com/machbase/neo-server/v8/mods/logging"
	"github.com/machbase/neo-server/v8/spi"
)

type UserScope struct {
	User string
}

type connectFunc func(context.Context, string) (*sql.Conn, error)

func NewProvider(opts ...Option) *Provider {
	ret := &Provider{
		log:     logging.GetLog("model"),
		connect: spi.Connect,
	}
	for _, o := range opts {
		o(ret)
	}
	return ret
}

type Option func(*Provider)

type Provider struct {
	log       logging.Log
	configDir string

	schedDir  string
	bridgeDir string
	shellDir  string
	connect   connectFunc

	shellTableMu    sync.Mutex
	shellTableReady bool

	bridgeTableMu    sync.Mutex
	bridgeTableReady bool

	experimentMode func() bool
}

func WithConfigDirPath(path string) Option {
	return func(s *Provider) {
		s.configDir = path
	}
}

func WithExperimentModeProvider(provider func() bool) Option {
	return func(s *Provider) {
		s.experimentMode = provider
	}
}

func (s *Provider) Start() error {
	s.bridgeDir = filepath.Join(s.configDir, "bridges")
	if err := s.mkDirIfNotExists(s.bridgeDir, 0755); err != nil {
		return fmt.Errorf("bridge defs, %s", err.Error())
	}
	s.schedDir = filepath.Join(s.configDir, "schedules")
	if err := s.mkDirIfNotExists(s.schedDir, 0755); err != nil {
		return fmt.Errorf("schedule defs, %s", err.Error())
	}
	s.shellDir = filepath.Join(s.configDir, "shell")
	if err := s.mkDirIfNotExists(s.shellDir, 0700); err != nil {
		return fmt.Errorf("shell defs, %s", err.Error())
	}
	return nil
}

func (s *Provider) Stop() {
}

func (s *Provider) mkDirIfNotExists(path string, mode fs.FileMode) error {
	_, err := os.Stat(path)
	if err != nil && os.IsNotExist(err) {
		if err := os.Mkdir(path, mode); err != nil {
			return err
		}
	} else if err != nil {
		return err
	}
	return nil
}
