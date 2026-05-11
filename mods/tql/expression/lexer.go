package expression

import "unicode"

type lexerStream struct {
	source    []rune
	position  int
	length    int
	positions []SourcePosition
}

func newLexerStream(source string) *lexerStream {
	ret := &lexerStream{}
	ret.source = []rune(source)
	ret.length = len(ret.source)
	ret.positions = make([]SourcePosition, ret.length+1)

	line := 1
	column := 1
	for i, r := range ret.source {
		ret.positions[i] = SourcePosition{
			Offset: i,
			Line:   line,
			Column: column,
		}
		if r == '\n' {
			line++
			column = 1
		} else {
			column++
		}
	}
	ret.positions[ret.length] = SourcePosition{
		Offset: ret.length,
		Line:   line,
		Column: column,
	}
	return ret
}

func (ls *lexerStream) readCharacter() rune {
	r := ls.source[ls.position]
	ls.position++
	return r
}

func (ls *lexerStream) rewind(amount int) {
	ls.position -= amount
	if ls.position < 0 {
		ls.position = 0
	}
}

func (ls *lexerStream) canRead() bool {
	return ls.position < ls.length
}

func (ls *lexerStream) span(start, end int) SourceSpan {
	if start < 0 {
		start = 0
	}
	if end < start {
		end = start
	}
	if end > ls.length {
		end = ls.length
	}
	return SourceSpan{
		Start: ls.positions[start],
		End:   ls.positions[end],
	}
}

func (ls *lexerStream) currentSpan(start int) SourceSpan {
	return ls.span(start, ls.tokenEnd())
}

func (ls *lexerStream) raw(start, end int) string {
	return ls.span(start, end).RawFrom(ls.source)
}

func (ls *lexerStream) currentRaw(start int) string {
	return ls.raw(start, ls.tokenEnd())
}

func (ls *lexerStream) tokenEnd() int {
	end := ls.position
	for end > 0 && end <= ls.length && unicode.IsSpace(ls.source[end-1]) {
		end--
	}
	return end
}
