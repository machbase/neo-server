package logging

import (
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

func (l *levelLogger) _log(lvl Level, callstackOffset int, args []any) {
	l._logf(lvl, callstackOffset, "", args)
}

func (l *levelLogger) _logf(lvl Level, callstackOffset int, format string, args []any) {
	if lvl < l.level {
		return
	}

	totalCounter.Inc(1)
	if lvl == LevelWarn {
		warnCounter.Inc(1)
	} else if lvl == LevelError {
		errorCounter.Inc(1)
	}

	name := l.name
	var srcFileName string
	var srcFileLine int
	if l.enableSrcLoc {
		_, srcFileName, srcFileLine, _ = runtime.Caller(2 + callstackOffset)
		srcFileName = filepath.Base(srcFileName)
		width := (l.prefixWidth - len(srcFileName) - 5)
		if width <= 0 {
			width = 1
		}
		nameForm := fmt.Sprintf("%%-%ds %%s %%3d", width)
		name = fmt.Sprintf(nameForm, name, srcFileName, srcFileLine)
	} else {
		nameForm := fmt.Sprintf("%%-%ds", l.prefixWidth)
		name = fmt.Sprintf(nameForm, l.name)
	}

	levelColorBegin, levelColorEnd := "", ""
	if lvl == LevelWarn {
		levelColorBegin, levelColorEnd = yellow, reset
	} else if lvl == LevelError {
		levelColorBegin, levelColorEnd = red, reset
	}

	timestamp := time.Now()

	levelWithPname := fmt.Sprintf("%-5s", logLevelNames[lvl])

	for _, w := range l.underlying {
		var fnew string
		var forg = format
		if format == "" {
			forg = "%s"
		}
		if w.isTerm {
			fnew = fmt.Sprintf("%v %s%s%s %s %s\n",
				timestamp.Format("2006/01/02 15:04:05.000"),
				levelColorBegin, levelWithPname, levelColorEnd,
				name, forg)
		} else {
			fnew = fmt.Sprintf("%v %s %s %s\n",
				timestamp.Format("2006/01/02 15:04:05.000"),
				levelWithPname, name, forg)
		}
		line := ""
		if format == "" {
			toks := make([]string, len(args)+1)
			for i, a := range args {
				if s, ok := a.(string); ok {
					toks[i] = s
				} else {
					toks[i] = fmt.Sprintf("%v", a)
				}
			}
			line = fmt.Sprintf(fnew, strings.Join(toks, " "))
		} else {
			line = fmt.Sprintf(fnew, args...)
		}
		if w.isTerm {
			w.Write([]byte(line))
		} else {
			w.Write([]byte(removeEscape(line)))
		}
	}
}

func removeEscape(str string) string {
	for {
		idx := strings.Index(str, "\033[")
		if idx == -1 {
			break
		}
		period := strings.Index(str[idx:], "m")
		str = str[0:idx] + str[idx+period+1:]
	}
	return str
}
