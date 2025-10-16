package cmd

import (
	"fmt"

	"github.com/machbase/neo-server/v8/mods/eventbus"
)

func (p *Processor) SendMessage(typ eventbus.BodyType, body *eventbus.BodyUnion) {
	evt := &eventbus.Event{
		Type:    eventbus.EVT_MSG,
		Session: p.Session,
		Message: &eventbus.Message{
			Ver:  "1.0",
			ID:   p.MsgID,
			Type: typ,
			Body: body,
		},
	}
	eventbus.Default.Publish(p.Topic, evt)
}

func (p *Processor) Printf(format string, args ...any) {
	text := fmt.Sprintf(format, args...)
	p.Print(text)
}

func (p *Processor) Println(args ...any) {
	text := fmt.Sprintln(args...)
	p.Print(text)
}

func (p *Processor) Print(text string) {
	evt := &eventbus.Event{
		Type:    eventbus.EVT_MSG,
		Session: p.Session,
		Message: &eventbus.Message{
			Ver:  "1.0",
			ID:   p.MsgID,
			Type: eventbus.BodyTypeStreamBlockDelta,
			Body: &eventbus.BodyUnion{
				OfStreamBlockDelta: &eventbus.StreamBlockDelta{
					ContentType: "text",
					Text:        text,
				},
			},
		},
	}
	eventbus.Default.Publish(p.Topic, evt)
}
