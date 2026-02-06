package readline

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/nyaosorg/go-readline-ny"
)

var _ readline.IHistory = (*History)(nil)

type History struct {
	buffer   []string
	limit    int
	filepath string
}

func PrefDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = ".jsh"
	}
	dir := filepath.Join(home, ".config", ".jsh")
	os.MkdirAll(dir, 0755)
	return dir
}

func NewHistory(filename string, limit int) *History {
	ret := &History{
		limit:  limit,
		buffer: make([]string, 0, limit),
	}
	ret.filepath = filepath.Join(PrefDir(), filename)
	bytes, err := os.ReadFile(ret.filepath)
	if err != nil {
		return ret
	}
	prev := ""
	for _, line := range strings.Split(string(bytes), "\n") {
		if strings.HasSuffix(line, "\\") {
			prev += strings.TrimSuffix(line, "\\") + "\n"
			continue
		}
		line = prev + line
		prev = ""
		if line == "" {
			continue
		}
		ret.buffer = append(ret.buffer, line)
	}
	ret.trimLimit()
	return ret
}
func (h *History) Len() int {
	return len(h.buffer)
}
func (h *History) At(at int) string {
	return h.buffer[at]
}
func (h *History) Add(line string) {
	if len(h.buffer) > 0 && h.buffer[len(h.buffer)-1] == line {
		return
	}
	h.buffer = append(h.buffer, line)
	h.trimLimit()
}
func (h *History) trimLimit() {
	if len(h.buffer) > h.limit {
		h.buffer = h.buffer[len(h.buffer)-h.limit:]
	}
	h.flush()
}
func (h *History) flush() {
	out, err := os.OpenFile(h.filepath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer out.Close()
	for _, line := range h.buffer {
		line = strings.ReplaceAll(line, "\n", "\\\n")
		out.WriteString(line + "\n")
	}
}
