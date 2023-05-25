package pio

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"

	"github.com/machbase/neo-server/mods/stream/spec"
	"github.com/machbase/neo-server/mods/util"
)

type pout struct {
	bin  string
	args []string

	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout io.ReadCloser
	mutex  sync.Mutex
}

func New(cmdLine string) (spec.OutputStream, error) {
	fields := util.SplitFields(cmdLine, true)
	if len(fields) == 0 {
		return nil, errors.New("empty command line")
	}
	out := &pout{bin: fields[0]}

	if len(fields) > 1 {
		out.args = fields[1:]
	}

	if err := out.reset(); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *pout) Write(p []byte) (n int, err error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if s.cmd == nil {
		return 0, io.EOF
	}

	return s.stdin.Write(p)
}

func (s *pout) Flush() error {
	return nil
}

func (s *pout) Close() error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if s.stdin != nil {
		if err := s.stdin.Close(); err != nil {
			return err
		}
		s.stdin = nil
	}

	if s.cmd != nil {
		if err := s.cmd.Wait(); err != nil {
			return err
		}
		code := s.cmd.ProcessState.ExitCode()
		if code != 0 {
			return fmt.Errorf("'%s %s' exit %d", s.bin, strings.Join(s.args, " "), code)
		}
		s.cmd = nil
	}

	return nil
}

func (s *pout) reset() error {
	s.Close()

	s.mutex.Lock()
	defer s.mutex.Unlock()

	var err error

	s.cmd = exec.Command(s.bin, s.args...)
	s.stdin, err = s.cmd.StdinPipe()
	if err != nil {
		fmt.Println("ERR", err.Error())
		return err
	}
	s.stdout, err = s.cmd.StdoutPipe()
	if err != nil {
		fmt.Println("ERR", err.Error())
		return err
	}

	go func() {
		io.Copy(os.Stdout, s.stdout)
	}()

	err = s.cmd.Start()
	if err != nil {
		fmt.Println("ERR start", err.Error())
		return err
	}

	return nil
}
