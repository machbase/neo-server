package util

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
)

func MkDirIfNotExists(path string) error {
	return MkDirIfNotExistsMode(path, 0755)
}

var windowsDrivePattern = regexp.MustCompile("[A-Za-z]:")

func MkDirIfNotExistsMode(path string, mode fs.FileMode) error {
	sep := string(filepath.Separator)
	dirs := strings.Split(path, sep)
	if runtime.GOOS != "windows" && strings.HasPrefix(path, sep) {
		path = "/"
	} else {
		path = ""
	}
	for n, d := range dirs {
		if d == "" {
			continue
		}
		if runtime.GOOS == "windows" && n == 0 {
			if windowsDrivePattern.MatchString(d) {
				path = d + "\\"
				continue
			}
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
