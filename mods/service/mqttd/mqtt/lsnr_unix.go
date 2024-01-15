package mqtt

import (
	"fmt"
	"io/fs"
	"net"
	"os"
	"strconv"
	"sync"

	"github.com/machbase/neo-server/mods/logging"
	"github.com/machbase/neo-server/mods/service/allowance"
)

type unixSocketListener struct {
	raw        net.Listener
	path       string
	permission int
	acceptChan chan<- any
	allowance  allowance.Allowance
	log        logging.Log
	alive      bool
	name       string
	closeWait  sync.WaitGroup
}

func newUnixSocketListener(cfg *UnixSocketListenerConfig, acceptChan chan<- any) (*unixSocketListener, error) {
	lsnr := &unixSocketListener{
		path:       cfg.Path,
		permission: cfg.Permission,
		acceptChan: acceptChan,
		alive:      false,
		name:       "mqtt-unix",
		closeWait:  sync.WaitGroup{},
	}
	lsnr.log = logging.GetLog(lsnr.name)
	if err := lsnr.buildRawListener(); err != nil {
		return nil, err
	}
	return lsnr, nil
}

func (l *unixSocketListener) buildRawListener() error {
	var err error
	if err = os.RemoveAll(l.path); err != nil {
		return err
	}
	if l.raw, err = net.Listen("unix", l.path); err != nil {
		return err
	}

	// convert dec base10 to base8
	perm, _ := strconv.ParseInt(fmt.Sprintf("0%d", l.permission), 8, 32)
	l.permission = int(perm)

	if err = os.Chmod(l.path, fs.FileMode(l.permission)); err != nil {
		return err
	}
	l.path = l.raw.Addr().String()
	return err
}

func (l *unixSocketListener) Address() string {
	return l.path
}

func (l *unixSocketListener) Start() error {
	if l.alive {
		return nil
	}
	if l.raw == nil {
		if err := l.buildRawListener(); err != nil {
			return err
		}
	}
	l.alive = true
	l.closeWait.Add(1)
	go l.runListener()
	return nil
}

func (l *unixSocketListener) Stop() error {
	if !l.alive {
		return nil
	}
	l.alive = false
	if l.raw != nil {
		if err := l.raw.Close(); err != nil {
			return err
		}
		l.raw = nil
	}
	os.RemoveAll(l.path)
	l.closeWait.Wait()
	return nil
}

func (l *unixSocketListener) Name() string {
	return l.name
}
func (l *unixSocketListener) IsAlive() bool {
	return l.alive
}
func (l *unixSocketListener) SetAllowance(allowance allowance.Allowance) {
	l.allowance = allowance
}

func (l *unixSocketListener) runListener() {
	listenAddr := l.raw.Addr()
	l.log.Infof("MQTT Listen unix://%s 0%o", listenAddr, l.permission)
	defer func() {
		l.log.Tracef("Stop listen %s", listenAddr)
		l.closeWait.Done()
	}()

	for {
		conn, err := l.raw.Accept()
		if err != nil {
			if !l.alive {
				return
			}
			if ne, ok := err.(net.Error); ok {
				if ne.Temporary() {
					l.log.Warnf("accept temporary failed: %s", err)
					continue
				}
			}
			l.log.Errorf("socket failed: %s", err)
			return
		}
		l.acceptChan <- NewTcpConnection(conn)
	}
}
