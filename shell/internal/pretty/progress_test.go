package pretty

import "testing"

func TestProgress(t *testing.T) {
	tests := []TestCase{
		{
			name: "Progress_basic",
			script: `
				const pretty = require('/usr/lib/pretty');
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
			output: []string{
				"Done",
			},
		},
	}

	for _, tc := range tests {
		RunTest(t, tc)
	}
}
