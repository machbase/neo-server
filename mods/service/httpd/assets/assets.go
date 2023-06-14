package assets

import (
	"embed"
	"io"
	"net/http"
	"strings"
)

//go:embed favicon.ico
var favicon []byte

//go:embed apple-touch-icon.png
var appleTouchIcon []byte

//go:embed apple-touch-icon-precomposed.png
var appleTouchIconPrecomposed []byte

//go:embed echarts/*
var echartsDir embed.FS

const echartsPrefix = "/echarts/"

func Handler(w http.ResponseWriter, r *http.Request) {
	switch r.RequestURI {
	case "/favicon.ico":
		w.Header().Set("Content-Type", "image/x-icon")
		w.WriteHeader(http.StatusOK)
		w.Write(favicon)
	case "/apple-touch-icon.png":
		w.Header().Set("Content-Type", "image/png")
		w.WriteHeader(http.StatusOK)
		w.Write(appleTouchIcon)
	case "/apple-touch-icon-precomposed.png":
		w.Header().Set("Content-Type", "image/png")
		w.WriteHeader(http.StatusOK)
		w.Write(appleTouchIconPrecomposed)
	default:
		if strings.HasPrefix(r.RequestURI, echartsPrefix) {
			path := strings.TrimPrefix(r.RequestURI, "/")
			if file, err := echartsDir.Open(path); err == nil {
				w.WriteHeader(http.StatusOK)
				io.Copy(w, file)
				file.Close()
			} else {
				w.WriteHeader(http.StatusNotFound)
			}
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}
}
