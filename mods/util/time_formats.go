package util

import (
	"strings"
)

func GetTimeformat(f string) string {
	if m, ok := _timeformats[strings.ToUpper(f)]; ok {
		return m
	}
	return f
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
	"S_NS":        "05.999999999",
	"S_US":        "05.999999",
	"S_MS":        "05.999",
	"S.NS":        "05.000000000",
	"S.US":        "05.000000",
	"S.MS":        "05.000",
}

func HelpTimeformats() string {
	return `    epoch
      ns             nanoseconds
      us             microseconds
      ms             milliseconds
      s              seconds
      s_ns           sec.nanoseconds
      s_us           sec.microseconds
      s_ms           sec.milliseconds
      s.ns           sec.nanoseconds (zero padding)
      s.us           sec.microseconds (zero padding)
      s.ms           sec.milliseconds (zero padding)
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
