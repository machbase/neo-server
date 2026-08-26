package model

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/machbase/neo-server/v8/mods/logging"
)

type ServicePort struct {
	Service string
	Address string
}

func NewProvider(opts ...Option) *Provider {
	ret := &Provider{
		log: logging.GetLog("scheduler"),
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
