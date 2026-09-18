package tql

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	client "github.com/machbase/neo-client/v2"
	"github.com/machbase/neo-server/v8/mods/util"
)

const defaultShellTailLines = 1000

var _httpServer string

func SetHttpAddresses(addrs []string) {
	for _, addr := range addrs {
		// if strings.HasPrefix(addr, "unix://") && runtime.GOOS != "windows" {
		// 	_httpServer = addr
		// 	break
		// }
		if strings.HasPrefix(addr, "tcp://127.0.0.1:") {
			_httpServer = addr
			break
		} else {
			_httpServer = addr
		}
	}
}

var _serviceControllerAddr string
var _serviceWorkspace string
var _serverKeyPath string

func SetServiceControllerAddress(addr string) {
	_serviceControllerAddr = addr
}

func SetServiceWorkspace(workspace string) {
	_serviceWorkspace = workspace
}

func SetServerKeyPath(path string) {
	_serverKeyPath = path
}

func (node *Node) fmShell(cmd0 string, args0 ...any) {
	stripQuote := false
	subCmdList := []string{}
	subArgs := [][]string{}
	tailLines := defaultShellTailLines
	cmdArgs := []string{}
	for _, arg := range args0 {
		switch v := arg.(type) {
		case string:
			cmdArgs = append(cmdArgs, v)
		case *QueryLimit:
			limit, err := tqlLimitValue("SHELL", v, defaultShellTailLines)
			if err != nil {
				node.emit(ErrorRecord(err))
				return
			}
			tailLines = limit
		default:
			node.emit(ErrorRecord(fmt.Errorf("SHELL invalid argument %T", arg)))
			return
		}
	}

	if len(cmdArgs) == 0 {
		buff := []string{}
		for _, line := range strings.Split(cmd0, "\n") {
			line = strings.TrimSpace(line)
			buff = append(buff, line)
			if !strings.HasSuffix(line, ";") {
				continue
			}
			line = strings.TrimSuffix(strings.Join(buff, " "), ";")
			buff = []string{}

			toks := util.SplitFields(line, stripQuote)
			if len(toks) == 0 || toks[0] == "" {
				continue
			}
			subCmdList = append(subCmdList, toks[0])
			subArgs = append(subArgs, toks[1:])
		}
		if len(buff) > 0 {
			line := strings.TrimSuffix(strings.Join(buff, " "), ";")
			toks := util.SplitFields(line, stripQuote)
			if len(toks) > 0 {
				subCmdList = append(subCmdList, toks[0])
				subArgs = append(subArgs, toks[1:])
			}
		}
	} else {
		subCmdList = append(subCmdList, cmd0)
		subArgs = append(subArgs, cmdArgs)
	}

	tmpFile, err := os.CreateTemp("", "runner*.sql")
	if err != nil {
		node.emit(ErrorRecord(err))
		return
	}
	defer os.Remove(tmpFile.Name())
	for i, subCmd := range subCmdList {
		args := subArgs[i]

		switch strings.ToLower(subCmd) {
		case "exit", "quit", "set", "help", "clear", "shutdown":
			node.emit(ErrorRecord(fmt.Errorf("command %q is not supported", subCmd)))
			continue
		default:
			line := strings.Join(append([]string{subCmd}, args...), " ")
			fmt.Fprintln(tmpFile, line+";")
		}
	}
	tmpFile.Close()

	var cmd *exec.Cmd
	if args, err := ShellExecutable(_httpServer, tmpFile.Name()); err != nil {
		node.emit(ErrorRecord(err))
		return
	} else {
		cmd = exec.Command(args[0], args[1:]...)
		cmd.Env = append(os.Environ(), "NEOSHELL_USER="+node.ensureRuntime().ConsoleUser())
		cmd.Env = append(cmd.Env, "NEOSHELL_PASSWORD="+node.ensureRuntime().ConsoleOTP())
		if _, ok := node.GetValue("shell"); !ok {
			cols := []*client.Column{
				client.MakeColumnRownum(),
				client.MakeColumnString("RESULT"),
			}
			node.ensureRuntime().SetResultColumns(cols)
		}
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			node.emit(ErrorRecord(err))
			return
		}
		stderr, err := cmd.StderrPipe()
		if err != nil {
			node.emit(ErrorRecord(err))
			return
		}
		if err := cmd.Start(); err != nil {
			node.emit(ErrorRecord(err))
			return
		}

		var wg sync.WaitGroup
		var output []string
		var errOutput []string
		var outputErr error
		var errOutputErr error
		wg.Add(2)
		go func() {
			defer wg.Done()
			output, _, outputErr = shellTailLines(stdout, tailLines)
		}()
		go func() {
			defer wg.Done()
			errOutput, _, errOutputErr = shellTailLines(stderr, tailLines)
		}()

		waitErr := cmd.Wait()
		wg.Wait()
		if outputErr != nil {
			node.emit(ErrorRecord(outputErr))
			return
		}
		if errOutputErr != nil {
			node.emit(ErrorRecord(errOutputErr))
			return
		}
		if waitErr != nil {
			err := waitErr
			if len(errOutput) > 0 {
				err = fmt.Errorf("%s", strings.Join(errOutput, "\n"))
			}
			node.ensureRuntime().LogError(err.Error())
			node.emit(ErrorRecord(err))
		} else {
			var rowNum = 1
			for _, ln := range output {
				node.emit(NewRecord(rowNum, ln))
				rowNum++
			}
		}
	}
}

func shellTailLines(reader io.Reader, limit int) ([]string, int, error) {
	if limit <= 0 {
		limit = defaultShellTailLines
	}
	buffered := make([]string, 0, limit)
	total := 0
	scanner := bufio.NewReader(reader)
	lastHadNewLine := false
	for {
		line, err := scanner.ReadString('\n')
		if len(line) > 0 {
			total++
			lastHadNewLine = strings.HasSuffix(line, "\n")
			line = strings.TrimRight(line, "\r\n")
			if len(buffered) < limit {
				buffered = append(buffered, line)
			} else {
				copy(buffered, buffered[1:])
				buffered[len(buffered)-1] = line
			}
		}
		if err == io.EOF {
			if lastHadNewLine && len(buffered) < limit {
				total++
				buffered = append(buffered, "")
			}
			return buffered, total, nil
		}
		if err != nil {
			return buffered, total, err
		}
	}
}

// intended expose this var to be replaced in shell test cases
var ShellExecutable = func(serverAddr string, scriptPath string) ([]string, error) {
	ex, err := os.Executable()
	if err != nil {
		return nil, err
	}
	serverAddr = strings.TrimPrefix(serverAddr, "tcp://")
	return []string{
		ex, "shell", "--server", serverAddr,
		"-v", "/work=" + _serviceWorkspace,
		"-v", "/tmp=" + filepath.Dir(scriptPath),
		"-e", "SERVICE_CONTROLLER=" + _serviceControllerAddr,
		"-e", "NEOSHELL_IDENTITY_FILE=@" + _serverKeyPath,
		"run",
		"/tmp/" + filepath.Base(scriptPath),
	}, nil
}
