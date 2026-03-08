package engine_test

import (
	"testing"

	"github.com/machbase/neo-server/v8/jsh/test_engine"
)

func TestEvents(t *testing.T) {
	tests := []test_engine.TestCase{
		{
			Name: "event_emitter_basic",
			Script: `
				const EventEmitter = require('/lib/events');
				const emitter = new EventEmitter();
				
				emitter.on("greet", function(name) {
					console.println("Hello, " + name + "!");
				});

				emitter.emit("greet", "Alice");
				emitter.emit("greet", "Bob");
			`,
			Output: []string{
				"Hello, Alice!",
				"Hello, Bob!",
			},
		},
		{
			Name: "event_emitter_basic",
			Script: `
				const EventEmitter = require('/lib/events');
				const emitter = new EventEmitter();
				
				emitter.once("greet", function(name) {
					console.println("Hello, " + name + "!");
				});

				emitter.emit("greet", "Alice");
				emitter.emit("greet", "Bob");
			`,
			Output: []string{
				"Hello, Alice!",
			},
		},
	}
	for _, tc := range tests {
		test_engine.RunTest(t, tc)
	}
}
