package model

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

type ShellDefinition struct {
	Name string   `json:"-"`
	Args []string `json:"args"`

	Attributes map[string]string `json:"attributes,omitempty"`
}

func (def *ShellDefinition) Clone() *ShellDefinition {
	fmt.Println("---1")
	ret := &ShellDefinition{}
	ret.Name = def.Name
	fmt.Println("---2")
	copy(ret.Args, def.Args)
	fmt.Println("---3")
	if len(def.Attributes) > 0 {
		ret.Attributes = map[string]string{}
	}
	fmt.Println("---4")
	for k, v := range def.Attributes {
		ret.Attributes[k] = v
	}
	fmt.Println("---5")
	return ret
}

type WebShellProvider interface {
	GetAllWebShells() []*WebShell
	GetWebShell(id string) (*WebShell, error)
	CopyWebShell(id string) (*WebShell, error)
	RemoveWebShell(id string) error
	UpdateWebShell(s *WebShell) error
}

type WebShell struct {
	Id         string              `json:"id"`
	Type       string              `json:"type"`
	Icon       string              `json:"icon,omitempty"`
	Label      string              `json:"label"`
	Content    string              `json:"content,omitempty"`
	Theme      string              `json:"theme,omitempty"`
	Attributes *WebShellAttributes `json:"attributes,omitempty"`
}

type WebShellAttributes struct {
	Removable bool `json:"removable"`
	Cloneable bool `json:"cloneable"`
	Editable  bool `json:"editable"`
}

func (att *WebShellAttributes) MarshalJSON() ([]byte, error) {
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

func (att *WebShellAttributes) UnmarshalJSON(data []byte) error {
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
