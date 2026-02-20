# Readline Module

A JSH native module that provides interactive line-editing capabilities. Based on the go-readline-ny and go-multiline-ny libraries.

## Installation

```javascript
const { ReadLine } = require('/lib/readline');
```

## Classes

### ReadLine

An interactive line reader with support for multi-line input, custom prompts, and keyboard input handling.

#### Constructor

```javascript
new ReadLine(options)
```

**Parameters:**

- `options` (Object): Optional configuration object
  - `prompt` (Function): Custom prompt function that receives line number and returns prompt string
  - `submitOnEnterWhen` (Function): Function that determines when Enter should submit input
  - `autoInput` (Array<string>): Array of input strings for automated testing

**Example:**

```javascript
const { ReadLine } = require('/lib/readline');
const r = new ReadLine({
    prompt: (lineno) => { 
        return lineno === 0 ? "prompt> " : "....... "
    },
});
```

## Methods

### readLine(options)

Reads a line of input from the user.

**Parameters:**
- `options` (Object): Optional configuration object (same as constructor options)

**Returns:** string | Error - The input line or an Error object

**Example:**

```javascript
const line = r.readLine();
if (line instanceof Error) {
    throw line;
}
console.println("Input:", line);
```

### close()

Closes the readline session and cancels any pending read operations.

**Example:**

```javascript
r.close();
```

## Options

### prompt

A function that generates the prompt string for each line.

**Type:** `(lineno: number) => string`

**Parameters:**
- `lineno` (number): The current line number (0-based)

**Returns:** string - The prompt to display

**Example:**

```javascript
const r = new ReadLine({
    prompt: (lineno) => { 
        return lineno === 0 ? "> " : ". "
    },
});
```

### submitOnEnterWhen

A function that determines whether pressing Enter should submit the input or insert a new line.

**Type:** `(lines: string[], idx: number) => boolean`

**Parameters:**
- `lines` (string[]): Array of input lines
- `idx` (number): Current line index

**Returns:** boolean - `true` to submit, `false` to continue to next line

**Example:**

```javascript
const r = new ReadLine({
    submitOnEnterWhen: (lines, idx) => {
        // Submit only when line ends with semicolon
        return lines[idx].endsWith(";");
    },
});
```

### autoInput

An array of strings to automatically feed as input. Useful for automated testing.

**Type:** `string[]`

**Example:**

```javascript
const r = new ReadLine({
    autoInput: [
        "Hello World", 
        ReadLine.Enter,
    ],
});
```

## Static Properties (Key Constants)

The ReadLine class provides constants for special keys and control sequences:

### Control Keys

- `ReadLine.CtrlA` to `ReadLine.CtrlZ` - Control key combinations
- `ReadLine.CtrlHome`, `ReadLine.CtrlEnd` - Control + Home/End
- `ReadLine.CtrlPageUp`, `ReadLine.CtrlPageDown` - Control + Page Up/Down
- `ReadLine.CtrlLeft`, `ReadLine.CtrlRight` - Control + Arrow Left/Right
- `ReadLine.CtrlUp`, `ReadLine.CtrlDown` - Control + Arrow Up/Down

### Alt Keys

- `ReadLine.AltA` to `ReadLine.AltZ` - Alt key combinations
- `ReadLine.ALTBackspace` - Alt + Backspace

### Special Keys

- `ReadLine.Enter` - Enter key (`\r`)
- `ReadLine.Escape` - Escape key
- `ReadLine.Backspace` - Backspace key
- `ReadLine.Delete` - Delete key
- `ReadLine.ShiftTab` - Shift + Tab

### Navigation Keys

- `ReadLine.Up`, `ReadLine.Down` - Arrow up/down
- `ReadLine.Left`, `ReadLine.Right` - Arrow left/right
- `ReadLine.Home`, `ReadLine.End` - Home/End keys
- `ReadLine.PageUp`, `ReadLine.PageDown` - Page Up/Down

### Function Keys

- `ReadLine.F1` to `ReadLine.F24` - Function keys

## Complete Usage Examples

### Basic Input

```javascript
const { ReadLine } = require('/lib/readline');

const r = new ReadLine({
    prompt: (lineno) => { return "prompt> "},
});

console.println("PS:", r.options.prompt(0));
const line = r.readLine();
if (line instanceof Error) {
    throw line;
}
console.println("OK:", line);
```

**Output:**
```
PS: prompt> 
OK: Hello World
```

### Multi-line Input with Custom Submit Logic

```javascript
const { ReadLine } = require('/lib/readline');

const r = new ReadLine({
    submitOnEnterWhen: (lines, idx) => {
        // Submit only when the current line ends with a semicolon
        return lines[idx].endsWith(";");
    },
});

const line = r.readLine();
if (line instanceof Error) {
    throw line;
}
console.println("OK:", line);
```

**Input:**
```
Submit by
semi-colon;
```

**Output:**
```
OK: Submit by
semi-colon;
```

### Automated Input (for testing)

```javascript
const { ReadLine } = require('/lib/readline');

const r = new ReadLine({
    autoInput: [
        "Hello World",
        ReadLine.Enter,
    ],
});

const line = r.readLine();
console.println("Input:", line);
```

### Canceling Input

```javascript
const { ReadLine } = require('/lib/readline');

try {
    const r = new ReadLine();
    
    // Close the readline after 200ms
    const timeout = setTimeout(() => { 
        r.close() 
    }, 200);
    
    const line = r.readLine();
    console.println("OK:", line);
    clearTimeout(timeout);
} catch(e) {
    console.println("ERR:", e.message);
}
```

**Output:**
```
ERR: EOF
```

### Custom Prompt for Each Line

```javascript
const { ReadLine } = require('/lib/readline');

const r = new ReadLine({
    prompt: (lineno) => {
        if (lineno === 0) {
            return ">>> ";
        } else {
            return "... ";
        }
    },
});

const line = r.readLine();
console.println("Result:", line);
```

## Notes

- The readline interface supports full line editing with cursor movement, history, and multi-line input
- When `submitOnEnterWhen` is not provided, Enter always submits the input
- The `autoInput` option is primarily intended for automated testing
- Closing the reader with `close()` will cause any pending `readLine()` call to return an EOF error
- The module uses native TTY handling for proper terminal interaction

## Dependencies

- [go-readline-ny](https://github.com/nyaosorg/go-readline-ny)
- [go-multiline-ny](https://github.com/hymkor/go-multiline-ny)
- [go-ttyadapter](https://github.com/nyaosorg/go-ttyadapter)
