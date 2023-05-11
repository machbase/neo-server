package main

import (
	"bufio"
	"fmt"
	"image/color"
	"io"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
	"unicode"

	_ "embed"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/data/binding"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/machbase/neo-server/main/neow/res"
	"github.com/machbase/neo-server/mods/util"
)

type neoAgent struct {
	exePath   string
	exeArgs   []string
	autoStart bool

	stateC    chan NeoState
	process   *os.Process
	processWg sync.WaitGroup

	outputs     []LogLine
	outputLock  sync.Mutex
	outputLimit int

	mainWindow     fyne.Window
	mainTextScroll *container.Scroll
	mainTextGrid   *widget.TextGrid
}

type NeoState string

const (
	NeoStarting NeoState = "starting"
	NeoRunning  NeoState = "running"
	NeoStopping NeoState = "stopping"
	NeoStopped  NeoState = "not running"
)

func (na *neoAgent) Start() {
	iconLogo := fyne.NewStaticResource("logo.png", res.Logo)
	iconLightYellow := fyne.NewStaticResource("sig_yellow.png", res.CircleYellow)
	iconLightGreen := fyne.NewStaticResource("sig_green.png", res.CircleGreen)
	iconLightRed := fyne.NewStaticResource("sig_red.png", res.CircleRed)
	a := app.NewWithID("com.machbase.neow")
	a.SetIcon(iconLogo)
	a.Settings().SetTheme(newAppTheme())
	if args := a.Preferences().String("args"); len(args) > 0 {
		na.exeArgs = util.SplitFields(args, true)
	} else {
		home, err := os.UserHomeDir()
		if err != nil {
			if runtime.GOOS == "windows" {
				home = "C:\\"
			} else {
				home = "/tmp"
			}
		}
		args = fmt.Sprintf(`--data "%s"`, filepath.Join(home, "machbase_home"))
		a.Preferences().SetString("args", args)
		na.exeArgs = util.SplitFields(args, true)
	}
	a.Lifecycle().SetOnStopped(func() {
		na.Stop()
	})

	na.mainWindow = a.NewWindow("machbase-neo")
	na.mainWindow.SetMaster()

	var playAndStopButton *widget.Button
	var openBrowserButton *widget.Button
	var statusBox *fyne.Container
	var startOptionEntry *widget.Entry

	var startOptionString = binding.NewString()
	startOptionString.Set(strings.Join(na.exeArgs, " "))
	startOptionString.AddListener(binding.NewDataListener(func() {
		if str, err := startOptionString.Get(); err != nil {
			return
		} else {
			na.exeArgs = util.SplitFields(str, true)
			a.Preferences().SetString("args", str)
		}
	}))
	const StartDatabaseText = "machbase-neo serve"
	const StopDatabaseText = "Stop machbase-neo "
	itmShowLogs := fyne.NewMenuItem("Show window", func() {
		na.doShowMainWindow()
	})
	itmOpenWebUI := fyne.NewMenuItem("Open in Web Browser", func() {
		na.doOpenWebUI()
	})
	playAndStopButton = widget.NewButtonWithIcon("", theme.ComputerIcon(), func() {
		if playAndStopButton.Text == StartDatabaseText {
			na.doStartDatabase()
		} else if playAndStopButton.Text == StopDatabaseText {
			na.doStopDatabase()
		}
	})
	openBrowserButton = widget.NewButtonWithIcon("Open in browser", theme.GridIcon(), func() {
		na.doOpenWebUI()
	})
	openBrowserButton.Disable()

	statusBox = container.New(layout.NewHBoxLayout())

	startOptionEntry = widget.NewEntryWithData(startOptionString)
	startOptionEntry.SetPlaceHolder("flags")
	m := fyne.NewMenu("machbase-neo",
		itmShowLogs,
		itmOpenWebUI,
		// fyne.NewMenuItemSeparator(),
	)

	if desk, ok := a.(desktop.App); ok && runtime.GOOS != "windows" {
		desk.SetSystemTrayIcon(iconLogo)
		desk.SetSystemTrayMenu(m)
	}

	go func() {
		// There is some weird behavior on macOS
		// Guessing some issue related timing between SetSystemTrayMenu() and menu.Refresh()
		time.Sleep(1000 * time.Millisecond)

		menu := m // capturing
		for state := range na.stateC {
			var statusLight *widget.Icon
			switch state {
			case NeoStarting:
				statusLight = widget.NewIcon(iconLightYellow)
				itmOpenWebUI.Disabled = true
				openBrowserButton.Disable()
				playAndStopButton.Disable()
				startOptionEntry.Disable()
			case NeoRunning:
				statusLight = widget.NewIcon(iconLightGreen)
				itmOpenWebUI.Disabled = false
				openBrowserButton.Enable()
				playAndStopButton.SetText(StopDatabaseText)
				playAndStopButton.SetIcon(theme.MediaStopIcon())
				playAndStopButton.Enable()
				startOptionEntry.Disable()
			case NeoStopping:
				statusLight = widget.NewIcon(iconLightYellow)
				itmOpenWebUI.Disabled = true
				openBrowserButton.Disable()
				playAndStopButton.Disable()
				startOptionEntry.Disable()
			case NeoStopped:
				statusLight = widget.NewIcon(iconLightRed)
				itmOpenWebUI.Disabled = true
				openBrowserButton.Disable()
				playAndStopButton.SetText(StartDatabaseText)
				playAndStopButton.SetIcon(theme.MediaPlayIcon())
				playAndStopButton.Enable()
				startOptionEntry.Enable()
			}

			statusBox.RemoveAll()
			statusBox.Add(statusLight)
			statusBox.Add(widget.NewLabel(strings.ToUpper(string(state))))
			statusBox.Refresh()
			menu.Refresh()
		}
	}()

	playAndStop := container.New(layout.NewHBoxLayout(), playAndStopButton)
	topBox := container.New(layout.NewBorderLayout(nil, nil, playAndStop, nil), startOptionEntry, playAndStop)
	bottomBox := container.New(layout.NewBorderLayout(nil, nil, statusBox, openBrowserButton), statusBox, openBrowserButton)

	na.mainTextGrid = widget.NewTextGrid()
	na.mainTextScroll = container.NewScroll(na.mainTextGrid)
	middleBox := container.New(layout.NewMaxLayout(), na.mainTextScroll)

	mainBox := container.New(layout.NewBorderLayout(topBox, bottomBox, nil, nil), topBox, middleBox, bottomBox)
	na.mainWindow.SetContent(mainBox)
	na.mainWindow.SetCloseIntercept(func() {
		if na.process != nil {
			title := "Database is running..."
			message := "Are you sure to shutdown the database and quit?"
			d := dialog.NewConfirm(title, message, func(confirm bool) {
				if !confirm {
					return
				}
				na.doStopDatabase()
				a.Quit()
			}, na.mainWindow)
			d.Show()
		} else {
			a.Quit()
		}
	})

	// initialize state
	na.stateC <- NeoStopped

	if na.autoStart {
		go na.doStartDatabase()
	}

	na.mainWindow.Resize(fyne.NewSize(800, 600))
	na.mainWindow.Show()

	a.Run()
	na.Stop()
}

func (na *neoAgent) Stop() {
	na.doStopDatabase()
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
	tabWidth := 4
	rows := make([]widget.TextGridRow, len(na.outputs))
	for i, line := range na.outputs {
		cells := line.ToTextGridCell(tabWidth)
		rows[i] = widget.TextGridRow{Cells: cells}
	}

	na.mainTextGrid.Rows = rows
	na.mainTextGrid.Refresh()
	na.mainTextScroll.ScrollToBottom()

	na.outputLock.Unlock()
}

func nextTab(column int, tabWidth int) int {
	tabStop, _ := math.Modf(float64(column+tabWidth) / float64(tabWidth))
	return tabWidth * int(tabStop)
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

func (na *neoAgent) clearLogs() {
	na.outputs = []LogLine{}
}

func (na *neoAgent) doStartDatabase() {
	na.stateC <- NeoStarting

	na.clearLogs()
	pname := ""
	pargs := []string{}
	if runtime.GOOS == "windows" {
		pname = "cmd.exe"
		pargs = append(pargs, "/c")
		pargs = append(pargs, na.exePath)
		pargs = append(pargs, "serve")
		pargs = append(pargs, na.exeArgs...)
	} else {
		pname = na.exePath
		pargs = append(pargs, "serve")
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

	go func() {
		na.stateC <- NeoRunning
		na.processWg.Add(1)
		state, err := na.process.Wait()
		na.processWg.Done()
		if err != nil {
			na.log(fmt.Sprintf("Shutdown failed %s", err.Error()))
		} else {
			na.log(fmt.Sprintf("Shutdown exit(%d)", state.ExitCode()))
		}
		na.process = nil
		na.stateC <- NeoStopped
	}()
}

func (na *neoAgent) doStopDatabase() {
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
		na.processWg.Wait()
	}
}

func (na *neoAgent) doShowMainWindow() {
	na.mainWindow.Show()
}

func (na *neoAgent) doOpenWebUI() {
	addr := fmt.Sprintf("http://%s/web/ui/", "127.0.0.1:5654")
	switch runtime.GOOS {
	case "linux":
		exec.Command("xdg-open", addr).Start()
	case "darwin":
		exec.Command("open", addr).Start()
	case "windows":
		exec.Command("rundll32", "url.dll,FileProtocolHandler", addr).Start()
	}
}

var ConsoleStyle widget.TextGridStyle = &consoleStyle{}

type consoleStyle struct {
}

func (cs *consoleStyle) TextColor() color.Color {
	return widget.TextGridStyleDefault.TextColor()
}

func (cs *consoleStyle) BackgroundColor() color.Color {
	return widget.TextGridStyleDefault.BackgroundColor()
}

type LogLine []byte

func (ll LogLine) String() string {
	return string(ll)
}

const (
	asciiBell      = 7
	asciiBackspace = 8
	asciiEscape    = 27

	noEscape = math.MaxInt
)

func (ll LogLine) ToTextGridCell(tabWidth int) []widget.TextGridCell {
	cells := make([]widget.TextGridCell, 0, len(ll))
	esc := noEscape
	code := ""
	osc := false
	style := ConsoleStyle

	for i, r := range string(ll) {
		if r == asciiEscape {
			esc = i
			continue
		}
		if esc == i-1 {
			if r == '[' {
				continue
			}
			switch r {
			case '\\':
				code = ""
				osc = false
			case ']':
				osc = true
			}
			esc = noEscape
			continue
		}
		if osc {
			if r == asciiBell || r == 0 {
				code = ""
				osc = false
			} else {
				code += string(r)
			}
			continue
		} else if esc != noEscape {
			code += string(r)
			if (r < '0' || r > '9') && r != ';' && r != '=' && r != '?' {
				if strings.HasSuffix(code, "m") {
					code = strings.TrimSuffix(code, "m")
					bfToks := strings.SplitN(code, ";", 2)
					if len(bfToks) == 1 && bfToks[0] == "0" {
						style = ConsoleStyle
					} else {
						bg := ansiColor(bfToks[0], ConsoleStyle.BackgroundColor())
						fg := ansiColor(bfToks[1], ConsoleStyle.TextColor())
						style = &widget.CustomTextGridStyle{FGColor: fg, BGColor: bg}
					}
				}
				code = ""
				esc = noEscape
			}
			continue
		}
		cells = append(cells, widget.TextGridCell{Rune: r, Style: style})
		if r == '\t' {
			col := len(cells)
			next := nextTab(col-1, tabWidth)
			for i := col; i < next; i++ {
				cells = append(cells, widget.TextGridCell{Rune: ' ', Style: style})
			}
		} else if unicode.Is(unicode.Hangul, r) || unicode.Is(unicode.Han, r) || unicode.Is(unicode.Javanese, r) {
			// CJK Unicode block
			cells = append(cells, widget.TextGridCell{Rune: ' ', Style: style})
		}
	}
	return cells
}

func ansiColor(code string, def color.Color) color.Color {
	switch code {
	case "0": // reset
		return def
	case "30": // black
		return color.Black
	case "31": // red
		return color.RGBA{R: 255, G: 65, B: 54, A: 255}
	case "32": // green
		return color.RGBA{R: 149, G: 189, B: 64, A: 255}
	case "33": // yellow
		return color.RGBA{R: 213, G: 217, B: 17, A: 255}
	case "34": // blue
		return color.RGBA{R: 0, G: 116, B: 217, A: 255}
	case "35": // magenta
		return color.RGBA{R: 177, G: 13, B: 201, A: 255}
	case "36": // cyan
		return color.RGBA{R: 105, G: 206, B: 245, A: 255}
	case "37": // white
		return color.RGBA{R: 255, G: 255, B: 255, A: 255}
	}
	return def
}
