package ymds

import (
	"fmt"
	"strconv"
	"time"
)

type Match interface {
	Match([]rune) ([]rune, int64, bool)
}

var _ Match = &mText{}
var _ Match = &mDigit{}
var _ Match = &mYYYY{}
var _ Match = &mMM{}
var _ Match = &mDD{}

type mText struct {
	runes []rune
}

func (m *mText) Match(input []rune) ([]rune, int64, bool) {
	if len(input) < len(m.runes) {
		return input, 0, false
	}
	for i, r := range m.runes {
		if input[i] != r {
			return input, 0, false
		}
	}
	return input[len(m.runes):], 0, true
}

func (m *mText) String() string {
	return fmt.Sprintf("mText(%q)", string(m.runes))
}

type mYYYY struct{}

func (m *mYYYY) Match(input []rune) ([]rune, int64, bool) {
	if len(input) < 4 {
		return input, 0, false
	}
	part := input[0:4]
	n, err := strconv.ParseInt(string(part), 10, 64)
	if err != nil {
		return input, 0, false
	}
	return input[4:], n, true
}

type mMM struct{}

func (m *mMM) Match(input []rune) ([]rune, int64, bool) {
	if len(input) < 2 {
		return input, 0, false
	}
	part := input[0:2]
	n, err := strconv.ParseInt(string(part), 10, 64)
	if err != nil {
		return input, 0, false
	}
	return input[2:], n, true
}

type mDD struct{}

func (m *mDD) Match(input []rune) ([]rune, int64, bool) {
	if len(input) < 2 {
		return input, 0, false
	}
	part := input[0:2]
	n, err := strconv.ParseInt(string(part), 10, 64)
	if err != nil {
		return input, 0, false
	}
	return input[2:], n, true
}

type mDigit struct {
	length   int
	multiply int64
}

func (d *mDigit) Match(input []rune) ([]rune, int64, bool) {
	if len(input) < d.length {
		return input, 0, false
	}
	part := input[0:d.length]
	n, err := strconv.ParseInt(string(part), 10, 64)
	if err != nil {
		return input, 0, false
	}
	return input[d.length:], n * d.multiply, true
}

func peek(rs []rune, idx int, expect []rune) (int, bool) {
	if len(rs)-idx < len(expect) {
		return idx, false
	}
	for i, r := range expect {
		if rs[idx+i] != r {
			return idx, false
		}
	}
	return idx + len(expect) - 1, true
}

var pYYYY = []rune("YYYY")
var pMM = []rune("MM")
var pDD = []rune("DD")
var pHH24 = []rune("HH24")
var pHH = []rune("HH")
var pMI = []rune("MI")
var pSS = []rune("SS")
var pmmm = []rune("mmm")
var puuu = []rune("uuu")
var pnnn = []rune("nnn")

type Parser struct {
	layout   string
	matches  []Match
	location *time.Location

	debug   bool
	remains *mText
}

func NewParser(layout string) *Parser {
	ret := &Parser{
		layout:   layout,
		location: time.Local,
	}
	rs := []rune(layout)
	peekOk := false

	for idx := 0; idx < len(rs); idx++ {
		if idx, peekOk = peek(rs, idx, pYYYY); peekOk {
			ret.append(&mYYYY{})
		} else if idx, peekOk = peek(rs, idx, pMM); peekOk {
			ret.append(&mMM{})
		} else if idx, peekOk = peek(rs, idx, pDD); peekOk {
			ret.append(&mDD{})
		} else if idx, peekOk = peek(rs, idx, pHH24); peekOk {
			ret.append(&mDigit{length: 2, multiply: 36_00_000_000_000})
		} else if idx, peekOk = peek(rs, idx, pHH); peekOk {
			ret.append(&mDigit{length: 2, multiply: 36_00_000_000_000})
		} else if idx, peekOk = peek(rs, idx, pMI); peekOk {
			ret.append(&mDigit{length: 2, multiply: 60_000_000_000})
		} else if idx, peekOk = peek(rs, idx, pSS); peekOk {
			ret.append(&mDigit{length: 2, multiply: 1000_000_000})
		} else if idx, peekOk = peek(rs, idx, pmmm); peekOk {
			ret.append(&mDigit{length: 3, multiply: 1000_000})
		} else if idx, peekOk = peek(rs, idx, puuu); peekOk {
			ret.append(&mDigit{length: 3, multiply: 1000})
		} else if idx, peekOk = peek(rs, idx, pnnn); peekOk {
			ret.append(&mDigit{length: 3, multiply: 1})
		} else {
			ret.remain(rs[idx])
		}
	}
	return ret
}

func (p *Parser) WithLocation(tz *time.Location) *Parser {
	p.location = tz
	return p
}

func (p *Parser) WithDebug() *Parser {
	p.debug = true
	return p
}

func (p *Parser) append(m Match) {
	if p.remains != nil {
		p.matches = append(p.matches, p.remains)
		p.remains = nil
	}
	p.matches = append(p.matches, m)
}

func (p *Parser) remain(r rune) {
	if p.remains == nil {
		p.remains = &mText{}
	}
	p.remains.runes = append(p.remains.runes, r)
}

func (p *Parser) Parse(str string) (time.Time, error) {
	input := []rune(str)
	var tick int64
	var year int
	var month int
	var day int

	for _, m := range p.matches {
		if renew, amount, ok := m.Match(input); ok {
			switch m.(type) {
			case *mYYYY:
				year = int(amount)
			case *mMM:
				month = int(amount)
			case *mDD:
				day = int(amount)
			default:
				tick += amount
			}
			if p.debug {
				fmt.Printf("  match %*T %*s => %-*s  %d/%d/%d %d\n", 16, m, 30, string(input), 30, string(renew), year, month, day, tick)
			}
			input = renew
		} else {
			return time.Time{}, fmt.Errorf("time parse fail (%v), remains:%q", m, string(input))
		}
	}

	sec := int(tick / int64(time.Second))
	nsec := int(tick % int64(time.Second))

	ret := time.Date(year, time.Month(month), day, 0, 0, sec, nsec, p.location)

	return ret, nil
}
