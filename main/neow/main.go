package main

import (
	"bufio"
	"context"
	_ "embed"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/getlantern/systray"
	"github.com/machbase/neo-server/main/neow/icon"
	"github.com/machbase/neo-server/mods/util"
	"github.com/machbase/neo-server/mods/util/ini"
	"github.com/plgd-dev/go-coap/v3/net"
	"github.com/robert-nix/ansihtml"
)

func main() {
	neoExePath, err := getMachbaseNeoPath()
	if err != nil {
		panic(err)
	}

	neoExeArgs := []string{"serve"}
	autoStart := false

	iniPath, err := getStartupIni()
	if err == nil {
		cfg := ini.Load(iniPath)
		sect, err := cfg.Section(cfg.DefaultSectionName())
		if err == nil {
			valueString := sect.GetValueWithDefault("args", "")
			values := util.SplitFields(valueString, true)
			neoExeArgs = append(neoExeArgs, values...)

			autoStart = sect.GetBoolWithDefault("auto-start", false)
		}
	}

	na := &neoAgent{
		exePath:     neoExePath,
		exeArgs:     neoExeArgs,
		autoStart:   autoStart,
		stateC:      make(chan NeoState, 1),
		outputLimit: 500,
	}
	na.Start()
}

func getMachbaseNeoPath() (string, error) {
	selfPath, _ := os.Executable()
	selfDir := filepath.Dir(selfPath)
	neoExePath := ""
	if runtime.GOOS == "windows" {
		neoExePath = filepath.Join(selfDir, "machbase-neo.exe")
	} else {
		neoExePath = filepath.Join(selfDir, "machbase-neo")
	}

	if _, err := os.Stat(neoExePath); err != nil {
		return "", err
	}

	return neoExePath, nil
}

func getStartupIni() (string, error) {
	selfPath := os.Args[0]
	selfDir := filepath.Dir(selfPath)
	iniPath := filepath.Join(selfDir, "neow.ini")
	if _, err := os.Stat(iniPath); err != nil {
		return "", err
	}
	return iniPath, nil
}

type neoAgent struct {
	exePath   string
	exeArgs   []string
	autoStart bool

	stateC  chan NeoState
	process *os.Process

	outputs     []LogLine
	outputLock  sync.Mutex
	outputLimit int

	httpAddr string
	httpSvr  *http.Server

	mStart *systray.MenuItem
	mStop  *systray.MenuItem
	mLog   *systray.MenuItem
	mQuit  *systray.MenuItem
}

type NeoState string

const (
	NeoStarting NeoState = "starting"
	NeoRunning  NeoState = "running"
	NeoStopping NeoState = "stopping"
	NeoStopped  NeoState = "not running"
)

func (na *neoAgent) Start() {
	lsnr, _ := net.NewTCPListener("tcp", "127.0.0.1:0")
	na.httpAddr = lsnr.Addr().String()
	na.httpSvr = &http.Server{Handler: na}
	go na.httpSvr.Serve(lsnr)

	// only effective on Windows
	winMain(na)

	// wait until 'systray.Quit` called
	systray.Run(na.onReady, na.onExit)
}

func (na *neoAgent) Stop() {
	na.doStop()
	if na.httpSvr != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		na.httpSvr.Shutdown(ctx)
	}
	systray.Quit()
}

func (na *neoAgent) onReady() {
	systray.SetIcon(icon.Logo)
	na.mStart = systray.AddMenuItem("Start", "Start machbase-neo")
	na.mStop = systray.AddMenuItem("Stop", "Stop machbase-neo")
	na.mStop.Disable()
	systray.AddSeparator()

	na.mLog = systray.AddMenuItem("Show logs", "Show logs")
	systray.AddSeparator()

	na.mQuit = systray.AddMenuItem("Quit", "Quit neow")
	na.mQuit.SetIcon(icon.Shutdown)

	go na.run()

	if na.autoStart {
		na.doStart()
	}
}

func (na *neoAgent) onExit() {
}

func (na *neoAgent) run() {
	for {
		select {
		case <-na.mStart.ClickedCh:
			go na.doStart()
		case <-na.mStop.ClickedCh:
			go na.doStop()
		case <-na.mQuit.ClickedCh:
			go na.Stop()
		case <-na.mLog.ClickedCh:
			go na.doShowLog()
		case state := <-na.stateC:
			switch state {
			case NeoStarting:
				na.mStart.Disable()
				na.mStop.Disable()
				systray.SetTitle("Starting...")
			case NeoStopping:
				na.mStart.Disable()
				na.mStop.Disable()
				systray.SetTitle("Stopping...")
			case NeoRunning:
				na.mStart.Disable()
				na.mStop.Enable()
				systray.SetTitle("Running...")
			case NeoStopped:
				na.mStart.Enable()
				na.mStop.Disable()
				systray.SetTitle("Stopped")
			default:
				na.mStart.Disable()
				na.mStop.Disable()
				systray.SetTitle(string(state))
			}
		}
	}
}

func (na *neoAgent) log(line string) {
	na.appendOutput([]byte(strings.TrimSpace(line)))
}

func (na *neoAgent) appendOutput(line []byte) {
	na.outputLock.Lock()
	na.outputs = append(na.outputs, LogLine(line))
	if len(na.outputs) > na.outputLimit {
		na.outputs = na.outputs[(len(na.outputs) - na.outputLimit):]
	}
	na.outputLock.Unlock()
}

func copyReader(src io.ReadCloser, appender func([]byte)) {
	reader := bufio.NewReader(src)
	var parts []byte
	for {
		buf, isPrefix, err := reader.ReadLine()
		if err != nil {
			return
		}
		parts = append(parts, buf...)
		if isPrefix {
			continue
		}
		line := parts
		parts = []byte{}
		appender(line)
	}
}

func (na *neoAgent) doStart() {
	na.stateC <- NeoStarting

	pname := ""
	pargs := []string{}
	if runtime.GOOS == "windows" {
		pname = "cmd.exe"
		pargs = append(pargs, "/c")
		pargs = append(pargs, na.exePath)
		pargs = append(pargs, na.exeArgs...)
	} else {
		pname = na.exePath
		pargs = append(pargs, na.exeArgs...)
	}
	cmd := exec.Command(pname, pargs...)
	sysProcAttr(cmd)
	stdout, _ := cmd.StdoutPipe()
	go copyReader(stdout, na.appendOutput)

	stderr, _ := cmd.StderrPipe()
	go copyReader(stderr, na.appendOutput)

	if err := cmd.Start(); err != nil {
		panic(err)
	}
	na.process = cmd.Process

	na.stateC <- NeoRunning
}

func (na *neoAgent) doStop() {
	if na.process != nil {
		na.stateC <- NeoStopping
		if runtime.GOOS == "windows" {
			// On Windows, sending os.Interrupt to a process with os.Process.Signal is not implemented;
			// it will return an error instead of sending a signal.
			// so, this will not work => na.process.Signal(syscall.SIGINT)
			cmd := exec.Command("cmd.exe", "/c", na.exePath, "shell", "shutdown")
			sysProcAttr(cmd)
			cmd.Run()
		} else {
			err := na.process.Signal(os.Interrupt)
			if err != nil {
				na.log(err.Error())
			}
		}
		state, err := na.process.Wait()
		if err != nil {
			na.log(fmt.Sprintf("Shutdown failed %s", err.Error()))
		} else {
			na.log(fmt.Sprintf("Shutdown exit(%d)", state.ExitCode()))
		}
		na.process = nil
	}
	na.stateC <- NeoStopped
}

type LogLine []byte

func (ll LogLine) String() string {
	return string(ll)
}

func (ll LogLine) ToHTML() []byte {
	return ansihtml.ConvertToHTML(ll)
}

func (na *neoAgent) doShowLog() {
	addr := fmt.Sprintf("http://%s/log", na.httpAddr)
	switch runtime.GOOS {
	case "linux":
		exec.Command("xdg-open", addr).Start()
	case "darwin":
		exec.Command("open", addr).Start()
	case "windows":
		exec.Command("rundll32", "url.dll,FileProtocolHandler", addr).Start()
	}
}

var htmlHeader = []byte(`<html>
	<head>
		<meta charset="utf-8">
		<link rel="stylesheet" href="/terminal.css">
		<style>
		body {
			padding: 0; margin: 0;
			background: #1a1e24;
			width: 100%;
			min-height: 100vh;
			display: -webkit-box; display: -ms-flexbox; display: flex;
			-webkit-box-align: center; -ms-flex-align: center; align-items: center;
			-webkit-box-pack: center; -ms-flex-pack: center; justify-content: center;
			-webkit-font-smoothing: antialiased; -moz-osx-font-smoothing: grayscale;
		}
		</style>
	</head>
	<body>
	<div id="termynal" data-termynal style="width:95%">`)
var htmlFooter = []byte(`</div></body></html>`)

//go:embed terminal.css
var htmlTerminalCss []byte

func (na *neoAgent) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/log" {
		w.Header().Set("Content-Type", "text/html")
		w.Write(htmlHeader)
		na.outputLock.Lock()
		defer na.outputLock.Unlock()
		for _, l := range na.outputs {
			w.Write([]byte(`<span data-ty>`))
			w.Write(l.ToHTML())
			w.Write([]byte(`</span>`))
		}
		w.Write(htmlFooter)
	} else if r.URL.Path == "/terminal.css" {
		w.Header().Set("Content-Type", "text/css")
		w.Write(htmlTerminalCss)
	} else {
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte(r.URL.Path))
	}
}
