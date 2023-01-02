package shell

import (
	"bytes"
	"strconv"

	"github.com/c-bata/go-prompt"
)

// PosixWriter generates Posix escape sequences.
type PosixWriter struct {
	buffer []byte
}

// WriteRaw to write raw byte array
func (w *PosixWriter) WriteRaw(data []byte) {
	w.buffer = append(w.buffer, data...)
}

// Write to write safety byte array by removing control sequences.
func (w *PosixWriter) Write(data []byte) {
	w.WriteRaw(bytes.Replace(data, []byte{0x1b}, []byte{'?'}, -1))
}

// WriteRawStr to write raw string
func (w *PosixWriter) WriteRawStr(data string) {
	w.WriteRaw([]byte(data))
}

// WriteStr to write safety string by removing control sequences.
func (w *PosixWriter) WriteStr(data string) {
	w.Write([]byte(data))
}

/* Erase */

// EraseScreen erases the screen with the background colour and moves the cursor to home.
func (w *PosixWriter) EraseScreen() {
	w.WriteRaw([]byte{0x1b, '[', '2', 'J'})
}

// EraseUp erases the screen from the current line up to the top of the screen.
func (w *PosixWriter) EraseUp() {
	w.WriteRaw([]byte{0x1b, '[', '1', 'J'})
}

// EraseDown erases the screen from the current line down to the bottom of the screen.
func (w *PosixWriter) EraseDown() {
	w.WriteRaw([]byte{0x1b, '[', 'J'})
}

// EraseStartOfLine erases from the current cursor position to the start of the current line.
func (w *PosixWriter) EraseStartOfLine() {
	w.WriteRaw([]byte{0x1b, '[', '1', 'K'})
}

// EraseEndOfLine erases from the current cursor position to the end of the current line.
func (w *PosixWriter) EraseEndOfLine() {
	w.WriteRaw([]byte{0x1b, '[', 'K'})
}

// EraseLine erases the entire current line.
func (w *PosixWriter) EraseLine() {
	w.WriteRaw([]byte{0x1b, '[', '2', 'K'})
}

/* Cursor */

// ShowCursor stops blinking cursor and show.
func (w *PosixWriter) ShowCursor() {
	w.WriteRaw([]byte{0x1b, '[', '?', '1', '2', 'l', 0x1b, '[', '?', '2', '5', 'h'})
}

// HideCursor hides cursor.
func (w *PosixWriter) HideCursor() {
	w.WriteRaw([]byte{0x1b, '[', '?', '2', '5', 'l'})
}

// CursorGoTo sets the cursor position where subsequent text will begin.
func (w *PosixWriter) CursorGoTo(row, col int) {
	if row == 0 && col == 0 {
		// If no row/column parameters are provided (ie. <ESC>[H), the cursor will move to the home position.
		w.WriteRaw([]byte{0x1b, '[', 'H'})
		return
	}
	r := strconv.Itoa(row)
	c := strconv.Itoa(col)
	w.WriteRaw([]byte{0x1b, '['})
	w.WriteRaw([]byte(r))
	w.WriteRaw([]byte{';'})
	w.WriteRaw([]byte(c))
	w.WriteRaw([]byte{'H'})
}

// CursorUp moves the cursor up by 'n' rows; the default count is 1.
func (w *PosixWriter) CursorUp(n int) {
	if n == 0 {
		return
	} else if n < 0 {
		w.CursorDown(-n)
		return
	}
	s := strconv.Itoa(n)
	w.WriteRaw([]byte{0x1b, '['})
	w.WriteRaw([]byte(s))
	w.WriteRaw([]byte{'A'})
}

// CursorDown moves the cursor down by 'n' rows; the default count is 1.
func (w *PosixWriter) CursorDown(n int) {
	if n == 0 {
		return
	} else if n < 0 {
		w.CursorUp(-n)
		return
	}
	s := strconv.Itoa(n)
	w.WriteRaw([]byte{0x1b, '['})
	w.WriteRaw([]byte(s))
	w.WriteRaw([]byte{'B'})
}

// CursorForward moves the cursor forward by 'n' columns; the default count is 1.
func (w *PosixWriter) CursorForward(n int) {
	if n == 0 {
		return
	} else if n < 0 {
		w.CursorBackward(-n)
		return
	}
	s := strconv.Itoa(n)
	w.WriteRaw([]byte{0x1b, '['})
	w.WriteRaw([]byte(s))
	w.WriteRaw([]byte{'C'})
}

// CursorBackward moves the cursor backward by 'n' columns; the default count is 1.
func (w *PosixWriter) CursorBackward(n int) {
	if n == 0 {
		return
	} else if n < 0 {
		w.CursorForward(-n)
		return
	}
	s := strconv.Itoa(n)
	w.WriteRaw([]byte{0x1b, '['})
	w.WriteRaw([]byte(s))
	w.WriteRaw([]byte{'D'})
}

// AskForCPR asks for a cursor position report (CPR).
func (w *PosixWriter) AskForCPR() {
	// CPR: Cursor Position Request.
	w.WriteRaw([]byte{0x1b, '[', '6', 'n'})
}

// SaveCursor saves current cursor position.
func (w *PosixWriter) SaveCursor() {
	w.WriteRaw([]byte{0x1b, '[', 's'})
}

// UnSaveCursor restores cursor position after a Save Cursor.
func (w *PosixWriter) UnSaveCursor() {
	w.WriteRaw([]byte{0x1b, '[', 'u'})
}

/* Scrolling */

// ScrollDown scrolls display down one line.
func (w *PosixWriter) ScrollDown() {
	w.WriteRaw([]byte{0x1b, 'D'})
}

// ScrollUp scroll display up one line.
func (w *PosixWriter) ScrollUp() {
	w.WriteRaw([]byte{0x1b, 'M'})
}

/* Title */

// SetTitle sets a title of terminal window.
func (w *PosixWriter) SetTitle(title string) {
	titleBytes := []byte(title)
	patterns := []struct {
		from []byte
		to   []byte
	}{
		{
			from: []byte{0x13},
			to:   []byte{},
		},
		{
			from: []byte{0x07},
			to:   []byte{},
		},
	}
	for i := range patterns {
		titleBytes = bytes.Replace(titleBytes, patterns[i].from, patterns[i].to, -1)
	}

	w.WriteRaw([]byte{0x1b, ']', '2', ';'})
	w.WriteRaw(titleBytes)
	w.WriteRaw([]byte{0x07})
}

// ClearTitle clears a title of terminal window.
func (w *PosixWriter) ClearTitle() {
	w.WriteRaw([]byte{0x1b, ']', '2', ';', 0x07})
}

/* Font */

// SetColor sets text and background colors. and specify whether text is bold.
func (w *PosixWriter) SetColor(fg, bg prompt.Color, bold bool) {
	if bold {
		w.SetDisplayAttributes(fg, bg, prompt.DisplayBold)
	} else {
		// If using `DisplayDefualt`, it will be broken in some environment.
		// Details are https://github.com/c-bata/go-prompt/pull/85
		w.SetDisplayAttributes(fg, bg, prompt.DisplayReset)
	}
}

// SetDisplayAttributes to set Posix display attributes.
func (w *PosixWriter) SetDisplayAttributes(fg, bg prompt.Color, attrs ...prompt.DisplayAttribute) {
	w.WriteRaw([]byte{0x1b, '['}) // control sequence introducer
	defer w.WriteRaw([]byte{'m'}) // final character

	var separator byte = ';'
	for i := range attrs {
		p, ok := displayAttributeParameters[attrs[i]]
		if !ok {
			continue
		}
		w.WriteRaw(p)
		w.WriteRaw([]byte{separator})
	}

	f, ok := foregroundANSIColors[fg]
	if !ok {
		f = foregroundANSIColors[prompt.DefaultColor]
	}
	w.WriteRaw(f)
	w.WriteRaw([]byte{separator})
	b, ok := backgroundANSIColors[bg]
	if !ok {
		b = backgroundANSIColors[prompt.DefaultColor]
	}
	w.WriteRaw(b)
}

var displayAttributeParameters = map[prompt.DisplayAttribute][]byte{
	prompt.DisplayReset:        {'0'},
	prompt.DisplayBold:         {'1'},
	prompt.DisplayLowIntensity: {'2'},
	prompt.DisplayItalic:       {'3'},
	prompt.DisplayUnderline:    {'4'},
	prompt.DisplayBlink:        {'5'},
	prompt.DisplayRapidBlink:   {'6'},
	prompt.DisplayReverse:      {'7'},
	prompt.DisplayInvisible:    {'8'},
	prompt.DisplayCrossedOut:   {'9'},
	prompt.DisplayDefaultFont:  {'1', '0'},
}

var foregroundANSIColors = map[prompt.Color][]byte{
	prompt.DefaultColor: {'3', '9'},

	// Low intensity.
	prompt.Black:     {'3', '0'},
	prompt.DarkRed:   {'3', '1'},
	prompt.DarkGreen: {'3', '2'},
	prompt.Brown:     {'3', '3'},
	prompt.DarkBlue:  {'3', '4'},
	prompt.Purple:    {'3', '5'},
	prompt.Cyan:      {'3', '6'},
	prompt.LightGray: {'3', '7'},

	// High intensity.
	prompt.DarkGray:  {'9', '0'},
	prompt.Red:       {'9', '1'},
	prompt.Green:     {'9', '2'},
	prompt.Yellow:    {'9', '3'},
	prompt.Blue:      {'9', '4'},
	prompt.Fuchsia:   {'9', '5'},
	prompt.Turquoise: {'9', '6'},
	prompt.White:     {'9', '7'},
}

var backgroundANSIColors = map[prompt.Color][]byte{
	prompt.DefaultColor: {'4', '9'},

	// Low intensity.
	prompt.Black:     {'4', '0'},
	prompt.DarkRed:   {'4', '1'},
	prompt.DarkGreen: {'4', '2'},
	prompt.Brown:     {'4', '3'},
	prompt.DarkBlue:  {'4', '4'},
	prompt.Purple:    {'4', '5'},
	prompt.Cyan:      {'4', '6'},
	prompt.LightGray: {'4', '7'},

	// High intensity
	prompt.DarkGray:  {'1', '0', '0'},
	prompt.Red:       {'1', '0', '1'},
	prompt.Green:     {'1', '0', '2'},
	prompt.Yellow:    {'1', '0', '3'},
	prompt.Blue:      {'1', '0', '4'},
	prompt.Fuchsia:   {'1', '0', '5'},
	prompt.Turquoise: {'1', '0', '6'},
	prompt.White:     {'1', '0', '7'},
}
