package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"github.com/getlantern/systray"
	"github.com/machbase/neo-server/main/neow/icon"
)

func main() {
	neoExePath, err := getMachbaseNeoPath()
	if err != nil {
		panic(err)
	}

	na := &neoAgent{
		exepath:     neoExePath,
		stateC:      make(chan NeoState, 1),
		outputLimit: 500,
	}
	na.Start()
}

func getMachbaseNeoPath() (string, error) {
	selfPath := os.Args[0]
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

type neoAgent struct {
	exepath string

	stateC  chan NeoState
	process *os.Process

	outputs     []string
	outputLock  sync.Mutex
	outputLimit int

	mStart *systray.MenuItem
	mStop  *systray.MenuItem
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
	// wait until 'systray.Quit` called
	systray.Run(na.onReady, na.onExit)
}

func (na *neoAgent) Stop() {
	na.doStop()
	systray.Quit()
}

func (na *neoAgent) onReady() {
	systray.SetIcon(icon.Logo)
	na.mStart = systray.AddMenuItem("Start", "Start machbase-neo")
	na.mStop = systray.AddMenuItem("Stop", "Stop machbase-neo")
	na.mStop.Disable()
	systray.AddSeparator()

	na.mQuit = systray.AddMenuItem("Quit", "Quit machbase-neo")
	na.mQuit.SetIcon(icon.Shutdown)

	go na.run()
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
			return
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

func (na *neoAgent) appendOutput(line string) {
	na.outputLock.Lock()
	na.outputs = append(na.outputs, line)
	if len(na.outputs) > na.outputLimit {
		na.outputs = na.outputs[(len(na.outputs) - na.outputLimit):]
	}
	na.outputLock.Unlock()
	fmt.Println(line)
}

func copyReader(src io.ReadCloser, appender func(string)) {
	reader := bufio.NewReader(src)
	var parts []string
	for {
		buf, isPrefix, err := reader.ReadLine()
		if err != nil {
			return
		}
		parts = append(parts, string(buf))
		if isPrefix {
			continue
		}
		line := strings.Join(parts, "")
		parts = parts[:0]
		appender(line)
	}
}

func (na *neoAgent) doStart() {
	na.stateC <- NeoStarting

	cmd := exec.Command(na.exepath, "serve")
	stdout, _ := cmd.StdoutPipe()
	go copyReader(stdout, na.appendOutput)

	stderr, _ := cmd.StderrPipe()
	go copyReader(stderr, na.appendOutput)

	err := cmd.Start()
	if err != nil {
		panic(err)
	}
	na.process = cmd.Process

	na.stateC <- NeoRunning
}

func (na *neoAgent) doStop() {
	if na.process != nil {
		na.stateC <- NeoStopping
		na.process.Signal(os.Interrupt)
		state, err := na.process.Wait()
		if err != nil {
			na.appendOutput(fmt.Sprintf("Shutdown failed %s", err.Error()))
		} else {
			na.appendOutput(fmt.Sprintf("Shutdown %s exit(%d)", state.String(), state.ExitCode()))
		}
		na.process = nil
	}
	na.stateC <- NeoStopped
}
