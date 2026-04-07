package eventbus

import "encoding/json"

type Message struct {
	// ver currently "1.0"
	Ver string
	// id that assigned by sender
	// the replies should use the same ID
	ID int64
	// Type is body type
	Type BodyType
	// Payload is message body defined by application
	Body *BodyUnion
}

func (m Message) MarshalJSON() ([]byte, error) {
	obj := map[string]any{
		"ver":  m.Ver,
		"id":   m.ID,
		"type": m.Type,
		"body": m.Body.asAny(m.Type),
	}
	return json.Marshal(obj)
}

func (m *Message) UnmarshalJSON(data []byte) error {
	var obj struct {
		Ver  string          `json:"ver"`
		ID   int64           `json:"id"`
		Type BodyType        `json:"type"`
		Body json.RawMessage `json:"body"`
	}
	if err := json.Unmarshal(data, &obj); err != nil {
		return err
	}
	m.Ver = obj.Ver
	m.ID = obj.ID
	m.Type = obj.Type
	if len(obj.Body) > 0 {
		m.Body = &BodyUnion{}
		if err := m.Body.unmarshal(m.Type, obj.Body); err != nil {
			return err
		}
	}
	return nil
}

type BodyType string

const (
	BodyTypeCommand BodyType = "command"
	BodyTypeInput   BodyType = "input"
)

type BodyUnion struct {
	OfCommand *Command
	OfInput   *Input
}

func (bu *BodyUnion) asAny(typ BodyType) any {
	if bu == nil {
		return nil
	}
	switch typ {
	case BodyTypeInput:
		return bu.OfInput
	case BodyTypeCommand:
		return bu.OfCommand
	}
	return nil
}

func (bu *BodyUnion) unmarshal(typ BodyType, data []byte) error {
	switch typ {
	case BodyTypeInput:
		bu.OfInput = &Input{}
		if err := json.Unmarshal(data, bu.OfInput); err != nil {
			return err
		}
	case BodyTypeCommand:
		bu.OfCommand = &Command{}
		if err := json.Unmarshal(data, bu.OfCommand); err != nil {
			return err
		}
	}
	return nil
}

type Command struct {
	Line string `json:"line"`
}

type Input struct {
	Text    string `json:"text,omitempty"`
	Control string `json:"control,omitempty"`
}
