package tagql

import (
	"fmt"
	"os"
	"path/filepath"
)

type Loader interface {
	Load(path string) (Script, error)
}

type Script interface {
	Parse(table, tag string) (TagQL, error)
	String() string
}

type loader struct {
	dirs []string
}

func NewLoader(dirs []string) Loader {
	abs := []string{}

	for _, d := range dirs {
		ap, err := filepath.Abs(d)
		if err != nil {
			continue
		}
		abs = append(abs, ap)
	}
	return &loader{
		dirs: abs,
	}
}

func (ld *loader) Load(path string) (Script, error) {
	var ret *script
	for _, d := range ld.dirs {
		joined := filepath.Join(d, path)
		stat, err := os.Stat(joined)
		if err != nil || stat.IsDir() {
			continue
		}

		ret = &script{
			path: joined,
		}
		break
	}
	if ret == nil {
		return nil, fmt.Errorf("not found '%s'", path)
	}
	return ret, nil
}

type script struct {
	path string
}

func (sc *script) String() string {
	return fmt.Sprintf("path: %s", sc.path)
}

func (sc *script) Parse(table, tag string) (TagQL, error) {
	file, err := os.Open(sc.path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	tql, err := Parse(table, tag, file)
	if err != nil {
		return nil, err
	}
	return tql, nil
}