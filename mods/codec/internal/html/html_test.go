package html

import (
	"bytes"
	"io"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestExporterContentTypeAndImages(t *testing.T) {
	ex := NewEncoder()
	var buf bytes.Buffer
	ex.SetOutputStream(&buf)

	require.Equal(t, "application/xhtml+xml", ex.ContentType())
	require.NoError(t, ex.Open())
	require.NoError(t, ex.AddRow([]any{"image/png", []byte("png-bytes")}))
	require.NoError(t, ex.AddRow([]any{"image/jpeg", []byte("jpg-bytes")}))
	ex.Flush(true)
	ex.Close()

	out := buf.String()
	require.Contains(t, out, `data:image/png;base64,`)
	require.Contains(t, out, `data:image/jpeg;base64,`)
	require.Contains(t, out, `<img src=`)
}

func TestExporterInvalidOrIgnoredRows(t *testing.T) {
	ex := NewEncoder()
	ex.SetOutputStream(io.Discard)

	err := ex.AddRow([]any{"image/png", "not-bytes"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid image data type")

	require.NoError(t, ex.AddRow([]any{"text/plain", []byte("ignored")}))
	require.NoError(t, ex.AddRow([]any{"image/png"}))
}
