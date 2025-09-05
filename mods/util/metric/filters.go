package metric

import (
	"path"
)

func AllowNameFilter(of OutputFunc, patterns ...string) OutputFunc {
	return func(p Product) {
		// check if p.Measure matches any pattern
		// if matches, call of
		// else return without calling of
		name := p.Measure + ":" + p.Field
		for _, pattern := range patterns {
			if matched, _ := path.Match(pattern, name); matched {
				of(p)
				return // allow if any pattern matches
			}
		}
	}
}

func DenyNameFilter(of OutputFunc, patterns ...string) OutputFunc {
	return func(p Product) {
		// check if p.Measure matches any pattern
		// if matches, return without calling of
		// else call
		name := p.Measure + ":" + p.Field
		for _, pattern := range patterns {
			if matched, _ := path.Match(pattern, name); matched {
				return // deny if any pattern matches
			}
		}
		of(p)
	}
}
