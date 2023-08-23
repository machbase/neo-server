package model

import (
	"bytes"
	"encoding/json"
	"strconv"
	"strings"
)

type ShellType string

const (
	SHELL_TERM = "term"
)

const (
	SHELLID_SQL   = "SQL"
	SHELLID_TQL   = "TQL"
	SHELLID_WRK   = "WRK"
	SHELLID_TAZ   = "TAZ"
	SHELLID_SHELL = "SHELL"
)

var reservedShellNames = []string{"SQL", "TQL", "WORKSHEET", "TAG ANALYZER", "SHELL",
	/*and more for future uses*/
	"WORKBOOK", "SCRIPT", "RUN", "CMD", "COMMAND", "CONSOLE", "MONITOR", "CHART",
	"DASHBOARD", "LOG", "HOME", "PLAYGROUND", "GRAPH", "FLOW", "DIAGRAM", "PLOT"}

var reservedWebShellDef = map[string]*ShellDefinition{
	SHELLID_SQL: {Type: "sql", Label: "SQL", Icon: convShellIcon("SQL"), Id: SHELLID_SQL},
	SHELLID_TQL: {Type: "tql", Label: "TQL", Icon: convShellIcon("TQL"), Id: SHELLID_TQL},
	SHELLID_WRK: {Type: "wrk", Label: "WORKSHEET", Icon: convShellIcon("WRK"), Id: SHELLID_WRK},
	SHELLID_TAZ: {Type: "taz", Label: "TAG ANALYZER", Icon: convShellIcon("TAZ"), Id: SHELLID_TAZ},
	SHELLID_SHELL: {Type: SHELL_TERM, Label: "SHELL", Icon: convShellIcon("SHELL"), Id: SHELLID_SHELL,
		Attributes: &ShellAttributes{Cloneable: true},
	},
}

func convShellIcon(name string) string {
	switch name {
	case "SQL", "file-document-outline":
		return "VscFile"
	case "TQL", "chart-scatter-plot":
		return "VscGraphScatter"
	case "WRK", "clipboard-text-play-outline":
		return "VscNotebook"
	case "TAZ", "chart-line":
		return "VscGraphLine"
	case "SHELL", "console":
		return "VscTerminal"
	// Custom shell terminals
	case "console-network-outline":
		return "VscWindow"
	case "database", "database-outline":
		return "VscDatabase"
	case "console-line":
		return "VscVm"
	case "powershell":
		return "VscTerminalPowershell"
	default:
		return name
	}
}

type ShellDefinition struct {
	Id         string           `json:"id"`
	Type       string           `json:"type"`
	Icon       string           `json:"icon,omitempty"`
	Label      string           `json:"label"`
	Theme      string           `json:"theme,omitempty"`
	Command    string           `json:"command,omitempty"`
	Attributes *ShellAttributes `json:"attributes,omitempty"`
}

func (def *ShellDefinition) Clone() *ShellDefinition {
	ret := &ShellDefinition{}
	ret.Id = def.Id
	ret.Type = def.Type
	ret.Icon = convShellIcon(def.Icon)
	ret.Label = def.Label
	ret.Theme = def.Theme
	ret.Command = def.Command
	if def.Attributes != nil {
		ret.Attributes = &ShellAttributes{}
		ret.Attributes.Cloneable = def.Attributes.Cloneable
		ret.Attributes.Removable = def.Attributes.Removable
		ret.Attributes.Editable = def.Attributes.Editable
	}
	return ret
}

type ShellProvider interface {
	SetDefaultShellCommand(cmd string)
	GetAllShells(includeWebShells bool) []*ShellDefinition
	GetShell(id string) (found *ShellDefinition, err error)
	CopyShell(id string) (*ShellDefinition, error)
	SaveShell(def *ShellDefinition) error
	RemoveShell(id string) error
}

type ShellAttributes struct {
	Removable bool `json:"removable"`
	Cloneable bool `json:"cloneable"`
	Editable  bool `json:"editable"`
}

func (att *ShellAttributes) MarshalJSON() ([]byte, error) {
	itm := []string{}
	if att.Removable {
		itm = append(itm, `{"removable":true}`)
	}
	if att.Cloneable {
		itm = append(itm, `{"cloneable":true}`)
	}
	if att.Editable {
		itm = append(itm, `{"editable":true}`)
	}
	b := bytes.Buffer{}
	b.WriteString("[")
	b.WriteString(strings.Join(itm, ","))
	b.WriteString("]")
	return b.Bytes(), nil
}

func (att *ShellAttributes) UnmarshalJSON(data []byte) error {
	maps := []map[string]any{}
	err := json.Unmarshal(data, &maps)
	if err != nil {
		return err
	}
	toBool := func(v any) bool {
		switch vv := v.(type) {
		case bool:
			return vv
		case string:
			if b, err := strconv.ParseBool(vv); err != nil {
				return false
			} else {
				return b
			}
		default:
			return false
		}
	}
	for _, m := range maps {
		if v, ok := m["removable"]; ok {
			att.Removable = toBool(v)
		} else if v, ok := m["cloneable"]; ok {
			att.Cloneable = toBool(v)
		} else if v, ok := m["editable"]; ok {
			att.Editable = toBool(v)
		}
	}
	return nil
}
