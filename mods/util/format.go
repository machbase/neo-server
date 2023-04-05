package util

import (
	"golang.org/x/text/language"
	"golang.org/x/text/message"
)

func BytesUnit(v uint64, lang language.Tag) string {
	p := message.NewPrinter(lang)
	f := float64(v)
	u := ""
	switch {
	case v > 1024*1024*1024:
		f = f / (1024 * 1024 * 1024)
		u = "GB"
	case v > 1024*1024:
		f = f / (1024 * 1024)
		u = "MB"
	case v > 1024:
		f = f / 1024
		u = "KB"
	}
	return p.Sprintf("%.1f %s", f, u)
}
