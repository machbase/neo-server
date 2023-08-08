package python3

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"runtime/debug"
	"strings"

	"github.com/machbase/neo-server/mods/util"
)

type bridge struct {
	name string
	path string

	binPath string
	dirPath string
	envs    []string
}

func New(name string, path string) *bridge {

	return &bridge{name: name, path: path}
}

func (c *bridge) String() string {
	return fmt.Sprintf("bridge '%s' (python3)", c.name)
}

func (c *bridge) Name() string {
	return c.name
}

func (c *bridge) BeforeRegister() error {
	fields := util.SplitFields(c.path, true)
	for _, f := range fields {
		toks := strings.SplitN(f, "=", 2)
		k := strings.TrimSpace(toks[0])
		v := strings.TrimSpace(toks[1])
		v = util.StripQuote(v)
		switch k {
		case "bin":
			c.binPath = v
		case "dir":
			c.dirPath = v
		case "env":
			c.envs = append(c.envs, v)
		default:
			return fmt.Errorf("bridge '%s' contains unknown option '%s' in connection string", c.name, k)
		}
	}
	return nil
}

func (c *bridge) AfterUnregister() error {
	return nil
}

func (c *bridge) Version(ctx context.Context) (string, error) {
	defer func() {
		if o := recover(); o != nil {
			fmt.Printf("panic %s\n%s", o, debug.Stack())
		}
	}()
	_, stdout, stderr, err := c.Invoke(ctx, []string{"--version"}, nil)
	if err != nil {
		return "", err
	}
	var output []byte
	if stdout != nil {
		output = stdout
	}
	if stderr != nil {
		output = append(output, stderr...)
	}
	if len(output) == 0 {
		return "", nil
	} else {
		return strings.TrimSpace(string(output)), nil
	}
}

// returns exitCode int, stdout []byte, stderr []byte, err error
func (c *bridge) Invoke(ctx context.Context, args []string, input []byte) (int, []byte, []byte, error) {
	defer func() {
		if o := recover(); o != nil {
			fmt.Printf("panic %s\n%s", o, debug.Stack())
		}
	}()
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	cmd := exec.CommandContext(ctx, c.binPath, args...)
	if len(input) > 0 {
		cmd.Stdin = bytes.NewBuffer(input)
	}
	if len(c.envs) > 0 {
		cmd.Env = c.envs
	}
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if c.dirPath != "" {
		cmd.Dir = c.dirPath
	}

	if err := cmd.Start(); err != nil {
		return -1, stdout.Bytes(), stderr.Bytes(), err
	}

	if err := cmd.Wait(); err != nil {
		return -1, stdout.Bytes(), stderr.Bytes(), err
	}

	exitCode := cmd.ProcessState.ExitCode()
	return exitCode, stdout.Bytes(), stderr.Bytes(), nil
}
