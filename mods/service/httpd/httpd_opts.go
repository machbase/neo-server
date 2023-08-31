package httpd

import (
	"strings"

	"github.com/machbase/neo-server/mods/model"
	"github.com/machbase/neo-server/mods/service/security"
	"github.com/machbase/neo-server/mods/tql"
	"github.com/machbase/neo-server/mods/util/ssfs"
)

type Option func(s *httpd)

// ListenAddresses
func OptionListenAddress(addrs ...string) Option {
	return func(s *httpd) {
		s.listenAddresses = append(s.listenAddresses, addrs...)
	}
}

// AuthServer
func OptionAuthServer(authSvc security.AuthServer, enabled bool) Option {
	return func(s *httpd) {
		s.authServer = authSvc
		s.enableTokenAUth = enabled
		if enabled {
			s.log.Infof("HTTP token authentication enabled")
		} else {
			s.log.Infof("HTTP token authentication disabled")
		}
	}
}

// neo-shell address
func OptionNeoShellAddress(addrs ...string) Option {
	return func(s *httpd) {
		candidates := []string{}
		for _, addr := range addrs {
			if strings.HasPrefix(addr, "tcp://127.0.0.1:") || strings.HasPrefix(addr, "tcp://localhost:") {
				s.neoShellAddress = strings.TrimPrefix(addr, "tcp://")
				// if loopback is available, use it for web-terminal
				// eliminate other candiates
				candidates = candidates[:0]
				break
			} else if strings.HasPrefix(addr, "tcp://") {
				candidates = append(candidates, strings.TrimPrefix(addr, "tcp://"))
			}
		}
		if len(candidates) > 0 {
			// TODO choose one from the candidates, !EXCLUDE! virtual/tunnel ethernet addresses
			s.neoShellAddress = candidates[0]
		}
	}
}

// license file path
func OptionLicenseFilePath(path string) Option {
	return func(s *httpd) {
		s.licenseFilePath = path
	}
}

// Handler
func OptionHandler(prefix string, handler HandlerType) Option {
	return func(s *httpd) {
		s.handlers = append(s.handlers, &HandlerConfig{Prefix: prefix, Handler: handler})
	}
}

func OptionTqlLoader(loader tql.Loader) Option {
	return func(s *httpd) {
		s.tqlLoader = loader
	}
}

func OptionServerSideFileSystem(ssfs *ssfs.SSFS) Option {
	return func(s *httpd) {
		s.serverFs = ssfs
	}
}

func OptionDebugMode(isDebug bool) Option {
	return func(s *httpd) {
		s.debugMode = isDebug
	}
}

// experiement features
func OptionExperimentModeProvider(provider func() bool) Option {
	return func(s *httpd) {
		s.experimentModeProvider = provider
	}
}

func OptionReferenceProvider(provider func() []WebReferenceGroup) Option {
	return func(s *httpd) {
		s.referenceProvider = provider
	}
}

func OptionWebShellProvider(provider model.ShellProvider) Option {
	return func(s *httpd) {
		s.webShellProvider = provider
	}
}
