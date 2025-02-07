package server

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/machbase/neo-server/v8/api/bridge"
	"github.com/machbase/neo-server/v8/api/mgmt"
	"github.com/machbase/neo-server/v8/api/schedule"
	"github.com/machbase/neo-server/v8/mods/model"
	"github.com/machbase/neo-server/v8/mods/pkgs"
	"github.com/machbase/neo-server/v8/mods/tql"
	"github.com/machbase/neo-server/v8/mods/util/ssfs"
)

type HttpOption func(s *httpd)

// ListenAddresses
func WithHttpListenAddress(addrs ...string) HttpOption {
	return func(s *httpd) {
		s.listenAddresses = append(s.listenAddresses, addrs...)
	}
}

// AuthServer
func WithHttpAuthServer(authSvc AuthServer, enabled bool) HttpOption {
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
func WithHttpNeoShellAddress(addrs ...string) HttpOption {
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
func WithHttpLicenseFilePath(path string) HttpOption {
	return func(s *httpd) {
		s.licenseFilePath = path
	}
}

// End User License Agreement (EULA) file path
func WithHttpEulaFilePath(path string) HttpOption {
	return func(s *httpd) {
		s.eulaFilePath = path
	}
}

func WithHttpEnableWeb(enable bool) HttpOption {
	return func(s *httpd) {
		s.disableWeb = !enable
	}
}

func WithHttpTqlLoader(loader tql.Loader) HttpOption {
	return func(s *httpd) {
		s.tqlLoader = loader
	}
}

func WithHttpServerSideFileSystem(ssfs *ssfs.SSFS) HttpOption {
	return func(s *httpd) {
		s.serverFs = ssfs
	}
}

func WithHttpDebugMode(isDebug bool, filterLatency string) HttpOption {
	return func(s *httpd) {
		s.debugMode = isDebug
		if filterLatency != "" {
			s.debugLogFilterLatency, _ = time.ParseDuration(filterLatency)
		}
	}
}

func WithHttpReadBufSize(size int) HttpOption {
	return func(s *httpd) {
		s.readBufSize = size
	}
}

func WithHttpWriteBufSize(size int) HttpOption {
	return func(s *httpd) {
		s.writeBufSize = size
	}
}

func WithHttpWebDir(path string) HttpOption {
	return func(s *httpd) {
		s.uiContentFs = WrapAssets(path)
	}
}

// experiement features
func WithHttpExperimentModeProvider(provider func() bool) HttpOption {
	return func(s *httpd) {
		s.experimentModeProvider = provider
	}
}

func WithHttpWebShellProvider(provider model.ShellProvider) HttpOption {
	return func(s *httpd) {
		s.webShellProvider = provider
	}
}

func WithHttpStatzAllow(remotes ...string) HttpOption {
	return func(s *httpd) {
		s.statzAllowed = append(s.statzAllowed, remotes...)
	}
}

func WithHttpServerInfoFunc(fn func() (*mgmt.ServerInfoResponse, error)) HttpOption {
	return func(s *httpd) {
		s.serverInfoFunc = fn
	}
}

func WithHttpMqttInfoFunc(fn func() map[string]any) HttpOption {
	return func(s *httpd) {
		s.mqttInfoFunc = fn
	}
}

func WithHttpMqttWsHandlerFunc(fn http.HandlerFunc) HttpOption {
	return func(s *httpd) {
		s.mqttWsHandler = gin.WrapF(fn)
	}
}

func WithHttpServerSessionsFunc(fn func(statz, session bool) (*mgmt.Statz, []*mgmt.Session, error)) HttpOption {
	return func(s *httpd) {
		s.serverSessionsFunc = fn
	}
}

func WithHttpManagementServer(handler mgmt.ManagementServer) HttpOption {
	return func(s *httpd) {
		s.mgmtImpl = handler
	}
}

func WithHttpScheduleServer(handler schedule.ManagementServer) HttpOption {
	return func(s *httpd) {
		s.schedMgmtImpl = handler
	}
}

func WithHttpBridgeServer(handler any) HttpOption {
	return func(s *httpd) {
		if o, ok := handler.(bridge.ManagementServer); ok {
			s.bridgeMgmtImpl = o
		}
		if o, ok := handler.(bridge.RuntimeServer); ok {
			s.bridgeRuntimeImpl = o
		}
	}
}

func WithHttpBackupService(handler *backupd) HttpOption {
	return func(s *httpd) {
		s.bakd = handler
	}
}

func WithHttpPackageManager(pm *pkgs.PkgManager) HttpOption {
	return func(s *httpd) {
		s.pkgMgr = pm
	}
}

func WithHttpPathMap(name string, realPath string) HttpOption {
	return func(s *httpd) {
		s.pathMap[name] = realPath
	}
}
