package sshd

type ShellDefinition struct {
	Name string   `json:"-"`
	Args []string `json:"args"`
}
