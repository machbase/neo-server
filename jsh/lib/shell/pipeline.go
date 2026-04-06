package shell

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/machbase/neo-server/v8/jsh/engine"
	"github.com/machbase/neo-server/v8/jsh/lib/shell/internal"
	"github.com/machbase/neo-server/v8/jsh/log"
)

func (sh *Shell) printShellError(format string, args ...any) {
	message := fmt.Sprintf(format, args...)
	if sh != nil && sh.env != nil && sh.env.Writer() != nil {
		_, _ = fmt.Fprintln(sh.env.Writer(), message)
	}
	log.Println(message)
}

func (sh *Shell) runStatement(stmt *Statement) (int, bool) {
	if len(stmt.Pipelines) == 0 {
		return 0, true
	}

	expandedPipelines := make([]*Pipeline, 0, len(stmt.Pipelines))
	for _, pipe := range stmt.Pipelines {
		expanded, err := sh.expandPipeline(pipe)
		if err != nil {
			sh.printShellError("%s", strings.TrimPrefix(err.Error(), "Error: "))
			return 1, true
		}
		expandedPipelines = append(expandedPipelines, expanded)
	}

	if len(expandedPipelines) == 1 {
		return sh.runSinglePipeline(expandedPipelines[0])
	}
	return sh.runStreamingPipeline(expandedPipelines), true
}

func (sh *Shell) runSinglePipeline(pipe *Pipeline) (int, bool) {
	if pipe.Command == "exit" || pipe.Command == "quit" {
		return 0, false
	}

	// Reject assignment-only statements (no command)
	if pipe.Command == "" && len(pipe.Assignments) > 0 {
		sh.printShellError("assignment without command is not supported")
		return 1, true
	}

	if internal.IsCommand(pipe.Command) {
		if len(pipe.Assignments) > 0 {
			sh.printShellError("temporary environment for internal commands is not supported")
			return 1, true
		}
		if pipe.Stdin != nil || pipe.Stdout != nil || pipe.Stderr != nil {
			log.Printf("redirection is not implemented for internal command: %s\n", pipe.Command)
			return 1, true
		}
		if exitCode, ok := internal.Run(sh.env, sh.env.Writer(), pipe.Command, pipe.Args...); ok {
			return exitCode, true
		} else {
			log.Printf("command not found: %s\n", pipe.Command)
			return 1, true
		}
	}

	return sh.runExternalPipelineStage(pipe), true
}

func (sh *Shell) runStreamingPipeline(pipelines []*Pipeline) int {
	if sh.env == nil {
		log.Println("pipeline execution requires shell environment")
		return 1
	}
	for i, pipe := range pipelines {
		if pipe.Command == "exit" || pipe.Command == "quit" {
			log.Printf("command cannot be used in pipeline: %s\n", pipe.Command)
			return 1
		}
		if internal.IsCommand(pipe.Command) {
			log.Printf("command cannot be used in pipeline: %s\n", pipe.Command)
			return 1
		}
		if pipe.Stdin != nil && i != 0 {
			log.Println("stdin redirection is only supported on the first pipeline stage")
			return 1
		}
		if pipe.Stdout != nil && i != len(pipelines)-1 {
			log.Println("stdout redirection is only supported on the final pipeline stage")
			return 1
		}
		if pipe.Stderr != nil && pipe.Stderr.Type != "2>" && pipe.Stderr.Type != "2>>" && pipe.Stderr.Type != "2>&1" {
			log.Printf("stderr redirection is not implemented: %s\n", pipe.Stderr.Type)
			return 1
		}
	}

	cmds := make([]*exec.Cmd, 0, len(pipelines))
	pipeReaders := make([]*os.File, 0, len(pipelines)-1)
	pipeWriters := make([]*os.File, 0, len(pipelines)-1)
	redirectClosers := []func(){}
	for _, pipe := range pipelines {
		cmd, err := sh.buildExternalExecCmd(pipe.Command, pipe.Args, pipe.Assignments)
		if err != nil {
			log.Println(strings.TrimPrefix(err.Error(), "Error: "))
			return 1
		}
		cmds = append(cmds, cmd)
	}

	cmds[0].Stdin = sh.env.Reader()
	last := len(cmds) - 1
	cmds[last].Stdout = sh.env.Writer()
	for _, cmd := range cmds {
		cmd.Stderr = sh.env.Writer()
	}

	for i := 0; i < len(cmds)-1; i++ {
		reader, writer, err := os.Pipe()
		if err != nil {
			log.Printf("pipeline pipe error: %v\n", err)
			closeFiles(pipeReaders)
			closeFiles(pipeWriters)
			closeResources(redirectClosers)
			return 1
		}
		cmds[i].Stdout = writer
		cmds[i+1].Stdin = reader
		pipeReaders = append(pipeReaders, reader)
		pipeWriters = append(pipeWriters, writer)
	}

	if pipelines[0].Stdin != nil {
		reader, closeFn, err := openInputRedirect(sh.env, pipelines[0].Stdin)
		if err != nil {
			closeFiles(pipeReaders)
			closeFiles(pipeWriters)
			closeResources(redirectClosers)
			log.Printf("pipeline input redirection error: %v\n", err)
			return 1
		}
		cmds[0].Stdin = reader
		redirectClosers = append(redirectClosers, closeFn)
	}
	if pipelines[last].Stdout != nil {
		writer, closeFn, err := openOutputRedirect(sh.env, pipelines[last].Stdout)
		if err != nil {
			closeFiles(pipeReaders)
			closeFiles(pipeWriters)
			closeResources(redirectClosers)
			log.Printf("pipeline output redirection error: %v\n", err)
			return 1
		}
		cmds[last].Stdout = writer
		redirectClosers = append(redirectClosers, closeFn)
	}
	for i, pipe := range pipelines {
		if pipe.Stderr != nil {
			writer, closeFn, err := openErrorRedirect(sh.env, pipe.Stderr, cmds[i].Stdout)
			if err != nil {
				closeFiles(pipeReaders)
				closeFiles(pipeWriters)
				closeResources(redirectClosers)
				log.Printf("pipeline stderr redirection error: %v\n", err)
				return 1
			}
			cmds[i].Stderr = writer
			redirectClosers = append(redirectClosers, closeFn)
		}
	}

	started := make([]*exec.Cmd, 0, len(cmds))
	for i, cmd := range cmds {
		if err := cmd.Start(); err != nil {
			closeFiles(pipeReaders)
			closeFiles(pipeWriters)
			closeResources(redirectClosers)
			killStarted(started)
			waitStarted(started)
			log.Printf("pipeline start error: %v\n", err)
			return 1
		}
		started = append(started, cmd)
		if i > 0 {
			_ = pipeReaders[i-1].Close()
			pipeReaders[i-1] = nil
		}
		if i < len(pipeWriters) {
			_ = pipeWriters[i].Close()
			pipeWriters[i] = nil
		}
	}
	closeFiles(pipeReaders)
	closeFiles(pipeWriters)

	lastExitCode := 0
	for i, cmd := range started {
		exitCode, err := waitCommand(cmd)
		if err != nil {
			log.Printf("pipeline wait error: %v\n", err)
		}
		if i == len(started)-1 {
			lastExitCode = exitCode
		}
	}
	closeResources(redirectClosers)
	return lastExitCode
}

func (sh *Shell) runExternalPipelineStage(pipe *Pipeline) int {
	if sh.env == nil {
		log.Println("command execution requires shell environment")
		return 1
	}
	cmd, err := sh.buildExternalExecCmd(pipe.Command, pipe.Args, pipe.Assignments)
	if err != nil {
		log.Println(strings.TrimPrefix(err.Error(), "Error: "))
		return 1
	}

	redirectClosers := []func(){}
	cmd.Stdin = sh.env.Reader()
	cmd.Stdout = sh.env.Writer()
	cmd.Stderr = sh.env.Writer()

	if pipe.Stdin != nil {
		reader, closeFn, err := openInputRedirect(sh.env, pipe.Stdin)
		if err != nil {
			log.Printf("input redirection error: %v\n", err)
			return 1
		}
		cmd.Stdin = reader
		redirectClosers = append(redirectClosers, closeFn)
	}
	if pipe.Stdout != nil {
		writer, closeFn, err := openOutputRedirect(sh.env, pipe.Stdout)
		if err != nil {
			closeResources(redirectClosers)
			log.Printf("output redirection error: %v\n", err)
			return 1
		}
		cmd.Stdout = writer
		redirectClosers = append(redirectClosers, closeFn)
	}
	if pipe.Stderr != nil {
		writer, closeFn, err := openErrorRedirect(sh.env, pipe.Stderr, cmd.Stdout)
		if err != nil {
			closeResources(redirectClosers)
			log.Printf("stderr redirection error: %v\n", err)
			return 1
		}
		cmd.Stderr = writer
		redirectClosers = append(redirectClosers, closeFn)
	}

	if err := cmd.Start(); err != nil {
		closeResources(redirectClosers)
		log.Printf("command start error: %v\n", err)
		return 1
	}
	exitCode, err := waitCommand(cmd)
	closeResources(redirectClosers)
	if err != nil {
		log.Printf("command wait error: %v\n", err)
		return 1
	}
	return exitCode
}

func (sh *Shell) buildExternalExecCmd(command string, args []string, assignments []Assignment) (*exec.Cmd, error) {
	if sh.env == nil {
		return nil, fmt.Errorf("shell environment is not initialized")
	}
	resolvedPath, resolvedArgs, err := sh.resolveExternalCommand(command, args)
	if err != nil {
		return nil, err
	}
	if strings.HasPrefix(resolvedPath, "@") {
		return exec.Command(resolvedPath[1:], resolvedArgs...), nil
	}
	builder := sh.env.ExecBuilder()
	if builder == nil {
		return nil, fmt.Errorf("no command builder defined")
	}
	argv := make([]string, 0, len(resolvedArgs)+1)
	argv = append(argv, resolvedPath)
	argv = append(argv, resolvedArgs...)
	envMap := overlayEnv(sh.env, assignments)
	return builder("", argv, envMap)
}

func (sh *Shell) resolveExternalCommand(command string, args []string) (string, []string, error) {
	if sh.env == nil {
		return "", nil, fmt.Errorf("shell environment is not initialized")
	}
	resolvedPath := sh.env.Which(command)
	if resolvedPath == "" && !strings.HasSuffix(command, ".js") {
		resolvedPath = sh.env.Which(command + "/index.js")
	}
	if resolvedPath == "" {
		return "", nil, fmt.Errorf("command not found: %s", command)
	}
	return resolvedPath, append([]string{}, args...), nil
}

func snapshotEnv(env *engine.Env) map[string]any {
	vars := make(map[string]any)
	env.ForEach(func(key string, value any) {
		vars[key] = value
	})
	return vars
}

// overlayEnv creates a snapshot of the shell environment and overlays the given
// assignment values on top. The assignments are applied in order so that later
// assignments with the same name win. The original shell environment is not modified.
func overlayEnv(env *engine.Env, assignments []Assignment) map[string]any {
	vars := snapshotEnv(env)
	for _, a := range assignments {
		vars[a.Name] = a.Value
	}
	return vars
}

func openInputRedirect(env *engine.Env, redir *Redirect) (io.Reader, func(), error) {
	if redir == nil || redir.Type != "<" {
		return nil, nil, fmt.Errorf("unsupported input redirection")
	}
	fileSystem, err := shellFilesystem(env)
	if err != nil {
		return nil, nil, err
	}
	fd, err := fileSystem.OpenFD(env.ResolveAbsPath(redir.Target), os.O_RDONLY, 0)
	if err != nil {
		return nil, nil, err
	}
	reader, err := fileSystem.HostReaderFD(fd)
	if err != nil {
		_ = fileSystem.CloseFD(fd)
		return nil, nil, err
	}
	return reader, func() { _ = fileSystem.CloseFD(fd) }, nil
}

func openOutputRedirect(env *engine.Env, redir *Redirect) (io.Writer, func(), error) {
	if redir == nil {
		return nil, nil, fmt.Errorf("missing output redirection")
	}
	flags := os.O_CREATE | os.O_WRONLY
	switch redir.Type {
	case ">":
		flags |= os.O_TRUNC
	case ">>":
		flags |= os.O_APPEND
	default:
		return nil, nil, fmt.Errorf("unsupported output redirection: %s", redir.Type)
	}
	fileSystem, err := shellFilesystem(env)
	if err != nil {
		return nil, nil, err
	}
	fd, err := fileSystem.OpenFD(env.ResolveAbsPath(redir.Target), flags, 0644)
	if err != nil {
		return nil, nil, err
	}
	writer, err := fileSystem.HostWriterFD(fd)
	if err != nil {
		_ = fileSystem.CloseFD(fd)
		return nil, nil, err
	}
	return writer, func() { _ = fileSystem.CloseFD(fd) }, nil
}

func openErrorRedirect(env *engine.Env, redir *Redirect, stdout io.Writer) (io.Writer, func(), error) {
	if redir == nil {
		return nil, nil, fmt.Errorf("missing stderr redirection")
	}
	switch redir.Type {
	case "2>":
		return openOutputRedirect(env, &Redirect{Type: ">", Target: redir.Target})
	case "2>>":
		return openOutputRedirect(env, &Redirect{Type: ">>", Target: redir.Target})
	case "2>&1":
		if stdout == nil {
			return nil, nil, fmt.Errorf("stdout destination is not available for 2>&1")
		}
		return stdout, func() {}, nil
	default:
		return nil, nil, fmt.Errorf("unsupported stderr redirection: %s", redir.Type)
	}
}

func shellFilesystem(env *engine.Env) (*engine.FS, error) {
	fileSystem, ok := env.Filesystem().(*engine.FS)
	if !ok || fileSystem == nil {
		return nil, fmt.Errorf("shell filesystem does not support host-backed redirection")
	}
	return fileSystem, nil
}

func waitCommand(cmd *exec.Cmd) (int, error) {
	if err := cmd.Wait(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return exitErr.ExitCode(), nil
		}
		return -1, err
	}
	return 0, nil
}

func killStarted(cmds []*exec.Cmd) {
	for _, cmd := range cmds {
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
	}
}

func waitStarted(cmds []*exec.Cmd) {
	for _, cmd := range cmds {
		if cmd.Process != nil {
			_, _ = waitCommand(cmd)
		}
	}
}

func closeFiles(files []*os.File) {
	for _, file := range files {
		_ = file.Close()
	}
}

func closeResources(closers []func()) {
	for _, closer := range closers {
		closer()
	}
}
