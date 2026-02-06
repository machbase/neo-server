package engine

import (
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"

	"github.com/dop251/goja_nodejs/require"
)

type Env struct {
	writer      io.Writer
	reader      io.Reader
	fs          fs.FS
	execBuilder ExecBuilderFunc
	vars        map[string]any

	aliases            map[string][]string
	aliasCaseSensitive bool
}

// ExecBuilderFunc is a function that builds an *exec.Cmd given the source and arguments.
// if code is empty, it indicates that the file is being executed from file named in args[0].
// if code is non-empty, it indicates that the code is being executed.
type ExecBuilderFunc func(code string, args []string, env map[string]any) (*exec.Cmd, error)

// Which looks for the command in the PATH environment variable and returns the full path to the command.
func (env *Env) Which(command string) string {
	if !strings.HasSuffix(command, ".js") {
		command += ".js"
	}
	if strings.HasPrefix(command, "/") {
		return command
	}
	filesystem := env.Filesystem()
	pathVar := env.Get("PATH")
	if pathStr, ok := pathVar.(string); ok {
		paths := strings.Split(pathStr, ":")
		for _, dir := range paths {
			fullPath := filepath.Join(dir, command)
			fullPath = env.ResolvePath(fullPath)
			if fi, err := filesystem.Open(fullPath); err == nil {
				fi.Close()
				return fullPath
			}
		}
	}
	return ""
}

func (env *Env) SetAliasCaseSensitive(caseSensitive bool) {
	env.aliasCaseSensitive = caseSensitive
}

func (env *Env) SetAlias(command string, alias []string) {
	if env.aliases == nil {
		env.aliases = make(map[string][]string)
	}
	key := command
	if !env.aliasCaseSensitive {
		key = strings.ToLower(command)
	}
	env.aliases[key] = alias
}

func (env *Env) Alias(command string) []string {
	key := command
	if !env.aliasCaseSensitive {
		key = strings.ToLower(command)
	}
	if alias, ok := env.aliases[key]; ok {
		return alias
	}
	return []string{command}
}

type SecureString string

const SecureMask = "********"
const SecureStringPrefix = "SecureString:"

func (s SecureString) String() string {
	return SecureMask
}

func (s SecureString) Value() string {
	return string(s)
}

// MarshalJSON implements json.Marshaler interface
// It prefixes the value with "SecureString:" to preserve the original value
func (s SecureString) MarshalJSON() ([]byte, error) {
	return json.Marshal(SecureStringPrefix + string(s))
}

// UnmarshalJSON implements json.Unmarshaler interface
// It converts JSON string to SecureString type, handling both prefixed and non-prefixed values
func (s *SecureString) UnmarshalJSON(data []byte) error {
	var str string
	if err := json.Unmarshal(data, &str); err != nil {
		return err
	}
	// Remove prefix if present
	str, _ = strings.CutPrefix(str, SecureStringPrefix)
	*s = SecureString(str)
	return nil
}

// expand $VAR and ${VAR} form environment
// Single quoted strings are not expanded
func (env *Env) Expand(str string) string {
	var result strings.Builder
	inSingleQuote := false
	inDoubleQuote := false
	i := 0

	for i < len(str) {
		ch := str[i]

		// Handle quote state changes
		if ch == '\'' && !inDoubleQuote {
			inSingleQuote = !inSingleQuote
			result.WriteByte(ch)
			i++
			continue
		}
		if ch == '"' && !inSingleQuote {
			inDoubleQuote = !inDoubleQuote
			result.WriteByte(ch)
			i++
			continue
		}

		// If in single quotes, don't expand variables
		if inSingleQuote {
			result.WriteByte(ch)
			i++
			continue
		}

		// Handle variable expansion
		if ch == '$' && i+1 < len(str) {
			varStart := i
			i++ // skip $

			var varName string
			if str[i] == '{' {
				// ${VAR} format
				i++ // skip {
				end := strings.IndexByte(str[i:], '}')
				if end == -1 {
					// No closing brace, treat as literal
					result.WriteString(str[varStart:i])
					continue
				}
				varName = str[i : i+end]
				i += end + 1 // skip varName and }
			} else {
				// $VAR format
				start := i
				for i < len(str) && (isAlphaNumeric(str[i]) || str[i] == '_') {
					i++
				}
				varName = str[start:i]
			}

			// Expand variable
			if varName != "" {
				val := env.Get(varName)
				switch v := val.(type) {
				case string:
					result.WriteString(v)
				case SecureString:
					result.WriteString(SecureMask)
				case nil:
					// empty
				default:
					result.WriteString(fmt.Sprintf("%v", v))
				}
			}
		} else {
			result.WriteByte(ch)
			i++
		}
	}

	return result.String()
}

func isAlphaNumeric(ch byte) bool {
	return (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9')
}

// ResolvePath resolves a path by expanding ~ to HOME and expanding environment variables.
func (env *Env) ResolvePath(path string) string {
	if strings.HasPrefix(path, "~") {
		home := env.Get("HOME")
		if homeStr, ok := home.(string); ok {
			path = filepath.Join(homeStr, path[1:])
		}
	}
	path = os.Expand(path, func(varName string) string {
		val := env.Get(varName)
		if valStr, ok := val.(string); ok {
			return valStr
		}
		return ""
	})
	path = filepath.Clean(path)
	return filepath.ToSlash(path)
}

func (env *Env) ResolveAbsPath(path string) string {
	path = env.ResolvePath(path)
	if !strings.HasPrefix(path, "/") {
		cwd := env.Get("PWD")
		if cwdStr, ok := cwd.(string); ok {
			path = filepath.Join(cwdStr, path)
			path = filepath.ToSlash(path)
		}
	}
	return path
}

func (env *Env) GlobalFolders() []string {
	path := env.Get("LIBRARY_PATH")
	if pathStr, ok := path.(string); ok && pathStr != "" {
		parts := strings.Split(pathStr, ":")
		return parts
	}
	return []string{}
}

func (env *Env) PathResolver(base, path string) string {
	if base == "." || strings.HasPrefix(base, "./") {
		cwd := env.Get("PWD")
		if cwdStr, ok := cwd.(string); ok {
			base = filepath.ToSlash(filepath.Join(cwdStr, base))
		}
	} else if base == ".." || strings.HasPrefix(base, "../") {
		cwd := env.Get("PWD")
		if cwdStr, ok := cwd.(string); ok {
			base = filepath.ToSlash(filepath.Join(cwdStr, base))
		}
	}
	resolved := require.DefaultPathResolver(base, path)
	resolved = filepath.ToSlash(resolved)

	var filesystem fs.FS = env.Filesystem()
	// resolve as .js file
	asFile := resolved
	if !strings.HasSuffix(asFile, ".js") {
		asFile += ".js"
	}
	if f, err := filesystem.Open(asFile); err == nil {
		f.Close()
		return asFile
	}
	// resolve as directory/index.js
	asIndex := resolved + "/index.js"
	if f, err := filesystem.Open(asIndex); err == nil {
		f.Close()
		return asIndex
	}
	// resolve as directory/package.json main entry
	pkgPath := resolved + "/package.json"
	pkgFile, err := filesystem.Open(pkgPath)
	if err == nil {
		defer pkgFile.Close()
		pkgData, err := io.ReadAll(pkgFile)
		if err == nil {
			var mainEntry struct {
				Main string `json:"main"`
			}
			if err := json.Unmarshal(pkgData, &mainEntry); err == nil {
				if mainEntry.Main != "" {
					mainPath := filepath.Join(resolved, mainEntry.Main)
					mainPath = filepath.ToSlash(mainPath)
					if !strings.HasSuffix(mainPath, ".js") {
						mainPath += ".js"
					}
					if f, err := filesystem.Open(mainPath); err == nil {
						f.Close()
						return mainPath
					}
				}
			}
		}
	}
	return resolved
}

func (env *Env) LoadSource(moduleName string) ([]byte, error) {
	moduleName = filepath.ToSlash(moduleName) // for Windows compatibility
	var fileSystem fs.FS = env.Filesystem()
	if fileSystem == nil {
		return nil, fmt.Errorf("no filesystem available to load module: %s", moduleName)
	}
	if strings.HasPrefix(moduleName, "/") {
		b, err := loadSource(fileSystem, moduleName)
		if err == nil {
			return b, nil
		}
	}
	return nil, require.ModuleFileDoesNotExistError
}

func loadSource(fileSystem fs.FS, moduleName string) ([]byte, error) {
	file, err := fileSystem.Open(moduleName)
	if err != nil {
		if !strings.HasSuffix(moduleName, ".js") {
			file, err = fileSystem.Open(moduleName + ".js")
		}
		if err != nil {
			return nil, err
		}
	}
	defer file.Close()
	isDir := false
	if fi, err := file.Stat(); err != nil {
		return nil, err
	} else if fi.IsDir() {
		isDir = true
	}
	if isDir {
		return loadSourceFromDir(fileSystem, moduleName)
	} else {
		return io.ReadAll(file)
	}
}

func loadSourceFromDir(fileSystem fs.FS, moduleName string) ([]byte, error) {
	// look for package.json
	pkgFile, err := fileSystem.Open(moduleName + "/package.json")
	if err == nil {
		defer pkgFile.Close()
		pkgData, err := io.ReadAll(pkgFile)
		if err != nil {
			return nil, err
		}
		var mainEntry struct {
			Main string `json:"main"`
		}
		if err := json.Unmarshal(pkgData, &mainEntry); err != nil {
			return nil, err
		}
		if mainEntry.Main != "" {
			mainPath := filepath.Join(moduleName, mainEntry.Main)
			mainPath = filepath.ToSlash(mainPath)
			if !strings.HasSuffix(mainPath, ".js") {
				mainPath += ".js"
			}
			if main, err := fileSystem.Open(mainPath); err == nil {
				defer main.Close()
				return io.ReadAll(main)
			}
		}
	} else {
		// look for index.js
		indexPath := moduleName + "/index.js"
		if f, err := fileSystem.Open(indexPath); err == nil {
			defer f.Close()
			return io.ReadAll(f)
		}
	}
	return nil, fs.ErrNotExist
}

// cleanPath normalizes a path and ensures it starts with /
func CleanPath(p string) string {
	if p == "" || p == "/" || p == "." {
		return "/"
	}
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	p = path.Clean(p)
	if p == "." {
		return "/"
	}
	return p
}

func NewEnv(opts ...EnvOption) *Env {
	ret := &Env{}
	for _, opt := range opts {
		opt(ret)
	}
	return ret
}

type EnvOption func(*Env)

func WithAliases(aliases map[string]string) EnvOption {
	return func(de *Env) {
		for k, v := range aliases {
			de.SetAlias(k, strings.Split(v, " "))
		}
	}
}

func WithFilesystem(fs fs.FS) EnvOption {
	return func(de *Env) {
		de.fs = fs
	}
}

func WithWriter(w io.Writer) EnvOption {
	return func(de *Env) {
		de.writer = w
	}
}

func WithReader(r io.Reader) EnvOption {
	return func(de *Env) {
		de.reader = r
	}
}

func WithExecBuilder(eb ExecBuilderFunc) EnvOption {
	return func(de *Env) {
		de.execBuilder = eb
	}
}

func (e *Env) Reader() io.Reader {
	if e.reader != nil {
		return e.reader
	}
	return os.Stdin
}

func (e *Env) Writer() io.Writer {
	if e.writer != nil {
		return e.writer
	}
	return os.Stdout
}

func (e *Env) Filesystem() fs.FS {
	return e.fs
}

func (e *Env) ExecBuilder() ExecBuilderFunc {
	return e.execBuilder
}

func (e *Env) Set(key string, value any) {
	if e.vars == nil {
		e.vars = make(map[string]any)
	}
	if value == nil {
		delete(e.vars, key)
		return
	}
	e.vars[key] = value
}

func (e *Env) Get(key string) any {
	if e.vars == nil {
		return nil
	}
	return e.vars[key]
}
