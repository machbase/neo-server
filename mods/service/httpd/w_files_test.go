package httpd

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestImageFiles(t *testing.T) {
	require.Equal(t, "image/apng", contentTypeOfFile("some/dir/file.apng"))
	require.Equal(t, "image/avif", contentTypeOfFile("some/dir/file.avif"))
	require.Equal(t, "image/gif", contentTypeOfFile("some/dir/file.gif"))
	require.Equal(t, "image/jpeg", contentTypeOfFile("some/dir/file.Jpeg"))
	require.Equal(t, "image/jpeg", contentTypeOfFile("some/dir/file.JPG"))
	require.Equal(t, "image/png", contentTypeOfFile("some/dir/file.PNG"))
	require.Equal(t, "image/svg+xml", contentTypeOfFile("some/dir/file.svg"))
	require.Equal(t, "image/webp", contentTypeOfFile("some/dir/file.webp"))
	require.Equal(t, "image/bmp", contentTypeOfFile("some/dir/file.BMP"))
	require.Equal(t, "image/x-icon", contentTypeOfFile("some/dir/file.ico"))
	require.Equal(t, "image/tiff", contentTypeOfFile("some/dir/file.tiff"))
	require.Equal(t, "", contentTypeOfFile("some/dir/file.txt"))
}
