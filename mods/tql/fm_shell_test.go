package tql

import (
	"errors"
	"io"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestShellPipeLifecycle(t *testing.T) {
	for _, tc := range []struct {
		name      string
		lineRange lineRangeOption
		waitErr   error
	}{
		{name: "tail", lineRange: lineRangeOption{offset: -1}},
		{name: "forward", lineRange: lineRangeOption{offset: 1, count: 1}},
		{name: "exit_error", lineRange: lineRangeOption{offset: -1}, waitErr: errors.New("exit status 1")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			stdout, stdoutWriter := io.Pipe()
			stderr, stderrWriter := io.Pipe()
			t.Cleanup(func() {
				stdout.Close()
				stderr.Close()
				stdoutWriter.Close()
				stderrWriter.Close()
			})
			writeErrors := make(chan error, 2)
			for _, stream := range []struct {
				writer *io.PipeWriter
				text   string
			}{
				{writer: stdoutWriter, text: "discarded\nstdout"},
				{writer: stderrWriter, text: "discarded\nstderr"},
			} {
				go func(writer *io.PipeWriter, text string) {
					_, err := io.WriteString(writer, text)
					writer.Close()
					writeErrors <- err
				}(stream.writer, stream.text)
			}
			var output, errOutput []string
			var outputErr, errOutputErr error
			readsCompleted := make(chan struct{})
			waitCalled := false
			waitErr := shellReadAndWait(func() {
				var readers sync.WaitGroup
				readers.Add(2)
				go func() {
					defer readers.Done()
					output, _, outputErr = shellReadLines(stdout, tc.lineRange)
				}()
				go func() {
					defer readers.Done()
					errOutput, _, errOutputErr = shellReadLines(stderr, tc.lineRange)
				}()
				readers.Wait()
				close(readsCompleted)
			}, func() error {
				waitCalled = true
				select {
				case <-readsCompleted:
				default:
					t.Error("process wait called before output reads completed")
				}
				stdout.Close()
				stderr.Close()
				return tc.waitErr
			})
			require.True(t, waitCalled)
			require.Equal(t, tc.waitErr, waitErr)
			require.NoError(t, <-writeErrors)
			require.NoError(t, <-writeErrors)
			require.NoError(t, outputErr)
			require.NoError(t, errOutputErr)
			require.Equal(t, []string{"stdout"}, output)
			require.Equal(t, []string{"stderr"}, errOutput)
		})
	}
}

func TestSetHttpAddresses(t *testing.T) {
	_httpServer = ""
	SetHttpAddresses([]string{"tcp://10.0.0.1:8888", "tcp://127.0.0.1:7777", "tcp://127.0.0.1:6666"})
	require.Equal(t, "tcp://127.0.0.1:7777", _httpServer)

	_httpServer = ""
	SetHttpAddresses([]string{"http://example.com", "http://other.example.com"})
	require.Equal(t, "http://other.example.com", _httpServer)
}

func TestSetServiceControllerAddress(t *testing.T) {
	_serviceControllerAddr = ""
	SetServiceControllerAddress("unix:///tmp/controller.sock")
	require.Equal(t, "unix:///tmp/controller.sock", _serviceControllerAddr)
}

func TestSetServiceWorkspace(t *testing.T) {
	_serviceWorkspace = ""
	SetServiceWorkspace("/tmp/service-workspace")
	require.Equal(t, "/tmp/service-workspace", _serviceWorkspace)
}
