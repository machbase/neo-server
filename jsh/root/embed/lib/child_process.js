// child_process.js - Node.js compatible child_process implementation for jsh
// Provides API for spawning child processes
'use strict';

const EventEmitter = require('events');
const Stream = require('stream');

// Get process functions from jsh's native process module
const _process = process;

// Default spawn options
const defaultSpawnOptions = {
  cwd: null,
  env: null,
  stdio: 'pipe',
  shell: false,
  timeout: 0,
  maxBuffer: 1024 * 1024, // 1MB
  encoding: 'buffer',
  windowsHide: false
};

class ChildProcess extends EventEmitter {
  constructor() {
    super();
    this.pid = null;
    this.exitCode = null;
    this.signalCode = null;
    this.killed = false;
    this.connected = false;
    
    // Stdio streams
    this.stdin = null;
    this.stdout = null;
    this.stderr = null;
    this.stdio = [];
  }

  kill(signal) {
    if (this.killed) return false;
    
    signal = signal || 'SIGTERM';
    this.killed = true;
    
    // TODO: Implement actual killing via jsh when available
    this.emit('exit', null, signal);
    return true;
  }

  send(message, sendHandle, options, callback) {
    throw new Error('IPC is not supported in jsh child_process');
  }

  disconnect() {
    throw new Error('IPC is not supported in jsh child_process');
  }

  ref() {
    // No-op for jsh
  }

  unref() {
    // No-op for jsh
  }
}

// Execute a command and return a ChildProcess
function spawn(command, args, options) {
  args = args || [];
  options = Object.assign({}, defaultSpawnOptions, options);

  const child = new ChildProcess();
  
  // Set up stdio streams
  if (options.stdio === 'pipe' || Array.isArray(options.stdio)) {
    child.stdin = new Stream.Writable();
    child.stdout = new Stream.Readable();
    child.stderr = new Stream.Readable();
    child.stdio = [child.stdin, child.stdout, child.stderr];
  } else if (options.stdio === 'inherit') {
    child.stdin = process.stdin;
    child.stdout = process.stdout;
    child.stderr = process.stderr;
    child.stdio = [child.stdin, child.stdout, child.stderr];
  }

  // Execute asynchronously
  process.nextTick(() => {
    try {
      // Build command string
      let cmdStr = command;
      if (options.shell) {
        // If shell is true, execute as shell command
        cmdStr = command + (args.length > 0 ? ' ' + args.join(' ') : '');
      } else {
        // For jsh, we need to execute .js files
        if (!command.endsWith('.js')) {
          cmdStr = command + '.js';
        }
      }

      // Change directory if specified
      const originalCwd = _process.cwd();
      if (options.cwd) {
        try {
          _process.chdir(options.cwd);
        } catch (e) {
          child.emit('error', new Error('ENOENT: no such directory, ' + options.cwd));
          return;
        }
      }

      let exitCode;
      try {
        // Execute the command using jsh's exec
        if (options.shell || !cmdStr.endsWith('.js')) {
          // Execute as string
          exitCode = _process.execString(cmdStr, ...args);
        } else {
          // Execute as file
          exitCode = _process.exec(cmdStr, ...args);
        }
      } catch (err) {
        child.emit('error', err);
        return;
      } finally {
        // Restore original directory
        if (options.cwd) {
          _process.chdir(originalCwd);
        }
      }

      // Emit exit event
      child.exitCode = exitCode;
      child.emit('exit', exitCode, null);
      child.emit('close', exitCode, null);

    } catch (error) {
      child.emit('error', error);
    }
  });

  return child;
}

// Execute a command in a shell and buffer output
function exec(command, options, callback) {
  if (typeof options === 'function') {
    callback = options;
    options = {};
  }
  
  options = Object.assign({}, defaultSpawnOptions, options, {
    shell: true,
    stdio: 'pipe'
  });

  const child = spawn('/bin/sh', ['-c', command], options);
  
  let stdout = '';
  let stderr = '';
  let encoding = options.encoding || 'utf8';
  
  if (child.stdout) {
    child.stdout.on('data', (data) => {
      stdout += encoding === 'buffer' ? data : data.toString(encoding);
      
      if (options.maxBuffer && stdout.length > options.maxBuffer) {
        const err = new Error('stdout maxBuffer exceeded');
        err.code = 'ERR_CHILD_PROCESS_STDIO_MAXBUFFER';
        child.kill();
        if (callback) callback(err, stdout, stderr);
      }
    });
  }
  
  if (child.stderr) {
    child.stderr.on('data', (data) => {
      stderr += encoding === 'buffer' ? data : data.toString(encoding);
      
      if (options.maxBuffer && stderr.length > options.maxBuffer) {
        const err = new Error('stderr maxBuffer exceeded');
        err.code = 'ERR_CHILD_PROCESS_STDIO_MAXBUFFER';
        child.kill();
        if (callback) callback(err, stdout, stderr);
      }
    });
  }
  
  child.on('close', (code, signal) => {
    if (callback) {
      if (code !== 0) {
        const err = new Error('Command failed: ' + command);
        err.code = code;
        err.signal = signal;
        err.cmd = command;
        callback(err, stdout, stderr);
      } else {
        callback(null, stdout, stderr);
      }
    }
  });
  
  child.on('error', (err) => {
    if (callback) callback(err, stdout, stderr);
  });
  
  // Handle timeout
  if (options.timeout > 0) {
    setTimeout(() => {
      if (!child.killed) {
        child.kill(options.killSignal || 'SIGTERM');
        const err = new Error('Command timeout: ' + command);
        err.killed = true;
        err.code = null;
        err.signal = options.killSignal || 'SIGTERM';
        if (callback) callback(err, stdout, stderr);
      }
    }, options.timeout);
  }
  
  return child;
}

// Execute a file with arguments
function execFile(file, args, options, callback) {
  if (typeof args === 'function') {
    callback = args;
    args = [];
    options = {};
  } else if (typeof options === 'function') {
    callback = options;
    options = {};
  }
  
  args = args || [];
  options = Object.assign({}, defaultSpawnOptions, options, {
    shell: false,
    stdio: 'pipe'
  });

  // For jsh, ensure file has .js extension
  let jsFile = file;
  if (!jsFile.endsWith('.js')) {
    jsFile = file + '.js';
  }

  const child = spawn(jsFile, args, options);
  
  let stdout = '';
  let stderr = '';
  let encoding = options.encoding || 'utf8';
  
  if (child.stdout) {
    child.stdout.on('data', (data) => {
      stdout += encoding === 'buffer' ? data : data.toString(encoding);
      
      if (options.maxBuffer && stdout.length > options.maxBuffer) {
        const err = new Error('stdout maxBuffer exceeded');
        err.code = 'ERR_CHILD_PROCESS_STDIO_MAXBUFFER';
        child.kill();
        if (callback) callback(err, stdout, stderr);
      }
    });
  }
  
  if (child.stderr) {
    child.stderr.on('data', (data) => {
      stderr += encoding === 'buffer' ? data : data.toString(encoding);
      
      if (options.maxBuffer && stderr.length > options.maxBuffer) {
        const err = new Error('stderr maxBuffer exceeded');
        err.code = 'ERR_CHILD_PROCESS_STDIO_MAXBUFFER';
        child.kill();
        if (callback) callback(err, stdout, stderr);
      }
    });
  }
  
  child.on('close', (code, signal) => {
    if (callback) {
      if (code !== 0) {
        const err = new Error('Command failed: ' + file);
        err.code = code;
        err.signal = signal;
        err.path = file;
        callback(err, stdout, stderr);
      } else {
        callback(null, stdout, stderr);
      }
    }
  });
  
  child.on('error', (err) => {
    if (callback) callback(err, stdout, stderr);
  });
  
  // Handle timeout
  if (options.timeout > 0) {
    setTimeout(() => {
      if (!child.killed) {
        child.kill(options.killSignal || 'SIGTERM');
        const err = new Error('Command timeout: ' + file);
        err.killed = true;
        err.code = null;
        err.signal = options.killSignal || 'SIGTERM';
        if (callback) callback(err, stdout, stderr);
      }
    }, options.timeout);
  }
  
  return child;
}

// Synchronous version of spawn (limited implementation)
function spawnSync(command, args, options) {
  args = args || [];
  options = Object.assign({}, defaultSpawnOptions, options);

  const result = {
    pid: null,
    output: [null, null, null],
    stdout: null,
    stderr: null,
    status: null,
    signal: null,
    error: null
  };

  try {
    // Build command
    let cmdStr = command;
    if (!options.shell && !command.endsWith('.js')) {
      cmdStr = command + '.js';
    }

    // Change directory if specified
    const originalCwd = _process.cwd();
    if (options.cwd) {
      _process.chdir(options.cwd);
    }

    try {
      // Execute synchronously
      let exitCode;
      if (options.shell || !cmdStr.endsWith('.js')) {
        exitCode = _process.execString(cmdStr, ...args);
      } else {
        exitCode = _process.exec(cmdStr, ...args);
      }
      result.status = exitCode;
    } finally {
      // Restore directory
      if (options.cwd) {
        _process.chdir(originalCwd);
      }
    }

  } catch (error) {
    result.error = error;
    result.status = -1;
  }

  return result;
}

// Synchronous version of exec
function execSync(command, options) {
  options = Object.assign({}, defaultSpawnOptions, options, {
    shell: true
  });

  const result = spawnSync('/bin/sh', ['-c', command], options);
  
  if (result.error) {
    throw result.error;
  }
  
  if (result.status !== 0) {
    const err = new Error('Command failed: ' + command);
    err.status = result.status;
    err.signal = result.signal;
    err.stdout = result.stdout;
    err.stderr = result.stderr;
    throw err;
  }
  
  return result.stdout || '';
}

// Synchronous version of execFile
function execFileSync(file, args, options) {
  if (!Array.isArray(args)) {
    options = args;
    args = [];
  }
  
  args = args || [];
  options = Object.assign({}, defaultSpawnOptions, options, {
    shell: false
  });

  // For jsh, ensure file has .js extension
  let jsFile = file;
  if (!jsFile.endsWith('.js')) {
    jsFile = file + '.js';
  }

  const result = spawnSync(jsFile, args, options);
  
  if (result.error) {
    throw result.error;
  }
  
  if (result.status !== 0) {
    const err = new Error('Command failed: ' + file);
    err.status = result.status;
    err.signal = result.signal;
    err.path = file;
    err.stdout = result.stdout;
    err.stderr = result.stderr;
    throw err;
  }
  
  return result.stdout || '';
}

// Fork is not supported in jsh (requires IPC)
function fork(modulePath, args, options) {
  throw new Error('child_process.fork() is not supported in jsh (no IPC support)');
}

// Export all functions
module.exports = {
  ChildProcess,
  spawn,
  exec,
  execFile,
  fork,
  spawnSync,
  execSync,
  execFileSync
};
