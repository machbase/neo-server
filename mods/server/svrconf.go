package server

import (
	_ "embed"
)

//go:embed svrconf.hcl
var DefaultFallbackConfig []byte

var DefaultFallbackPname string = "neo"

func (s *svr) GetConfig() string {
	return string(DefaultFallbackConfig)
}
