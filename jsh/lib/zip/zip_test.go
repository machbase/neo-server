package zip_test

import (
	"testing"

	"github.com/dop251/goja"
	ziplib "github.com/machbase/neo-server/v8/jsh/lib/zip"
	"github.com/machbase/neo-server/v8/jsh/test_engine"
)

func TestZipModule(t *testing.T) {
	rt := goja.New()
	module := rt.NewObject()
	exports := rt.NewObject()
	module.Set("exports", exports)
	ziplib.Module(rt, module)
	exportsObj := module.Get("exports").(*goja.Object)
	for _, name := range []string{"createZip", "createUnzip", "zip", "unzip", "zipSync", "unzipSync"} {
		if exportsObj.Get(name) == nil || goja.IsUndefined(exportsObj.Get(name)) {
			t.Errorf("Expected %s to be exported", name)
		}
	}
}

func TestZipSync(t *testing.T) {
	tests := []test_engine.TestCase{
		{
			Name: "zipSync-unzipSync-single-entry",
			Script: `
				const zip = require('zip');
				const archive = zip.zipSync('zip payload');
				const entries = zip.unzipSync(archive);
				const text = String.fromCharCode.apply(null, new Uint8Array(entries[0].data));
				console.println('archive type:', archive.constructor.name);
				console.println('entry count:', entries.length);
				console.println('entry name:', entries[0].name);
				console.println('entry text:', text);
			`,
			Output: []string{
				"archive type: ArrayBuffer",
				"entry count: 1",
				"entry name: data",
				"entry text: zip payload",
			},
		},
		{
			Name: "zipSync-unzipSync-multi-entry",
			Script: `
				const zip = require('zip');
				const archive = zip.zipSync([
					{ name: 'alpha.txt', data: 'Alpha' },
					{ name: 'dir/beta.txt', data: 'Beta' }
				]);
				const entries = zip.unzipSync(archive);
				const value0 = String.fromCharCode.apply(null, new Uint8Array(entries[0].data));
				const value1 = String.fromCharCode.apply(null, new Uint8Array(entries[1].data));
				console.println('entry count:', entries.length);
				console.println('entry0:', entries[0].name + '=' + value0);
				console.println('entry1:', entries[1].name + '=' + value1);
			`,
			Output: []string{
				"entry count: 2",
				"entry0: alpha.txt=Alpha",
				"entry1: dir/beta.txt=Beta",
			},
		},
	}
	for _, tc := range tests {
		test_engine.RunTest(t, tc)
	}
}

func TestZipStream(t *testing.T) {
	tests := []test_engine.TestCase{
		{
			Name: "createZip-createUnzip-entry-events",
			Script: `
				const zip = require('zip');
				const archiveWriter = zip.createZip();
				let archive = null;

				archiveWriter.on('data', function(chunk) {
					archive = chunk;
				});

				archiveWriter.on('end', function() {
					const archiveReader = zip.createUnzip();
					archiveReader.on('entry', function(entry) {
						const text = String.fromCharCode.apply(null, new Uint8Array(entry.data));
						console.println('entry:', entry.name + '=' + text);
					});
					archiveReader.on('end', function() {
						console.println('unzip ended:', true);
					});
					archiveReader.write(archive);
					archiveReader.end();
				});

				archiveWriter.write({ name: 'one.txt', data: 'One' });
				archiveWriter.write({ name: 'two.txt', data: 'Two' });
				archiveWriter.end();
			`,
			Output: []string{
				"entry: one.txt=One",
				"entry: two.txt=Two",
				"unzip ended: true",
			},
		},
	}
	for _, tc := range tests {
		test_engine.RunTest(t, tc)
	}
}

func TestZipClass(t *testing.T) {
	tests := []test_engine.TestCase{
		{
			Name: "zip-class-addFile-writeTo",
			Script: `
				const fs = require('fs');
				const zip = require('zip');
				const base = '/tmp/jsh-zip-class';
				fs.rmSync(base, { recursive: true, force: true });
				fs.mkdirSync(base + '/input', { recursive: true });
				fs.writeFileSync(base + '/input/file1.txt', 'alpha', 'utf8');
				fs.writeFileSync(base + '/input/file2.txt', 'beta', 'utf8');

				const z = new zip.Zip();
				z.addFile(base + '/input/file1.txt');
				z.addFile(base + '/input/file2.txt');
				z.writeTo(base + '/file.zip');

				const entries = zip.unzipSync(fs.readFileSync(base + '/file.zip', 'buffer'));
				console.println('entry count:', entries.length);
				console.println('entry0 name:', entries[0].name);
				console.println('entry1 name:', entries[1].name);
			`,
			Output: []string{
				"entry count: 2",
				"entry0 name: file1.txt",
				"entry1 name: file2.txt",
			},
		},
		{
			Name: "zip-class-addBuffer-addEntry",
			Script: `
				const zip = require('zip');
				const z = new zip.Zip();
				z.addBuffer('alpha', 'alpha.txt');
				z.addEntry({ name: 'beta.txt', data: 'beta' });
				const entriesBeforeWrite = z.getEntries();
				const archive = zip.zipSync(entriesBeforeWrite);
				const entries = zip.unzipSync(archive);
				const a = String.fromCharCode.apply(null, new Uint8Array(entries[0].data));
				const b = String.fromCharCode.apply(null, new Uint8Array(entries[1].data));
				console.println('entry count:', entriesBeforeWrite.length);
				console.println('entry0:', entries[0].name + '=' + a);
				console.println('entry1:', entries[1].name + '=' + b);
			`,
			Output: []string{
				"entry count: 2",
				"entry0: alpha.txt=alpha",
				"entry1: beta.txt=beta",
			},
		},
		{
			Name: "zip-class-extractAllTo",
			Script: `
				const fs = require('fs');
				const zip = require('zip');
				const base = '/tmp/jsh-zip-extract';
				fs.rmSync(base, { recursive: true, force: true });
				fs.mkdirSync(base, { recursive: true });
				const archive = zip.zipSync([
					{ name: 'dir/a.txt', data: 'A' },
					{ name: 'dir/b.txt', data: 'B' }
				]);
				fs.writeFileSync(base + '/file.zip', Array.from(new Uint8Array(archive)), 'buffer');

				const z = new zip.Zip(base + '/file.zip');
				z.extractAllTo(base + '/out', true);

				console.println('a:', fs.readFileSync(base + '/out/dir/a.txt', 'utf8'));
				console.println('b:', fs.readFileSync(base + '/out/dir/b.txt', 'utf8'));
			`,
			Output: []string{
				"a: A",
				"b: B",
			},
		},
		{
			Name: "zip-class-extractAllTo-callback-filter-and-conflict",
			Script: `
				const fs = require('fs');
				const zip = require('zip');
				const base = '/tmp/jsh-zip-filter';
				fs.rmSync(base, { recursive: true, force: true });
				fs.mkdirSync(base, { recursive: true });
				const archive = zip.zipSync([
					{ name: 'keep.txt', data: 'KEEP' },
					{ name: 'skip.txt', data: 'SKIP' }
				]);
				fs.writeFileSync(base + '/file.zip', Array.from(new Uint8Array(archive)), 'buffer');
				const z = new zip.Zip(base + '/file.zip');
				z.extractAllTo(base + '/out', {
					overwrite: true,
					filter: function(entry) {
						return entry.name === 'keep.txt';
					}
				});
				console.println('keep exists:', fs.existsSync(base + '/out/keep.txt'));
				console.println('skip exists:', fs.existsSync(base + '/out/skip.txt'));
				let msg = '';
				try {
					z.extractAllTo(base + '/out', false, {
						filter: function(entry) {
							return entry.name === 'keep.txt';
						}
					});
				} catch (err) {
					msg = err.message;
				}
				console.println('has conflict msg:', msg.includes('overwrite is false') && msg.includes('keep.txt'));
			`,
			Output: []string{
				"keep exists: true",
				"skip exists: false",
				"has conflict msg: true",
			},
		},
		{
			Name: "zip-class-getEntries-from-file",
			Script: `
				const fs = require('fs');
				const zip = require('zip');
				const base = '/tmp/jsh-zip-getentries';
				fs.rmSync(base, { recursive: true, force: true });
				fs.mkdirSync(base, { recursive: true });
				const archive = zip.zipSync([
					{ name: 'one.txt', data: 'one' },
					{ name: 'two.txt', data: 'two' }
				]);
				fs.writeFileSync(base + '/file.zip', Array.from(new Uint8Array(archive)), 'buffer');
				const z = new zip.Zip(base + '/file.zip');
				const entries = z.getEntries();
				console.println('entry count:', entries.length);
				console.println('entry names:', entries.map(e => e.name).join(','));
			`,
			Output: []string{
				"entry count: 2",
				"entry names: one.txt,two.txt",
			},
		},
	}
	for _, tc := range tests {
		test_engine.RunTest(t, tc)
	}
}
