package eventbus

import (
	"encoding/json"
	"testing"
)

func TestMessageMarshalJSON(t *testing.T) {
	msg := Message{
		Ver:  "1.0",
		ID:   7,
		Type: BodyTypeCommand,
		Body: &BodyUnion{
			OfCommand: &Command{Line: "select * from dual"},
		},
	}

	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal map failed: %v", err)
	}

	if decoded["ver"] != "1.0" {
		t.Fatalf("unexpected ver: %v", decoded["ver"])
	}
	if decoded["type"] != string(BodyTypeCommand) {
		t.Fatalf("unexpected type: %v", decoded["type"])
	}
	body, ok := decoded["body"].(map[string]any)
	if !ok {
		t.Fatalf("unexpected body type: %T", decoded["body"])
	}
	if body["line"] != "select * from dual" {
		t.Fatalf("unexpected command line: %v", body["line"])
	}
}

func TestMessageMarshalJSONWithNilBody(t *testing.T) {
	msg := Message{Ver: "1.0", ID: 9, Type: BodyType("unknown")}

	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal map failed: %v", err)
	}

	if decoded["body"] != nil {
		t.Fatalf("expected nil body, got %v", decoded["body"])
	}
}

func TestBodyUnionAsAnyInputAndUnknown(t *testing.T) {
	union := &BodyUnion{
		OfInput: &Input{Text: "hello", Control: "enter"},
	}

	input, ok := union.asAny(BodyTypeInput).(*Input)
	if !ok {
		t.Fatalf("expected input body, got %T", union.asAny(BodyTypeInput))
	}
	if input.Text != "hello" || input.Control != "enter" {
		t.Fatalf("unexpected input body: %+v", input)
	}

	if got := union.asAny(BodyType("unknown")); got != nil {
		t.Fatalf("expected unknown type to return nil, got %T", got)
	}
}

func TestMessageUnmarshalJSONCommand(t *testing.T) {
	raw := []byte(`{"ver":"1.0","id":11,"type":"command","body":{"line":"show tables"}}`)

	var msg Message
	if err := json.Unmarshal(raw, &msg); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if msg.Ver != "1.0" || msg.ID != 11 || msg.Type != BodyTypeCommand {
		t.Fatalf("unexpected message metadata: %+v", msg)
	}
	if msg.Body == nil || msg.Body.OfCommand == nil {
		t.Fatal("expected command body to be decoded")
	}
	if msg.Body.OfCommand.Line != "show tables" {
		t.Fatalf("unexpected command line: %s", msg.Body.OfCommand.Line)
	}
}

func TestMessageUnmarshalJSONInput(t *testing.T) {
	raw := []byte(`{"ver":"1.0","id":12,"type":"input","body":{"text":"hello","control":"enter"}}`)

	var msg Message
	if err := json.Unmarshal(raw, &msg); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if msg.Body == nil || msg.Body.OfInput == nil {
		t.Fatal("expected input body to be decoded")
	}
	if msg.Body.OfInput.Text != "hello" || msg.Body.OfInput.Control != "enter" {
		t.Fatalf("unexpected input body: %+v", msg.Body.OfInput)
	}
}

func TestMessageUnmarshalJSONWithoutBody(t *testing.T) {
	raw := []byte(`{"ver":"1.0","id":13,"type":"command"}`)

	var msg Message
	if err := json.Unmarshal(raw, &msg); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if msg.Body != nil {
		t.Fatalf("expected nil body, got %+v", msg.Body)
	}
}

func TestMessageUnmarshalJSONInvalidDocument(t *testing.T) {
	var msg Message
	if err := json.Unmarshal([]byte(`{"ver":`), &msg); err == nil {
		t.Fatal("expected invalid JSON document to fail")
	}
}

func TestMessageUnmarshalJSONInvalidPayload(t *testing.T) {
	var msg Message
	if err := json.Unmarshal([]byte(`{"ver":"1.0","id":14,"type":"command","body":{"line":123}}`), &msg); err == nil {
		t.Fatal("expected command body unmarshal to fail")
	}
}

func TestMessageUnmarshalJSONUnknownTypeBody(t *testing.T) {
	raw := []byte(`{"ver":"1.0","id":15,"type":"unknown","body":{"value":"ignored"}}`)

	var msg Message
	if err := json.Unmarshal(raw, &msg); err != nil {
		t.Fatalf("unexpected error for unknown type: %v", err)
	}

	if msg.Body == nil {
		t.Fatal("expected body union to be allocated")
	}
	if msg.Body.OfCommand != nil || msg.Body.OfInput != nil {
		t.Fatalf("expected unknown body type to leave union empty: %+v", msg.Body)
	}
}
