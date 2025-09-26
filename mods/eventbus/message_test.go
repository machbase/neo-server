package eventbus_test

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/machbase/neo-server/v8/mods/eventbus"
	"github.com/stretchr/testify/require"
)

func TestQuestion(t *testing.T) {
	bs := &bytes.Buffer{}
	enc := json.NewEncoder(bs)
	enc.SetIndent("", "  ")

	msg := &eventbus.Message{
		Ver:  "1.0",
		ID:   12345,
		Type: "question",
	}
	err := enc.Encode(msg)
	require.NoError(t, err)
	require.JSONEq(t, `{
		"ver": "1.0",
		"id": 12345,
		"type": "question",
		"body": null
	}`, bs.String())
	bs.Reset()

	msg.Body = &eventbus.BodyUnion{
		OfQuestion: &eventbus.Question{
			Provider: "ollama",
			Model:    "llm-model",
			Text:     "Hello, world!",
		},
	}

	err = enc.Encode(msg)
	require.NoError(t, err)
	require.JSONEq(t, `{
		"ver": "1.0",
		"id": 12345,
		"type": "question",
		"body": {
			"provider": "ollama",
			"model": "llm-model",
			"text": "Hello, world!"
		}
	}`, bs.String())

	var msg2 eventbus.Message
	err = json.Unmarshal(bs.Bytes(), &msg2)
	require.NoError(t, err)
	require.Equal(t, msg.Ver, msg2.Ver)
	require.Equal(t, msg.ID, msg2.ID)
	require.Equal(t, msg.Type, msg2.Type)
	require.NotNil(t, msg2.Body)
	require.NotNil(t, msg2.Body.OfQuestion)
	require.Equal(t, msg.Body.OfQuestion.Provider, msg2.Body.OfQuestion.Provider)
	require.Equal(t, msg.Body.OfQuestion.Model, msg2.Body.OfQuestion.Model)
	require.Equal(t, msg.Body.OfQuestion.Text, msg2.Body.OfQuestion.Text)
}
