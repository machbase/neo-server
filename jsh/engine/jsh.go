package engine

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/dop251/goja"
	"github.com/dop251/goja_nodejs/eventloop"
	"github.com/dop251/goja_nodejs/require"
)

func New(conf Config) (*JSRuntime, error) {
	// Build filesystem from FSTabs
	filesystem := NewFS()
	for _, tab := range conf.FSTabs {
		if tab.FS == nil {
			if dirfs, err := DirFS(tab.Source); err != nil {
				return nil, fmt.Errorf("error mounting %s to %s: %v", tab.Source, tab.MountPoint, err)
			} else {
				filesystem.Mount(tab.MountPoint, dirfs)
			}
		} else {
			filesystem.Mount(tab.MountPoint, tab.FS)
		}
	}

	var reader io.Reader = os.Stdin
	if conf.Reader != nil {
		reader = conf.Reader
	}
	var writer io.Writer = os.Stdout
	if conf.Writer != nil {
		writer = conf.Writer
	}
	opts := []EnvOption{
		WithFilesystem(filesystem),
		WithReader(reader),
		WithWriter(writer),
		WithExecBuilder(conf.ExecBuilder),
		WithAliases(conf.Aliases),
	}
	env := NewEnv(opts...)
	for k, v := range conf.Env {
		env.Set(k, v)
	}
	// Default environment variables
	if env.Get("PATH") == nil {
		env.Set("PATH", "/sbin:.")
	}
	if env.Get("LIBRARY_PATH") == nil {
		env.Set("LIBRARY_PATH", "./node_modules:/lib")
	}
	if env.Get("HOME") == nil {
		env.Set("HOME", "/")
	}
	if env.Get("PWD") == nil {
		env.Set("PWD", "/")
	}

	script := ""
	scriptName := ""
	scriptArgs := []string{}
	if conf.Code == "" {
		cmd := ""
		if len(conf.Args) > 0 {
			cmd = conf.Args[0]
		}
		if len(conf.Args) > 1 {
			scriptArgs = conf.Args[1:]
		}
		if cmd == "" {
			// No command or script file provided
			// start default command
			cmd := env.Which(conf.Default)
			b, _ := env.LoadSource(cmd)
			scriptName = conf.Default
			script = string(b)
		} else {
			// Check aliases and resolved command
			var resolved string
			if alias := env.Alias(cmd); len(alias) > 0 {
				cmd = alias[0]
				if len(alias) > 1 {
					scriptArgs = append(alias[1:], scriptArgs...)
				}
			}
			resolved = env.Which(cmd)
			if resolved == "" {
				return nil, fmt.Errorf("command not found: %s", cmd)
			}
			b, err := env.LoadSource(resolved)
			if err != nil {
				return nil, fmt.Errorf("command not executable: %s", cmd)
			}
			// replace shebang line as javascript comment
			if len(b) > 2 && b[0] == '#' && b[1] == '!' {
				b[0], b[1] = '/', '/'
			}
			scriptName = cmd
			script = string(b)
		}
	} else {
		scriptName = conf.Name
		script = conf.Code
		scriptArgs = conf.Args
	}
	if scriptName == "" {
		scriptName = "ad-hoc"
	}

	jr := &JSRuntime{
		Name:       scriptName,
		Source:     script,
		Args:       scriptArgs,
		Env:        env,
		filesystem: filesystem,
	}

	jr.registry = require.NewRegistry(
		require.WithLoader(jr.Env.LoadSource),
		require.WithPathResolver(jr.Env.PathResolver),
		require.WithGlobalFolders(jr.Env.GlobalFolders()...),
	)
	jr.eventLoop = NewEventLoop(
		eventloop.EnableConsole(false),
		eventloop.WithRegistry(jr.registry),
	)
	return jr, nil
}

func (jr *JSRuntime) Main() int {
	if err := jr.Run(); err != nil {
		if ie, ok := err.(*goja.InterruptedError); ok {
			frame := ie.Stack()[0]
			if exit, ok := ie.Value().(Exit); ok {
				if exit.Code < 0 {
					fmt.Printf("exit status %d at %v\n", exit.Code, frame.Position())
				}
				return exit.Code
			}
		}
		return 1
	}
	return jr.ExitCode()
}

// DirFS checks that the given directory exists and is a directory, returning an fs.FS for it.
func DirFS(dir string) (fileSystem fs.FS, err error) {
	if dir == "" {
		wd, err := os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("getting working directory: %v", err)
		}
		return os.DirFS(wd), nil
	}
	info, err := os.Stat(dir)
	if err != nil {
		return nil, fmt.Errorf("stating directory %q: %v", dir, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("not a directory: %q", dir)
	}
	absDir, err := os.Readlink(dir)
	if err != nil {
		absDir = dir
	}
	return os.DirFS(absDir), nil
}

type Config struct {
	Name    string            `json:"name"`
	Code    string            `json:"code"`
	Args    []string          `json:"args"`
	Env     map[string]any    `json:"env"`
	Aliases map[string]string `json:"aliases,omitempty"`
	FSTabs  FSTabs            `json:"fstabs,omitempty"`

	Default     string          `json:"default,omitempty"`
	Writer      io.Writer       `json:"-"`
	Reader      io.Reader       `json:"-"`
	ExecBuilder ExecBuilderFunc `json:"-"`
}

// UnmarshalJSON implements custom unmarshaling for Config to handle SecureString types in Env
func (c *Config) UnmarshalJSON(data []byte) error {
	type Alias Config // avoid recursion
	aux := &struct {
		Env map[string]json.RawMessage `json:"env"`
		*Alias
	}{
		Alias: (*Alias)(c),
	}

	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	// Process Env map to convert SecureString-prefixed values
	if aux.Env != nil {
		c.Env = make(map[string]any)
		for k, v := range aux.Env {
			c.Env[k] = processValue(v)
		}
	}

	return nil
}

// processValue recursively processes JSON values to convert SecureString-prefixed strings
func processValue(raw json.RawMessage) any {
	if len(raw) == 0 {
		return nil
	}

	// Fast path: check first byte to determine type
	switch raw[0] {
	case 'n': // null
		if len(raw) == 4 && raw[1] == 'u' && raw[2] == 'l' && raw[3] == 'l' {
			return nil
		}
	case 't', 'f': // true or false
		var b bool
		if err := json.Unmarshal(raw, &b); err == nil {
			return b
		}
	case '"': // string
		var str string
		if err := json.Unmarshal(raw, &str); err == nil {
			if strings.HasPrefix(str, SecureStringPrefix) {
				return SecureString(strings.TrimPrefix(str, SecureStringPrefix))
			}
			return str
		}
	case '[': // array
		var arr []json.RawMessage
		if err := json.Unmarshal(raw, &arr); err == nil {
			result := make([]any, len(arr))
			for i, item := range arr {
				result[i] = processValue(item)
			}
			return result
		}
	case '{': // object
		var obj map[string]json.RawMessage
		if err := json.Unmarshal(raw, &obj); err == nil {
			result := make(map[string]any)
			for k, v := range obj {
				result[k] = processValue(v)
			}
			return result
		}
	default: // number (or other)
		var num float64
		if err := json.Unmarshal(raw, &num); err == nil {
			return num
		}
	}

	// Fallback for unrecognized types
	return nil
}

type EnvVars map[string]any

func (e EnvVars) String() string {
	if len(e) == 0 {
		return ""
	}
	b, err := json.Marshal(e)
	if err != nil {
		return err.Error()
	}
	return string(b)
}

// Set(string) error is required to implement flag.Value interface.
// Set parses and adds a new Env variable from the given string.
// The format is name=value where value is a JSON value.
func (e EnvVars) Set(value string) error {
	fmt.Println("EnvVars Set:", value)
	tokens := strings.SplitN(value, "=", 2)
	if len(tokens) != 2 {
		return fmt.Errorf("invalid env variable: %s", value)
	}
	e[tokens[0]] = tokens[1]
	return nil
}

type FSTab struct {
	MountPoint string `json:"mountPoint"`
	Source     string `json:"source"`
	FS         fs.FS  `json:"-"`
}

type FSTabs []FSTab

// Set(string) error is required to implement flag.Value interface.
// Set parses and adds a new FSTab from the given string.
// The format is /mountpoint=source
func (m *FSTabs) Set(value string) error {
	tokens := strings.SplitN(value, "=", 2)
	if len(tokens) != 2 {
		return fmt.Errorf("invalid mount option: %s", value)
	}
	*m = append(*m, FSTab{
		MountPoint: tokens[0],
		Source:     tokens[1],
	})
	return nil
}

func (m FSTabs) HasMountPoint(mountPoint string) bool {
	for _, tab := range m {
		if tab.MountPoint == mountPoint {
			return true
		}
	}
	return false
}

func (m FSTabs) String() string {
	b, err := json.Marshal(m)
	if err != nil {
		return err.Error()
	}
	return string(b)
}

func (m FSTabs) MarshalJSON() ([]byte, error) {
	type fstabAlias FSTab
	aliasList := []fstabAlias{}
	for _, tab := range m {
		if tab.MountPoint == "" || tab.Source == "" {
			continue
		}
		aliasList = append(aliasList, fstabAlias{
			MountPoint: tab.MountPoint,
			Source:     tab.Source,
		})
	}
	return json.Marshal(aliasList)
}

func (m *FSTabs) UnmarshalJSON(data []byte) error {
	type fstabAlias FSTab
	aliasList := []fstabAlias{}
	if err := json.Unmarshal(data, &aliasList); err != nil {
		return err
	}
	for _, tab := range aliasList {
		*m = append(*m, FSTab{
			MountPoint: tab.MountPoint,
			Source:     tab.Source,
		})
	}
	return nil
}

type SecretBox struct {
	secretFile string
}

func NewSecretBox(secret any) (*SecretBox, error) {
	// gen random file name
	randomBytes := make([]byte, 8)
	rand.Read(randomBytes)
	filename := fmt.Sprintf("jsh-%d-%s", os.Getpid(), hex.EncodeToString(randomBytes))

	secretFile := filepath.Join(os.TempDir(), filename)

	// 0600 owner read/write
	fd, err := os.OpenFile(secretFile, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
	if err != nil {
		return nil, err
	}
	defer fd.Close()

	enc := json.NewEncoder(fd)
	if err := enc.Encode(secret); err != nil {
		return nil, err
	}

	return &SecretBox{secretFile: secretFile}, nil
}

func (sb *SecretBox) FilePath() string {
	return sb.secretFile
}

func (sb *SecretBox) Cleanup() {
	if sb.secretFile == "" {
		return
	}
	os.WriteFile(sb.secretFile, []byte(""), 0600)
	os.Remove(sb.secretFile)
}

func ReadSecretBox(secretFile string, o interface{}) error {
	defer func() {
		// delete the file
		os.WriteFile(secretFile, []byte(""), 0600)
		os.Remove(secretFile)
	}()

	fd, err := os.Open(secretFile)
	if err != nil {
		return err
	}
	defer fd.Close()

	dec := json.NewDecoder(fd)
	return dec.Decode(o)
}
