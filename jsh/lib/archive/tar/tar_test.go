package tar_test

import (
	"testing"

	"github.com/dop251/goja"
	tarlib "github.com/machbase/neo-server/v8/jsh/lib/archive/tar"
	"github.com/machbase/neo-server/v8/jsh/test_engine"
)

func TestTarModule(t *testing.T) {
	rt := goja.New()
	module := rt.NewObject()
	exports := rt.NewObject()
	module.Set("exports", exports)
	tarlib.Module(rt, module)
	exportsObj := module.Get("exports").(*goja.Object)
	for _, name := range []string{"createTar", "createUntar", "tar", "untar", "tarSync", "untarSync"} {
		if exportsObj.Get(name) == nil || goja.IsUndefined(exportsObj.Get(name)) {
			t.Errorf("Expected %s to be exported", name)
		}
	}
}

func TestTarSync(t *testing.T) {
	tests := []test_engine.TestCase{
		{
			Name: "tarSync-untarSync-single-entry",
			Script: `
				const tar = require('archive/tar');
				const archive = tar.tarSync('tar payload');
				const entries = tar.untarSync(archive);
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
				"entry text: tar payload",
			},
		},
		{
			Name: "tarSync-untarSync-multi-entry",
			Script: `
				const tar = require('archive/tar');
				const archive = tar.tarSync([
					{ name: 'alpha.txt', data: 'Alpha' },
					{ name: 'dir/beta.txt', data: 'Beta' }
				]);
				const entries = tar.untarSync(archive);
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
		{
			Name: "tarSync-untarSync-directory-and-symlink-entries",
			Script: `
				const tar = require('archive/tar');
				const archive = tar.tarSync([
					{ name: 'assets', isDir: true, type: 'dir' },
					{ name: 'assets/readme.txt', data: 'hello' },
					{ name: 'current', type: 'symlink', linkname: 'assets/readme.txt' }
				]);
				const entries = tar.untarSync(archive);
				console.println('entry count:', entries.length);
				console.println('dir entry:', entries[0].name + '|' + entries[0].isDir + '|' + entries[0].type);
				console.println('file entry:', entries[1].name + '|' + entries[1].size + '|' + entries[1].type);
				console.println('link entry:', entries[2].name + '|' + entries[2].type + '|' + entries[2].linkname);
			`,
			Output: []string{
				"entry count: 3",
				"dir entry: assets/|true|dir",
				"file entry: assets/readme.txt|5|file",
				"link entry: current|symlink|assets/readme.txt",
			},
		},
	}
	for _, tc := range tests {
		test_engine.RunTest(t, tc)
	}
}

func TestTarStream(t *testing.T) {
	tests := []test_engine.TestCase{
		{
			Name: "createTar-createUntar-entry-events",
			Script: `
				const tar = require('archive/tar');
				const archiveWriter = tar.createTar();
				let archive = null;

				archiveWriter.on('data', function(chunk) {
					archive = chunk;
				});

				archiveWriter.on('end', function() {
					const archiveReader = tar.createUntar();
					archiveReader.on('entry', function(entry) {
						const text = String.fromCharCode.apply(null, new Uint8Array(entry.data));
						console.println('entry:', entry.name + '=' + text);
					});
					archiveReader.on('end', function() {
						console.println('untar ended:', true);
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
				"untar ended: true",
			},
		},
	}
	for _, tc := range tests {
		test_engine.RunTest(t, tc)
	}
}

func TestTarClass(t *testing.T) {
	tests := []test_engine.TestCase{
		{
			Name: "tar-class-addFile-writeTo",
			Script: `
				const fs = require('fs');
				const tar = require('archive/tar');
				const base = '/tmp/jsh-tar-class';
				fs.rmSync(base, { recursive: true, force: true });
				fs.mkdirSync(base + '/input', { recursive: true });
				fs.writeFileSync(base + '/input/file1.txt', 'alpha', 'utf8');
				fs.writeFileSync(base + '/input/file2.txt', 'beta', 'utf8');

				const t = new tar.Tar();
				t.addFile(base + '/input/file1.txt');
				t.addFile(base + '/input/file2.txt');
				t.writeTo(base + '/file.tar');

				const entries = tar.untarSync(fs.readFileSync(base + '/file.tar', 'buffer'));
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
			Name: "tar-class-addBuffer-addEntry",
			Script: `
				const tar = require('archive/tar');
				const t = new tar.Tar();
				t.addBuffer('alpha', 'alpha.txt');
				t.addEntry({ name: 'beta.txt', data: 'beta' });
				const entriesBeforeWrite = t.getEntries();
				const archive = tar.tarSync(entriesBeforeWrite);
				const entries = tar.untarSync(archive);
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
			Name: "tar-class-extractAllTo",
			Script: `
				const fs = require('fs');
				const tar = require('archive/tar');
				const base = '/tmp/jsh-tar-extract';
				fs.rmSync(base, { recursive: true, force: true });
				fs.mkdirSync(base, { recursive: true });
				const archive = tar.tarSync([
					{ name: 'dir/a.txt', data: 'A' },
					{ name: 'dir/b.txt', data: 'B' }
				]);
				fs.writeFileSync(base + '/file.tar', Array.from(new Uint8Array(archive)), 'buffer');

				const t = new tar.Tar(base + '/file.tar');
				t.extractAllTo(base + '/out', true);

				console.println('a:', fs.readFileSync(base + '/out/dir/a.txt', 'utf8'));
				console.println('b:', fs.readFileSync(base + '/out/dir/b.txt', 'utf8'));
			`,
			Output: []string{
				"a: A",
				"b: B",
			},
		},
		{
			Name: "tar-class-extractAllTo-callback-filter-and-conflict",
			Script: `
				const fs = require('fs');
				const tar = require('archive/tar');
				const base = '/tmp/jsh-tar-filter';
				fs.rmSync(base, { recursive: true, force: true });
				fs.mkdirSync(base, { recursive: true });
				const archive = tar.tarSync([
					{ name: 'keep.txt', data: 'KEEP' },
					{ name: 'skip.txt', data: 'SKIP' }
				]);
				fs.writeFileSync(base + '/file.tar', Array.from(new Uint8Array(archive)), 'buffer');
				const t = new tar.Tar(base + '/file.tar');
				t.extractAllTo(base + '/out', {
					overwrite: true,
					filter: function(entry) {
						return entry.name === 'keep.txt';
					}
				});
				console.println('keep exists:', fs.existsSync(base + '/out/keep.txt'));
				console.println('skip exists:', fs.existsSync(base + '/out/skip.txt'));
				let msg = '';
				try {
					t.extractAllTo(base + '/out', false, {
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
			Name: "tar-class-getEntries-from-file",
			Script: `
				const fs = require('fs');
				const tar = require('archive/tar');
				const base = '/tmp/jsh-tar-getentries';
				fs.rmSync(base, { recursive: true, force: true });
				fs.mkdirSync(base, { recursive: true });
				const archive = tar.tarSync([
					{ name: 'one.txt', data: 'one' },
					{ name: 'two.txt', data: 'two' }
				]);
				fs.writeFileSync(base + '/file.tar', Array.from(new Uint8Array(archive)), 'buffer');
				const t = new tar.Tar(base + '/file.tar');
				const entries = t.getEntries();
				console.println('entry count:', entries.length);
				console.println('entry names:', entries.map(e => e.name).join(','));
			`,
			Output: []string{
				"entry count: 2",
				"entry names: one.txt,two.txt",
			},
		},
		{
			Name: "tar-class-addEntry-directory-and-link-metadata",
			Script: `
				const tar = require('archive/tar');
				const t = new tar.Tar();
				t.addEntry({ name: 'docs', isDir: true, type: 'dir' });
				t.addEntry({ name: 'docs/readme.txt', data: 'doc' });
				t.addEntry({ name: 'latest', type: 'symlink', linkname: 'docs/readme.txt' });
				const entries = tar.untarSync(t.tarSync ? t.tarSync() : tar.tarSync(t.getEntries()));
				console.println('dir entry:', entries[0].name + '|' + entries[0].isDir + '|' + entries[0].type);
				console.println('file entry:', entries[1].name + '|' + entries[1].size + '|' + entries[1].type);
				console.println('link entry:', entries[2].name + '|' + entries[2].type + '|' + entries[2].linkname);
			`,
			Output: []string{
				"dir entry: docs/|true|dir",
				"file entry: docs/readme.txt|3|file",
				"link entry: latest|symlink|docs/readme.txt",
			},
		},
	}
	for _, tc := range tests {
		test_engine.RunTest(t, tc)
	}
}
