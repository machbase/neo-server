# tar Module for jsh

TAR archive module for jsh. This module handles file-entry based TAR archive creation and extraction.

## Installation

```javascript
const tar = require('archive/tar');
```

## API

### Convenience functions

- `tarSync(data)` creates a TAR archive from a single payload or an array of `{ name, data }` entries.
- `untarSync(buffer)` extracts TAR archive bytes and returns an array of entry objects.
- `tar(data, callback)` and `untar(buffer, callback)` provide callback-style async wrappers.

### Streams

- `createTar()` buffers written entries and emits archive bytes through `data` on `end()`.
- `createUntar()` buffers archive bytes and emits each extracted file through `entry` on `end()`.

### Tar class

```javascript
const tar = require('archive/tar');

const t = new tar.Tar();
t.addFile('./input/file1.txt');
t.addFile('./input/file2.txt');
t.writeTo('file.tar');
```

```javascript
const tar = require('archive/tar');

const t = new tar.Tar('file.tar');
t.extractAllTo('/output_folder', true);
```

```javascript
const tar = require('archive/tar');

const t = new tar.Tar('/tmp/file.tar');
const entries = t.getEntries();

for (const entry of entries) {
	console.println(entry.name, entry.size, entry.type);
}
```

```javascript
const tar = require('archive/tar');

const t = new tar.Tar();
t.addEntry({ name: 'assets', isDir: true, type: 'dir' });
t.addEntry({ name: 'assets/readme.txt', data: 'hello tar' });
t.addEntry({ name: 'current', type: 'symlink', linkname: 'assets/readme.txt' });
t.writeTo('bundle.tar');
```

`Tar` supports:

- `new Tar(filePath?)`
- `addFile(filePath, entryName?)`
- `addBuffer(data, entryName, options?)`
- `addEntry({ name, data, mode?, modified?, type?, typeflag?, linkname?, isDir? })`
- `getEntries()`
- `writeTo(filePath)`
- `extractAllTo(outputDir, overwrite)`

`extractAllTo()` also accepts an options object:

```javascript
t.extractAllTo('/output_folder', {
	overwrite: true,
	filter: function(entry) {
		return entry.name.endsWith('.txt') && !entry.name.startsWith('tmp/');
	}
});
```

`filter` can be a callback that receives each entry and returns `true` or `false`.

```javascript
const tar = require('archive/tar');

const t = new tar.Tar('file.tar');
t.extractAllTo('/output_folder', {
	overwrite: true,
	filter(entry) {
		return entry.name === 'keep.txt';
	}
});
```

Entry objects returned by `getEntries()` and `untarSync()` include `name`, `data`, `mode`, `size`, `isDir`, `modified`, `typeflag`, `type`, and `linkname`.

Directory entries can be created with `isDir: true` or `type: 'dir'`. Symbolic and hard links can be described with `type: 'symlink'` or `type: 'link'` together with `linkname`.