package plugins

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"text/template"

	"github.com/machbase/cemlib/logging"
	"github.com/machbase/neo-server/plugins/bridge"
	"github.com/machbase/neo-server/plugins/instancemgr"
)

type Manager interface {
	Start() error
	Stop()
}

type manager struct {
	log         logging.Log
	basedir     string
	serverAddrs []string

	instanceProviders []instancemgr.InstanceProvider
}

func New(basedir string, serverAddrs []string) Manager {
	basedir = filepath.Clean(basedir) + string(filepath.Separator)
	mgr := &manager{
		basedir:           basedir,
		serverAddrs:       serverAddrs,
		instanceProviders: []instancemgr.InstanceProvider{},
	}
	return mgr
}

func (m *manager) Start() error {
	m.log = logging.GetLog("plugins")
	metaList := m.scan()
	for _, pjson := range metaList {
		m.log.Infof("Plugin %+v", pjson)
		ip := NewInstanceProvider(pjson.NewInstance)
		m.instanceProviders = append(m.instanceProviders, ip)
	}
	return nil
}

func (m *manager) Stop() {

}

type PluginJSON struct {
	Id   string              `json:"ID"`
	Grpc bridge.GrpcSettings `json:"gRPC"`
	Bin  string              `json:"bin"`
	Args []string            `json:"args"`

	GrpcServerAddr string `json:"-"`
	DirPath        string `json:"-"`
}

func (pjson *PluginJSON) NewInstance(settings PluginInstanceSettings) (bridge.PluginInstance, error) {
	return nil, nil
}

type PredefVars struct {
	OS         string
	ARCH       string
	ServerAddr string
}

func (m *manager) scan() []*PluginJSON {
	// choose suitable server addr
	serverAddr := ""
	for i, addr := range m.serverAddrs {
		if i == 0 {
			serverAddr = addr
			continue
		}
		// prefer unix domain socket over tcp
		if strings.HasPrefix(addr, "unix://") && !strings.HasPrefix(serverAddr, "unix://") {
			serverAddr = addr
		}
	}
	vars := &PredefVars{
		OS:         runtime.GOOS,
		ARCH:       runtime.GOARCH,
		ServerAddr: serverAddr,
	}
	list := []*PluginJSON{}
	filepath.WalkDir(m.basedir, func(path string, d os.DirEntry, err error) error {
		if filepath.Base(path) == "plugin.json" {
			content, err := os.ReadFile(path)
			if err != nil {
				path = strings.TrimPrefix(path, m.basedir)
				m.log.Debug("ignore", path, err.Error())
				return nil
			}
			tmpl, err := template.New("plugin.json").Parse(string(content))
			if err != nil {
				path = strings.TrimPrefix(path, m.basedir)
				m.log.Debug("parse", path, err.Error())
				return nil
			}
			buff := bytes.Buffer{}
			err = tmpl.Execute(&buff, vars)
			if err != nil {
				path = strings.TrimPrefix(path, m.basedir)
				m.log.Debug("load", path, err.Error())
				return nil
			}

			pj := PluginJSON{}
			pj.DirPath = filepath.Dir(path)
			path = strings.TrimPrefix(path, m.basedir)
			err = json.Unmarshal(buff.Bytes(), &pj)
			if err != nil {
				m.log.Debug("unmarshal", path, err.Error())
				return nil
			}
			list = append(list, &pj)
		}
		return nil
	})
	return list
}
