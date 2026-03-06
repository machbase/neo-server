package pretty_test

import (
	"testing"

	"github.com/machbase/neo-server/v8/jsh/test_engine"
)

func TestProgress(t *testing.T) {
	tests := []test_engine.TestCase{
		{
			Name: "Progress_basic",
			Script: `
				const pretty = require('pretty');
				const pw = pretty.Progress();
				const tracker = pw.tracker({message: 'Processing', total: 0});
				
				let interval = setInterval(() => {
					tracker.increment(10);
					if (tracker.value() >= 100) {
						tracker.markAsDone();							
						clearInterval(interval);
						setTimeout(() => {
							if(tracker.isDone()) {
								console.println("Done");
							} else {
								console.println("Not Done");
							}
						}, 500);
					}
				}, 500); // keep alive
			`,
			Output: []string{
				"Done",
			},
		},
	}

	for _, tc := range tests {
		test_engine.RunTest(t, tc)
	}
}
