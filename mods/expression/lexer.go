package expression

type lexerStream struct {
	source   []rune
	position int
	length   int
}

func newLexerStream(source string) *lexerStream {
	ret := &lexerStream{}
	ret.source = []rune(source)
	ret.length = len(ret.source)
	return ret
}

func (ls *lexerStream) readCharacter() rune {
	if len(ls.source) <= ls.position {
		return '\n'
	}
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
