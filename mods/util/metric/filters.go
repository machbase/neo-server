package metric

import (
	"path"
	"regexp"
	"strconv"
	"strings"
)

type Filter interface {
	Match(string) bool
}

func IsFilterPattern(s string) bool {
	return strings.ContainsAny(s, "*?[]")
}

// Compile compiles a list of glob patterns into a Filter.
//
// f, _ := Compile([]string{"abc", "def", "ghi*"})
// f.Match("abc") => true
// f.Match("def") => true
// f.Match("ghibelline") => true
// f.Match("defy") => false
//
// separators are only used for glob patterns
//
// f, _ := Compile([]string{"abc:*:def"}, ':')
// f.Match("abc:def") => false
// f.Match("abc:xyz:def") => true
// f.Match("abc:opq:xyz:ghi") => false
//
// if the patterns contains brackets with digits, it can be used to match range of numbers
// e.g. "metric:name[0-3]" matches "metric:name0", "metric:name1", "metric:name2", "metric:name3"
//
//	"metric:name[1-3]" matches "metric:name1", "metric:name2", "metric:name3"
//	"metric:name[2-4]" matches "metric:name2", "metric:name3", "metric:name4"
func Compile(filters []string, separators ...rune) (Filter, error) {
	if len(filters) == 0 {
		return nil, nil
	}

	sep := byte(':')
	if len(separators) > 0 {
		sep = byte(separators[0])
	}

	var compiled []compiledPattern
	for _, pat := range filters {
		p := pat
		if sep != ':' {
			p = replaceSeparators(p, sep)
		}
		cp, err := compilePattern(p)
		if err != nil {
			return nil, err
		}
		compiled = append(compiled, cp)
	}

	return &filterList{
		patterns:  compiled,
		separator: sep,
	}, nil
}

func MustCompile(filters []string, separators ...rune) Filter {
	f, err := Compile(filters, separators...)
	if err != nil {
		panic(err)
	}
	return f
}

type compiledPattern struct {
	glob     string
	regex    *regexp.Regexp
	hasRange bool
}

func compilePattern(pattern string) (compiledPattern, error) {
	re := regexp.MustCompile(`\[(\d+)-(\d+)\]`)
	matches := re.FindAllStringSubmatchIndex(pattern, -1)
	if len(matches) == 0 {
		return compiledPattern{glob: pattern}, nil
	}

	var regexPattern strings.Builder
	last := 0
	for _, m := range matches {
		// add text before the range
		regexPattern.WriteString(regexp.QuoteMeta(pattern[last:m[0]]))
		start, _ := strconv.Atoi(pattern[m[2]:m[3]])
		end, _ := strconv.Atoi(pattern[m[4]:m[5]])
		regexPattern.WriteString("(")
		for i := start; i <= end; i++ {
			if i > start {
				regexPattern.WriteString("|")
			}
			regexPattern.WriteString(strconv.Itoa(i))
		}
		regexPattern.WriteString(")")
		last = m[1]
	}
	// add remaining text after the last range
	regexPattern.WriteString(regexp.QuoteMeta(pattern[last:]))

	// transform glob wildcards to regex
	regexStr := regexPattern.String()
	regexStr = strings.ReplaceAll(regexStr, `\*`, ".*")
	regexStr = strings.ReplaceAll(regexStr, `\?`, ".")
	regexStr = "^" + regexStr + "$"

	r, err := regexp.Compile(regexStr)
	if err != nil {
		return compiledPattern{}, err
	}
	return compiledPattern{regex: r, hasRange: true}, nil
}

type filterList struct {
	patterns  []compiledPattern
	separator byte
}

func (f *filterList) Match(s string) bool {
	normalized := s
	if f.separator != ':' {
		normalized = replaceSeparators(s, f.separator)
	}
	for _, cp := range f.patterns {
		if cp.hasRange {
			if cp.regex.MatchString(normalized) {
				return true
			}
		} else {
			// also try replacing path separators with underscores
			// e.g. disk:/mnt/c:used_percent => disk:_mnt_c:used_percent
			// so that pattern disk:*:used_percent can match
			normalized = strings.Map(func(r rune) rune {
				switch r {
				case '/':
					return '_'
				default:
					return r
				}
			}, normalized)
			if matched, _ := path.Match(cp.glob, normalized); matched {
				return true
			}
		}
	}
	return false
}

func replaceSeparators(s string, sep byte) string {
	// replace ':' with sep
	if sep == ':' {
		return s
	}
	var result string
	for i := 0; i < len(s); i++ {
		if s[i] == ':' {
			result += string(sep)
		} else {
			result += string(s[i])
		}
	}
	return result
}

func IncludeNames(of OutputFunc, patterns ...string) OutputFunc {
	filter, _ := Compile(patterns, ':')
	return func(p Product) error {
		// check if p.Measure matches any pattern
		// if matches, call of
		// else return without calling of
		if filter != nil && filter.Match(p.Name) {
			of(p)
		}
		return nil
	}
}

func ExcludeNames(of OutputFunc, patterns ...string) OutputFunc {
	filter, _ := Compile(patterns, ':')
	return func(p Product) error {
		// check if p.Measure matches any pattern
		// if matches, return without calling of
		// else call
		if filter != nil && filter.Match(p.Name) {
			return nil // deny if any pattern matches
		}
		return of(p)
	}
}

func AndFilter(a Filter, b Filter) Filter {
	if a == nil {
		return b
	}
	if b == nil {
		return a
	}
	return &andFilter{
		aFilter: a,
		bFilter: b,
	}
}

type andFilter struct {
	aFilter Filter
	bFilter Filter
}

func (af *andFilter) Match(s string) bool {
	return af.aFilter.Match(s) && af.bFilter.Match(s)
}

func OrFilter(a Filter, b Filter) Filter {
	if a == nil {
		return b
	}
	if b == nil {
		return a
	}
	return &orFilter{
		aFilter: a,
		bFilter: b,
	}
}

type orFilter struct {
	aFilter Filter
	bFilter Filter
}

func (of *orFilter) Match(s string) bool {
	return of.aFilter.Match(s) || of.bFilter.Match(s)
}

type IncludeAndExclude struct {
	includeFilter Filter
	excludeFilter Filter
}

func CompileIncludeAndExclude(includes []string, excludes []string, separators ...rune) (Filter, error) {
	var ret = &IncludeAndExclude{}
	var errs []error
	if len(includes) > 0 {
		if filter, err := Compile(includes, separators...); err != nil {
			errs = append(errs, err)
		} else {
			ret.includeFilter = filter
		}
	}
	if len(excludes) > 0 {
		if filter, err := Compile(excludes); err != nil {
			errs = append(errs, err)
		} else {
			ret.excludeFilter = filter
		}
	}
	if len(errs) > 0 {
		return nil, MultipleError(errs)
	}
	return ret, nil
}

func (iae *IncludeAndExclude) Match(s string) bool {
	if iae.includeFilter == nil && iae.excludeFilter == nil {
		return true
	}
	if iae.excludeFilter == nil {
		// only include is set
		return iae.includeFilter.Match(s)
	} else if iae.includeFilter == nil {
		// only exclude is set
		return !iae.excludeFilter.Match(s)
	} else {
		// include and exclude are both set
		if !iae.includeFilter.Match(s) {
			return false
		}
		if iae.excludeFilter.Match(s) {
			return false
		}
		return true
	}
}
