package util

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"
)

type WriteDescriptor struct {
	// tql destination
	TqlPath string
	// table destination
	Method   string
	Table    string
	Format   string
	Compress string
	// common parameters
	Timeformat   string
	TimeLocation *time.Location
	Delimiter    string
	Heading      bool
}

func (wd *WriteDescriptor) IsTqlDestination() bool {
	return wd.TqlPath != ""
}

func NewWriteDescriptor(path string) (*WriteDescriptor, error) {
	wd := &WriteDescriptor{
		Timeformat:   "ns",
		TimeLocation: time.UTC,
		Delimiter:    ",",
		Heading:      false,
	}

	var taskPath string
	if strings.Contains(path, "?") {
		toks := strings.SplitN(path, "?", 2)
		taskPath = toks[0]
		if vals, err := url.ParseQuery(toks[1]); err != nil {
			return nil, fmt.Errorf("invalid parameters %s, %s", toks[1], err.Error())
		} else {
			for k, vs := range vals {
				v := ""
				if len(vs) > 0 {
					v = vs[0]
				}
				switch strings.ToLower(k) {
				case "timeformat":
					wd.Timeformat = v
				case "tz":
					wd.TimeLocation = ParseTimeLocation(v, time.UTC)
				case "delimiter":
					wd.Delimiter = v
				case "heading":
					wd.Heading = strings.ToLower(v) == "true"
				}
			}
		}
	} else {
		taskPath = path
	}

	if strings.HasSuffix(taskPath, ".tql") {
		wd.TqlPath = taskPath
	} else {
		if strings.HasPrefix(taskPath, "db/append/") {
			taskPath = strings.TrimPrefix(taskPath, "db/append/")
			wd.Method = "append"
		} else if strings.HasPrefix(taskPath, "db/write/") {
			taskPath = strings.TrimPrefix(taskPath, "db/write/")
			wd.Method = "insert"
		} else {
			return nil, fmt.Errorf("unsupported destination '%s'", taskPath)
		}
		wp, err := ParseWritePath(taskPath)
		if err != nil {
			return nil, err
		}
		if wp.Format == "" {
			wp.Format = "json"
		}
		switch wp.Format {
		case "json":
		case "csv":
		default:
			return nil, fmt.Errorf("unsupported format '%s'", wp.Format)
		}
		switch wp.Compress {
		case "": // no compression
		case "-": // no compression
		case "gzip": // gzip compression
		default: // others
			return nil, fmt.Errorf("unsupproted compression '%s", wp.Compress)
		}
		wd.Table = wp.Table
		wd.Format = wp.Format
		wd.Compress = wp.Compress
	}

	return wd, nil
}

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
