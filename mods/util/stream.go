package util

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
)

func NewOutputStream(output string) (out io.Writer, err error) {
	var outputFields = strings.Fields(output)
	if len(outputFields) > 0 && outputFields[0] == "exec" {
		binArgs := strings.TrimSpace(strings.TrimPrefix(output, "exec"))
		out, err = NewPipeWriter(binArgs)
		if err != nil {
			return
		}
	} else {
		out, err = NewFileWriter(output)
		if err != nil {
			return
		}
	}
	return
}

type fout struct {
	path  string
	w     io.WriteCloser
	buf   *bufio.Writer
	mutex sync.Mutex
}

var _ io.WriteCloser = (*fout)(nil)
var _ interface{ Flush() error } = (*fout)(nil)

func NewFileWriter(path string) (io.Writer, error) {
	out := &fout{
		path: path,
	}
	if err := out.reset(); err != nil {
		return nil, err
	}
	return out, nil
}

func (out *fout) Write(p []byte) (n int, err error) {
	if out.buf == nil {
		return 0, io.EOF
	}
	return out.buf.Write(p)
}

func (out *fout) Flush() error {
	out.mutex.Lock()
	defer out.mutex.Unlock()

	if out.buf == nil {
		return nil
	}
	return out.buf.Flush()
}

// Deprecated do not call from outside.
func (out *fout) reset() error {
	out.Close()

	out.mutex.Lock()
	defer out.mutex.Unlock()

	if out.path == "-" {
		out.w = os.Stdout
	} else {
		var err error
		out.w, err = os.OpenFile(out.path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
		if err != nil {
			return err
		}
	}
	out.buf = bufio.NewWriter(out.w)
	return nil
}

func (out *fout) Close() error {
	out.mutex.Lock()
	defer out.mutex.Unlock()

	if out.buf != nil {
		if err := out.buf.Flush(); err != nil {
			return err
		}
		out.buf = nil
	}
	if out.w != nil && out.path != "-" {
		if err := out.w.Close(); err != nil {
			return err
		}
		out.w = nil
	}
	return nil
}

type pout struct {
	bin  string
	args []string

	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout io.ReadCloser
	mutex  sync.Mutex
}

var _ io.WriteCloser = (*pout)(nil)
var _ interface{ Flush() error } = (*pout)(nil)

func NewPipeWriter(cmdLine string) (io.Writer, error) {
	fields := SplitFields(cmdLine, true)
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
