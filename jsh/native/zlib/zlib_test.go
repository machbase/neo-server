package zlib

import (
	"bytes"
	"strings"
	"testing"

	"github.com/dop251/goja"
	"github.com/machbase/neo-server/v8/jsh/engine"
	"github.com/machbase/neo-server/v8/jsh/root"
)

func TestZlibModule(t *testing.T) {
	rt := goja.New()

	// Create module object
	module := rt.NewObject()
	exports := rt.NewObject()
	module.Set("exports", exports)

	// Initialize zlib module
	Module(rt, module)

	// Test that all expected functions are exported
	exportsObj := module.Get("exports").(*goja.Object)

	testCases := []string{
		"createGzip",
		"createGunzip",
		"createDeflate",
		"createInflate",
		"createDeflateRaw",
		"createInflateRaw",
		"createUnzip",
		"gzip",
		"gunzip",
		"deflate",
		"inflate",
		"deflateRaw",
		"inflateRaw",
		"unzip",
		"gzipSync",
		"gunzipSync",
		"deflateSync",
		"inflateSync",
		"deflateRawSync",
		"inflateRawSync",
		"unzipSync",
		"constants",
	}

	for _, name := range testCases {
		if exportsObj.Get(name) == nil || goja.IsUndefined(exportsObj.Get(name)) {
			t.Errorf("Expected %s to be exported", name)
		}
	}
}

type TestCase struct {
	name   string
	script string
	input  []string
	output []string
	err    string
	vars   map[string]any
}

func RunTest(t *testing.T, tc TestCase) {
	t.Helper()
	t.Run(tc.name, func(t *testing.T) {
		t.Helper()
		conf := engine.Config{
			Name:   tc.name,
			Code:   tc.script,
			FSTabs: []engine.FSTab{root.RootFSTab(), {MountPoint: "/tmp", Source: "../../../tmp/"}},
			Env:    tc.vars,
			Reader: &bytes.Buffer{},
			Writer: &bytes.Buffer{},
		}
		jr, err := engine.New(conf)
		if err != nil {
			t.Fatalf("Failed to create JSRuntime: %v", err)
		}
		jr.RegisterNativeModule("@jsh/zlib", Module)
		jr.RegisterNativeModule("@jsh/fs", jr.Filesystem)

		if len(tc.input) > 0 {
			conf.Reader.(*bytes.Buffer).WriteString(strings.Join(tc.input, ""))
		}
		if err := jr.Run(); err != nil {
			if tc.err == "" || !strings.Contains(err.Error(), tc.err) {
				t.Fatalf("Unexpected error: %v", err)
			}
			return
		}

		gotOutput := conf.Writer.(*bytes.Buffer).String()
		lines := strings.Split(gotOutput, "\n")
		if len(lines) != len(tc.output)+1 { // +1 for trailing newline
			t.Fatalf("Expected %d output lines, got %d\n%s", len(tc.output), len(lines)-1, gotOutput)
		}
		for i, expectedLine := range tc.output {
			if lines[i] != expectedLine {
				t.Errorf("Output line %d: expected %q, got %q", i, expectedLine, lines[i])
			}
		}
	})
}

func TestZlibSync(t *testing.T) {
	tests := []TestCase{
		{
			name: "gzipSync-gunzipSync",
			script: `
				const zlib = require('@jsh/zlib');
				const testData = "Hello, World! This is a test string for compression.";
				
				const compressed = zlib.gzipSync(testData);
				console.println('compressed size:', compressed.byteLength);
				console.println('compressed type:', compressed.constructor.name);
				
				const decompressed = zlib.gunzipSync(compressed);
				const result = String.fromCharCode.apply(null, new Uint8Array(decompressed));
				
				console.println('result:', result);
			`,
			output: []string{
				"compressed size: 76",
				"compressed type: ArrayBuffer",
				"result: Hello, World! This is a test string for compression.",
			},
		},
		{
			name: "deflateSync-inflateSync",
			script: `
				const zlib = require('@jsh/zlib');
				const testData = "Test data for deflate compression";
				
				const compressed = zlib.deflateSync(testData);
				console.println('compressed size:', compressed.byteLength);
				
				const decompressed = zlib.inflateSync(compressed);
				const result = String.fromCharCode.apply(null, new Uint8Array(decompressed));
				
				console.println('result:', result);
			`,
			output: []string{
				"compressed size: 39",
				"result: Test data for deflate compression",
			},
		},
		{
			name: "deflateRawSync-inflateRawSync",
			script: `
				const zlib = require('@jsh/zlib');
				const testData = "Raw deflate test";
				
				const compressed = zlib.deflateRawSync(testData);
				const decompressed = zlib.inflateRawSync(compressed);
				const result = String.fromCharCode.apply(null, new Uint8Array(decompressed));
				
				console.println('result:', result);
			`,
			output: []string{
				"result: Raw deflate test",
			},
		},
		{
			name: "constants",
			script: `
				const zlib = require('@jsh/zlib');
				const c = zlib.constants;
				
				console.println('Z_NO_FLUSH:', typeof c.Z_NO_FLUSH);
				console.println('Z_BEST_COMPRESSION:', c.Z_BEST_COMPRESSION);
				console.println('Z_DEFAULT_COMPRESSION:', c.Z_DEFAULT_COMPRESSION);
			`,
			output: []string{
				"Z_NO_FLUSH: number",
				"Z_BEST_COMPRESSION: 9",
				"Z_DEFAULT_COMPRESSION: -1",
			},
		},
		{
			name: "destructuring",
			script: `
				const { gzipSync, gunzipSync, constants } = require('@jsh/zlib');
				
				const data = "test";
				const compressed = gzipSync(data);
				const decompressed = gunzipSync(compressed);
				const result = String.fromCharCode.apply(null, new Uint8Array(decompressed));
				
				console.println('result:', result);
				console.println('has constants:', typeof constants);
			`,
			output: []string{
				"result: test",
				"has constants: object",
			},
		},
	}

	for _, tc := range tests {
		RunTest(t, tc)
	}
}

func TestZlibStream(t *testing.T) {
	tests := []TestCase{
		{
			name: "createGzip-stream-methods",
			script: `
				const zlib = require('@jsh/zlib');
				const gzip = zlib.createGzip();
				
				console.println('has write:', typeof gzip.write);
				console.println('has end:', typeof gzip.end);
				console.println('has on:', typeof gzip.on);
				console.println('has pipe:', typeof gzip.pipe);
			`,
			output: []string{
				"has write: function",
				"has end: function",
				"has on: function",
				"has pipe: function",
			},
		},
		{
			name: "createGunzip-stream-methods",
			script: `
				const zlib = require('@jsh/zlib');
				const gunzip = zlib.createGunzip();
				
				console.println('has write:', typeof gunzip.write);
				console.println('has end:', typeof gunzip.end);
				console.println('has on:', typeof gunzip.on);
			`,
			output: []string{
				"has write: function",
				"has end: function",
				"has on: function",
			},
		},
		{
			name: "createDeflate-createInflate",
			script: `
				const zlib = require('@jsh/zlib');
				
				console.println('createDeflate:', typeof zlib.createDeflate);
				console.println('createInflate:', typeof zlib.createInflate);
				console.println('createDeflateRaw:', typeof zlib.createDeflateRaw);
				console.println('createInflateRaw:', typeof zlib.createInflateRaw);
				console.println('createUnzip:', typeof zlib.createUnzip);
			`,
			output: []string{
				"createDeflate: function",
				"createInflate: function",
				"createDeflateRaw: function",
				"createInflateRaw: function",
				"createUnzip: function",
			},
		},
	}

	for _, tc := range tests {
		RunTest(t, tc)
	}
}

func TestZlibPipe(t *testing.T) {
	tests := []TestCase{
		{
			name: "pipe-with-invalid-dest",
			script: `
				const zlib = require('@jsh/zlib');
				
				const gzip = zlib.createGzip();
				
				let errorOccurred = false;
				let errorMsg = '';
				
				try {
					gzip.pipe({ notWriter: "invalid" });
				} catch (e) {
					errorOccurred = true;
					errorMsg = e.message;
				}
				
				console.println('error occurred:', errorOccurred);
				console.println('has map error:', errorMsg.includes('map'));
			`,
			output: []string{
				"error occurred: true",
				"has map error: true",
			},
		},
		{
			name: "pipe-with-invalid-type",
			script: `
				const zlib = require('@jsh/zlib');
				
				const gzip = zlib.createGzip();
				
				let errorOccurred = false;
				let errorMsg = '';
				
				try {
					gzip.pipe("not a writer");
				} catch (e) {
					errorOccurred = true;
					errorMsg = e.message;
				}
				
				console.println('error occurred:', errorOccurred);
				console.println('has dest error:', errorMsg.includes('dest must be'));
			`,
			output: []string{
				"error occurred: true",
				"has dest error: true",
			},
		},
		{
			name: "pipe-with-null",
			script: `
				const zlib = require('@jsh/zlib');
				
				const gzip = zlib.createGzip();
				
				let errorOccurred = false;
				
				try {
					gzip.pipe(null);
				} catch (e) {
					errorOccurred = true;
				}
				
				console.println('error occurred:', errorOccurred);
			`,
			output: []string{
				"error occurred: true",
			},
		},
		{
			name: "pipe-with-file",
			script: `
				const zlib = require('zlib');
				const fs = require('fs');
				
				// Clean up any existing file
				const outputPath = '/tmp/output_test.gz';
				try {
					if (fs.existsSync(outputPath)) {
						fs.unlinkSync(outputPath);
					}
				} catch (e) {
					// ignore
				}
				
				// Write compressed data
				const gzip = zlib.createGzip();
				const outFile = fs.createWriteStream(outputPath);

				let writeErrorOccurred = false;
				const testData = 'Test data for gzip compression';

				try {
					gzip.pipe(outFile);
					gzip.write(testData);
					gzip.end();
					
					// Explicitly close the output file
					outFile.end();
					
					// Wait for file writing to complete
					const start = Date.now();
					while (Date.now() - start < 300) {
						// wait for file to be fully written and flushed
					}
				} catch (e) {
					writeErrorOccurred = true;
					console.println('write exception:', e.message);
				}

				console.println('write error occurred:', writeErrorOccurred);
				
				// Read and verify compressed data using stream with pipe()
				let readErrorOccurred = false;
				let verifySuccess = false;
				let result = '';
				
				try {
					// Create read stream for compressed file
					const inFile = fs.createReadStream(outputPath, 'buffer');
					const gunzip = zlib.createGunzip();
					
					// Pipe input file through gunzip
					inFile.pipe(gunzip);
					
					inFile.on('error', function(err) {
						readErrorOccurred = true;
						console.println('read file error:', err.message);
					});
					
					// Read decompressed data
					gunzip.on('data', function(chunk) {
						result += String.fromCharCode.apply(null, new Uint8Array(chunk));
					});
					
					gunzip.on('end', function() {
						verifySuccess = (result === testData);
						console.println('decompressed data:', result);
						console.println('verification success:', verifySuccess);
						console.println('read error occurred:', readErrorOccurred);
					});
					
					gunzip.on('error', function(err) {
						readErrorOccurred = true;
						console.println('gunzip error:', err.message);
					});
					
					// Wait for decompression to complete
					const start = Date.now();
					while (Date.now() - start < 500) {
						// wait for decompression to complete
					}
				} catch (e) {
					readErrorOccurred = true;
					console.println('read error:', e.message);
				}
			`,
			output: []string{
				"write error occurred: false",
				"decompressed data: Test data for gzip compression",
				"verification success: true",
				"read error occurred: false",
			},
		},
	}

	for _, tc := range tests {
		RunTest(t, tc)
	}
}
