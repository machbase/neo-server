package tail_test

import (
	"testing"

	"github.com/machbase/neo-server/v8/jsh/test_engine"
)

func TestUtilTail(t *testing.T) {
	testCases := []test_engine.TestCase{
		{
			Name: "util_tail_poll_append_and_callback",
			Script: `
                const fs = require('fs');
                const tail = require('util/tail');

                const file = '/tmp/tail-basic.log';
        fs.writeFileSync(file, 'old-line\n');

                const follower = tail.create(file);
                let first = [];
                follower.poll((lines) => { first = lines; });
                console.println('first-len:', first.length);

                fs.appendFileSync(file, 'new-1\nnew-2\n');
                const second = follower.poll((lines) => {
                    console.println('callback-lines:', JSON.stringify(lines));
                });
                console.println('poll-lines:', JSON.stringify(second));
                follower.close();
            `,
			Output: []string{
				"first-len: 0",
				"callback-lines: [\"new-1\",\"new-2\"]",
				"poll-lines: [\"new-1\",\"new-2\"]",
			},
		},
		{
			Name: "util_tail_follow_rotation",
			Script: `
                const fs = require('fs');
                const tail = require('util/tail');

                const file = '/tmp/tail-rotate.log';
                const rotated = '/tmp/tail-rotate.log.1';

                fs.writeFileSync(file, 'old-a\n');
                const follower = tail.create(file);
                follower.poll();

                fs.renameSync(file, rotated);
                fs.writeFileSync(file, 'new-a\nnew-b\n');

                const lines = follower.poll();
                console.println('rotated-lines:', JSON.stringify(lines));
                follower.close();
            `,
			Output: []string{
				"rotated-lines: [\"new-a\",\"new-b\"]",
			},
		},
		{
			Name: "util_tail_from_start_and_truncate",
			Script: `
                const fs = require('fs');
                const tail = require('util/tail');

                const file = '/tmp/tail-truncate.log';
                fs.writeFileSync(file, 'l1\nl2\n');

                const follower = tail.create(file, { fromStart: true });
                console.println('initial:', JSON.stringify(follower.poll()));

                fs.truncateSync(file, 0);
                console.println('after-truncate-empty:', JSON.stringify(follower.poll()));
                fs.appendFileSync(file, 'after-truncate\n');
                console.println('after-truncate:', JSON.stringify(follower.poll()));
                follower.close();
            `,
			Output: []string{
				"initial: [\"l1\",\"l2\"]",
				"after-truncate-empty: []",
				"after-truncate: [\"after-truncate\"]",
			},
		},
		{
			Name: "util_tail_sse_adapter_polling",
			Script: `
                const fs = require('fs');
                const tailSSE = require('util/tail/sse');

                const file = '/tmp/tail-sse.log';
                fs.writeFileSync(file, 'boot\n');

                let out = '';
                const adapter = tailSSE.create(file, {
                    write: (chunk) => { out += chunk; },
                    event: 'log',
                    retryMs: 1200,
                });

                adapter.writeHeaders();
                adapter.poll();

                fs.appendFileSync(file, 'line-1\nline-2\n');
                const lines = adapter.poll();
                adapter.send('manual-msg', 'notice');
                adapter.comment('tail-running');
                adapter.close();

                console.println('sse-lines:', JSON.stringify(lines));
                console.println('sse-has-header:', out.indexOf('Content-Type: text/event-stream') >= 0);
                console.println('sse-has-event:', out.indexOf('event: log') >= 0);
                console.println('sse-has-data1:', out.indexOf('data: line-1') >= 0);
                console.println('sse-has-data2:', out.indexOf('data: line-2') >= 0);
                console.println('sse-has-manual:', out.indexOf('event: notice') >= 0 && out.indexOf('data: manual-msg') >= 0);
                console.println('sse-has-comment:', out.indexOf(': tail-running') >= 0);
            `,
			Output: []string{
				"sse-lines: [\"line-1\",\"line-2\"]",
				"sse-has-header: true",
				"sse-has-event: true",
				"sse-has-data1: true",
				"sse-has-data2: true",
				"sse-has-manual: true",
				"sse-has-comment: true",
			},
		},
	}

	for _, tc := range testCases {
		test_engine.RunTest(t, tc)
	}
}
