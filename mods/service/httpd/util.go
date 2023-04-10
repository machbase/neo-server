package httpd

import (
	"strconv"
	"strings"
	"time"
)

func strBool(str string, def bool) bool {
	if str == "" {
		return def
	}
	return strings.ToLower(str) == "true" || str == "1"
}

func strInt(str string, def int) int {
	if str == "" {
		return def
	}
	v, err := strconv.Atoi(str)
	if err != nil {
		return def
	}
	return v
}

func strString(str string, def string) string {
	if str == "" {
		return def
	}
	return str
}

func strTimeLocation(str string, def *time.Location) *time.Location {
	if str == "" {
		return def
	}

	tz := strings.ToLower(str)
	if tz == "local" {
		return time.Local
	} else if tz == "utc" {
		return time.UTC
	} else {
		if loc, err := time.LoadLocation(str); err != nil {
			return def
		} else {
			return loc
		}
	}
}

func strDuration(str string, def time.Duration) time.Duration {
	if str == "" {
		return def
	}
	d, err := time.ParseDuration(str)
	if err != nil {
		return def
	}
	return d
}
