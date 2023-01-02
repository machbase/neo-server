package server

import (
	_ "embed"
)

//go:embed svrconf.hcl
var DefaultFallbackConfig []byte

var DefaultFallbackPname string = "machsvr"

// import hcl "github.com/hashicorp/hcl/v2"
func (s *svr) GetConfig() string {
	//hcl.Mar https://github.com/alecthomas/hcl
	// hcl.Marshal(c)
	// return s.conf.String()
	return string(DefaultFallbackConfig)
}
