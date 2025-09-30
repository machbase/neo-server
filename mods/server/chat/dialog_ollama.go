package chat

import (
	"context"
	"fmt"

	"github.com/machbase/neo-server/v8/mods/eventbus"
)

func NewOllamaDialog(topic string, msgID int64, model string) *DialogOllama {
	const systemMessage = "You are a friendly AI assistant for Machbase Neo DB."
	ret := &DialogOllama{
		topic:          topic,
		msgID:          msgID,
		model:          model,
		Url:            "http://127.0.0.1:11434",
		SystemMessages: []string{systemMessage},
	}
	return ret
}

type DialogOllama struct {
	Url            string   `json:"url"`
	SystemMessages []string `json:"system_messages,omitempty"`

	topic string `json:"-"`
	msgID int64  `json:"-"`
	model string `json:"-"`
}

func (d *DialogOllama) publish(typ eventbus.BodyType, body *eventbus.BodyUnion) {
	eventbus.PublishMessage(d.topic, &eventbus.Message{
		Ver:  "1.0",
		ID:   d.msgID,
		Type: typ,
		Body: body,
	})
}

func (d *DialogOllama) Talk(ctx context.Context, message string) {
	d.publish(eventbus.BodyTypeStreamBlockStart, nil)
	d.publish(eventbus.BodyTypeStreamBlockDelta,
		&eventbus.BodyUnion{
			OfStreamBlockDelta: &eventbus.StreamBlockDelta{
				ContentType: "text",
				Text:        fmt.Sprintf("message from %s\n", d.model),
			},
		})
	d.publish(eventbus.BodyTypeStreamBlockStop, nil)
}
