package httpsvr

import (
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/machbase/cemlib/logging"
	mach "github.com/machbase/neo-engine"
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

	logvaultAppender *mach.Appender
}

func (svr *Server) Start() error {
	return nil
}

func (svr *Server) Stop() {
	if svr.logvaultAppender != nil {
		svr.logvaultAppender.Close()
	}
}

func (svr *Server) Route(r *gin.Engine) {
	checkLogTableOnce := sync.Once{}
	for _, h := range svr.conf.Handlers {
		prefix := h.Prefix
		// remove trailing slash
		for strings.HasSuffix(prefix, "/") {
			prefix = prefix[0 : len(prefix)-1]
		}

		svr.log.Infof("Add handler %-10s '%s'", h.Handler, prefix)

		switch h.Handler {
		case "influx": // "influx line protocol"
			r.POST(prefix+"/:oper", svr.handleLineProtocol)
		case "logvault":
			r.POST(prefix+"/:oper", svr.handleLogVault)
			checkLogTableOnce.Do(svr.checkLogTable)
		default: // "machbase"
			r.GET(prefix+"/query", svr.handleQuery)
			r.POST(prefix+"/query", svr.handleQuery)
			r.POST(prefix+"/write", svr.handleWrite)
			r.POST(prefix+"/write/:table", svr.handleWrite)
		}
	}
}
