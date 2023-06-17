package fcom

import "github.com/machbase/neo-server/mods/expression"

var Functions = map[string]expression.Function{
	"len":       to_len,    // len( array| string)
	"count":     count,     // count(V)
	"round":     round,     // round(number, number)
	"time":      to_time,   // time(ts [, delta])
	"roundTime": roundTime, // roundTime(time, duration)
	"element":   element,   // element(list, idx)
	// math
	"sin":   sin,
	"cos":   cos,
	"tan":   tan,
	"exp":   exp,
	"exp2":  exp2,
	"log":   log,
	"log10": log10,
	//
	"linspace": linspace,
	"meshgrid": meshgrid,
}
