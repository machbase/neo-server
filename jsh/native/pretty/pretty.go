package pretty

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/dop251/goja"
	"golang.org/x/term"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
)

func Module(rt *goja.Runtime, module *goja.Object) {
	// Export native functions
	exports := module.Get("exports").(*goja.Object)
	exports.Set("Table", Table)
	exports.Set("MakeRow", MakeRow)
	exports.Set("Progress", Progress)
	// formatting helpers
	exports.Set("Bytes", Bytes)
	exports.Set("Ints", Ints)
	exports.Set("Durations", Durations)
	// time parsing helper
	exports.Set("parseTime", parseTime)
	// terminal helpers
	exports.Set("isTerminal", IsTerminal)
	exports.Set("getTerminalSize", GetTerminalSize)
	exports.Set("pauseTerminal", PauseTerminal)
}

func parseTime(value string, format string, tz string) (time.Time, error) {
	switch format {
	case "ns":
		i, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return time.Time{}, fmt.Errorf("failed to parse time '%s' as integer: %v", value, err)
		}
		return time.Unix(0, i), nil
	case "us":
		i, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return time.Time{}, fmt.Errorf("failed to parse time '%s' as integer: %v", value, err)
		}
		return time.Unix(0, i*1000), nil
	case "ms":
		i, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return time.Time{}, fmt.Errorf("failed to parse time '%s' as integer: %v", value, err)
		}
		return time.Unix(0, i*1000000), nil
	case "s":
		i, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return time.Time{}, fmt.Errorf("failed to parse time '%s' as integer: %v", value, err)
		}
		return time.Unix(i, 0), nil
	}
	loc := time.UTC
	if tz != "" {
		if strings.ToLower(tz) == "local" {
			loc = time.Local
			goto LOAD_LOC_DONE
		} else if strings.ToLower(tz) == "utc" {
			loc = time.UTC
			goto LOAD_LOC_DONE
		}
		if l, err := time.LoadLocation(tz); err == nil {
			loc = l
		} else {
			return time.Time{}, fmt.Errorf("failed to load location '%s': %v", tz, err)
		}
	}
LOAD_LOC_DONE:
	if format == "" {
		// try to parse with RFC3339
		if t, err := time.ParseInLocation(time.RFC3339, value, loc); err == nil {
			return t, nil
		} else {
			return time.Time{}, fmt.Errorf("failed to parse time '%s' with RFC3339: %v", value, err)
		}
	} else {
		if t, err := time.ParseInLocation(format, value, loc); err == nil {
			return t, nil
		} else {
			return time.Time{}, fmt.Errorf("failed to parse time '%s' with format '%s': %v", value, format, err)
		}
	}
}

func IsTerminal() bool {
	return term.IsTerminal(int(os.Stdin.Fd()))
}

type TermSize struct {
	Width  int
	Height int
}

func (ts TermSize) String() string {
	return fmt.Sprintf("{Width: %d, Height: %d}", ts.Width, ts.Height)
}

func GetTerminalSize() (TermSize, error) {
	if x, y, err := term.GetSize(int(os.Stdout.Fd())); err == nil {
		return TermSize{Width: x, Height: y}, nil
	} else {
		return TermSize{}, err
	}
}

// PauseTerminal waits for user to press a key. Returns false if user pressed 'q' or 'Q'.
// Otherwise returns true.
func PauseTerminal() bool {
	fmt.Fprintf(os.Stdout, ":")
	// switch stdin into 'raw' mode
	if oldState, err := term.MakeRaw(int(os.Stdin.Fd())); err == nil {
		var b []byte = make([]byte, 3)
		if _, err := os.Stdin.Read(b); err == nil {
			term.Restore(int(os.Stdin.Fd()), oldState)
			// remove prompt, erase the current line
			fmt.Fprintf(os.Stdout, "\x1b[2K")
			// cursor backward
			fmt.Fprintf(os.Stdout, "\x1b[1D")
			if b[0] == 'q' || b[0] == 'Q' {
				return false
			}
			return true
		}
		term.Restore(int(os.Stdin.Fd()), oldState)
	}
	return true
}

var (
	defaultLang language.Tag = language.English
)

func Bytes(v int64) string {
	p := message.NewPrinter(defaultLang)
	f := float64(v)
	u := ""
	switch {
	case v >= 1024*1024*1024*1024:
		f = f / (1024 * 1024 * 1024 * 1024)
		u = "TB"
	case v >= 1024*1024*1024:
		f = f / (1024 * 1024 * 1024)
		u = "GB"
	case v >= 1024*1024:
		f = f / (1024 * 1024)
		u = "MB"
	case v >= 1024:
		f = f / 1024
		u = "KB"
	default:
		return p.Sprintf("%dB", v)
	}
	return p.Sprintf("%.1f%s", f, u)
}

func Ints(v int64) string {
	p := message.NewPrinter(defaultLang)
	return p.Sprintf("%d", v)
}

func Durations(v time.Duration) string {
	p := message.NewPrinter(defaultLang)
	totalNanos := int64(v.Nanoseconds())

	// Handle sub-60-second durations with decimal notation
	if totalNanos < 60*int64(time.Second) {
		if totalNanos < int64(time.Microsecond) {
			// Less than 1μs, just show nanoseconds
			return p.Sprintf("%dns", totalNanos)
		}
		if totalNanos < int64(time.Millisecond) {
			// Show as X.XXμs
			micros := float64(totalNanos) / float64(time.Microsecond)
			if micros == float64(int64(micros)) {
				return p.Sprintf("%dμs", int64(micros))
			}
			return p.Sprintf("%.2fμs", micros)
		}
		if totalNanos < int64(time.Second) {
			// Show as X.XXms
			millis := float64(totalNanos) / float64(time.Millisecond)
			if millis == float64(int64(millis)) {
				return p.Sprintf("%dms", int64(millis))
			}
			return p.Sprintf("%.2fms", millis)
		}
		// Show as X.XXs (under 60 seconds)
		seconds := float64(totalNanos) / float64(time.Second)
		if seconds == float64(int64(seconds)) {
			return p.Sprintf("%ds", int64(seconds))
		}
		return p.Sprintf("%.2fs", seconds)
	}

	// Handle 60+ seconds: show only two highest units
	totalSeconds := int64(v.Seconds())
	days := totalSeconds / 86400
	hours := (totalSeconds % 86400) / 3600
	minutes := (totalSeconds % 3600) / 60
	seconds := totalSeconds % 60

	if days > 0 {
		// Show days and hours
		return p.Sprintf("%dd %dh", days, hours)
	}
	if hours > 0 {
		// Show hours and minutes
		return p.Sprintf("%dh %dm", hours, minutes)
	}
	// Show minutes and seconds
	return p.Sprintf("%dm %ds", minutes, seconds)
}
