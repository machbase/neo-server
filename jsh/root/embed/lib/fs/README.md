# fs Module for jsh

A Node.js-compatible filesystem module for jsh (JavaScript Shell). This module provides familiar filesystem operations that work with jsh's native filesystem API.

## Installation

The module is located at `/lib/fs` and can be required in your jsh scripts:

```javascript
const fs = require('/lib/fs');
```

## Features

- **Node.js Compatible API**: Familiar function names and behavior similar to Node.js fs module
- **Synchronous Operations**: All operations are synchronous (Sync suffix on function names)
- **Path Resolution**: Automatically resolves relative paths to absolute paths
- **Error Handling**: Proper error codes (ENOENT, EACCES, etc.) for better error handling
- **File Type Detection**: Check if path is file, directory, symlink, etc.

## API Reference

### File Operations

#### readFileSync(path, options)
Read file contents synchronously.

```javascript
// Read as string (default UTF-8)
const content = fs.readFileSync('/path/to/file.txt', 'utf8');

// Read as byte array
const bytes = fs.readFileSync('/path/to/file.bin', { encoding: null });
```

**Parameters:**
- `path` (string): File path (absolute or relative)
- `options` (string|object): Encoding ('utf8', 'buffer', null) or options object

**Returns:** String or byte array

#### writeFileSync(path, data, options)
Write data to file synchronously (overwrites existing file).

```javascript
// Write string
fs.writeFileSync('/tmp/output.txt', 'Hello World\n', 'utf8');

// Write byte array
fs.writeFileSync('/tmp/data.bin', [0x48, 0x65, 0x6c, 0x6c, 0x6f], { encoding: null });
```

**Parameters:**
- `path` (string): File path
- `data` (string|Array): Data to write
- `options` (string|object): Encoding options

#### appendFileSync(path, data, options)
Append data to file synchronously.

```javascript
fs.appendFileSync('/tmp/log.txt', 'New log entry\n', 'utf8');
```

**Parameters:**
- `path` (string): File path
- `data` (string|Array): Data to append
- `options` (string|object): Encoding options

#### copyFileSync(src, dest, flags)
Copy a file synchronously.

```javascript
// Simple copy
fs.copyFileSync('/tmp/source.txt', '/tmp/dest.txt');

// Copy with EXCL flag (fail if destination exists)
fs.copyFileSync('/tmp/source.txt', '/tmp/dest.txt', fs.constants.COPYFILE_EXCL);
```

**Parameters:**
- `src` (string): Source file path
- `dest` (string): Destination file path
- `flags` (number): Optional copy flags

#### unlinkSync(path)
Delete a file synchronously.

```javascript
fs.unlinkSync('/tmp/file.txt');
```

**Parameters:**
- `path` (string): File path to delete

#### renameSync(oldPath, newPath)
Rename or move a file/directory synchronously.

```javascript
fs.renameSync('/tmp/old-name.txt', '/tmp/new-name.txt');
```

**Parameters:**
- `oldPath` (string): Current path
- `newPath` (string): New path

#### truncateSync(path, len)
Truncate file to specified length.

```javascript
// Truncate to 0 bytes (empty file)
fs.truncateSync('/tmp/file.txt');

// Truncate to 100 bytes
fs.truncateSync('/tmp/file.txt', 100);
```

**Parameters:**
- `path` (string): File path
- `len` (number): Length to truncate to (default: 0)

### Directory Operations

#### readdirSync(path, options)
Read directory contents synchronously.

```javascript
// Get array of filenames
const files = fs.readdirSync('/tmp');

// Get array of Dirent objects with file types
const entries = fs.readdirSync('/tmp', { withFileTypes: true });
entries.forEach(entry => {
    if (entry.isDirectory()) {
        console.println('[DIR]', entry.name);
    } else {
        console.println('[FILE]', entry.name);
    }
});
```

**Parameters:**
- `path` (string): Directory path
- `options` (object): Options { withFileTypes: boolean }

**Returns:** Array of strings or Dirent objects

#### mkdirSync(path, options)
Create a directory synchronously.

```javascript
// Create single directory
fs.mkdirSync('/tmp/newdir');

// Create nested directories
fs.mkdirSync('/tmp/a/b/c', { recursive: true });
```

**Parameters:**
- `path` (string): Directory path
- `options` (object): Options { recursive: boolean, mode: number }

#### rmdirSync(path, options)
Remove a directory synchronously.

```javascript
// Remove empty directory
fs.rmdirSync('/tmp/emptydir');

// Remove directory and all contents
fs.rmdirSync('/tmp/somedir', { recursive: true });
```

**Parameters:**
- `path` (string): Directory path
- `options` (object): Options { recursive: boolean }

#### rmSync(path, options)
Remove a file or directory synchronously (newer API).

```javascript
// Remove file
fs.rmSync('/tmp/file.txt');

// Remove directory recursively
fs.rmSync('/tmp/somedir', { recursive: true });

// Remove without throwing error if not exists
fs.rmSync('/tmp/maybe-exists', { force: true });
```

**Parameters:**
- `path` (string): File or directory path
- `options` (object): Options { recursive: boolean, force: boolean }

### File Information

#### existsSync(path)
Check if file or directory exists.

```javascript
if (fs.existsSync('/tmp/file.txt')) {
    console.println('File exists');
}
```

**Parameters:**
- `path` (string): Path to check

**Returns:** Boolean

#### statSync(path)
Get file or directory statistics.

```javascript
const stats = fs.statSync('/tmp/file.txt');

console.println('Is file:', stats.isFile());
console.println('Is directory:', stats.isDirectory());
console.println('Size:', stats.size);
console.println('Modified:', stats.mtime);
```

**Parameters:**
- `path` (string): Path to stat

**Returns:** Stats object with methods:
- `isFile()`: Returns true if file
- `isDirectory()`: Returns true if directory
- `isSymbolicLink()`: Returns true if symlink
- `isBlockDevice()`: Returns true if block device
- `isCharacterDevice()`: Returns true if character device
- `isFIFO()`: Returns true if FIFO/pipe
- `isSocket()`: Returns true if socket
- Properties: `size`, `mode`, `mtime`, `atime`, `ctime`, `birthtime`, `name`

#### lstatSync(path)
Get file or directory statistics (same as statSync for now).

```javascript
const stats = fs.lstatSync('/tmp/symlink');
```

**Parameters:**
- `path` (string): Path to stat

**Returns:** Stats object

#### accessSync(path, mode)
Check file access permissions.

```javascript
// Check if file exists
fs.accessSync('/tmp/file.txt', fs.constants.F_OK);

// Check if readable
fs.accessSync('/tmp/file.txt', fs.constants.R_OK);

// Check if writable
fs.accessSync('/tmp/file.txt', fs.constants.W_OK);
```

**Parameters:**
- `path` (string): Path to check
- `mode` (number): Access mode (F_OK, R_OK, W_OK, X_OK)

**Throws:** Error if access denied

### Symbolic Links

#### symlinkSync(target, path)
Create a symbolic link.

```javascript
fs.symlinkSync('/tmp/original.txt', '/tmp/link.txt');
```

**Parameters:**
- `target` (string): Link target
- `path` (string): Link path

#### readlinkSync(path)
Read symbolic link target.

```javascript
const target = fs.readlinkSync('/tmp/link.txt');
console.println('Link points to:', target);
```

**Parameters:**
- `path` (string): Link path

**Returns:** String (link target)

#### realpathSync(path)
Resolve path to real path (following symlinks).

```javascript
const realPath = fs.realpathSync('/tmp/link.txt');
console.println('Real path:', realPath);
```

**Parameters:**
- `path` (string): Path to resolve

**Returns:** String (resolved path)

### Permissions

#### chmodSync(path, mode)
Change file permissions.

```javascript
fs.chmodSync('/tmp/file.txt', 0o644);
```

**Parameters:**
- `path` (string): File path
- `mode` (number|string): Permission mode

#### chownSync(path, uid, gid)
Change file owner.

```javascript
fs.chownSync('/tmp/file.txt', 1000, 1000);
```

**Parameters:**
- `path` (string): File path
- `uid` (number): User ID
- `gid` (number): Group ID

### Constants

```javascript
// File Access Constants
fs.constants.F_OK  // File exists
fs.constants.R_OK  // Read permission
fs.constants.W_OK  // Write permission
fs.constants.X_OK  // Execute permission

// File Copy Constants
fs.constants.COPYFILE_EXCL           // Fail if destination exists
fs.constants.COPYFILE_FICLONE        // Copy-on-write clone
fs.constants.COPYFILE_FICLONE_FORCE  // Force copy-on-write

// File Open Constants
fs.constants.O_RDONLY  // Read only
fs.constants.O_WRONLY  // Write only
fs.constants.O_RDWR    // Read/write
fs.constants.O_CREAT   // Create if not exists
fs.constants.O_EXCL    // Fail if exists
fs.constants.O_TRUNC   // Truncate
fs.constants.O_APPEND  // Append
```

## Examples

### Example 1: Read and Parse JSON File

```javascript
const fs = require('/lib/fs');

try {
    const content = fs.readFileSync('/path/to/config.json', 'utf8');
    const config = JSON.parse(content);
    console.println('Config loaded:', config);
} catch (e) {
    console.println('Error reading config:', e);
}
```

### Example 2: Write Log File

```javascript
const fs = require('/lib/fs');

function log(message) {
    const timestamp = new Date().toISOString();
    const logEntry = `[${timestamp}] ${message}\n`;
    fs.appendFileSync('/tmp/app.log', logEntry, 'utf8');
}

log('Application started');
log('Processing request');
```

### Example 3: Directory Tree Walker

```javascript
const fs = require('/lib/fs');

function walkDir(dir, callback, indent = '') {
    const entries = fs.readdirSync(dir, { withFileTypes: true });
    
    entries.forEach(entry => {
        const fullPath = dir + '/' + entry.name;
        
        if (entry.isDirectory()) {
            console.println(indent + '[DIR] ' + entry.name);
            walkDir(fullPath, callback, indent + '  ');
        } else {
            console.println(indent + entry.name);
            callback(fullPath);
        }
    });
}

walkDir('/tmp', (file) => {
    // Process each file
});
```

### Example 4: File Backup

```javascript
const fs = require('/lib/fs');

function backupFile(path) {
    if (!fs.existsSync(path)) {
        throw new Error('File does not exist');
    }
    
    const timestamp = Date.now();
    const backupPath = path + '.backup.' + timestamp;
    
    fs.copyFileSync(path, backupPath);
    console.println('Backup created:', backupPath);
    
    return backupPath;
}

backupFile('/tmp/important.txt');
```

### Example 5: Safe File Write

```javascript
const fs = require('/lib/fs');

function safeWriteFile(path, data) {
    const tempPath = path + '.tmp';
    
    try {
        // Write to temporary file first
        fs.writeFileSync(tempPath, data, 'utf8');
        
        // If successful, rename to target
        fs.renameSync(tempPath, path);
        
        console.println('File written safely');
    } catch (e) {
        // Clean up temp file if it exists
        if (fs.existsSync(tempPath)) {
            fs.unlinkSync(tempPath);
        }
        throw e;
    }
}

safeWriteFile('/tmp/data.txt', 'Important data');
```

## Testing

Run the example script to test the fs module:

```bash
jsh /test/fs-example.js
```

## Compatibility Notes

- All functions are synchronous (no async/callback versions yet)
- Some advanced features may not be fully implemented depending on jsh's native filesystem capabilities
- Error codes and messages follow Node.js conventions where possible
- Path resolution assumes Unix-style paths

## See Also

- [cat.js](/jsh/engine/root/sbin/cat.js) - Example usage of filesystem reading
- [ls.js](/jsh/engine/root/sbin/ls.js) - Example usage of directory listing
- [Node.js fs documentation](https://nodejs.org/api/fs.html) - For reference
