package util

import (
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"
)

func ParseTime(field string, format string, location *time.Location) (time.Time, error) {
	timeLayout := GetTimeformat(format)
	var ts int64
	var err error
	switch timeLayout {
	case "s":
		if ts, err = strconv.ParseInt(field, 10, 64); err != nil {
			return time.Time{}, errors.Wrap(err, "unable parse time in timeformat")
		}
		return time.Unix(ts, 0), nil
	case "ms":
		var ts int64
		if ts, err = strconv.ParseInt(field, 10, 64); err != nil {
			return time.Time{}, errors.Wrap(err, "unable parse time in timeformat")
		}
		return time.Unix(0, ts*int64(time.Millisecond)), nil
	case "us":
		var ts int64
		if ts, err = strconv.ParseInt(field, 10, 64); err != nil {
			return time.Time{}, errors.Wrap(err, "unable parse time in timeformat")
		}
		return time.Unix(0, ts*int64(time.Microsecond)), nil
	case "ns": // "ns"
		var ts int64
		if ts, err = strconv.ParseInt(field, 10, 64); err != nil {
			return time.Time{}, errors.Wrap(err, "unable parse time in timeformat")
		}
		return time.Unix(0, ts), nil
	default:
		return time.ParseInLocation(timeLayout, field, location)
	}
}

// Refer: https://gosamples.dev/date-time-format-cheatsheet/
var _timeformats = map[string]string{
	"-":           "2006-01-02 15:04:05.999",
	"DEFAULT":     "2006-01-02 15:04:05.999",
	"NUMERIC":     "01/02 03:04:05PM '06 -0700", // The reference time, in numerical order.
	"ANSIC":       "Mon Jan _2 15:04:05 2006",
	"UNIX":        "Mon Jan _2 15:04:05 MST 2006",
	"RUBY":        "Mon Jan 02 15:04:05 -0700 2006",
	"RFC822":      "02 Jan 06 15:04 MST",
	"RFC822Z":     "02 Jan 06 15:04 -0700", // RFC822 with numeric zone
	"RFC850":      "Monday, 02-Jan-06 15:04:05 MST",
	"RFC1123":     "Mon, 02 Jan 2006 15:04:05 MST",
	"RFC1123Z":    "Mon, 02 Jan 2006 15:04:05 -0700", // RFC1123 with numeric zone
	"RFC3339":     "2006-01-02T15:04:05Z07:00",
	"RFC3339NANO": "2006-01-02T15:04:05.999999999Z07:00",
	"KITCHEN":     "3:04:05PM",
	"STAMP":       "Jan _2 15:04:05",
	"STAMPMILLI":  "Jan _2 15:04:05.000",
	"STAMPMICRO":  "Jan _2 15:04:05.000000",
	"STAMPNANO":   "Jan _2 15:04:05.000000000",
}

func GetTimeformat(f string) string {
	if m, ok := _timeformats[strings.ToUpper(f)]; ok {
		return m
	}
	return f
}

func HelpTimeformats() string {
	return `    epoch
      ns             nanoseconds
      us             microseconds
      ms             milliseconds
      s              seconds
    abbreviations
      Default,-      2006-01-02 15:04:05.999
      Numeric        01/02 03:04:05PM '06 -0700
      Ansic          Mon Jan _2 15:04:05 2006
      Unix           Mon Jan _2 15:04:05 MST 2006
      Ruby           Mon Jan 02 15:04:05 -0700 2006
      RFC822         02 Jan 06 15:04 MST
      RFC822Z        02 Jan 06 15:04 -0700
      RFC850         Monday, 02-Jan-06 15:04:05 MST
      RFC1123        Mon, 02 Jan 2006 15:04:05 MST
      RFC1123Z       Mon, 02 Jan 2006 15:04:05 -0700
      RFC3339        2006-01-02T15:04:05Z07:00
      RFC3339Nano    2006-01-02T15:04:05.999999999Z07:00
      Kitchen        3:04:05PM
      Stamp          Jan _2 15:04:05
      StampMili      Jan _2 15:04:05.000
      StampMicro     Jan _2 15:04:05.000000
      StampNano      Jan _2 15:04:05.000000000
    custom format
      year           2006
      month          01
      day            02
      hour           03 or 15
      minute         04
      second         05 or with sub-seconds '05.999999'
`
}
