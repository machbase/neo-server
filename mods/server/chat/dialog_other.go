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

func (d *UnknownDialog) publish(typ eventbus.BodyType, body *eventbus.BodyUnion) {
	eventbus.PublishMessage(d.topic, d.session,
		&eventbus.Message{
			Ver:  "1.0",
			ID:   d.msgID,
			Type: typ,
			Body: body,
		})
}

func (d *UnknownDialog) Talk(ctx context.Context, _ string) {
	d.publish(eventbus.BodyTypeAnswerStart, nil)
	d.publish(eventbus.BodyTypeStreamMessageStart, nil)
	d.publish(eventbus.BodyTypeStreamBlockStart, &eventbus.BodyUnion{
		OfStreamBlockDelta: &eventbus.StreamBlockDelta{
			ContentType: "text",
		},
	})
	d.publish(eventbus.BodyTypeStreamBlockDelta, &eventbus.BodyUnion{
		OfStreamBlockDelta: &eventbus.StreamBlockDelta{
			ContentType: "error",
			Text:        d.error,
		},
	})
	d.publish(eventbus.BodyTypeStreamBlockStop, &eventbus.BodyUnion{
		OfStreamBlockDelta: &eventbus.StreamBlockDelta{
			ContentType: "text",
		},
	})
	d.publish(eventbus.BodyTypeStreamMessageStop, nil)
	d.publish(eventbus.BodyTypeAnswerStop, nil)
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
