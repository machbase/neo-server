package shell

import (
	"fmt"
	"io"
	"strings"

	"github.com/dop251/goja"
)

// slashCmdHandler is the function signature for REPL slash command handlers.
// args contains the whitespace-trimmed text after the command name.
// Return exit=true to terminate the REPL loop; val becomes the loop return value.
type slashCmdHandler func(rt *goja.Runtime, args string, w io.Writer) (val goja.Value, exit bool)

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
		handle: func(rt *goja.Runtime, args string, w io.Writer) (goja.Value, bool) {
			return rt.ToValue(0), true
		},
	})
	repl.registerCommand(&slashEntry{
		aliases: []string{"help"},
		summary: "List available slash commands",
		handle: func(rt *goja.Runtime, args string, w io.Writer) (goja.Value, bool) {
			fmt.Fprintln(w, "Slash commands:")
			fmt.Fprintln(w, "  Constraints: no 'await', no 'import'. Use require('module').")
			fmt.Fprintln(w, "  Buffer and URL are available implicitly.")
			fmt.Fprintln(w)
			for _, e := range repl.cmds {
				names := make([]string, len(e.aliases))
				for i, a := range e.aliases {
					names[i] = `\` + a
				}
				fmt.Fprintf(w, "  %-20s  %s\n", strings.Join(names, ", "), e.summary)
			}
			return nil, false
		},
	})
	repl.registerCommand(&slashEntry{
		aliases: []string{"clear"},
		summary: "Clear the terminal screen",
		handle: func(rt *goja.Runtime, args string, w io.Writer) (goja.Value, bool) {
			fmt.Fprint(w, "\033[2J\033[H")
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
func (repl *Repl) dispatchSlashCommand(cmd, args string, w io.Writer) (goja.Value, bool) {
	for _, e := range repl.cmds {
		for _, alias := range e.aliases {
			if alias == cmd {
				return e.handle(repl.rt, args, w)
			}
		}
	}
	fmt.Fprintf(w, "Unknown command: \\%s  (type \\help for available commands)\n", cmd)
	return nil, false
}
