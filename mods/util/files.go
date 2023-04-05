package util

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

func MkDirIfNotExists(path string) error {
	return MkDirIfNotExistsMode(path, 0755)
}

func MkDirIfNotExistsMode(path string, mode fs.FileMode) error {
	sep := string(filepath.Separator)
	dirs := strings.Split(path, sep)
	if strings.HasPrefix(path, sep) {
		path = "/"
	} else {
		path = ""
	}
	for _, d := range dirs {
		if d == "" {
			continue
		}
		path = filepath.Join(path, d)
		_, err := os.Stat(path)
		if err != nil && os.IsNotExist(err) {
			if err := os.Mkdir(path, mode); err != nil {
				return err
			}
		} else if err != nil {
			return err
		}
	}
	return nil
}
