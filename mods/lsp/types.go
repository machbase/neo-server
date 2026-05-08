package lsp

import "context"

type Language string

const (
	LanguageSQL Language = "sql"
	LanguageTQL Language = "tql"
	LanguageJSH Language = "jsh"
)

type Document struct {
	URI      string   `json:"uri"`
	Language Language `json:"language"`
	Text     string   `json:"text"`
}

type Position struct {
	Line   int `json:"line"`
	Column int `json:"column"`
}

type Range struct {
	Start Position `json:"start"`
	End   Position `json:"end"`
}

type DiagnosticSeverity int

const (
	SeverityError DiagnosticSeverity = iota + 1
	SeverityWarning
	SeverityInformation
	SeverityHint
)

type Diagnostic struct {
	Range    Range              `json:"range"`
	Severity DiagnosticSeverity `json:"severity"`
	Code     string             `json:"code,omitempty"`
	Source   string             `json:"source,omitempty"`
	Message  string             `json:"message"`
}

type CompletionItemKind int

const (
	CompletionText CompletionItemKind = iota + 1
	CompletionMethod
	CompletionFunction
	CompletionConstructor
	CompletionField
	CompletionVariable
	CompletionClass
	CompletionInterface
	CompletionModule
	CompletionProperty
	CompletionUnit
	CompletionValue
	CompletionEnum
	CompletionKeyword
	CompletionSnippet
)

type CompletionItem struct {
	Label         string             `json:"label"`
	Kind          CompletionItemKind `json:"kind"`
	Detail        string             `json:"detail,omitempty"`
	Documentation string             `json:"documentation,omitempty"`
	InsertText    string             `json:"insertText,omitempty"`
}

type Hover struct {
	Range    Range  `json:"range"`
	Contents string `json:"contents"`
}

type SignatureHelp struct {
	Signatures      []SignatureInfo `json:"signatures"`
	ActiveSignature int             `json:"activeSignature"`
	ActiveParameter int             `json:"activeParameter"`
}

type LanguageService interface {
	Diagnostics(ctx context.Context, doc Document) ([]Diagnostic, error)
	Completion(ctx context.Context, doc Document, pos Position) ([]CompletionItem, error)
	Hover(ctx context.Context, doc Document, pos Position) (*Hover, error)
	SignatureHelp(ctx context.Context, doc Document, pos Position) (*SignatureHelp, error)
}
