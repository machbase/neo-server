package parser

import (
	"bytes"
	"strings"
	"testing"

	"github.com/machbase/neo-server/v8/jsh/engine"
	"github.com/machbase/neo-server/v8/jsh/native/stream"
	"github.com/machbase/neo-server/v8/jsh/root"
)

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
			FSTabs: []engine.FSTab{root.RootFSTab(), {MountPoint: "/work", Source: "../../test/"}},
			Env:    tc.vars,
			Reader: &bytes.Buffer{},
			Writer: &bytes.Buffer{},
		}
		jr, err := engine.New(conf)
		if err != nil {
			t.Fatalf("Failed to create JSRuntime: %v", err)
		}
		jr.RegisterNativeModule("@jsh/process", jr.Process)
		jr.RegisterNativeModule("@jsh/fs", jr.Filesystem)
		jr.RegisterNativeModule("@jsh/stream", stream.Module)
		jr.RegisterNativeModule("@jsh/parser", Module)

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

func TestParserModule(t *testing.T) {
	tests := []TestCase{
		{
			name: "module-exports",
			script: `
				const parser = require('/lib/parser');
				console.println('csv:', typeof parser.csv);
				console.println('ndjson:', typeof parser.ndjson);
				console.println('CSVParser:', typeof parser.CSVParser);
				console.println('NDJSONParser:', typeof parser.NDJSONParser);
			`,
			output: []string{
				"csv: function",
				"ndjson: function",
				"CSVParser: function",
				"NDJSONParser: function",
			},
		},
		{
			name: "csv-basic-parsing",
			script: `
				const fs = require('/lib/fs');
				const parser = require('/lib/parser');
				
				const csvContent = 'name,age,city\nAlice,30,New York\nBob,25,Los Angeles';
				fs.writeFileSync('/work/test.csv', csvContent);
				
				let rowCount = 0;
				let headerReceived = false;
				
				fs.createReadStream('/work/test.csv')
					.pipe(parser.csv())
					.on('headers', (headers) => {
						headerReceived = true;
						console.println('Headers:', headers.join(','));
					})
					.on('data', (row) => {
						rowCount++;
						console.println('Row ' + rowCount + ':', row.name + ',' + row.age + ',' + row.city);
					})
					.on('end', () => {
						console.println('Total rows:', rowCount);
						fs.unlinkSync('/work/test.csv');
					});
			`,
			output: []string{
				"Headers: name,age,city",
				"Row 1: Alice,30,New York",
				"Row 2: Bob,25,Los Angeles",
				"Total rows: 2",
			},
		},
		{
			name: "csv-tsv-parsing",
			script: `
				const fs = require('/lib/fs');
				const parser = require('/lib/parser');
				
				const tsvContent = 'name\tage\tcity\nDavid\t40\tBoston\nEve\t28\tSeattle';
				fs.writeFileSync('/work/test.tsv', tsvContent);
				
				let count = 0;
				
				fs.createReadStream('/work/test.tsv')
					.pipe(parser.csv({ separator: '\t' }))
					.on('data', (row) => {
						count++;
						console.println(row.name + ',' + row.age + ',' + row.city);
					})
					.on('end', () => {
						console.println('Count:', count);
						fs.unlinkSync('/work/test.tsv');
					});
			`,
			output: []string{
				"David,40,Boston",
				"Eve,28,Seattle",
				"Count: 2",
			},
		},
		{
			name: "csv-quoted-fields",
			script: `
				const fs = require('/lib/fs');
				const parser = require('/lib/parser');
				
				const csvContent = 'name,description,price\n"Product A","A product with, comma",19.99\n"Product B","Another product",29.99';
				fs.writeFileSync('/work/quoted.csv', csvContent);
				
				let count = 0;
				
				fs.createReadStream('/work/quoted.csv')
					.pipe(parser.csv())
					.on('data', (row) => {
						count++;
						console.println(count + ': ' + row.name + ' - ' + row.price);
					})
					.on('end', () => {
						fs.unlinkSync('/work/quoted.csv');
					});
			`,
			output: []string{
				"1: Product A - 19.99",
				"2: Product B - 29.99",
			},
		},
		{
			name: "csv-custom-separator",
			script: `
				const fs = require('/lib/fs');
				const parser = require('/lib/parser');
				
				const csvContent = 'name;age;city\nAlice;30;New York\nBob;25;Los Angeles';
				fs.writeFileSync('/work/semicolon.csv', csvContent);
				
				let count = 0;
				
				fs.createReadStream('/work/semicolon.csv')
					.pipe(parser.csv({ separator: ';' }))
					.on('data', (row) => {
						count++;
						console.println(count + ': ' + row.name);
					})
					.on('end', () => {
						console.println('Done');
						fs.unlinkSync('/work/semicolon.csv');
					});
			`,
			output: []string{
				"1: Alice",
				"2: Bob",
				"Done",
			},
		},
		{
			name: "csv-mapValues",
			script: `
				const fs = require('/lib/fs');
				const parser = require('/lib/parser');
				
				const csvContent = 'name,age,city\nAlice,30,New York\nBob,25,Los Angeles';
				fs.writeFileSync('/work/map.csv', csvContent);
				
				fs.createReadStream('/work/map.csv')
					.pipe(parser.csv({
						mapValues: ({ header, value }) => {
							if (header === 'age') {
								return parseInt(value, 10);
							}
							return value;
						}
					}))
					.on('data', (row) => {
						console.println(row.name + ',' + typeof row.age + ',' + row.age);
					})
					.on('end', () => {
						fs.unlinkSync('/work/map.csv');
					});
			`,
			output: []string{
				"Alice,number,30",
				"Bob,number,25",
			},
		},
		{
			name: "ndjson-basic-parsing",
			script: `
				const fs = require('/lib/fs');
				const parser = require('/lib/parser');
				
				const ndjsonContent = '{"name":"Alice","age":30,"city":"New York"}\n{"name":"Bob","age":25,"city":"Los Angeles"}\n{"name":"Charlie","age":35,"city":"Chicago"}';
				fs.writeFileSync('/work/test.ndjson', ndjsonContent);
				
				let count = 0;
				
				fs.createReadStream('/work/test.ndjson')
					.pipe(parser.ndjson())
					.on('data', (obj) => {
						count++;
						console.println(count + ': ' + obj.name + ',' + obj.age + ',' + obj.city);
					})
					.on('end', () => {
						console.println('Total:', count);
						fs.unlinkSync('/work/test.ndjson');
					});
			`,
			output: []string{
				"1: Alice,30,New York",
				"2: Bob,25,Los Angeles",
				"3: Charlie,35,Chicago",
				"Total: 3",
			},
		},
		{
			name: "ndjson-non-strict-mode",
			script: `
				const fs = require('/lib/fs');
				const parser = require('/lib/parser');
				
				const ndjsonContent = '{"name":"Valid JSON"}\nInvalid JSON line\n{"name":"Another valid JSON"}';
				fs.writeFileSync('/work/invalid.ndjson', ndjsonContent);
				
				let count = 0;
				let warnings = 0;
				
				fs.createReadStream('/work/invalid.ndjson')
					.pipe(parser.ndjson({ strict: false }))
					.on('data', (obj) => {
						count++;
						console.println(count + ': ' + obj.name);
					})
					.on('warning', (warn) => {
						warnings++;
						console.println('Warning at line ' + warn.line);
					})
					.on('end', () => {
						console.println('Valid objects:', count);
						console.println('Warnings:', warnings);
						fs.unlinkSync('/work/invalid.ndjson');
					});
			`,
			output: []string{
				"1: Valid JSON",
				"Warning at line 2",
				"2: Another valid JSON",
				"Valid objects: 2",
				"Warnings: 1",
			},
		},
		{
			name: "ndjson-empty-lines",
			script: `
				const fs = require('/lib/fs');
				const parser = require('/lib/parser');
				
				const ndjsonContent = '{"id":1}\n\n{"id":2}\n\n\n{"id":3}';
				fs.writeFileSync('/work/empty.ndjson', ndjsonContent);
				
				let count = 0;
				
				fs.createReadStream('/work/empty.ndjson')
					.pipe(parser.ndjson())
					.on('data', (obj) => {
						count++;
						console.println('ID:', obj.id);
					})
					.on('end', () => {
						console.println('Count:', count);
						fs.unlinkSync('/work/empty.ndjson');
					});
			`,
			output: []string{
				"ID: 1",
				"ID: 2",
				"ID: 3",
				"Count: 3",
			},
		},
		{
			name: "ndjson-different-structures",
			script: `
				const fs = require('/lib/fs');
				const parser = require('/lib/parser');
				
				const ndjsonContent = '{"type":"person","name":"Alice","age":30}\n{"type":"product","name":"Laptop","price":999.99}\n{"type":"person","name":"Bob","age":25}';
				fs.writeFileSync('/work/mixed.ndjson', ndjsonContent);
				
				let persons = 0;
				let products = 0;
				
				fs.createReadStream('/work/mixed.ndjson')
					.pipe(parser.ndjson())
					.on('data', (obj) => {
						if (obj.type === 'person') {
							persons++;
							console.println('Person:', obj.name);
						} else if (obj.type === 'product') {
							products++;
							console.println('Product:', obj.name);
						}
					})
					.on('end', () => {
						console.println('Persons:', persons);
						console.println('Products:', products);
						fs.unlinkSync('/work/mixed.ndjson');
					});
			`,
			output: []string{
				"Person: Alice",
				"Product: Laptop",
				"Person: Bob",
				"Persons: 2",
				"Products: 1",
			},
		},
		{
			name: "csv-no-headers",
			script: `
				const fs = require('/lib/fs');
				const parser = require('/lib/parser');
				
				const csvContent = 'Alice,30,New York\nBob,25,Los Angeles';
				fs.writeFileSync('/work/noheader.csv', csvContent);
				
				fs.createReadStream('/work/noheader.csv')
					.pipe(parser.csv({ headers: false }))
					.on('data', (row) => {
						console.println(row['0'] + ',' + row['1'] + ',' + row['2']);
					})
					.on('end', () => {
						fs.unlinkSync('/work/noheader.csv');
					});
			`,
			output: []string{
				"Alice,30,New York",
				"Bob,25,Los Angeles",
			},
		},
		{
			name: "csv-custom-headers",
			script: `
				const fs = require('/lib/fs');
				const parser = require('/lib/parser');
				
				const csvContent = 'Alice,30,New York\nBob,25,Los Angeles';
				fs.writeFileSync('/work/custom.csv', csvContent);
				
				fs.createReadStream('/work/custom.csv')
					.pipe(parser.csv({ headers: ['fullname', 'years', 'location'] }))
					.on('data', (row) => {
						console.println(row.fullname + ' from ' + row.location);
					})
					.on('end', () => {
						fs.unlinkSync('/work/custom.csv');
					});
			`,
			output: []string{
				"Alice from New York",
				"Bob from Los Angeles",
			},
		},
	}

	for _, tc := range tests {
		RunTest(t, tc)
	}
}
