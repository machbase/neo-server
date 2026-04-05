package shell

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/dop251/goja"
)

// slashCtx is the execution context passed to every slash command handler.
// It carries everything a handler needs without coupling the handler to the
// full EditorSession or Repl internals.
type slashCtx struct {
	Writer  io.Writer
	History SessionHistory
	Profile RuntimeProfile
	// InitialGlobals is the set of global variable names that existed when the
	// REPL session started (after profile startup). \reset uses this to identify
	// and clear user-defined globals without touching engine-injected globals.
	InitialGlobals map[string]struct{}
}

// slashCmdHandler is the function signature for REPL slash command handlers.
// args contains the whitespace-trimmed text after the command name.
// Return exit=true to terminate the REPL loop; val becomes the loop return value.
type slashCmdHandler func(rt *goja.Runtime, args string, ctx slashCtx) (val goja.Value, exit bool)

// slashEntry holds one slot in the slash command registry.
type slashEntry struct {
	aliases []string // first alias is the canonical name
	summary string
	handle  slashCmdHandler
}

// registerBuiltinCommands populates repl.cmds with the built-in slash commands.
// It must be called after repl.cmds is initialized so that handlers that
// iterate repl.cmds (e.g. \help) see the full registry at call time.
func (repl *Repl) registerBuiltinCommands() {
	repl.registerCommand(&slashEntry{
		aliases: []string{"quit", "q", "exit"},
		summary: "Exit the REPL",
		handle: func(rt *goja.Runtime, args string, ctx slashCtx) (goja.Value, bool) {
			return rt.ToValue(0), true
		},
	})
	repl.registerCommand(&slashEntry{
		aliases: []string{"help"},
		summary: "List available slash commands",
		handle: func(rt *goja.Runtime, args string, ctx slashCtx) (goja.Value, bool) {
			fmt.Fprintln(ctx.Writer, "Slash commands:")
			for _, e := range repl.cmds {
				names := make([]string, len(e.aliases))
				for i, a := range e.aliases {
					names[i] = `\` + a
				}
				fmt.Fprintf(ctx.Writer, "  %-24s  %s\n", strings.Join(names, ", "), e.summary)
			}
			fmt.Fprintln(ctx.Writer)
			fmt.Fprintln(ctx.Writer, "Constraints:")
			fmt.Fprintln(ctx.Writer, "  no 'await'  — async APIs are wrapped as synchronous calls")
			fmt.Fprintln(ctx.Writer, "  no 'import' — use require('module') instead")
			fmt.Fprintln(ctx.Writer, "  Buffer and URL are available implicitly")
			return nil, false
		},
	})
	repl.registerCommand(&slashEntry{
		aliases: []string{"clear"},
		summary: "Clear the terminal screen",
		handle: func(rt *goja.Runtime, args string, ctx slashCtx) (goja.Value, bool) {
			fmt.Fprint(ctx.Writer, "\033[2J\033[H")
			return nil, false
		},
	})
	repl.registerCommand(&slashEntry{
		aliases: []string{"modules"},
		summary: "List modules available in this profile",
		handle: func(rt *goja.Runtime, args string, ctx slashCtx) (goja.Value, bool) {
			mods := ctx.Profile.KnownModules
			if len(mods) == 0 {
				fmt.Fprintln(ctx.Writer, "No modules listed for this profile.")
				fmt.Fprintln(ctx.Writer, "Use require('module') to load any JSH module.")
				return nil, false
			}
			fmt.Fprintln(ctx.Writer, "Available modules:")
			for _, m := range mods {
				fmt.Fprintf(ctx.Writer, "  %s\n", m)
			}
			return nil, false
		},
	})
	repl.registerCommand(&slashEntry{
		aliases: []string{"globals"},
		summary: "List non-standard global variables",
		handle: func(rt *goja.Runtime, args string, ctx slashCtx) (goja.Value, bool) {
			keys := rt.GlobalObject().Keys()
			sort.Strings(keys)
			// Standard JS globals to de-emphasize (still shown but marked).
			standard := jsStandardGlobals()
			fmt.Fprintln(ctx.Writer, "Global variables:")
			for _, k := range keys {
				if _, isStd := standard[k]; isStd {
					continue // skip pure built-ins; users can inspect them manually
				}
				val := rt.GlobalObject().Get(k)
				typeName := "value"
				if val != nil {
					typeName = val.ExportType().String()
					if strings.Contains(typeName, "func") {
						typeName = "function"
					}
				}
				fmt.Fprintf(ctx.Writer, "  %-24s  %s\n", k, typeName)
			}
			return nil, false
		},
	})
	repl.registerCommand(&slashEntry{
		aliases: []string{"history"},
		summary: "Show command history",
		handle: func(rt *goja.Runtime, args string, ctx slashCtx) (goja.Value, bool) {
			n := ctx.History.Len()
			if n == 0 {
				fmt.Fprintln(ctx.Writer, "History is empty.")
				return nil, false
			}
			for i := 0; i < n; i++ {
				entry := ctx.History.At(i)
				// Truncate long entries for readability.
				display := strings.ReplaceAll(entry, "\n", "↵ ")
				if len(display) > 80 {
					display = display[:77] + "..."
				}
				fmt.Fprintf(ctx.Writer, "  %3d  %s\n", i+1, display)
			}
			return nil, false
		},
	})
	repl.registerCommand(&slashEntry{
		aliases: []string{"reset"},
		summary: "Clear user-defined globals and re-run profile startup",
		handle: func(rt *goja.Runtime, args string, ctx slashCtx) (goja.Value, bool) {
			// Remove all globals that were added after the session started.
			keys := rt.GlobalObject().Keys()
			cleared := 0
			for _, k := range keys {
				if _, keep := ctx.InitialGlobals[k]; !keep {
					rt.Set(k, goja.Undefined())
					cleared++
				}
			}
			// Re-run profile startup to restore profile-injected globals (e.g. user).
			if err := ctx.Profile.RunStartup(rt); err != nil {
				fmt.Fprintf(ctx.Writer, "Warning: profile startup error: %s\n", err)
			}
			fmt.Fprint(ctx.Writer, "\033[2J\033[H")
			if cleared > 0 {
				fmt.Fprintf(ctx.Writer, "Reset complete. Cleared %d user-defined variable(s).\n", cleared)
			} else {
				fmt.Fprintln(ctx.Writer, "Reset complete. No user-defined variables to clear.")
			}
			return nil, false
		},
	})
	repl.registerCommand(&slashEntry{
		aliases: []string{"inspect"},
		summary: "Evaluate an expression and pretty-print with type info  (\\inspect <expr>)",
		handle: func(rt *goja.Runtime, args string, ctx slashCtx) (goja.Value, bool) {
			if strings.TrimSpace(args) == "" {
				fmt.Fprintln(ctx.Writer, "Usage: \\inspect <expression>")
				return nil, false
			}
			result, err := rt.RunString(args)
			if err != nil {
				fmt.Fprintf(ctx.Writer, "Error: %s\n", err.Error())
				return nil, false
			}
			fmt.Fprintln(ctx.Writer, inspectValue(rt, result))
			return nil, false
		},
	})
	repl.registerCommand(&slashEntry{
		aliases: []string{"json"},
		summary: "Evaluate an expression and print as JSON  (\\json <expr>)",
		handle: func(rt *goja.Runtime, args string, ctx slashCtx) (goja.Value, bool) {
			if strings.TrimSpace(args) == "" {
				fmt.Fprintln(ctx.Writer, "Usage: \\json <expression>")
				return nil, false
			}
			result, err := rt.RunString(args)
			if err != nil {
				fmt.Fprintf(ctx.Writer, "Error: %s\n", err.Error())
				return nil, false
			}
			fmt.Fprintln(ctx.Writer, toJSON(rt, result))
			return nil, false
		},
	})
}

// registerCommand appends e to the slash command registry.
func (repl *Repl) registerCommand(e *slashEntry) {
	repl.cmds = append(repl.cmds, e)
}

// parseSlashInput splits a slash command line into command name and arguments.
// The leading backslash is stripped; the remainder is split on the first space.
// Input "\load foo.js" → ("load", "foo.js"). Input "\quit" → ("quit", "").
func parseSlashInput(line string) (cmd, args string) {
	trimmed := strings.TrimPrefix(line, `\`)
	parts := strings.SplitN(trimmed, " ", 2)
	cmd = parts[0]
	if len(parts) > 1 {
		args = strings.TrimSpace(parts[1])
	}
	return cmd, args
}

// dispatchSlashCommand looks up cmd in the registry and calls the matching
// handler. Returns (val, true) when the handler requests loop exit. If no
// command matches, it prints an error message and returns (nil, false).
func (repl *Repl) dispatchSlashCommand(cmd, args string, ctx slashCtx) (goja.Value, bool) {
	for _, e := range repl.cmds {
		for _, alias := range e.aliases {
			if alias == cmd {
				return e.handle(repl.rt, args, ctx)
			}
		}
	}
	fmt.Fprintf(ctx.Writer, "Unknown command: \\%s  (type \\help for available commands)\n", cmd)
	return nil, false
}

// inspectValue formats a goja value for human inspection with type annotations.
// Primitives are annotated with their JS type. Functions show their name.
// Objects and arrays are pretty-printed as indented JSON.
func inspectValue(rt *goja.Runtime, val goja.Value) string {
	if val == nil || val == goja.Null() {
		return "null"
	}
	if val == goja.Undefined() {
		return "undefined"
	}
	// Functions: show name when available.
	if _, isFunc := goja.AssertFunction(val); isFunc {
		name := ""
		if obj, ok := val.(*goja.Object); ok {
			if n := obj.Get("name"); n != nil && n != goja.Undefined() {
				name = n.String()
			}
		}
		if name != "" && name != "anonymous" {
			return "[Function: " + name + "]"
		}
		return "[Function (anonymous)]"
	}
	// Primitive values: annotate with JS type.
	switch exported := val.Export().(type) {
	case string:
		return fmt.Sprintf("%q  (string, len=%d)", exported, len(exported))
	case int64, float64:
		return fmt.Sprintf("%s  (number)", val.String())
	case bool:
		return fmt.Sprintf("%s  (boolean)", val.String())
	}
	// Objects and arrays: pretty JSON.
	return stringifyViaRuntime(rt, val)
}

// toJSON serializes a goja value as indented JSON.
// Functions are represented as "[Function]". undefined is shown as "undefined".
func toJSON(rt *goja.Runtime, val goja.Value) string {
	if val == nil || val == goja.Null() {
		return "null"
	}
	if val == goja.Undefined() {
		return "undefined"
	}
	if _, isFunc := goja.AssertFunction(val); isFunc {
		return "[Function]"
	}
	return stringifyViaRuntime(rt, val)
}

// stringifyViaRuntime stores val in a private temp global, calls
// JSON.stringify on it, then removes the temp global. This avoids
// re-evaluating the original expression (which could have side effects)
// and handles array/object values correctly.
func stringifyViaRuntime(rt *goja.Runtime, val goja.Value) string {
	if val == nil || val == goja.Null() || val == goja.Undefined() {
		return val.String()
	}
	const tmpKey = "__replInspectTmp"
	rt.Set(tmpKey, val)
	defer rt.Set(tmpKey, goja.Undefined())
	jsonVal, err := rt.RunString("JSON.stringify(" + tmpKey + ", null, 2)")
	if err == nil && jsonVal != nil && jsonVal != goja.Undefined() {
		return jsonVal.String()
	}
	return val.String()
}

// jsStandardGlobals returns a set of standard JavaScript built-in names
// that are excluded from \globals output to reduce noise.
// The private temp global used by stringifyViaRuntime is also excluded.
func jsStandardGlobals() map[string]struct{} {
	names := []string{
		"Object", "Function", "Array", "String", "Boolean", "Number", "BigInt",
		"Symbol", "Date", "RegExp", "Error", "EvalError", "RangeError",
		"ReferenceError", "SyntaxError", "TypeError", "URIError",
		"Math", "JSON", "Reflect", "Proxy",
		"Map", "Set", "WeakMap", "WeakSet",
		"Promise", "Generator", "GeneratorFunction", "AsyncFunction",
		"ArrayBuffer", "DataView", "SharedArrayBuffer",
		"Int8Array", "Uint8Array", "Uint8ClampedArray",
		"Int16Array", "Uint16Array", "Int32Array", "Uint32Array",
		"Float32Array", "Float64Array", "BigInt64Array", "BigUint64Array",
		"Infinity", "NaN", "undefined",
		"parseInt", "parseFloat", "isNaN", "isFinite",
		"decodeURI", "decodeURIComponent", "encodeURI", "encodeURIComponent",
		"eval", "escape", "unescape",
		"globalThis",
		"__replInspectTmp", // internal temp used by \inspect / \json
	}
	m := make(map[string]struct{}, len(names))
	for _, n := range names {
		m[n] = struct{}{}
	}
	return m
}
