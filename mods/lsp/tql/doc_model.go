package tql

type tqlDocInfo struct {
	Label       string
	Draft       bool
	Kind        string
	Category    string
	Signatures  []tqlDocSignature
	Slots       []tqlDocSlot
	Description string
	Markdown    string
	Related     []string
	Roles       map[string]tqlDocVariant
}

type tqlDocVariant struct {
	Role        string
	Kind        string
	Category    string
	Signatures  []tqlDocSignature
	Slots       []tqlDocSlot
	Description string
	Markdown    string
	Related     []string
}

type tqlDocSignature struct {
	Label      string
	Parameters []string
}

type tqlDocSlot struct {
	Name        string
	Required    bool
	Repeat      bool
	Accepts     string
	Suggestions []string
}
