package stream_test

import (
	"testing"

	"github.com/machbase/neo-server/v8/jsh/test_engine"
)

func TestStreamModule(t *testing.T) {
	tests := []test_engine.TestCase{
		{
			Name: "module-exports",
			Script: `
				const stream = require("/lib/stream");
				console.println("Readable:", typeof stream.Readable);
				console.println("Writable:", typeof stream.Writable);
				console.println("Duplex:", typeof stream.Duplex);
				console.println("PassThrough:", typeof stream.PassThrough);
			`,
			Output: []string{
				"Readable: function",
				"Writable: function",
				"Duplex: function",
				"PassThrough: function",
			},
		},
		{
			Name: "passthrough-basic",
			Script: `
				const { PassThrough } = require('/lib/stream');
				const pt = new PassThrough();
				
				pt.on('finish', () => {
					console.println('Finished');
				});
				
				pt.write('Hello, ');
				pt.write('World!');
				pt.end();
			`,
			Output: []string{
				"Finished",
			},
		},
		{
			Name: "passthrough-events",
			Script: `
				const { PassThrough } = require('/lib/stream');
				const pt = new PassThrough();
				
				pt.on('finish', () => {
					console.println('finish');
				});
				
				pt.on('close', () => {
					console.println('close');
				});
				
				pt.write('Test');
				pt.end();
			`,
			Output: []string{
				"finish",
				"close",
			},
		},
		{
			Name: "writable-properties",
			Script: `
				const { PassThrough } = require('/lib/stream');
				const stream = new PassThrough();
				console.println('writable:', stream.writable);
				console.println('writableEnded:', stream.writableEnded);
				stream.end();
				console.println('writableEnded after end:', stream.writableEnded);
			`,
			Output: []string{
				"writable: true",
				"writableEnded: false",
				"writableEnded after end: true",
			},
		},
		{
			Name: "readable-properties",
			Script: `
				const { PassThrough } = require('/lib/stream');
				const stream = new PassThrough();
				console.println('readable:', stream.readable);
				console.println('readableEnded:', stream.readableEnded);
			`,
			Output: []string{
				"readable: true",
				"readableEnded: false",
			},
		},
		{
			Name: "pause-resume",
			Script: `
				const { PassThrough } = require('/lib/stream');
				const stream = new PassThrough();
				
				console.println('isPaused:', stream.isPaused());
				stream.pause();
				console.println('isPaused after pause:', stream.isPaused());
				stream.resume();
				console.println('isPaused after resume:', stream.isPaused());
			`,
			Output: []string{
				"isPaused: false",
				"isPaused after pause: true",
				"isPaused after resume: false",
			},
		},
		{
			Name: "write-after-end-error",
			Script: `
				const { PassThrough } = require('/lib/stream');
				const stream = new PassThrough();
				
				stream.on('error', (err) => {
					console.println('error:', err.message);
				});
				
				stream.end('first');
				stream.write('second'); // This should emit error
			`,
			Output: []string{
				"error: Stream is not writable",
			},
		},
		{
			Name: "multiple-writes",
			Script: `
				const { PassThrough } = require('/lib/stream');
				const stream = new PassThrough();
				
				stream.on('finish', () => {
					console.println('Write complete');
				});
				
				stream.write('first\n');
				stream.write('second\n');
				stream.write('third\n');
				stream.end();
			`,
			Output: []string{
				"Write complete",
			},
		},
		{
			Name: "destroy-with-error",
			Script: `
				const { PassThrough } = require('/lib/stream');
				const stream = new PassThrough();
				let errorCount = 0;
				let closeCount = 0;
				
				stream.on('error', (err) => {
					if (errorCount === 0) {
						console.println('error:', err.message);
					}
					errorCount++;
				});
				
				stream.on('close', () => {
					if (closeCount === 0) {
						console.println('close');
					}
					closeCount++;
				});
				
				stream.destroy(new Error('Test error'));
				console.println('destroyed');
			`,
			Output: []string{
				"error: Test error",
				"close",
				"destroyed",
			},
		},
		{
			Name: "write-string-encoding",
			Script: `
				const { PassThrough } = require('/lib/stream');
				const stream = new PassThrough();
				
				stream.on('finish', () => {
					console.println('Write finished');
				});
				
				stream.write('Hello', 'utf8');
				stream.end();
			`,
			Output: []string{
				"Write finished",
			},
		},
		{
			Name: "end-with-data",
			Script: `
				const { PassThrough } = require('/lib/stream');
				const stream = new PassThrough();
				
				stream.on('finish', () => {
					console.println('finish');
				});
				
				stream.write('Hello ');
				stream.end('World!');
			`,
			Output: []string{
				"finish",
			},
		},
		{
			Name: "stream-properties-state",
			Script: `
				const { PassThrough } = require('/lib/stream');
				const stream = new PassThrough();
				
				console.println('Initial state:');
				console.println('  readable:', stream.readable);
				console.println('  writable:', stream.writable);
				
				stream.end();
				
				console.println('After end:');
				console.println('  readable:', stream.readable);
				console.println('  writable:', stream.writable);
			`,
			Output: []string{
				"Initial state:",
				"  readable: true",
				"  writable: true",
				"After end:",
				"  readable: true",
				"  writable: false",
			},
		},
		{
			Name: "event-order",
			Script: `
				const { PassThrough } = require('/lib/stream');
				const stream = new PassThrough();
				
				stream.on('finish', () => {
					console.println('1. finish');
				});
				
				stream.on('close', () => {
					console.println('2. close');
				});
				
				stream.end('data');
			`,
			Output: []string{
				"1. finish",
				"2. close",
			},
		},
		{
			Name: "double-end-error",
			Script: `
				const { PassThrough } = require('/lib/stream');
				const stream = new PassThrough();
				
				stream.end('first');
				
				// Try to end again
				stream.end('second', (err) => {
					if (err) {
						console.println('error:', err.message);
					} else {
						console.println('no error');
					}
				});
			`,
			Output: []string{
				"error: Stream already ended",
			},
		},
		{
			Name: "write-callback",
			Script: `
				const { PassThrough } = require('/lib/stream');
				const stream = new PassThrough();
				
				stream.write('test', (err) => {
					if (err) {
						console.println('error:', err.message);
					} else {
						console.println('write callback called');
					}
				});
				
				stream.end();
			`,
			Output: []string{
				"write callback called",
			},
		},
		{
			Name: "end-callback",
			Script: `
				const { PassThrough } = require('/lib/stream');
				const stream = new PassThrough();
				
				stream.end('final data', (err) => {
					if (err) {
						console.println('error:', err.message);
					} else {
						console.println('end callback called');
					}
				});
			`,
			Output: []string{
				"end callback called",
			},
		},
		{
			Name: "buffer-types",
			Script: `
				const { PassThrough } = require('/lib/stream');
				const stream = new PassThrough();
				
				// Write string
				const result1 = stream.write('string data');
				console.println('write string:', result1);
				
				stream.end();
			`,
			Output: []string{
				"write string: true",
			},
		},
		{
			Name: "closed-stream-write",
			Script: `
				const { PassThrough } = require('/lib/stream');
				const stream = new PassThrough();
				
				stream.on('error', (err) => {
					console.println('error:', err.message);
				});
				
				stream.close();
				
				// Try to write after close
				stream.write('should fail');
			`,
			Output: []string{
				"error: Stream is not writable",
			},
		},
		{
			Name: "highWaterMark-properties",
			Script: `
				const { PassThrough } = require('/lib/stream');
				const stream = new PassThrough();
				
				console.println('readableHighWaterMark:', stream.readableHighWaterMark);
				console.println('writableHighWaterMark:', stream.writableHighWaterMark);
			`,
			Output: []string{
				"readableHighWaterMark: 16384",
				"writableHighWaterMark: 16384",
			},
		},
		{
			Name: "flowing-state",
			Script: `
				const { PassThrough } = require('/lib/stream');
				const stream = new PassThrough();
				
				console.println('initial flowing:', stream.readableFlowing);
				stream.pause();
				console.println('after pause:', stream.readableFlowing);
				stream.resume();
				console.println('after resume:', stream.readableFlowing);
			`,
			Output: []string{
				"initial flowing: null",
				"after pause: false",
				"after resume: true",
			},
		},
	}

	for _, tc := range tests {
		test_engine.RunTest(t, tc)
	}
}
