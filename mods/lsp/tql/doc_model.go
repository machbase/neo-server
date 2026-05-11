package tql

type tqlDocInfo struct {
	Label       string
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
