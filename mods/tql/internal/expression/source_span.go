package expression

type SourcePosition struct {
	Offset int
	Line   int
	Column int
}

type SourceSpan struct {
	Start SourcePosition
	End   SourcePosition
}

func (sp SourceSpan) IsZero() bool {
	return sp.Start == (SourcePosition{}) && sp.End == (SourcePosition{})
}

func (sp SourceSpan) RawFrom(source []rune) string {
	start := sp.Start.Offset
	end := sp.End.Offset
	if start < 0 {
		start = 0
	}
	if end < start {
		end = start
	}
	if end > len(source) {
		end = len(source)
	}
	return string(source[start:end])
}
