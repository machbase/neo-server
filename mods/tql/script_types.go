package tql

import "github.com/machbase/neo-server/v8/mods/tql/internal/expression"

type StatementKind int

const (
	StatementUnknown StatementKind = iota
	StatementComment
	StatementPragma
	StatementSource
	StatementMap
	StatementSink
	StatementSourceOrMap
	StatementSourceOrSink
)

type TQLScript struct {
	Source     string
	Statements []*Statement
}

type Statement struct {
	Text      string
	Span      expression.SourceSpan
	Line      int
	IsComment bool
	IsPragma  bool
	Kind      StatementKind
	Name      string
	Expr      *expression.Expression
}

func (s *Statement) IsCode() bool {
	return s != nil && !s.IsComment && !s.IsPragma
}

func (s *Statement) toLine() *Line {
	if s == nil {
		return nil
	}
	return &Line{
		text:      s.Text,
		line:      s.Line,
		isComment: s.IsComment,
		isPragma:  s.IsPragma,
	}
}
