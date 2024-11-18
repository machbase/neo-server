package sshd

import (
	"encoding/hex"
	"io"
	"strings"

	"github.com/machbase/neo-server/v8/mods/logging"
)

func NewIODebugger(log logging.Log, prefix string) io.Writer {
	return &inoutWriter{
		prefix: prefix,
		log:    log,
	}
}

type inoutWriter struct {
	prefix string
	log    logging.Log
}

func (iow *inoutWriter) Write(b []byte) (int, error) {
	iow.log.Infof("%s %s", iow.prefix, strings.TrimSpace(hex.Dump(b)))
	return len(b), nil
}
