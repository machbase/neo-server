package server

import (
	"bufio"
	"bytes"
	_ "embed"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/pkg/errors"
)

//go:embed svrconf.hcl
var DefaultFallbackConfig []byte

var DefaultFallbackPname string = "neo"

func (s *svr) GetConfig() string {
	return string(DefaultFallbackConfig)
}

func (s *svr) checkRewriteMachbaseConf(confpath string) (bool, error) {
	shouldRewrite := false
	content, err := os.ReadFile(confpath)
	if err != nil {
		return false, errors.Wrap(err, "MACH machbase.conf not available")
	}
	reader := bufio.NewReader(bytes.NewBuffer(content))
	parts := []string{}
	for !shouldRewrite {
		str, isPrefix, err := reader.ReadLine()
		if err != nil {
			if err == io.EOF {
				break
			} else {
				return false, errors.Wrap(err, "MACH machbase.conf malformed")
			}
		}
		parts = append(parts, string(str))
		if isPrefix {
			continue
		}
		line := strings.TrimSpace(strings.Join(parts, ""))
		parts = parts[0:0]
		if strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if key == "PORT_NO" && strconv.FormatInt(int64(s.conf.Machbase.PORT_NO), 10) != value {
			s.log.Infof("MACH PORT_NO will be %d, previously %s", s.conf.Machbase.PORT_NO, value)
			shouldRewrite = true
		} else if key == "BIND_IP_ADDRESS" && s.conf.Machbase.BIND_IP_ADDRESS != value {
			s.log.Infof("MACH BIND_IP_ADDRESS will be %s, previously %s", s.conf.Machbase.BIND_IP_ADDRESS, value)
			shouldRewrite = true
		}
	}
	return shouldRewrite, nil
}

func (s *svr) rewriteMachbaseConf(confpath string) error {
	content, err := os.ReadFile(confpath)
	if err != nil {
		return errors.Wrap(err, "MACH machbase.conf not available")
	}
	reader := bufio.NewReader(bytes.NewBuffer(content))
	newConfLines := []string{}
	parts := []string{}
	for {
		str, isPrefix, err := reader.ReadLine()
		if err != nil {
			if err == io.EOF {
				break
			} else {
				return errors.Wrap(err, "MACH machbase.conf malformed")
			}
		}
		parts = append(parts, string(str))
		if isPrefix {
			continue
		}
		line := strings.TrimSpace(strings.Join(parts, ""))
		parts = parts[0:0]
		if strings.HasPrefix(line, "#") {
			newConfLines = append(newConfLines, line)
			continue
		}
		key, _, ok := strings.Cut(line, "=")
		if !ok {
			newConfLines = append(newConfLines, line)
			continue
		}
		key = strings.TrimSpace(key)
		if key == "PORT_NO" {
			newConfLines = append(newConfLines, fmt.Sprintf("PORT_NO = %d", s.conf.Machbase.PORT_NO))
		} else if key == "BIND_IP_ADDRESS" {
			newConfLines = append(newConfLines, fmt.Sprintf("BIND_IP_ADDRESS = %s", s.conf.Machbase.BIND_IP_ADDRESS))
		} else {
			newConfLines = append(newConfLines, line)
		}
	}
	fd, err := os.OpenFile(confpath, os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return errors.Wrap(err, "MACH machbase.conf unable to write")
	}
	if _, err = fd.Write([]byte(strings.Join(newConfLines, "\n"))); err != nil {
		return errors.Wrap(err, "MACH machbase.conf write error")
	}
	if err = fd.Close(); err != nil {
		return errors.Wrap(err, "MACH machbase.conf close error")
	}
	return nil
}
