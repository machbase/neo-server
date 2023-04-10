package assets

import (
	_ "embed"
	"net/http"
)

//go:embed favicon.ico
var favicon []byte

//go:embed apple-touch-icon.png
var appleTouchIcon []byte

//go:embed apple-touch-icon-precomposed.png
var appleTouchIconPrecomposed []byte

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
		w.WriteHeader(http.StatusNotFound)
	}
}
