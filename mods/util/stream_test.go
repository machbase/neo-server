package util_test

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/machbase/neo-server/v8/mods/util"
	"github.com/stretchr/testify/require"
)

type flushRecorder struct {
	bytes.Buffer
	flushed bool
}

func (f *flushRecorder) Flush() error {
	f.flushed = true
	return nil
}

func TestNopCloseWriterFlush(t *testing.T) {
	recorder := &flushRecorder{}
	writer := &util.NopCloseWriter{Writer: recorder}

	_, err := writer.Write([]byte("payload"))
	require.NoError(t, err)
	require.NoError(t, writer.Flush())
	require.NoError(t, writer.Close())

	require.True(t, recorder.flushed)
	require.Equal(t, "payload", recorder.String())
}

func TestNewFileWriterWritesToFile(t *testing.T) {
	filePath := filepath.Join(t.TempDir(), "out.log")

	writer, err := util.NewFileWriter(filePath)
	require.NoError(t, err)

	closer, ok := writer.(io.WriteCloser)
	require.True(t, ok)

	_, err = writer.Write([]byte("hello file"))
	require.NoError(t, err)
	require.NoError(t, closer.Close())

	content, err := os.ReadFile(filePath)
	require.NoError(t, err)
	require.Equal(t, "hello file", string(content))
}

func TestNewOutputStreamUsesFileWriter(t *testing.T) {
	filePath := filepath.Join(t.TempDir(), "stream.log")

	writer, err := util.NewOutputStream(filePath)
	require.NoError(t, err)

	closer, ok := writer.(io.WriteCloser)
	require.True(t, ok)

	_, err = writer.Write([]byte("through output stream"))
	require.NoError(t, err)
	require.NoError(t, closer.Close())

	content, err := os.ReadFile(filePath)
	require.NoError(t, err)
	require.Equal(t, "through output stream", string(content))
}

func TestNewPipeWriterRejectsEmptyCommand(t *testing.T) {
	writer, err := util.NewPipeWriter("")
	require.Nil(t, writer)
	require.EqualError(t, err, "empty command line")
}
