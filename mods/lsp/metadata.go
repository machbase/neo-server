package lsp

type Metadata struct {
	Language Language      `json:"language"`
	Version  string        `json:"version"`
	Keywords []KeywordInfo `json:"keywords"`
	Symbols  []SymbolInfo  `json:"symbols"`
	Modules  []ModuleInfo  `json:"modules,omitempty"`
}

type KeywordInfo struct {
	Label         string `json:"label"`
	Category      string `json:"category,omitempty"`
	Detail        string `json:"detail,omitempty"`
	Documentation string `json:"documentation,omitempty"`
}

type SymbolInfo struct {
	Label         string             `json:"label"`
	Kind          CompletionItemKind `json:"kind"`
	Category      string             `json:"category,omitempty"`
	Detail        string             `json:"detail,omitempty"`
	Documentation string             `json:"documentation,omitempty"`
	InsertText    string             `json:"insertText,omitempty"`
	StatementKind string             `json:"statementKind,omitempty"`
	Signature     *SignatureInfo     `json:"signature,omitempty"`
	Deprecated    bool               `json:"deprecated,omitempty"`
}

type ModuleInfo struct {
	ID            string       `json:"id"`
	Detail        string       `json:"detail,omitempty"`
	Documentation string       `json:"documentation,omitempty"`
	Exports       []SymbolInfo `json:"exports,omitempty"`
}

type SignatureInfo struct {
	Label         string          `json:"label"`
	Documentation string          `json:"documentation,omitempty"`
	Parameters    []ParameterInfo `json:"parameters,omitempty"`
}

type ParameterInfo struct {
	Label         string `json:"label"`
	Documentation string `json:"documentation,omitempty"`
}
