package action

import (
	"fmt"
	"sort"
	"strings"
)

var sqlCommands = []string{
	"select", "insert", "update", "delete", "alter",
	"create", "drop", "truncate", "exec",
	"mount", "umount", "backup",
}

var globalCommands = make(map[string]*Cmd)
var globalHelps = make(map[string]*CmdSpec)

func RegisterCmd(cmd *Cmd) error {
	name := strings.ToLower(cmd.Name)
	for _, c := range sqlCommands {
		if name == c {
			return fmt.Errorf("command %q can not be override", name)
		}
	}
	globalCommands[name] = cmd
	RegisterHelp(name, cmd.Spec)
	return nil
}

func RegisterHelp(name string, spec *CmdSpec) {
	globalHelps[name] = spec
}

func IsSqlCommand(cmd string) bool {
	cmd = strings.ToLower(cmd)
	for _, c := range sqlCommands {
		if c == cmd {
			return true
		}
	}
	return false
}

func FindCmd(name string) *Cmd {
	return globalCommands[name]
}

func Commands() []*Cmd {
	list := []*Cmd{}
	for _, v := range globalCommands {
		list = append(list, v)
	}
	sort.Slice(list, func(i, j int) bool {
		return list[i].Name <= list[j].Name
	})
	return list
}

func BuildPrefixCompleter() PrefixCompleterInterface {
	commands := []*Cmd{}
	for _, cmd := range globalCommands {
		commands = append(commands, cmd)
	}
	sort.Slice(commands, func(i, j int) bool {
		return commands[i].Name < commands[j].Name
	})
	pc := make([]PrefixCompleterInterface, 0)
	for _, cmd := range commands {
		if cmd.PcFunc != nil {
			pc = append(pc, cmd.PcFunc())
		}
	}
	return NewPrefixCompleter(pc...)
}

func AppendHistory(text string) {

}
