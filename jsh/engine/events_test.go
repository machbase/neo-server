package engine

import (
	"testing"
)

func TestEvents(t *testing.T) {
	tests := []TestCase{
		{
			name: "event_emitter_basic",
			script: `
				const EventEmitter = require('/lib/events');
				const emitter = new EventEmitter();
				
				emitter.on("greet", function(name) {
					console.println("Hello, " + name + "!");
				});

				emitter.emit("greet", "Alice");
				emitter.emit("greet", "Bob");
			`,
			output: []string{
				"Hello, Alice!",
				"Hello, Bob!",
			},
		},
		{
			name: "event_emitter_basic",
			script: `
				const EventEmitter = require('/lib/events');
				const emitter = new EventEmitter();
				
				emitter.once("greet", function(name) {
					console.println("Hello, " + name + "!");
				});

				emitter.emit("greet", "Alice");
				emitter.emit("greet", "Bob");
			`,
			output: []string{
				"Hello, Alice!",
			},
		},
	}
	for _, tc := range tests {
		RunTest(t, tc)
	}
}
