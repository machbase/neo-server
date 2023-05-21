package util

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

type TagPath struct {
	Table string
	Tag   string
	Term  TagPathTerm
}

type TagPathTerm struct {
	Column string
	Func   string
	Args   []TagPathTerm
}

func (term *TagPathTerm) IsEqual(other *TagPathTerm) bool {
	if term.Column != other.Column {
		fmt.Printf("column %s %s\n", term.Column, other.Column)
		return false
	}
	if term.Func != other.Func {
		fmt.Printf("fybc %s %s\n", term.Func, other.Func)
		return false
	}
	if len(term.Args) != len(other.Args) {
		fmt.Printf("argslen\n\t%s\n\t%s\n", term.Args, other.Args)
		return false
	}
	for i, a := range term.Args {
		if !a.IsEqual(&other.Args[i]) {
			return false
		}
	}
	return true
}

var regexpTagPath = regexp.MustCompile(`([a-zA-Z0-9_-]+)\/(.+)`)

// parse
// "<table>/<tag>"
// "<table>/<tag>#column"
// "<table>/<tag>#function()"
// "<table>/<tag>#function(<column>)"
// "<table>/<tag>#func2(func1())"
// "<table>/<tag>#func2(func1(<column>))"
func ParseTagPath(path string) (*TagPath, error) {
	toks := regexpTagPath.FindAllStringSubmatch(path, -1)
	// fmt.Println("PATH", path)
	// for i := range toks {
	// 	for n := range toks[i] {
	// 		fmt.Printf("  toks[%d][%d] %s\n", i, n, toks[i][n])
	// 	}
	// }
	if len(toks) != 1 || len(toks[0]) < 3 {
		return nil, errors.New("invalid syntax")
	}
	r := &TagPath{}
	r.Table = strings.ToUpper(strings.TrimSpace(toks[0][1]))
	termParts := strings.SplitN(toks[0][2], "#", 2)
	if len(termParts) == 1 {
		r.Tag = termParts[0]
		r.Term = TagPathTerm{Column: "VALUE"}
		return r, nil
	}

	r.Tag = termParts[0]
	args := parseTerm(termParts[1])
	if len(args) == 1 {
		r.Term = args[0]
	}

	return r, nil
}

var regexpTagPathTerm = regexp.MustCompile(`([a-zA-Z0-9_]+)\s*\((.+)\)`)

func parseTerm(part string) []TagPathTerm {
	toks := regexpTagPathTerm.FindAllStringSubmatch(part, -1)
	if len(toks) != 1 {
		argToks := strings.Split(part, ",")
		if len(argToks) < 2 {
			return []TagPathTerm{{Column: strings.ToUpper(strings.TrimSpace(argToks[0]))}}
		} else {
			arr := []TagPathTerm{{Column: strings.ToUpper(strings.TrimSpace(argToks[0]))}}
			for _, a := range argToks[1:] {
				r := parseTerm(a)
				if len(r) == 0 {
					continue
				}
				arr = append(arr, r[0])
			}
			return arr
		}
	}

	if len(toks[0]) == 3 {
		fname := toks[0][1]
		args := parseTerm(toks[0][2])
		return []TagPathTerm{{Func: strings.ToUpper(strings.TrimSpace(fname)), Args: args}}
	}
	return []TagPathTerm{}
}

type WritePath struct {
	Table     string
	Format    string
	Transform string
	Compress  string
}

// parse
// "<table>"                      default format is "json"
// "<table>:<format>"             format "json", "csv"
// "<table>:<format>:<compress>"  transformer "gzip" or "-", "" for no-compression
// "<table>:<format>:<transform>:<compress>" transformer "gzip" or "-", "" for no-compression
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
	case 4:
		r.Table = strings.ToUpper(strings.TrimSpace(toks[0]))
		r.Format = strings.ToLower(strings.TrimSpace(toks[1]))
		r.Transform = strings.ToLower(strings.TrimSpace(toks[2]))
		r.Compress = strings.ToLower(strings.TrimSpace(toks[3]))
	}
	return r, nil
}
