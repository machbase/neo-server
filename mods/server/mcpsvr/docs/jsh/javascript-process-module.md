# Machbase Neo JavaScript Processs Module

The `@jsh/process` module is specifically designed for use in JSH applications 
and is not available in the `SCRIPT()` function within TQL, unlike other JSH modules.

## pid()

Get the process id of the current process.

**Syntax**

```js
pid()
```

**Parameters**

None.

**Return value**

A number value that represents the process ID.

**Usage example**

```js
const m = require("@jsh/process")
console.log("my pid =", m.pid())
```

## ppid()

Get the process id of the parent process.

**Syntax**

```js
ppid()
```

**Parameters**

None.

**Return value**

A number value that represents the parent process ID.

**Usage example**

```js
const m = require("@jsh/process")
console.log("parent pid =", m.ppid())
```

## args()

Get command line arguments

**Syntax**

```js
args()
```

**Parameters**

None.

**Return value**

`String[]`

**Usage example**

```js
p = require("@jsh/process");
args = p.args();
x = parseInt(args[1]);
console.log(`x = ${x}`);
```

## cwd()

Get the current working directory

**Syntax**

```js
cwd()
```

**Parameters**

None.

**Return value**

String

**Usage example**

```js
p = require("@jsh/process");
console.log("cwd :", p.cwd());
```

## cd()

Change the current working directory.

**Syntax**

```js
cd(path)
```

**Parameters**

`path` : directory path to move

**Return value**

None.

**Usage example**

```js
p = require("@jsh/process");
p.cd('/dir/path');
console.log("cwd :", p.cwd());
```

## readDir()

Read files and sub-directories of the given directory.

**Syntax**

```js
readDir(path, callback)
```

**Parameters**

- `path`: `String` path to the directory
- `callback`: function ([DirEntry](#DirEntry)) [undefined|Boolean] callback function. if it returns false, the iteration will stop.

**Return value**

None.

**Usage example**

```js

```

## DirEntry

| Property           | Type       | Description        |
|:-------------------|:-----------|:-------------------|
| name               | String     |                    |
| isDir              | Boolean    |                    |
| readOnly           | Boolean    |                    |
| type               | String     |                    |
| size               | Number     |                    |
| virtual            | Boolean    |                    |

## print()

Write arguments into the output, the default output is the log file or stdout if log filename is not set.

**Syntax**

```js
print(...args)
```

**Parameters**

`args` `...any` Variable length of argument to write.

**Return value**

None.

**Usage example**

```js
p = require("@jsh/process")
p.print("Hello", "World!", "\n")
```

## println()

Write arguments into the output, the default output is the log file or stdout if log filename is not set.

**Syntax**

```js
print(...args)
```

**Parameters**

`args` `...any` Variable length of argument to write.

**Return value**

None.

**Usage example**

```js
p = require("@jsh/process")
p.println("Hello", "World!")
```

## exec()

Run another JavaScript application.

**Syntax**

```js
exec(cmd, ...args)
```

**Parameters**

`cmd` `String` .js file path to run
`args` `...String` arguments to pass to the cmd.

**Return value**

None.

**Usage example**

```js
p = require("@jsh/process")
p.exec("/sbin/hello.js")
```

## daemonize()

Run the current script file as a daemon process with its parent process ID set to `1`.

**Syntax**

```js
daemonize(opts)
```

**Parameters**

- `opts` `Object` Options

| Property           | Type       | Description        |
|:-------------------|:-----------|:-------------------|
| reload             | Boolean    | enable hot-reload  |

If `reload` is set to `true`, the daemon process starts with a source code change watcher.
When the main source code file is modified, the current daemon process is stopped and restarted immediately to apply the changes.
This feature is useful during development and testing
but should not be enabled in production environments, as it requires an additional system resources to monitor file changes.

**Return value**

None.

**Usage example**

```js
const p = require("@jsh/process")
if( p.ppid() == 1) {
    doBackgroundJob()
} else {
    p.daemonize()
    p.print("daemonize self, then exit")
}

function doBackgroundJob() {
    for(true){
        p.sleep(1000);
    }
}
```

## isDaemon()

Returns `true` if the parent process ID (`ppid()`) is `1`. This is equivalent to the condition `ppid() == 1`.

**Syntax**

```js
isDaemon()
```

**Parameters**

None.

**Return value**

Boolean

## isOrphan()

Returns `true` if the parent process ID is not assigned. This is equivalent to the condition `ppid() == 0xFFFFFFFF`.

**Syntax**

```js
isOrphan()
```

**Parameters**

None.

**Return value**

Boolean

## schedule()

Run the callback function according to the specified schedule.
The control flow remains blocked until the token's `stop()` method is invoked.

**Syntax**

```js
schedule(spec, callback)
```

**Parameters**

- `spec` `String` schedule spec. Refer to [Timer Schedule Spec.](/neo/timer/#timer-schedule-spec).
- `callback` `(time_epoch, token) => {}` The first parameter, `time_epoch`, is UNIX epoch timestamp in milliseconds unit.
    A callback function where the second parameter, `token`, can be used to stop the schedule.

**Return value**

None.

**Usage example**

```js
const {schedule} = require("@jsh/process");

var count = 0;
schedule("@every 2s", (ts, token)=>{
    count++;
    console.log(count, new Date(ts));
    if(count >= 5) token.stop();
})

// 1 2025-05-02 16:45:48
// 2 2025-05-02 16:45:50
// 3 2025-05-02 16:45:52
// 4 2025-05-02 16:45:54
// 5 2025-05-02 16:45:56
```

## sleep()

Pause the current control flow.

**Syntax**

```js
sleep(duration)
```

**Parameters**

`duration` `Number` sleep duration in milliseconds.

**Return value**

None.

**Usage example**

```js
p = require("@jsh/process")
p.sleep(1000) // 1 sec.
```

## kill()

Terminate a process using the specified process ID (pid).

**Syntax**

```js
kill(pid)
```

**Parameters**

`pid` `Number` pid of target process.

**Return value**

None.

**Usage example**

```js
p = require("@jsh/process")
p.kill(123)
```

## ps()

List all currently running processes.

**Syntax**

```js
ps()
```

**Parameters**

None.

**Return value**

`Object[]`: Array of [Process](#Process) objects.

**Usage example**

```js
p = require("@jsh/process")
list = p.ps()
for( const x of list ) {
    console.log(
        p.pid, 
        p.isOrphan() ? "-" : p.ppid,
        p.user, 
        p.name, 
        p.uptime)
}
```

## Process

Process information that returned by `ps()`.

| Property           | Type       | Description        |
|:-------------------|:-----------|:-------------------|
| pid                | Number     | process ID         |
| ppid               | Number     | process ID of the parent |
| user               | String     | username (e.g: `sys`)    |
| name               | String     | Script file name         |
| uptime             | String     | Elapse duration since started  |

## addCleanup()
Add a function to execute when the current JavaScript VM terminates.

**Syntax**

```js
addCleanup(fn)
```

**Parameters**

`fn` `()=>{}` callback function

**Return value**

`Number` A token for remove the cleanup callback.

**Usage example**

```js
p = require("@jsh/process")
p.addCleanup(()=>{ console.log("terminated") })
for(i = 0; i < 3; i++) {
    console.log("run -", i)
}

// run - 0
// run - 1
// run - 2
// terminated
```

## removeCleanup()

Remove a previously registered cleanup callback using the provided token.

**Syntax**

```js
removeCleanup(token)
```

**Parameters**

`token` `Number` token that returned by `addCleanup()`.

**Return value**

None.

**Usage example**

```js
p = require("@jsh/process")
token = p.addCleanup(()=>{ console.log("terminated") })
for(i = 0; i < 3; i++) {
    console.log("run -", i)
}
p.removeCleanup(token)

// run - 0
// run - 1
// run - 2
```