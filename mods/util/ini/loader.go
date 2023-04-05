package ini

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"
	"unicode"
)

type lineReader struct {
	reader *bufio.Scanner
}

func newLineReader(reader io.Reader) *lineReader {
	return &lineReader{
		reader: bufio.NewScanner(reader),
	}
}

func (lr *lineReader) readLine() (string, error) {
	if lr.reader.Scan() {
		return lr.reader.Text(), nil
	}
	err := lr.reader.Err()
	if err == nil {
		err = io.EOF
	}
	return "", err
}

func (lr *lineReader) readLinesUntilSuffix(suffix string) string {
	r := ""
	for {
		line, err := lr.readLine()
		if err != nil {
			break
		}
		t := strings.TrimRightFunc(line, unicode.IsSpace)
		if strings.HasSuffix(t, suffix) {
			r = r + t[0:len(t)-len(suffix)]
			break
		} else {
			r = r + line + "\n"
		}
	}
	return r
}

// if a line ends with '\', read the next line
func (lr *lineReader) readContinuationLines() string {
	r := ""
	for {
		line, err := lr.readLine()
		if err != nil {
			break
		}
		line = strings.TrimRightFunc(line, unicode.IsSpace)
		if t, continuation := removeContinuationSuffix(line); continuation {
			r = r + t
		} else {
			r = r + line
			break
		}
	}
	return r
}

/*
Load from the sources, the source can be one of:
  - fileName
  - a string includes .ini
  - io.Reader the reader to load the .ini contents
  - byte array incldues .ini content
*/
func (ini *Ini) Load(sources ...any) error {
	for _, source := range sources {
		switch src := source.(type) {
		case string:
			if _, err := os.Stat(src); err == nil {
				return ini.LoadFile(src)
			} else {
				return ini.LoadString(src)
			}
		case io.Reader:
			return ini.LoadReader(src)
		case []byte:
			return ini.LoadBytes(src)
		}
	}
	return nil
}

func (ini *Ini) LoadReader(reader io.Reader) error {
	lr := newLineReader(reader)
	var curSection *Section
	keyIndent := -1
	prevKey := ""
	for {
		line, err := lr.readLine()
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}

		// if this line is value of the key
		if keyIndent >= 0 && getIndent(line) > keyIndent && curSection != nil && prevKey != "" {
			v := curSection.GetValueWithDefault(prevKey, "")
			v = fmt.Sprintf("%s\n%s", v, fromEscape(removeComments(line)))
			curSection.Add(prevKey, v)
			continue
		}

		// empty line or comments line
		if isCommentLine(line) {
			continue
		}

		// if it is a section
		sectionName := parseSectionName(line)
		if sectionName != nil {
			curSection = ini.NewSection(*sectionName)
			prevKey = ""
			keyIndent = -1
			continue
		}
		// key&value is separated with = or :
		pos := strings.IndexAny(line, "=:")
		if pos == -1 {
			continue
		}
		keyIndent = getIndent(line)
		key := strings.TrimSpace(line[0:pos])
		prevKey = key
		value := strings.TrimLeftFunc(line[pos+1:], unicode.IsSpace)
		// if it is a multiline indicator """
		if strings.HasPrefix(value, "\"\"\"") {
			t := strings.TrimRightFunc(value, unicode.IsSpace)
			//if the end multiline indicator is found
			if strings.HasSuffix(t, "\"\"\"") {
				value = t[3 : len(t)-3]
			} else { // read lines until end multiline indicator is found
				value = value[3:] + "\n" + lr.readLinesUntilSuffix("\"\"\"")
			}
		} else {
			value = strings.TrimRightFunc(value, unicode.IsSpace)
			// if is it a continuation line
			if t, continuation := removeContinuationSuffix(value); continuation {
				value = t + lr.readContinuationLines()
			}
		}

		if len(key) > 0 {
			if curSection == nil && len(ini.defaultSectionName) > 0 {
				curSection = ini.NewSection(ini.defaultSectionName)
			}
			if curSection != nil {
				curSection.Add(key, strings.TrimSpace(fromEscape(removeComments(value))))
			}
		}
	}
}

func (ini *Ini) LoadFile(filename string) error {
	f, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer f.Close()
	return ini.Load(f)
}

func (ini *Ini) LoadString(content string) error {
	return ini.Load(bytes.NewBufferString(content))
}

func (ini *Ini) LoadBytes(content []byte) error {
	return ini.Load(bytes.NewBuffer(content))
}

func getIndent(s string) int {
	n := 0
	for i := 0; i < len(s); i++ {
		if unicode.IsSpace(rune(s[i])) {
			n++
		} else {
			break
		}
	}
	return n
}

func isCommentLine(line string) bool {
	line = strings.TrimSpace(line)
	return len(line) <= 0 || line[0] == ';' || line[0] == '#'
}

func parseSectionName(line string) *string {
	line = strings.TrimSpace(line)
	if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
		sectionName := strings.TrimSpace(line[1 : len(line)-1])
		return &sectionName
	}
	return nil
}
