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
	BodyTypeQuestion           BodyType = "question"
	BodyTypeStreamMessageStart BodyType = "stream-message-start"
	BodyTypeStreamMessageDelta BodyType = "stream-message-delta"
	BodyTypeStreamMessageStop  BodyType = "stream-message-stop"
	BodyTypeStreamBlockStart   BodyType = "stream-block-start"
	BodyTypeStreamBlockDelta   BodyType = "stream-block-delta"
	BodyTypeStreamBlockStop    BodyType = "stream-block-stop"
)

type BodyUnion struct {
	OfQuestion           *Question
	OfStreamMessageStart *StreamMessageStart
	OfStreamMessageDelta *StreamMessageDelta
	OfStreamMessageStop  *StreamMessageStop
	OfStreamBlockStart   *StreamBlockStart
	OfStreamBlockDelta   *StreamBlockDelta
	OfStreamBlockStop    *StreamBlockStop
}

func (bu *BodyUnion) asAny(typ BodyType) any {
	if bu == nil {
		return nil
	}
	switch typ {
	case BodyTypeQuestion:
		return bu.OfQuestion
	case BodyTypeStreamMessageStart:
		return bu.OfStreamMessageStart
	case BodyTypeStreamMessageDelta:
		return bu.OfStreamMessageDelta
	case BodyTypeStreamMessageStop:
		return bu.OfStreamMessageStop
	case BodyTypeStreamBlockStart:
		return bu.OfStreamBlockStart
	case BodyTypeStreamBlockDelta:
		return bu.OfStreamBlockDelta
	case BodyTypeStreamBlockStop:
		return bu.OfStreamBlockStop
	}
	return nil
}

func (bu *BodyUnion) unmarshal(typ BodyType, data []byte) error {
	switch typ {
	case BodyTypeQuestion:
		bu.OfQuestion = &Question{}
		if err := json.Unmarshal(data, bu.OfQuestion); err != nil {
			return err
		}
	case BodyTypeStreamMessageStart:
		bu.OfStreamMessageStart = &StreamMessageStart{}
		if err := json.Unmarshal(data, bu.OfStreamMessageStart); err != nil {
			return err
		}
	case BodyTypeStreamMessageDelta:
		bu.OfStreamMessageDelta = &StreamMessageDelta{}
		if err := json.Unmarshal(data, bu.OfStreamMessageDelta); err != nil {
			return err
		}
	case BodyTypeStreamMessageStop:
		bu.OfStreamMessageStop = &StreamMessageStop{}
		if err := json.Unmarshal(data, bu.OfStreamMessageStop); err != nil {
			return err
		}
	case BodyTypeStreamBlockStart:
		bu.OfStreamBlockStart = &StreamBlockStart{}
		if err := json.Unmarshal(data, bu.OfStreamBlockStart); err != nil {
			return err
		}
	case BodyTypeStreamBlockDelta:
		bu.OfStreamBlockDelta = &StreamBlockDelta{}
		if err := json.Unmarshal(data, bu.OfStreamBlockDelta); err != nil {
			return err
		}
	case BodyTypeStreamBlockStop:
		bu.OfStreamBlockStop = &StreamBlockStop{}
		if err := json.Unmarshal(data, bu.OfStreamBlockStop); err != nil {
			return err
		}
	}
	return nil
}

type Question struct {
	Provider string `json:"provider"`
	Model    string `json:"model"`
	Text     string `json:"text"`
}

type StreamMessageStart struct {
	Text string `json:"text"`
}

type StreamMessageDelta struct {
	Text string `json:"text"`
}

type StreamMessageStop struct {
}

type StreamBlockStart struct {
	Type string `json:"type"`
}

type StreamBlockDelta struct {
	Data string `json:"data"`
}

type StreamBlockStop struct {
}
