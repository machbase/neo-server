package util

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestParseArgs(t *testing.T) {
	tmpDir := t.TempDir()
	binName := "jsh"

	args := []string{"build", "-o"}
	if runtime.GOOS == "windows" {
		binName = "jsh.exe"
	}
	binPath := filepath.Join(tmpDir, binName)
	args = append(args, binPath)
	args = append(args, "../../main.go")
	cmd := exec.Command("go", args...)
	if err := cmd.Run(); err != nil {
		fmt.Println("Failed to build jsh binary for tests:", err)
		os.Exit(2)
	}

	tests := []struct {
		name           string
		args           []string
		stdinInput     string
		expectedOutput []string
	}{
		{
			name: "parseArgs-kebab-comprehensive",
			args: []string{"parseArgs-kebab-test"},
			expectedOutput: []string{
				"INFO  === Test 1: camelCase to kebab-case conversion ===",
				"INFO  userName:Alice",
				"INFO  ",
				"INFO  === Test 2: Complex camelCase ===",
				"INFO  maxRetryCount:10",
				"INFO  ",
				"INFO  === Test 3: Boolean with camelCase ===",
				"INFO  enableDebug:true",
				"INFO  ",
				"INFO  === Test 4: Negative boolean with camelCase ===",
				"INFO  enableDebug (negative):false",
				"INFO  ",
				"INFO  === Test 5: Multiple camelCase options ===",
				"INFO  userName:Bob",
				"INFO  maxConnections:100",
				"INFO  enableSsl:true",
				"INFO  ",
				"INFO  === Test 6: Simple name (no camelCase) still works ===",
				"INFO  port:8080",
				"INFO  verbose:true",
				"INFO  ",
				"INFO  === Test 7: Short option with camelCase long name ===",
				"INFO  userName (via -u):Charlie",
				"INFO  ",
				"INFO  === Test 8: Mix of kebab-case flag and camelCase property ===",
				"INFO  connectionTimeout:30",
				"INFO  maxRetryCount:3",
				"INFO  ",
				"INFO  All tests completed!",
			},
		},
		{
			name: "parseArgs-formatHelp",
			args: []string{"parseArgs-formatHelp-test"},
			expectedOutput: []string{
				"INFO  === Test 1: Basic formatHelp with camelCase options ===",
				"INFO  Usage: myapp [options] <file>",
				"",
				"Arguments:",
				"  file - Input file to process",
				"",
				"Options:",
				"  -u, --user-name          User name (default: guest)",
				"  -r, --max-retry-count    Maximum retry count (default: 3)",
				"  -d, --[no-]enable-debug  Enable debug mode (default: false)",
				"  -p, --port               Port number (default: 8080)",
				"INFO  ",
				"INFO  === Test 2: formatHelp with variadic positional ===",
				"INFO  Usage: command [options] <files...>",
				"",
				"Arguments:",
				"  files... - Files to process",
				"",
				"Options:",
				"  -o, --output-dir         Output directory",
				"  -v, --[no-]verbose-mode  Verbose output",
				"INFO  ",
				"INFO  === Test 3: toKebabCase function ===",
				"INFO  userName ->user-name",
				"INFO  maxRetryCount ->max-retry-count",
				"INFO  enableDebug ->enable-debug",
				"INFO  port ->port",
				"INFO  HTTPServer ->-h-t-t-p-server",
				"INFO  ",
				"INFO  === Test 4: formatHelp with sub-commands ===",
				"INFO  Usage: git <command> [options]",
				"",
				"Commands:",
				"  commit  Record changes to the repository",
				"  push    Update remote refs",
				"",
				"Global options:",
				"  -h, --help  Show help",
				"",
				"commit [options]",
				"  Record changes to the repository",
				"",
				"  Options:",
				"    -m, --message   Commit message",
				"    -a, --[no-]all  Stage all changes",
				"The commit command captures a snapshot of the project's currently staged changes.",
				"",
				"push [options]",
				"  Update remote refs",
				"",
				"  Arguments:",
				"    remote  Remote name",
				"    branch (optional)  Branch name",
				"",
				"  Options:",
				"    -f, --[no-]force  Force push",
			},
		},
		{
			name: "parseArgs-number-types",
			args: []string{"parseArgs-number-test"},
			expectedOutput: []string{
				"INFO  === Test 1: integer type ===",
				"INFO  port:8080type:number",
				"INFO  maxConnections:100type:number",
				"INFO  ",
				"INFO  === Test 2: float type ===",
				"INFO  threshold:3.14type:number",
				"INFO  ratio:0.5type:number",
				"INFO  ",
				"INFO  === Test 3: integer with short option ===",
				"INFO  port:5432",
				"INFO  count:10",
				"INFO  ",
				"INFO  === Test 4: negative integer ===",
				"INFO  offset:-5",
				"INFO  precision:-1",
				"INFO  ",
				"INFO  === Test 5: integer with inline value ===",
				"INFO  port:3000",
				"INFO  count:20",
				"INFO  ",
				"INFO  === Test 6: Error - decimal for integer ===",
				"INFO  Caught expected error:Option --count requires an integer value, got: 3.14",
				"INFO  ",
				"INFO  === Test 7: Error - invalid number ===",
				"INFO  Caught expected error:Option --port requires a valid integer value, got: abc",
				"INFO  ",
				"INFO  === Test 8: multiple integer values ===",
				"INFO  ids:[1, 2, 3]",
				"INFO  ",
				"INFO  === Test 9: mix of types ===",
				"INFO  port:8080type:number",
				"INFO  ratio:0.75type:number",
				"INFO  debug:truetype:boolean",
				"INFO  ",
				"INFO  All tests completed!",
			},
		},
		{
			name: "parseArgs-subcommand",
			args: []string{"parseArgs-subcommand-test"},
			expectedOutput: []string{
				"INFO  === Sub-command Test 1: add command ===",
				"INFO  command:add",
				"INFO  force:true",
				"INFO  message:Initial commit",
				"INFO  file:file.txt",
				"INFO  ",
				"INFO  === Sub-command Test 2: remove command ===",
				"INFO  command:remove",
				"INFO  recursive:true",
				"INFO  file:dir/",
				"INFO  ",
				"INFO  === Sub-command Test 3: default config (no matching command) ===",
				"INFO  command:null",
				"INFO  help:true",
				"INFO  ",
				"INFO  === Sub-command Test 4: git-like commit ===",
				"INFO  command:commit",
				"INFO  all:true",
				"INFO  message:Fix bug",
				"INFO  ",
				"INFO  === Sub-command Test 5: git-like push with positionals ===",
				"INFO  command:push",
				"INFO  force:true",
				"INFO  remote:origin",
				"INFO  branch:main",
				"INFO  ",
				"INFO  === All sub-command tests passed ===",
			},
		},
		{
			name: "parseArgs-subcommand-help",
			args: []string{"parseArgs-subcommand-test", "-h"},
			expectedOutput: []string{
				"INFO  === Sub-command Test 1: add command ===",
				"INFO  command:add",
				"INFO  force:true",
				"INFO  message:Initial commit",
				"INFO  file:file.txt",
				"INFO  ",
				"INFO  === Sub-command Test 2: remove command ===",
				"INFO  command:remove",
				"INFO  recursive:true",
				"INFO  file:dir/",
				"INFO  ",
				"INFO  === Sub-command Test 3: default config (no matching command) ===",
				"INFO  command:null",
				"INFO  help:true",
				"INFO  ",
				"INFO  === Sub-command Test 4: git-like commit ===",
				"INFO  command:commit",
				"INFO  all:true",
				"INFO  message:Fix bug",
				"INFO  ",
				"INFO  === Sub-command Test 5: git-like push with positionals ===",
				"INFO  command:push",
				"INFO  force:true",
				"INFO  remote:origin",
				"INFO  branch:main",
				"INFO  ",
				"INFO  === All sub-command tests passed ===",
			},
		},
		{
			name: "parseArgs-positional-camelCase",
			args: []string{"parseArgs-positional-camelCase-test"},
			expectedOutput: []string{
				"INFO  === Test 1: kebab-case positional argument name ===",
				"INFO  positionals:[my-tql-file]",
				"INFO  namedPositionals.tqlName:my-tql-file",
				"INFO  Expected: my-tql-file",
				"INFO  ",
				"INFO  === Test 2: Multiple kebab-case positional arguments ===",
				"INFO  namedPositionals.inputFile:input.txt",
				"INFO  namedPositionals.outputFile:output.txt",
				"INFO  Expected: input.txt, output.txt",
				"INFO  ",
				"INFO  === Test 3: Variadic kebab-case positional ===",
				"INFO  namedPositionals.sourceFiles:[file1.js, file2.js, file3.js]",
				"INFO  Expected: [file1.js, file2.js, file3.js]",
				"INFO  ",
				"INFO  === Test 4: Optional kebab-case positional with default ===",
				"INFO  namedPositionals.configFile:config.json",
				"INFO  Expected: config.json",
				"INFO  ",
				"INFO  === Test 5: Mix of simple and kebab-case names ===",
				"INFO  namedPositionals.command:cmd",
				"INFO  namedPositionals.commandParam:param",
				"INFO  Expected: cmd, param",
				"INFO  ",
				"INFO  === Test 6: Complex kebab-case with multiple hyphens ===",
				"INFO  namedPositionals.mySpecialConfigName:value",
				"INFO  Expected: value",
				"INFO  ",
				"INFO  All positional camelCase conversion tests completed!",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Prepare command: go run main.go <args>
			cmdArgs := append([]string{"-v", "/work=./test/"}, tt.args...)
			cmd := exec.Command(binPath, cmdArgs...)

			// Setup stdin with bytes.Buffer
			var stdin bytes.Buffer
			stdin.WriteString(tt.stdinInput)
			cmd.Stdin = &stdin

			// Setup stdout with bytes.Buffer
			var stdout bytes.Buffer
			cmd.Stdout = &stdout

			// Setup stderr to capture any errors
			var stderr bytes.Buffer
			cmd.Stderr = &stderr

			// Execute the command
			err := cmd.Run()
			if err != nil {
				t.Fatalf("Failed to execute command: %v\n%s", err, stdout.String())
			}

			// Get the output and trim whitespace
			actualOutput := strings.TrimSpace(stdout.String())
			expectedOutput := strings.TrimSpace(strings.Join(tt.expectedOutput, "\n"))

			// Compare output with expected
			expectedLines := strings.Split(expectedOutput, "\n")
			actualLines := strings.Split(actualOutput, "\n")
			for i := range expectedLines {
				if i >= len(actualLines) {
					t.Errorf("Output has fewer lines than expected. Missing line: %q", expectedLines[i])
					break
				}
				if strings.TrimSpace(expectedLines[i]) != strings.TrimSpace(actualLines[i]) {
					t.Errorf("Line %d mismatch:\nExpected: %q\nActual:   %q", i+1, expectedLines[i], actualLines[i])
				}
			}
		})
	}
}
