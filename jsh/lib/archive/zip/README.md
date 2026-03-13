# zip Module for jsh

ZIP archive module for jsh. This module handles file-entry based ZIP archive creation and extraction.

## Installation

```javascript
const zip = require('archive/zip');
```

## API

### Convenience functions

- `zipSync(data)` creates a ZIP archive from a single payload or an array of `{ name, data }` entries.
- `unzipSync(buffer)` extracts ZIP archive bytes and returns an array of entry objects.
- `zip(data, callback)` and `unzip(buffer, callback)` provide callback-style async wrappers.

### Streams

- `createZip()` buffers written entries and emits archive bytes through `data` on `end()`.
- `createUnzip()` buffers archive bytes and emits each extracted file through `entry` on `end()`.

### Zip class

```javascript
const zip = require('archive/zip');

const z = new zip.Zip();
z.addFile('./input/file1.txt');
z.addFile('./input/file2.txt');
z.writeTo('file.zip');
```

```javascript
const zip = require('archive/zip');

const z = new zip.Zip('file.zip');
z.extractAllTo('/output_folder', true);
```

```javascript
const zip = require('archive/zip');

const z = new zip.Zip('/tmp/file.zip');
const entries = z.getEntries();

for (const entry of entries) {
	console.println(entry.name, entry.size);
}
```

`Zip` supports:

- `new Zip(filePath?)`
- `addFile(filePath, entryName?)`
- `addBuffer(data, entryName, options?)`
- `addEntry({ name, data, comment?, method? })`
- `getEntries()`
- `writeTo(filePath)`
- `extractAllTo(outputDir, overwrite)`

`extractAllTo()` also accepts an options object:

```javascript
z.extractAllTo('/output_folder', {
	overwrite: true,
	filter: function(entry) {
		return entry.name.endsWith('.txt') && !entry.name.startsWith('tmp/');
	}
});
```

`filter` can be a callback that receives each entry and returns `true` or `false`.

```javascript
const zip = require('archive/zip');

const z = new zip.Zip('file.zip');
z.extractAllTo('/output_folder', {
	overwrite: true,
	filter(entry) {
		return entry.name === 'keep.txt';
	}
});
```

Entry objects returned by `getEntries()` and `unzipSync()` include `name`, `data`, `comment`, `method`, `compressedSize`, `size`, `isDir`, and `modified`.