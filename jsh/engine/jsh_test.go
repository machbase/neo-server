package engine

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestJshMain(t *testing.T) {
	tests := []struct {
		name           string
		args           []string
		stdinInput     string
		expectedOutput []string
	}{
		{
			name:           "hello_with_no_args",
			args:           []string{"hello"},
			stdinInput:     "",
			expectedOutput: []string{"Hello  from demo.js!"},
		},
		{
			name:           "hello_with_args",
			args:           []string{"hello", "world"},
			stdinInput:     "",
			expectedOutput: []string{"Hello world from demo.js!"},
		},
		{
			name:           "sbin_echo",
			args:           []string{"/sbin/echo", "Hello, Echo?"},
			stdinInput:     "",
			expectedOutput: []string{"Hello, Echo?"},
		},
		{
			name:           "exec",
			args:           []string{"exec"},
			stdinInput:     "",
			expectedOutput: []string{"Hello 世界 from demo.js!"},
		},
		{
			name:       "optparse-demo",
			args:       []string{"optparse-demo", "-v", "-h"},
			stdinInput: "",
			expectedOutput: []string{
				"command version 0.1.0",
				"Usage: command [options]",
				"",
				"Available options:",
				"  -h, --help      Show this help message",
				"  -v, --version   Show version information",
				"Options: {help:true, version:true}",
			},
		},
		{
			name: "stream-demo",
			args: []string{"stream-demo"},
			expectedOutput: []string{
				"INFO  === Stream Module Demo ===",
				"",
				"INFO  Demo 1: PassThrough Stream",
				"INFO  ---------------------------",
				"INFO  ",
				"",
				"INFO  Demo 2: Error Handling",
				"INFO  ----------------------",
				"INFO  Error stream closed",
				"INFO  Caught error:Stream is not writable",
				"INFO  ",
				"=== Demo Complete ===",
				"INFO  Stream finished",
				"INFO  Stream closed",
				"INFO  Error stream closed",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Prepare command: go run main.go <args>
			cmdArgs := append([]string{"-v", "/work=../test/"}, tt.args...)
			cmd := exec.Command("../tmp/jsh", cmdArgs...)

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
			if actualOutput != expectedOutput {
				t.Errorf("Output mismatch:\nExpected: %q\nActual:   %q", expectedOutput, actualOutput)
			}
		})
	}
}

func TestConfig_UnmarshalJSON(t *testing.T) {
	tests := []struct {
		name    string
		jsonStr string
		check   func(*testing.T, *Config)
	}{
		{
			name: "simple env with SecureString",
			jsonStr: `{
				"name": "test",
				"env": {
					"USER": "admin",
					"PASSWORD": "SecureString:my_password"
				}
			}`,
			check: func(t *testing.T, c *Config) {
				if c.Name != "test" {
					t.Errorf("Name = %v, want test", c.Name)
				}
				if c.Env["USER"] != "admin" {
					t.Errorf("Env[USER] = %v, want admin", c.Env["USER"])
				}
				pass, ok := c.Env["PASSWORD"].(SecureString)
				if !ok {
					t.Fatalf("Env[PASSWORD] is not SecureString, got %T", c.Env["PASSWORD"])
				}
				if pass != SecureString("my_password") {
					t.Errorf("Env[PASSWORD] = %v, want my_password", pass)
				}
			},
		},
		{
			name: "nested map with SecureString",
			jsonStr: `{
				"name": "nested",
				"env": {
					"DB_CONFIG": {
						"host": "localhost",
						"password": "SecureString:db_secret"
					}
				}
			}`,
			check: func(t *testing.T, c *Config) {
				dbConfig, ok := c.Env["DB_CONFIG"].(map[string]any)
				if !ok {
					t.Fatalf("Env[DB_CONFIG] is not map, got %T", c.Env["DB_CONFIG"])
				}
				if dbConfig["host"] != "localhost" {
					t.Errorf("DB_CONFIG[host] = %v, want localhost", dbConfig["host"])
				}
				pass, ok := dbConfig["password"].(SecureString)
				if !ok {
					t.Fatalf("DB_CONFIG[password] is not SecureString, got %T", dbConfig["password"])
				}
				if pass != SecureString("db_secret") {
					t.Errorf("DB_CONFIG[password] = %v, want db_secret", pass)
				}
			},
		},
		{
			name: "array with SecureString",
			jsonStr: `{
				"name": "array",
				"env": {
					"PASSWORDS": ["SecureString:pass1", "regular_string", "SecureString:pass2"]
				}
			}`,
			check: func(t *testing.T, c *Config) {
				passwords, ok := c.Env["PASSWORDS"].([]any)
				if !ok {
					t.Fatalf("Env[PASSWORDS] is not array, got %T", c.Env["PASSWORDS"])
				}
				if len(passwords) != 3 {
					t.Fatalf("PASSWORDS length = %d, want 3", len(passwords))
				}
				pass1, ok := passwords[0].(SecureString)
				if !ok {
					t.Fatalf("PASSWORDS[0] is not SecureString, got %T", passwords[0])
				}
				if pass1 != SecureString("pass1") {
					t.Errorf("PASSWORDS[0] = %v, want pass1", pass1)
				}
				if passwords[1] != "regular_string" {
					t.Errorf("PASSWORDS[1] = %v, want regular_string", passwords[1])
				}
				pass2, ok := passwords[2].(SecureString)
				if !ok {
					t.Fatalf("PASSWORDS[2] is not SecureString, got %T", passwords[2])
				}
				if pass2 != SecureString("pass2") {
					t.Errorf("PASSWORDS[2] = %v, want pass2", pass2)
				}
			},
		},
		{
			name: "mixed types",
			jsonStr: `{
				"name": "mixed",
				"env": {
					"STRING": "value",
					"NUMBER": 42,
					"BOOL": true,
					"SECURE": "SecureString:secret",
					"NULL": null
				}
			}`,
			check: func(t *testing.T, c *Config) {
				if c.Env["STRING"] != "value" {
					t.Errorf("STRING = %v, want value", c.Env["STRING"])
				}
				if c.Env["NUMBER"] != float64(42) {
					t.Errorf("NUMBER = %v, want 42", c.Env["NUMBER"])
				}
				if c.Env["BOOL"] != true {
					t.Errorf("BOOL = %v, want true", c.Env["BOOL"])
				}
				secure, ok := c.Env["SECURE"].(SecureString)
				if !ok {
					t.Fatalf("SECURE is not SecureString, got %T", c.Env["SECURE"])
				}
				if secure != SecureString("secret") {
					t.Errorf("SECURE = %v, want secret", secure)
				}
				if c.Env["NULL"] != nil {
					t.Errorf("NULL = %v, want nil", c.Env["NULL"])
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var config Config
			err := json.Unmarshal([]byte(tt.jsonStr), &config)
			if err != nil {
				t.Fatalf("Unmarshal error = %v", err)
			}
			tt.check(t, &config)
		})
	}
}

func TestReadSecretBox(t *testing.T) {
	// Create a temporary file with JSON config
	tmpfile := t.TempDir() + "/secret.json"

	jsonContent := `{
		"name": "secret-test",
		"code": "console.log('test')",
		"env": {
			"USER": "admin",
			"PASSWORD": "SecureString:super_secret",
			"DB": {
				"host": "localhost",
				"pass": "SecureString:db_password"
			},
			"TOKENS": ["SecureString:token1", "public_token", "SecureString:token2"]
		}
	}`

	if err := os.WriteFile(tmpfile, []byte(jsonContent), 0600); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	var config Config
	if err := ReadSecretBox(tmpfile, &config); err != nil {
		t.Fatalf("ReadSecretBox error = %v", err)
	}

	// Verify basic fields
	if config.Name != "secret-test" {
		t.Errorf("Name = %v, want secret-test", config.Name)
	}

	// Verify SecureString in env
	password, ok := config.Env["PASSWORD"].(SecureString)
	if !ok {
		t.Fatalf("PASSWORD is not SecureString, got %T", config.Env["PASSWORD"])
	}
	if password != SecureString("super_secret") {
		t.Errorf("PASSWORD = %v, want super_secret", password)
	}

	// Verify nested SecureString
	db, ok := config.Env["DB"].(map[string]any)
	if !ok {
		t.Fatalf("DB is not map, got %T", config.Env["DB"])
	}
	dbPass, ok := db["pass"].(SecureString)
	if !ok {
		t.Fatalf("DB pass is not SecureString, got %T", db["pass"])
	}
	if dbPass != SecureString("db_password") {
		t.Errorf("DB pass = %v, want db_password", dbPass)
	}

	// Verify array with SecureString
	tokens, ok := config.Env["TOKENS"].([]any)
	if !ok {
		t.Fatalf("TOKENS is not array, got %T", config.Env["TOKENS"])
	}
	token1, ok := tokens[0].(SecureString)
	if !ok {
		t.Fatalf("TOKENS[0] is not SecureString, got %T", tokens[0])
	}
	if token1 != SecureString("token1") {
		t.Errorf("TOKENS[0] = %v, want token1", token1)
	}

	// Verify file is deleted after reading
	if _, err := os.Stat(tmpfile); !os.IsNotExist(err) {
		t.Error("Secret file was not deleted after reading")
	}
}
