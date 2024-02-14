package util

import (
	"errors"
	"strings"
)

type WritePath struct {
	Table    string
	Format   string
	Compress string
}

// parse
// "<table>"                      default format is "json"
// "<table>:<format>"             format "json", "csv"
// "<table>:<format>:<compress>"  transformer "gzip" or "-", "" for no-compression
func ParseWritePath(path string) (*WritePath, error) {
	toks := strings.Split(path, ":")
	toksLen := len(toks)
	if toksLen == 0 || toksLen > 4 {
		return nil, errors.New("invalid syntax")
	}

	r := &WritePath{}
	switch toksLen {
	case 1:
		r.Table = strings.ToUpper(strings.TrimSpace(toks[0]))
	case 2:
		r.Table = strings.ToUpper(strings.TrimSpace(toks[0]))
		r.Format = strings.ToLower(strings.TrimSpace(toks[1]))
	case 3:
		r.Table = strings.ToUpper(strings.TrimSpace(toks[0]))
		r.Format = strings.ToLower(strings.TrimSpace(toks[1]))
		r.Compress = strings.ToLower(strings.TrimSpace(toks[2]))
	}
	return r, nil
}
