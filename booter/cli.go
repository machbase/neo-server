package booter

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"
)

type CommandLineParser interface {
	Parse() (CommandLineContext, error)
	AddHintBool(long string, short string, negatable bool)
}

func NewCommandLineParser(args []string) CommandLineParser {
	return &clparser{args: args, boolFlags: []string{}}
}

type CommandLineContext interface {
	Flag(long string, short string) *CommandLineFlag
	Args() []string
	Passthrough() []string
}

type CommandLineToken interface {
	commandLineToken()
}

func (cla *CommandLineArg) commandLineToken()  {}
func (clf *CommandLineFlag) commandLineToken() {}
func (cle *CommandLineEnd) commandLineToken()  {}

type CommandLineArg struct {
	value string
}

type CommandLineFlag struct {
	name          string
	value         string
	hasValue      bool
	hasSingleDash bool
}

func (f *CommandLineFlag) String(def string) string {
	if !f.hasValue {
		return def
	}
	return f.value
}

func (f *CommandLineFlag) Int64(def int64) int64 {
	if !f.hasValue {
		return def
	}
	i, e := strconv.ParseInt(f.value, 10, 64)
	if e == nil {
		return i
	}
	return def
}

func (f *CommandLineFlag) Int(def int) int {
	if !f.hasValue {
		return def
	}
	i, e := strconv.ParseInt(f.value, 10, 64)
	if e == nil {
		return int(i)
	}
	return def
}

func (f *CommandLineFlag) Bool(def bool) bool {
	if !f.hasValue {
		return def
	}
	b, e := strconv.ParseBool(f.value)
	if e == nil {
		return b
	}
	return def
}

type CommandLineEnd struct {
	passthrough []string
}

type clparser struct {
	args          []string
	boolFlags     []string
	negativeFlags []string
}

func (parser *clparser) AddHintBool(long string, short string, negatable bool) {
	parser.boolFlags = append(parser.boolFlags, "--"+long)
	parser.boolFlags = append(parser.boolFlags, "-"+short)
	if negatable {
		parser.negativeFlags = append(parser.negativeFlags, fmt.Sprintf("no-%s", long))
	}
}

type clctx struct {
	args        []string
	flags       map[string]*CommandLineFlag
	passthrough []string
}

func (c *clctx) LenFlags() int {
	return len(c.flags)
}

func (c *clctx) Args() []string {
	return c.args
}

func (c *clctx) Passthrough() []string {
	return c.passthrough
}

func (c *clctx) Flag(long string, short string) *CommandLineFlag {
	var v *CommandLineFlag
	if len(long) > 0 {
		v = c.FlagLong(long)
	}
	if v == nil && len(short) > 0 {
		v = c.FlagShort(short)
	}
	return v
}

func (c *clctx) FlagLong(long string) *CommandLineFlag {
	v, ok := c.flags[long]
	if ok && !v.hasSingleDash {
		return v
	}
	return nil
}

func (c *clctx) FlagShort(short string) *CommandLineFlag {
	v, ok := c.flags[short]
	if ok && v.hasSingleDash {
		return v
	}
	return v
}

func (c *clctx) HasFlag(long string, short string) bool {
	clf := c.Flag(long, short)
	return clf != nil
}

func (parser *clparser) Parse() (CommandLineContext, error) {
	ctx := &clctx{
		flags: make(map[string]*CommandLineFlag),
	}

	for {
		tok, err := parser.parseOne()
		if err != nil {
			return ctx, err
		}
		if cle, ok := tok.(*CommandLineEnd); ok {
			ctx.passthrough = cle.passthrough
			break
		}

		if clf, ok := tok.(*CommandLineFlag); ok {
			ctx.flags[clf.name] = clf
		} else if cla, ok := tok.(*CommandLineArg); ok {
			ctx.args = append(ctx.args, cla.value)
		}
	}
	return ctx, nil
}

func (parser *clparser) failf(format string, args ...any) error {
	return fmt.Errorf(format, args...)
}

func (parser *clparser) parseOne() (CommandLineToken, error) {
	if len(parser.args) == 0 {
		return &CommandLineEnd{passthrough: []string{}}, nil
	}
	s := parser.args[0]
	if len(s) < 2 || s[0] != '-' {
		parser.args = parser.args[1:]
		return &CommandLineArg{value: s}, nil
	}
	numMinuses := 1
	if s[1] == '-' {
		numMinuses++
		if len(s) == 2 { // "--" terminates the flags
			return &CommandLineEnd{passthrough: parser.args[1:]}, nil
		}
	}
	name := s[numMinuses:]
	if len(name) == 0 || name[0] == '-' || name[0] == '=' {
		return nil, parser.failf("bad flag syntax: %s", s)
	}

	fv := &CommandLineFlag{name: name}
	if numMinuses == 1 {
		fv.hasSingleDash = true
	}

	// it's a flag. does it have an argument?
	parser.args = parser.args[1:]
	for i := 1; i < len(name); i++ { // equals cannot be first
		if name[i] == '=' {
			fv.value = name[i+1:]
			fv.hasValue = true
			fv.name = name[0:i]
			break
		}
	}

	if !fv.hasValue {
		boolType := false
		whatLookingFor := ""
		if fv.hasSingleDash {
			whatLookingFor = "-" + fv.name
		} else {
			whatLookingFor = "--" + fv.name
		}
		for _, bf := range parser.boolFlags {
			if bf == whatLookingFor {
				boolType = true
				break
			}
		}
		if boolType {
			fv.value = "true"
			fv.hasValue = true
		} else {
			// If it must have a value, which might be the next argument.
			if !fv.hasValue && len(parser.args) > 0 && !strings.HasPrefix(parser.args[0], "-") {
				// value is the next arg
				fv.hasValue = true
				fv.value, parser.args = parser.args[0], parser.args[1:]
			}
		}
	}

	if fv.hasValue {
		for _, nf := range parser.negativeFlags {
			// handle negatable flag
			if nf == fv.name {
				b, err := strconv.ParseBool(fv.value)
				if err != nil {
					b = false
				}
				fv.value = strconv.FormatBool(!b)
				fv.name = strings.TrimPrefix(fv.name, "no-")
				break
			}
		}

		fv.value = StripQuote(fv.value)
	}
	return fv, nil
}

func StripQuote(str string) string {
	if len(str) == 0 {
		return str
	}
	c := []rune(str)[0]
	if unicode.In(c, unicode.Quotation_Mark) {
		return strings.Trim(str, string(c))
	}
	return str
}
