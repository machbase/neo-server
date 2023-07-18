package model

import (
	"bytes"
	"encoding/json"
	"strconv"
	"strings"
)

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
	ret.Icon = def.Icon
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
	GetAllWebShells() []*ShellDefinition
	GetWebShell(id string) (*ShellDefinition, error)
	CopyWebShell(id string) (*ShellDefinition, error)
	RemoveWebShell(id string) error
	UpdateWebShell(s *ShellDefinition) error
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
