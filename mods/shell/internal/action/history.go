package action

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/nyaosorg/go-readline-ny"
)

var _ readline.IHistory = &History{}

type History struct {
	buffer   []string
	limit    int
	filepath string
}

func NewHistory(limit int) *History {
	ret := &History{
		limit:  limit,
		buffer: make([]string, 0),
	}
	// TODO
	ret.filepath = filepath.Join(PrefDir(), ".neoshell_history")
	bytes, err := os.ReadFile(ret.filepath)
	if err != nil {
		return ret
	}
	ret.buffer = strings.Split(string(bytes), "\n")
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
	os.WriteFile(h.filepath, []byte(strings.Join(h.buffer, "\n")), 0644)
}
