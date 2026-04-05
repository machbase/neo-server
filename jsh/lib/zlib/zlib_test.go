package zlib_test

import (
	"bytes"
	"compress/gzip"
	"context"
	"testing"

	"github.com/dop251/goja"
	"github.com/machbase/neo-server/v8/jsh/lib/zlib"
	"github.com/machbase/neo-server/v8/jsh/test_engine"
)

func gzipTestInput(t *testing.T, data string) []byte {
	t.Helper()

	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	if _, err := zw.Write([]byte(data)); err != nil {
		t.Fatalf("failed to write gzip input: %v", err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("failed to close gzip writer: %v", err)
	}
	return buf.Bytes()
}

func TestZlibModule(t *testing.T) {
	rt := goja.New()

	// Create module object
	module := rt.NewObject()
	exports := rt.NewObject()
	module.Set("exports", exports)

	// Initialize zlib module
	zlib.Module(context.Background(), rt, module)

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

func TestZlibSync(t *testing.T) {
	tests := []test_engine.TestCase{
		{
			Name: "gzipSync-gunzipSync",
			Script: `
				const zlib = require('zlib');
				const testData = "Hello, World! This is a test string for compression.";
				
				const compressed = zlib.gzipSync(testData);
				console.println('compressed size:', compressed.byteLength);
				console.println('compressed type:', compressed.constructor.name);
				
				const decompressed = zlib.gunzipSync(compressed);
				const result = String.fromCharCode.apply(null, new Uint8Array(decompressed));
				
				console.println('result:', result);
			`,
			Output: []string{
				"compressed size: 76",
				"compressed type: ArrayBuffer",
				"result: Hello, World! This is a test string for compression.",
			},
		},
		{
			Name: "deflateSync-inflateSync",
			Script: `
				const zlib = require('zlib');
				const testData = "Test data for deflate compression";
				
				const compressed = zlib.deflateSync(testData);
				console.println('compressed size:', compressed.byteLength);
				
				const decompressed = zlib.inflateSync(compressed);
				const result = String.fromCharCode.apply(null, new Uint8Array(decompressed));
				
				console.println('result:', result);
			`,
			Output: []string{
				"compressed size: 39",
				"result: Test data for deflate compression",
			},
		},
		{
			Name: "deflateRawSync-inflateRawSync",
			Script: `
				const zlib = require('zlib');
				const testData = "Raw deflate test";
				
				const compressed = zlib.deflateRawSync(testData);
				const decompressed = zlib.inflateRawSync(compressed);
				const result = String.fromCharCode.apply(null, new Uint8Array(decompressed));
				
				console.println('result:', result);
			`,
			Output: []string{
				"result: Raw deflate test",
			},
		},
		{
			Name: "constants",
			Script: `
				const zlib = require('zlib');
				const c = zlib.constants;
				
				console.println('Z_NO_FLUSH:', typeof c.Z_NO_FLUSH);
				console.println('Z_BEST_COMPRESSION:', c.Z_BEST_COMPRESSION);
				console.println('Z_DEFAULT_COMPRESSION:', c.Z_DEFAULT_COMPRESSION);
			`,
			Output: []string{
				"Z_NO_FLUSH: number",
				"Z_BEST_COMPRESSION: 9",
				"Z_DEFAULT_COMPRESSION: -1",
			},
		},
		{
			Name: "destructuring",
			Script: `
				const { gzipSync, gunzipSync, constants } = require('zlib');
				
				const data = "test";
				const compressed = gzipSync(data);
				const decompressed = gunzipSync(compressed);
				const result = String.fromCharCode.apply(null, new Uint8Array(decompressed));
				
				console.println('result:', result);
				console.println('has constants:', typeof constants);
			`,
			Output: []string{
				"result: test",
				"has constants: object",
			},
		},
	}

	for _, tc := range tests {
		test_engine.RunTest(t, tc)
	}
}

func TestZlibStream(t *testing.T) {
	tests := []test_engine.TestCase{
		{
			Name: "createGzip-stream-methods",
			Script: `
				const zlib = require('zlib');
				const gzip = zlib.createGzip();
				
				console.println('has write:', typeof gzip.write);
				console.println('has end:', typeof gzip.end);
				console.println('has on:', typeof gzip.on);
				console.println('has pipe:', typeof gzip.pipe);
			`,
			Output: []string{
				"has write: function",
				"has end: function",
				"has on: function",
				"has pipe: function",
			},
		},
		{
			Name: "createGunzip-stream-methods",
			Script: `
				const zlib = require('zlib');
				const gunzip = zlib.createGunzip();
				
				console.println('has write:', typeof gunzip.write);
				console.println('has end:', typeof gunzip.end);
				console.println('has on:', typeof gunzip.on);
			`,
			Output: []string{
				"has write: function",
				"has end: function",
				"has on: function",
			},
		},
		{
			Name: "createDeflate-createInflate",
			Script: `
				const zlib = require('zlib');
				
				console.println('createDeflate:', typeof zlib.createDeflate);
				console.println('createInflate:', typeof zlib.createInflate);
				console.println('createDeflateRaw:', typeof zlib.createDeflateRaw);
				console.println('createInflateRaw:', typeof zlib.createInflateRaw);
				console.println('createUnzip:', typeof zlib.createUnzip);
			`,
			Output: []string{
				"createDeflate: function",
				"createInflate: function",
				"createDeflateRaw: function",
				"createInflateRaw: function",
				"createUnzip: function",
			},
		},
	}

	for _, tc := range tests {
		test_engine.RunTest(t, tc)
	}
}

func TestZlibPipe(t *testing.T) {
	tests := []test_engine.TestCase{
		{
			Name: "pipe-with-invalid-dest",
			Script: `
				const zlib = require('zlib');
				
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
			Output: []string{
				"error occurred: true",
				"has map error: true",
			},
		},
		{
			Name: "pipe-with-invalid-type",
			Script: `
				const zlib = require('zlib');
				
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
			Output: []string{
				"error occurred: true",
				"has dest error: true",
			},
		},
		{
			Name: "pipe-with-null",
			Script: `
				const zlib = require('zlib');
				
				const gzip = zlib.createGzip();
				
				let errorOccurred = false;
				
				try {
					gzip.pipe(null);
				} catch (e) {
					errorOccurred = true;
				}
				
				console.println('error occurred:', errorOccurred);
			`,
			Output: []string{
				"error occurred: true",
			},
		},
		{
			Name: "pipe-with-options-end-false",
			Script: `
				const zlib = require('zlib');
				
				const gzip = zlib.createGzip();
				let writeCount = 0;
				let endCalled = false;
				
				const dest = {
					write(chunk) {
						writeCount++;
						return true;
					},
					end() {
						endCalled = true;
					}
				};

				gzip.pipe(dest, { end: false });
				gzip.write('hello');
				gzip.end();

				console.println('write called:', writeCount > 0);
				console.println('end called:', endCalled);
			`,
			Output: []string{
				"write called: true",
				"end called: false",
			},
		},
		{
			Name: "pipe-with-options-default-end-true",
			Script: `
				const zlib = require('zlib');
				
				const gzip = zlib.createGzip();
				let writeCount = 0;
				let endCalled = false;
				
				const dest = {
					write(chunk) {
						writeCount++;
						return true;
					},
					end() {
						endCalled = true;
					}
				};

				gzip.pipe(dest);
				gzip.write('hello');
				gzip.end();

				console.println('write called:', writeCount > 0);
				console.println('end called:', endCalled);
			`,
			Output: []string{
				"write called: true",
				"end called: true",
			},
		},
		{
			Name: "pipe-with-progress-bytes",
			Script: `
				const zlib = require('zlib');
				
				const text = 'NAME,AGE\nAlice,30\nBob,25\nCharlie,40\n';
				const compressed = zlib.gzipSync(text);
				const compressedTotal = compressed.byteLength;

				const gunzip = zlib.createGunzip();
				let outTotal = 0;
				let sawProgress = false;

				gunzip.on('data', function(chunk) {
					outTotal += chunk.byteLength;
					if (gunzip.bytesWritten > 0 && gunzip.bytesRead >= outTotal) {
						sawProgress = true;
					}
				});

				gunzip.on('end', function() {
					console.println('input processed > 0:', gunzip.bytesWritten > 0);
					console.println('output processed > 0:', gunzip.bytesRead > 0);
					console.println('progress observed:', sawProgress);
					console.println('input reached total:', gunzip.bytesWritten === compressedTotal);
					console.println('output reached total:', gunzip.bytesRead === text.length);
				});

				gunzip.write(compressed);
				gunzip.end();
			`,
			Output: []string{
				"input processed > 0: true",
				"output processed > 0: true",
				"progress observed: true",
				"input reached total: true",
				"output reached total: true",
			},
		},
		{
			Name: "pipe-with-file",
			Script: `
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
			Output: []string{
				"write error occurred: false",
				"decompressed data: Test data for gzip compression",
				"verification success: true",
				"read error occurred: false",
			},
		},
		{
			Name: "pipe-with-csv-file",
			Script: `
				const zlib = require('zlib');
				const parser = require('parser');
				const fs = require('fs');
				
				// Clean up any existing file
				const outputPath = '/tmp/output_test.csv.gz';
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
				const testData = 'NAME,AGE\nAlice,30\nBob,25\n';

				try {
					gzip.pipe(outFile);
					gzip.write(testData);
					gzip.end();
					
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
				
				try {
					// Create read stream for compressed file
					const inFile = fs.createReadStream(outputPath, { highWaterMark: 2048, encoding: 'buffer' });
					const gunzip = zlib.createGunzip();
					const csvParser = parser.csv();

					// Pipe input file through gunzip and then through CSV parser
					const parsed = inFile.pipe(gunzip).pipe(csvParser);

					parsed.on('error', function(err) {
						readErrorOccurred = true;
						console.println('read file error:', err.message);
					});

					parsed.on('headers', function(headers) {
						console.println('header:', headers.join('|'));
					});

					parsed.on('data', function(rec) {
						console.println('record:', rec.NAME + ',' + rec.AGE);
					});

					parsed.on('end', function() {
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
			Output: []string{
				"write error occurred: false",
				"header: NAME|AGE",
				"record: Alice,30",
				"record: Bob,25",
				"read error occurred: false",
			},
		},
		{
			Name: "pipe-with-csv-stdin",
			Script: `
				const zlib = require('zlib');
				const parser = require('parser');
				const fs = require('fs');

				let readErrorOccurred = false;

				try {
					const inFile = fs.createReadStream('-', { highWaterMark: 2048, encoding: 'buffer' });
					const gunzip = zlib.createGunzip();
					const csvParser = parser.csv();

					const parsed = inFile.pipe(gunzip).pipe(csvParser);

					parsed.on('error', function(err) {
						readErrorOccurred = true;
						console.println('read file error:', err.message);
					});

					parsed.on('headers', function(headers) {
						console.println('header:', headers.join('|'));
					});

					parsed.on('data', function(rec) {
						console.println('record:', rec.NAME + ',' + rec.AGE);
					});

					parsed.on('end', function() {
						console.println('read error occurred:', readErrorOccurred);
					});

					gunzip.on('error', function(err) {
						readErrorOccurred = true;
						console.println('gunzip error:', err.message);
					});

					const start = Date.now();
					while (Date.now() - start < 500) {
						// wait for decompression to complete
					}
				} catch (e) {
					readErrorOccurred = true;
					console.println('read error:', e.message);
				}
			`,
			InputBytes: gzipTestInput(t, "NAME,AGE\nAlice,30\nBob,25\n"),
			Output: []string{
				"header: NAME|AGE",
				"record: Alice,30",
				"record: Bob,25",
				"read error occurred: false",
			},
		},
	}

	for _, tc := range tests {
		test_engine.RunTest(t, tc)
	}
}
