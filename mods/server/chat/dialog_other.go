package chat

import (
	"context"
	"fmt"

	"github.com/machbase/neo-server/v8/mods/eventbus"
)

type UnknownDialog struct {
	topic    string
	session  string
	msgID    int64
	provider string
	model    string
	error    string
}

func (d *UnknownDialog) Talk(ctx context.Context, _ string) {
	eventbus.PublishMessage(d.topic, d.session,
		&eventbus.Message{
			Ver:  "1.0",
			ID:   d.msgID,
			Type: eventbus.BodyTypeStreamBlockStart,
		})
	eventbus.PublishMessage(d.topic, d.session,
		&eventbus.Message{
			Ver:  "1.0",
			ID:   d.msgID,
			Type: eventbus.BodyTypeStreamBlockDelta,
			Body: &eventbus.BodyUnion{
				OfStreamBlockDelta: &eventbus.StreamBlockDelta{
					ContentType: "error",
					Text:        d.error,
				},
			},
		})
	eventbus.PublishMessage(d.topic, d.session,
		&eventbus.Message{
			Ver:  "1.0",
			ID:   d.msgID,
			Type: eventbus.BodyTypeStreamBlockStop,
		})
}

type TestingDialog struct {
	topic    string
	session  string
	msgID    int64
	provider string
	model    string
}

func (d *TestingDialog) Talk(ctx context.Context, message string) {
	// Simulate a response
	eventbus.PublishMessage(d.topic, d.session,
		&eventbus.Message{
			Ver:  "1.0",
			ID:   d.msgID,
			Type: eventbus.BodyTypeStreamMessageStart,
		})
	eventbus.PublishMessage(d.topic, d.session,
		&eventbus.Message{
			Ver:  "1.0",
			ID:   d.msgID,
			Type: eventbus.BodyTypeStreamBlockStart,
		})
	eventbus.PublishMessage(d.topic, d.session,
		&eventbus.Message{
			Ver:  "1.0",
			ID:   d.msgID,
			Type: eventbus.BodyTypeStreamBlockDelta,
			Body: &eventbus.BodyUnion{
				OfStreamBlockDelta: &eventbus.StreamBlockDelta{
					ContentType: "text",
					Text: fmt.Sprintf("This is a simulated response from %s model %s to your message: %s\n",
						d.provider, d.model, message),
				},
			},
		})
	eventbus.PublishMessage(d.topic, d.session,
		&eventbus.Message{
			Ver:  "1.0",
			ID:   d.msgID,
			Type: eventbus.BodyTypeStreamBlockStop,
		})
	eventbus.PublishMessage(d.topic, d.session,
		&eventbus.Message{
			Ver:  "1.0",
			ID:   d.msgID,
			Type: eventbus.BodyTypeStreamMessageStop,
		})

}
