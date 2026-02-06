// Example usage of the fs module

const fs = require('fs');
const process = require('process');

console.println('=== FS Module Example ===\n');

// 1. Read a file
try {
    console.println('1. Reading /lib/fs/index.js (first 100 chars):');
    const content = fs.readFileSync('/lib/fs/index.js', 'utf8');
    console.println(content.substring(0, 100) + '...\n');
} catch (e) {
    console.println('Error reading file:', e);
    process.exit(1);
}


// 2. Create tmp directory
try {
    console.println('2. Creating directory /work/tmp:');
    fs.mkdirSync('/work/tmp');
    console.println('Directory created');
    
    console.println('Checking if directory exists:', fs.existsSync('/work/tmp'));
} catch (e) {
    console.println('Error:', e);
    process.exit(1);
}
    

// 3. Write and read a file
try {
    console.println('3. Writing to /work/tmp/test.txt:');
    fs.writeFileSync('/work/tmp/test.txt', 'Hello from fs module!\n', 'utf8');
    console.println('File written successfully');
    
    console.println('Reading back /work/tmp/test.txt:');
    const content = fs.readFileSync('/work/tmp/test.txt', 'utf8');
    console.println(content);
} catch (e) {
    console.println('Error:', e);
    process.exit(1);
}

// 4. Append to a file
try {
    console.println('4. Appending to /work/tmp/test.txt:');
    fs.appendFileSync('/work/tmp/test.txt', 'Appended line!\n', 'utf8');
    const content = fs.readFileSync('/work/tmp/test.txt', 'utf8');
    console.println('File content after append:');
    console.println(content);
} catch (e) {
    console.println('Error:', e);
    process.exit(1);
}

// 5. Check if file exists
try {
    console.println('5. Checking if files exist:');
    console.println('/work/tmp/test.txt exists:', fs.existsSync('/work/tmp/test.txt'));
    console.println('/work/tmp/nonexistent.txt exists:', fs.existsSync('/work/tmp/nonexistent.txt'));
    console.println();
} catch (e) {
    console.println('Error:', e);
    process.exit(1);
}

// 6. Get file stats
try {
    console.println('6. Getting stats for /work/tmp/test.txt:');
    const stats = fs.statSync('/work/tmp/test.txt');
    console.println('Is file:', stats.isFile());
    console.println('Is directory:', stats.isDirectory());
    console.println('Size:', stats.size, 'bytes');
    console.println('Modified:', stats.mtime);
    console.println();
} catch (e) {
    console.println('Error:', e);
    process.exit(1);
}

// 7. List directory contents
try {
    console.println('7. Listing /lib directory:');
    const files = fs.readdirSync('/lib');
    files.forEach(file => console.println('  -', file));
    console.println();
} catch (e) {
    console.println('Error:', e);
    process.exit(1);
}

// 8. List directory with file types
try {
    console.println('8. Listing /lib with file types:');
    const entries = fs.readdirSync('/lib', { withFileTypes: true });
    entries.forEach(entry => {
        const type = entry.isDirectory() ? '[DIR]' : '[FILE]';
        console.println(`  ${type} ${entry.name}`);
    });
    console.println();
} catch (e) {
    console.println('Error:', e);
    process.exit(1);
}

// 9. Create nested directories
try {
    console.println('9. Creating nested directories /work/tmp/a/b/c:');
    fs.mkdirSync('/work/tmp/a/b/c', { recursive: true });
    console.println('Nested directories created');
    
    console.println('Removing nested directories:');
    fs.rmSync('/work/tmp/a', { recursive: true });
    console.println('Nested directories removed');
    console.println();
} catch (e) {
    console.println('Error:', e);
    process.exit(1);
}

// 10. Copy file
try {
    console.println('10. Copying /work/tmp/test.txt to /work/tmp/test-copy.txt:');
    fs.copyFileSync('/work/tmp/test.txt', '/work/tmp/test-copy.txt');
    console.println('File copied');
    
    const original = fs.readFileSync('/work/tmp/test.txt', 'utf8');
    const copy = fs.readFileSync('/work/tmp/test-copy.txt', 'utf8');
    console.println('Original and copy match:', original === copy);
    console.println();
} catch (e) {
    console.println('Error:', e);
    process.exit(1);
}

// 11. Rename file
try {
    console.println('11. Renaming /work/tmp/test-copy.txt to /work/tmp/test-renamed.txt:');
    fs.renameSync('/work/tmp/test-copy.txt', '/work/tmp/test-renamed.txt');
    console.println('File renamed');
    console.println('Old file exists:', fs.existsSync('/work/tmp/test-copy.txt'));
    console.println('New file exists:', fs.existsSync('/work/tmp/test-renamed.txt'));
    console.println();
} catch (e) {
    console.println('Error:', e);
    process.exit(1);
}

// 12. Clean up
try {
    console.println('12. Cleaning up test files:');
    fs.unlinkSync('/work/tmp/test.txt');
    fs.unlinkSync('/work/tmp/test-renamed.txt');
    console.println('Test files removed');
} catch (e) {
    console.println('Error:', e);
    process.exit(1);
}

// 13. Remove tmp directory
try {
    console.println('13. Removing directory:');
    fs.rmdirSync('/work/tmp');
    console.println('Directory removed');
    console.println();
} catch (e) {
    console.println('Error:', e);
    process.exit(1);
}

console.println('\n=== Example Complete ===');
