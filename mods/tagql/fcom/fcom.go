package fcom

import "github.com/machbase/neo-server/mods/expression"

var Functions = map[string]expression.Function{
	"len":       to_len,    // len( array| string)
	"round":     round,     // round(number, number)
	"time":      to_time,   // time(ts [, delta])
	"roundTime": roundTime, // roundTime(time, duration)
	"element":   element,   // element(list, idx)
}
