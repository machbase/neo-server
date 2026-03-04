# parser Module for jsh

Node.js-style streaming decoders for structured text formats.

## Installation

```javascript
const parser = require('/lib/parser');
```

## Features

- CSV decoder stream (`parser.csv()`)
- NDJSON decoder stream (`parser.ndjson()`)
- Streaming-friendly processing via `pipe()` chains
- Progress counters for large-file processing:
  - `bytesWritten`: input bytes consumed by decoder
  - `bytesRead`: bytes parsed/processed by decoder

## API

### parser.csv(options)

Creates a CSV decoder stream (Transform).

```javascript
const csvParser = parser.csv({ separator: ',', headers: true });
```

Common options:

- `separator` (string, default `,`)
- `headers` (`true` | `false` | `string[]`)
- `strict` (boolean)
- `mapHeaders` (function)
- `mapValues` (function)
- `skipLines` (number)
- `skipComments` (boolean|string)

Events:

- `headers` (header array)
- `data` (row object)
- `end`
- `error`

### parser.ndjson(options)

Creates an NDJSON decoder stream (Transform).

```javascript
const ndjsonParser = parser.ndjson({ strict: true });
```

Common options:

- `strict` (boolean, default `true`)

Events:

- `data` (parsed object)
- `warning` (when `strict: false` and invalid line is skipped)
- `end`
- `error`

## Progress Tracking

Both `csv()` and `ndjson()` decoder streams expose:

- `bytesWritten`: bytes received from upstream source
- `bytesRead`: bytes parsed/consumed by decoder

These values are updated during streaming, so they can be used inside `on('data')` for progress bars.

### CSV progress example

```javascript
const fs = require('/lib/fs');
const parser = require('/lib/parser');

const inputPath = '/tmp/sample.csv';
const totalBytes = fs.statSync(inputPath).size;

const decoder = parser.csv();

fs.createReadStream(inputPath, { highWaterMark: 64 * 1024 })
  .pipe(decoder)
  .on('data', (row) => {
    const ratio = totalBytes > 0 ? decoder.bytesWritten / totalBytes : 0;
    console.println('progress:', (ratio * 100).toFixed(1) + '%');
  })
  .on('end', () => {
    console.println('done, parsed bytes:', decoder.bytesRead);
  });
```

### NDJSON progress example

```javascript
const fs = require('/lib/fs');
const parser = require('/lib/parser');

const inputPath = '/tmp/sample.ndjson';
const totalBytes = fs.statSync(inputPath).size;

const decoder = parser.ndjson();

fs.createReadStream(inputPath, { highWaterMark: 64 * 1024 })
  .pipe(decoder)
  .on('data', (obj) => {
    const ratio = totalBytes > 0 ? decoder.bytesWritten / totalBytes : 0;
    console.println('progress:', (ratio * 100).toFixed(1) + '%');
  })
  .on('end', () => {
    console.println('done, parsed bytes:', decoder.bytesRead);
  });
```

## Notes

- For progress ratio against file size, `bytesWritten / fileSize` is usually the most practical metric.
- `bytesRead` is useful to track parser-side consumption/output progress.
- Counters are cumulative for the lifetime of each decoder instance.
