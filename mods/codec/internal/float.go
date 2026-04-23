package internal

import (
	"math"
	"strconv"
)

func AppendPrecisionFloat64(dst []byte, value float64, precision int, quoteSpecial bool) []byte {
	switch {
	case math.IsNaN(value):
		return appendSpecialFloatToken(dst, "NaN", quoteSpecial)
	case math.IsInf(value, -1):
		return appendSpecialFloatToken(dst, "-Inf", quoteSpecial)
	case math.IsInf(value, 1):
		return appendSpecialFloatToken(dst, "+Inf", quoteSpecial)
	case value == 0:
		if precision >= 0 {
			return strconv.AppendFloat(dst, 0, 'f', precision, 64)
		}
		return append(dst, '0')
	}

	prec := 6
	if precision >= 0 {
		prec = precision
	}

	r := strconv.AppendFloat(dst, value, 'f', prec, 64)
	if precision < 0 {
		for len(r) > 0 && r[len(r)-1] == '0' {
			r = r[:len(r)-1]
		}
		if len(r) > 0 && r[len(r)-1] == '.' {
			r = r[:len(r)-1]
		}
	}
	return r
}

func FormatPrecisionFloat64(value float64, precision int, quoteSpecial bool) string {
	return string(AppendPrecisionFloat64(nil, value, precision, quoteSpecial))
}

func appendSpecialFloatToken(dst []byte, token string, quote bool) []byte {
	if !quote {
		return append(dst, token...)
	}
	dst = append(dst, '"')
	dst = append(dst, token...)
	return append(dst, '"')
}
