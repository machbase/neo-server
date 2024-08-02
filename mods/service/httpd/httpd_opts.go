package httpd

import (
	"strings"

	"github.com/machbase/neo-client/machrpc"
	"github.com/machbase/neo-server/api/bridge"
	"github.com/machbase/neo-server/api/mgmt"
	"github.com/machbase/neo-server/api/schedule"
	"github.com/machbase/neo-server/mods/model"
	"github.com/machbase/neo-server/mods/pkgs"
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
		s.enableTokenAuth = enabled
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

func OptionEnableWeb(enable bool) Option {
	return func(s *httpd) {
		s.disableWeb = !enable
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

func OptionWebDir(path string) Option {
	return func(s *httpd) {
		s.uiContentFs = WrapAssets(path)
	}
}

// experiement features
func OptionExperimentModeProvider(provider func() bool) Option {
	return func(s *httpd) {
		s.experimentModeProvider = provider
	}
}

func OptionWebShellProvider(provider model.ShellProvider) Option {
	return func(s *httpd) {
		s.webShellProvider = provider
	}
}

func OptionStatzAllow(remotes ...string) Option {
	return func(s *httpd) {
		s.statzAllowed = append(s.statzAllowed, remotes...)
	}
}

func OptionServerInfoFunc(fn func() (*machrpc.ServerInfo, error)) Option {
	return func(s *httpd) {
		s.serverInfoFunc = fn
	}
}

func OptionServerSessionsFunc(fn func(statz, session bool) (*machrpc.Statz, []*machrpc.Session, error)) Option {
	return func(s *httpd) {
		s.serverSessionsFunc = fn
	}
}

func OptionManagementServer(handler mgmt.ManagementServer) Option {
	return func(s *httpd) {
		s.mgmtImpl = handler
	}
}

func OptionScheduleServer(handler schedule.ManagementServer) Option {
	return func(s *httpd) {
		s.schedMgmtImpl = handler
	}
}

func OptionBridgeServer(handler any) Option {
	return func(s *httpd) {
		if o, ok := handler.(bridge.ManagementServer); ok {
			s.bridgeMgmtImpl = o
		}
		if o, ok := handler.(bridge.RuntimeServer); ok {
			s.bridgeRuntimeImpl = o
		}
	}
}

func OptionPackageManager(pm *pkgs.PkgManager) Option {
	return func(s *httpd) {
		s.pkgMgr = pm
	}
}
