package jsh

import (
	"fmt"

	js "github.com/dop251/goja"
)

// https://en.wikipedia.org/wiki/ANSI_escape_code#C0_control_codes
const (
	C0_NUL = byte(0x00) // ^@ null
	C0_SOH = byte(0x01) // ^A start of heading
	C0_STX = byte(0x02) // ^B start of text
	C0_ETX = byte(0x03) // ^C end of text
	C0_EOT = byte(0x04) // ^D end of transmission
	C0_ENQ = byte(0x05) // ^E enquiry
	C0_ACK = byte(0x06) // ^F acknowledge
	C0_BEL = byte(0x07) // ^G
	C0_BS  = byte(0x08) // ^H Backspace
	C0_HT  = byte(0x09) // ^I Tab
	C0_LF  = byte(0x0A) // ^J line feed
	C0_VT  = byte(0x0B) // ^K vertical tab
	C0_FF  = byte(0x0C) // ^L form feed
	C0_CR  = byte(0x0D) // ^M carriage return
	C0_SO  = byte(0x0E) // ^N shift out
	C0_SI  = byte(0x0F) // ^O shift in
	C0_DLE = byte(0x10) // ^P data link escape
	C0_DC1 = byte(0x11) // ^Q device control 1
	C0_DC2 = byte(0x12) // ^R device control 2
	C0_DC3 = byte(0x13) // ^S device control 3
	C0_DC4 = byte(0x14) // ^T device control 4
	C0_NAK = byte(0x15) // ^U negative acknowledge
	C0_SYN = byte(0x16) // ^V synchronous idle
	C0_ETB = byte(0x17) // ^W end of transmission block
	C0_CAN = byte(0x18) // ^X cancel
	C0_EM  = byte(0x19) // ^Y end of medium
	C0_SUB = byte(0x1A) // ^Z substitute
	C0_ESC = byte(0x1B) // ^[
	C0_FS  = byte(0x1C) // ^\ file separator
	C0_GS  = byte(0x1D) // ^] group separator
	C0_RS  = byte(0x1E) // ^^ record separator
	C0_US  = byte(0x1F) // ^_ unit separator
	C0_DEL = byte(0x7F) // ^? delete
)

// https://en.wikipedia.org/wiki/ANSI_escape_code#Fe_Escape_sequences
const (
	FE_SS2 = byte('N')  // single shift 2
	FE_SS3 = byte('O')  // single shift 3
	FE_DCS = byte('P')  // device control string
	FE_CSI = byte('[')  // control sequence introducer
	FE_ST  = byte('\\') // string terminator
	FE_OSC = byte(']')  // operating system command
	FE_SOS = byte('X')  // start of string
	FE_PM  = byte('^')  // privacy message
	FE_APC = byte('_')  // application program command
)

type CHState int

const (
	CHStateNormal CHState = iota
	CHStateESC
	CHStateCSI
)

func (j *Jsh) readCh() (byte, error) {
	b := make([]byte, 1)
	if _, err := j.reader.Read(b); err != nil {
		return 0, err
	}
	return b[0], nil
}

// m.readLine()
func (j *Jsh) process_readLine() js.Value {
	line := []byte{}
	state := CHStateNormal
	nm := []byte{}

	defer func() {
		if e := recover(); e != nil {
			panic(j.vm.ToValue(fmt.Sprintf("panic: %v", e)))
		}
	}()

	for {
		var ch byte
		if c, err := j.readCh(); err != nil {
			panic(j.vm.ToValue(err.Error()))
		} else {
			ch = c
		}
		if ch == 0x00 {
			continue
		}
		if state == CHStateNormal {
			if ch >= 0x20 && ch <= 0x7E {
				// printable character
				if j.echo {
					j.writer.Write([]byte{ch})
				}
				line = append(line, ch)
			} else {
				switch {
				case ch == C0_ESC:
					state = CHStateESC
				case ch == C0_ETX: // ctrl-c
					j.writer.Write([]byte{0x0A, 0x0D})
					j.writer.Write([]byte("interrupted\n"))
					j.vm.Interrupt("interrupted")
					return j.vm.ToValue(string(line))
				case ch == C0_EOT: // ctrl-d
					j.writer.Write([]byte{0x0A, 0x0D})
					j.writer.Write([]byte("eof\n"))
					j.vm.Interrupt("eof")
					return j.vm.ToValue(string(line))
				case ch == C0_BS || ch == C0_DEL:
					if len(line) > 0 {
						if j.echo {
							j.writer.Write([]byte{0x08, 0x20, 0x08})
						}
						line = line[:len(line)-1]
					}
				case ch == C0_LF: // line feed
					if j.echo {
						j.writer.Write([]byte{0x0A, 0x0D})
					}
					return j.vm.ToValue(string(line))
				case ch == C0_CR: // return
					if j.echo {
						j.writer.Write([]byte{0x0A, 0x0D})
					}
					return j.vm.ToValue(string(line))
				default:
					fmt.Printf("C0: %x\n", ch)
				}
			}
		} else if state == CHStateESC {
			if ch == FE_CSI {
				state = CHStateCSI
			} else {
				state = CHStateNormal
			}
		} else if state == CHStateCSI {
			if ch >= '0' && ch <= '9' || ch == ';' {
				nm = append(nm, ch)
			} else {
				// https://en.wikipedia.org/wiki/ANSI_escape_code#Terminal_input_sequences
				switch {
				case ch == 'A': // up arrow (xterm sequence)
				case ch == 'B': // down arrow (xterm sequence)
				case ch == 'C': // right arrow (xterm sequence)
				case ch == 'D': // left arrow (xterm sequence)
				case ch == 'E': // cursor next line
				case ch == 'F': // cursor previous line
				case ch == 'G': // cursor to column
				case ch == 'H': // cursor to row
				case ch == 'J': // erase screen
				case ch == 'K': // erase line
				case ch == 'S': // scroll up
				case ch == 'T': // scroll down
				case ch == 'P' && string(nm) == "1": // F1 (xterm sequence)
					fmt.Printf("xterm sequence: F1\n")
				case ch == 'Q' && string(nm) == "1": // F2 (xterm sequence)
					fmt.Printf("xterm sequence: F2\n")
				case ch == 'R' && string(nm) == "1": // F3 (xterm sequence)
					fmt.Printf("xterm sequence: F3\n")
				case ch == 'S' && string(nm) == "1": // F4 (xterm sequence)
					fmt.Printf("xterm sequence: F4\n")
				case ch == '~': // end of vt sequence
					fmt.Printf("vt sequence: n:m=%s\n", string(nm))
				default:
					fmt.Printf("CSI: %c 0x%x n:m=%s\n", ch, ch, string(nm))
				}
				state = CHStateNormal
				if len(nm) > 0 {
					nm = nm[:0]
				}
			}
		}
	}
}
