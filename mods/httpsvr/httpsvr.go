package httpsvr

import (
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/machbase/cemlib/logging"
	mach "github.com/machbase/dbms-mach-go"
)

func New(conf *Config) (*Server, error) {
	return &Server{
		conf: conf,
		log:  logging.GetLog("httpsvr"),
		db:   mach.New(),
	}, nil
}

type Config struct {
	Handlers []HandlerConfig
}

type HandlerConfig struct {
	Prefix  string
	Handler string
}

type Server struct {
	conf *Config
	log  logging.Log
	db   *mach.Database
}

func (svr *Server) Start() error {
	return nil
}

func (svr *Server) Stop() {
}

func (svr *Server) Route(r *gin.Engine) {
	for _, h := range svr.conf.Handlers {
		prefix := h.Prefix
		// remove trailing slash
		for strings.HasSuffix(prefix, "/") {
			prefix = prefix[0 : len(prefix)-1]
		}

		svr.log.Infof("Add handler '%s' -> %s", prefix, h.Handler)

		switch h.Handler {
		case "influx": // "influx line protocol"
			r.POST(prefix+"/:oper", svr.handleLineProtocol)
		default: // "machbase"
			r.GET(prefix+"/query", svr.handleQuery)
			r.POST(prefix+"/query", svr.handleQuery)
			r.POST(prefix+"/write", svr.handleWrite)
			r.POST(prefix+"/write/:table", svr.handleWrite)
		}
	}
}
