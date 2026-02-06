package shell

import (
	"github.com/nyaosorg/go-ttyadapter"
	"github.com/nyaosorg/go-ttyadapter/tty8"
)

func NewTty() ttyadapter.Tty {
	return &TtyWrap{Tty: &tty8.Tty{}}
}

// TtyWrap is a wrapper around ttyadapter.Tty to provide default size values.
// This prevents issues with xterm.js that creates a PTY and it reports 0x0 size at initial state,
// that causes problems with multiline editor.
type TtyWrap struct {
	ttyadapter.Tty
}

var _ ttyadapter.Tty = (*TtyWrap)(nil)

func (tty *TtyWrap) Size() (int, int, error) {
	cols, rows, err := tty.Tty.Size()
	if cols == 0 {
		cols = 80
	}
	if rows == 0 {
		rows = 24
	}
	// Debugging output
	//
	// fd, _ := os.OpenFile("/tmp/jsh_debug_buffer.txt", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	// fmt.Fprintf(fd, "TtyWrap Size called: %d cols, %d rows, err=%v\n", cols, rows, err)
	return cols, rows, err
}
