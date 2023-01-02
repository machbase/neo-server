package shell

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/c-bata/go-prompt"
	"github.com/gliderlabs/ssh"
	"github.com/machbase/cemlib/logging"
	"github.com/machbase/cemlib/ssh/sshd"
	mach "github.com/machbase/dbms-mach-go"
	"github.com/pkg/errors"
)

type Server interface {
	GetConfig() string
}

type Config struct {
	Listeners     []string
	IdleTimeout   time.Duration
	ServerKeyPath string
}

type MachShell struct {
	conf  *Config
	log   logging.Log
	sshds []sshd.Server

	Server Server // injection point
}

func New(conf *Config) *MachShell {
	return &MachShell{
		conf: conf,
	}
}

func (svr *MachShell) Start() error {
	svr.log = logging.GetLog("machshell")
	svr.sshds = make([]sshd.Server, 0)

	for _, listen := range svr.conf.Listeners {
		listenAddress := strings.TrimPrefix(listen, "tcp://")
		cfg := sshd.Config{
			ListenAddress:      listenAddress,
			ServerKey:          svr.conf.ServerKeyPath,
			IdleTimeout:        svr.conf.IdleTimeout,
			AutoListenAndServe: false,
		}
		s := sshd.New(&cfg)
		err := s.Start()
		if err != nil {
			return errors.Wrap(err, "machsell")
		}
		s.SetHandler(svr.sessionHandler)
		s.SetShellProvider(svr.shellProvider)
		s.SetMotdProvider(svr.motdProvider)
		s.SetPasswordHandler(svr.passwordProvider)
		go func() {
			err := s.ListenAndServe()
			if err != nil {
				svr.log.Warnf("machshell-listen %s", err.Error())
			}
		}()
		svr.log.Infof("SSHD Listen %s", listen)
	}
	return nil
}

func (svr *MachShell) Stop() {
	for _, s := range svr.sshds {
		s.Stop()
	}
}

func (svr *MachShell) shellProvider(user string) *sshd.Shell {
	return &sshd.Shell{
		Cmd: "/bin/bash",
	}
}

func (svr *MachShell) motdProvider(user string) string {
	return fmt.Sprintf("Greeting, %s\r\nmachsvr %v\r\n", user, mach.VersionString())
}

func (svr *MachShell) passwordProvider(ctx ssh.Context, password string) bool {
	return true
}

func (svr *MachShell) sessionHandler(ss ssh.Session) {
	svr.log.Infof("shell login %s", ss.User())
	sess := Session{
		ss:     ss,
		log:    logging.GetLog(fmt.Sprintf("machsql-%s", ss.User())),
		db:     mach.New(),
		server: svr.Server,
	}

	if cmds := ss.Command(); len(cmds) > 0 {
		sess.Printf("Commands: %+v\r\n", cmds)
		return
	}
	sess.quitCh = make(chan bool, 1)

	pty, ptyCh, ok := ss.Pty()
	if !ok {
		sess.Println("ERR unable to get PTY")
		return
	}
	sess.window = pty.Window

	wg := sync.WaitGroup{}

	wg.Add(1)
	go func() {
		defer wg.Done()
		sess.ListenWindow(ptyCh)
		sess.Close()
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		sess.Run()
		sess.Close()
	}()

	wg.Wait()
	svr.log.Infof("shell exit %s", ss.User())
}

type Session struct {
	ss     ssh.Session
	window ssh.Window
	closed bool
	log    logging.Log
	quitCh chan bool

	db     *mach.Database
	server Server

	LivePrefix string
	IsEnable   bool

	PosixWriter
}

func (sess *Session) Close() {
	if sess.closed {
		return
	}
	sess.closed = true
	sess.quitCh <- true
}

func (sess *Session) ListenWindow(ptyCh <-chan ssh.Window) {
	defer sess.log.Trace("finish listener")
	for !sess.closed {
		select {
		case <-sess.quitCh:
			sess.log.Trace("Quit.")
			sess.Close()
			return
		case <-sess.ss.Context().Done():
			sess.log.Trace("Closed.")
			sess.Close()
			return
		case w := <-ptyCh:
			sess.window = w
			sess.log.Tracef("WIN %+v", sess.window)
		}
	}
}

func (sess *Session) Run() {
	defer sess.log.Trace("finish runner")
	p := prompt.New(
		sess.executor,
		sess.completer,
		prompt.OptionParser(prompt.ConsoleParser(sess)),
		prompt.OptionWriter(prompt.ConsoleWriter(sess)),
		prompt.OptionPrefix("machsql> "),
		prompt.OptionLivePrefix(sess.changeLivePrefix),
		prompt.OptionTitle("MACHSQL"),
		prompt.OptionPrefixTextColor(prompt.Yellow),
		prompt.OptionPreviewSuggestionTextColor(prompt.DarkGray),
		prompt.OptionSelectedSuggestionTextColor(prompt.White),
		prompt.OptionSelectedSuggestionBGColor(prompt.Blue),
		prompt.OptionSuggestionBGColor(prompt.LightGray),
		prompt.OptionSuggestionTextColor(prompt.DarkGray),
	)

	p.Run()
}

func (sess *Session) Printf(format string, args ...any) {
	str := fmt.Sprintf(format, args...)
	sess.WriteStr(str + "\r\n")
}

func (sess *Session) Println(strs ...string) {
	str := strings.Join(strs, " ")
	sess.WriteStr(str + "\r\n")
}

// //////////////
// prompt handler

func (sess *Session) changeLivePrefix() (string, bool) {
	return sess.LivePrefix, sess.IsEnable
}

// implements prompt.ConsoleParser
func (sess *Session) Setup() error {
	return nil
}

func (sess *Session) TearDown() error {
	return nil
}

func (sess *Session) GetWinSize() *prompt.WinSize {
	return &prompt.WinSize{Row: uint16(sess.window.Height), Col: uint16(sess.window.Width)}
}

func (sess *Session) Read() ([]byte, error) {
	if sess.closed {
		return []byte{byte(prompt.ControlD)}, nil
	}
	buff := make([]byte, 1024)
	n, err := sess.ss.Read(buff)
	if err != nil {
		return []byte{}, err
	}
	return buff[:n], nil
}

// / implements prompt.ConsoleWriter
func (sess *Session) Flush() error {
	sess.ss.Write(sess.PosixWriter.buffer)
	sess.PosixWriter.buffer = []byte{}
	return nil
}
