package util

import (
	"strconv"

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

func NumberFormat[T int | uint | int8 | uint8 | int16 | uint16 | int32 | uint32 | int64 | uint64](n T) string {
	in := strconv.FormatInt(int64(n), 10)
	numOfDigits := len(in)
	if n < 0 {
		numOfDigits-- // First character is the - sign (not a digit)
	}
	numOfCommas := (numOfDigits - 1) / 3

	out := make([]byte, len(in)+numOfCommas)
	if n < 0 {
		in, out[0] = in[1:], '-'
	}

	for i, j, k := len(in)-1, len(out)-1, 0; ; i, j = i-1, j-1 {
		out[j] = in[i]
		if i == 0 {
			return string(out)
		}
		if k++; k == 3 {
			j, k = j-1, 0
			out[j] = ','
		}
	}
}
