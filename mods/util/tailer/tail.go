package tailer

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

type ITail interface {
	Start() error
	Stop() error
	Lines() <-chan string
}

var _ ITail = (*Tail)(nil)
var _ ITail = (*MultiTail)(nil)

// MultiTail allows tailing multiple files and merging their output
type MultiTail struct {
	tails []ITail
	wg    sync.WaitGroup
	c     chan string
}

func NewMultiTail(tails ...ITail) ITail {
	buff := len(tails) * 100
	for _, tail := range tails {
		if t, ok := tail.(*Tail); ok && buff < t.bufferSize {
			buff = t.bufferSize
		}
	}
	mt := &MultiTail{
		tails: tails,
		c:     make(chan string, buff),
	}
	return mt
}

func (mt *MultiTail) Start() error {
	aliasWidth := 0
	for _, tail := range mt.tails {
		if err := tail.Start(); err != nil {
			return fmt.Errorf("failed to start tail for %w", err)
		}
		if t, ok := tail.(*Tail); ok {
			if l := len(StripAnsiCodes(t.label)); l > aliasWidth {
				aliasWidth = l
			}
		}
	}

	for _, tail := range mt.tails {
		mt.wg.Add(1)
		go func(t ITail) {
			defer mt.wg.Done()
			label := ""
			labelLen := 0
			if tt, ok := t.(*Tail); ok {
				label = tt.label
				labelLen = len(StripAnsiCodes(label))
			}
			if labelLen < aliasWidth {
				label = label + strings.Repeat(" ", aliasWidth-labelLen)
			}
			for line := range t.Lines() {
				mt.c <- label + " " + line
			}
		}(tail)
	}

	return nil
}

func (mt *MultiTail) Stop() error {
	var firstErr error
	for _, tail := range mt.tails {
		if err := tail.Stop(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	mt.wg.Wait()
	defer close(mt.c)

	return firstErr
}

func (mt *MultiTail) Lines() <-chan string {
	return mt.c
}

// Tail provides functionality to tail a file
// it works similar to 'tail -F' command in unix,
// which follows the file even if it is rotated
type Tail struct {
	filepath     string
	label        string // terminal display label for the file, it can contain ANSI color codes
	c            chan string
	stopChan     chan struct{}
	pollInterval time.Duration
	bufferSize   int
	patterns     []Pattern
	showLastN    int
	plugins      []Plugin
	file         *os.File
	lastSize     int64
	lastInode    uint64
	lastPos      int64
	wg           sync.WaitGroup
}

type Pattern []*regexp.Regexp

func (p Pattern) Match(s string) bool {
	matched := true
	for _, re := range p {
		if !re.MatchString(s) {
			matched = false
			break
		}
	}
	return matched
}

// Option is a functional option for Tail
type Option func(*Tail)

// WithPollInterval sets the polling interval for checking file changes
func WithPollInterval(d time.Duration) Option {
	return func(t *Tail) {
		t.pollInterval = d
	}
}

func WithBufferSize(size int) Option {
	return func(t *Tail) {
		t.bufferSize = size
	}
}

func WithPattern(patterns ...string) Option {
	return func(t *Tail) {
		var group Pattern
		for _, pattern := range patterns {
			re, err := regexp.Compile(pattern)
			if err == nil {
				group = append(group, re)
			}
		}
		t.patterns = append(t.patterns, group)
	}
}

func WithLast(n int) Option {
	return func(t *Tail) {
		t.showLastN = n
	}
}

func WithLabel(label string) Option {
	return func(t *Tail) {
		t.label = label
	}
}

func WithSyntaxHighlighting(syntax ...string) Option {
	return func(t *Tail) {
		t.plugins = append(t.plugins, NewWithSyntaxHighlighting(syntax...))
	}
}

func WithPlugins(p ...Plugin) Option {
	return func(t *Tail) {
		t.plugins = append(t.plugins, p...)
	}
}

// New creates Tail instance
func New(filename string, opts ...Option) ITail {
	t := &Tail{
		filepath:     filename,
		label:        filepath.Base(filename),
		bufferSize:   100,
		stopChan:     make(chan struct{}),
		pollInterval: 1 * time.Second,
		showLastN:    10,
	}

	for _, opt := range opts {
		opt(t)
	}

	t.c = make(chan string, t.bufferSize)
	return t
}

// Lines returns output channel
// caller can read lines from this channel
func (tail *Tail) Lines() <-chan string {
	return tail.c
}

// Start begins tailing the file
func (tail *Tail) Start() error {
	// Open the file initially
	if err := tail.openFile(); err != nil {
		return err
	}

	// Read last 10 lines before starting to tail
	if err := tail.readLastLines(tail.showLastN); err != nil {
		// If we can't read last lines, just seek to end
		pos, seekErr := tail.file.Seek(0, io.SeekEnd)
		if seekErr != nil {
			tail.file.Close()
			return fmt.Errorf("failed to seek to end: %w", seekErr)
		}
		tail.lastPos = pos
	}

	tail.wg.Add(1)
	go tail.run()

	return nil
}

// readLastLines reads the last n lines from the file and sends them to the channel
func (tail *Tail) readLastLines(n int) error {
	stat, err := tail.file.Stat()
	if err != nil {
		return fmt.Errorf("failed to stat file: %w", err)
	}

	fileSize := stat.Size()
	if fileSize == 0 {
		tail.lastPos = 0
		return nil
	}

	// Buffer to read file in chunks from the end
	const chunkSize = 4096
	var allData []byte
	bytesToRead := fileSize

	// Limit how much we read (don't read more than necessary)
	maxRead := int64(chunkSize * 4) // Read up to 16KB max
	if bytesToRead > maxRead {
		bytesToRead = maxRead
	}

	// Seek to position
	offset := fileSize - bytesToRead
	if _, err := tail.file.Seek(offset, io.SeekStart); err != nil {
		return fmt.Errorf("failed to seek: %w", err)
	}

	// Read the data
	buf := make([]byte, bytesToRead)
	readBytes, err := io.ReadFull(tail.file, buf)
	if err != nil && err != io.ErrUnexpectedEOF {
		return fmt.Errorf("failed to read: %w", err)
	}
	allData = buf[:readBytes]

	// Split into lines
	var lines []string
	var lineStart int

	for i := 0; i < len(allData); i++ {
		if allData[i] == '\n' {
			line := string(allData[lineStart:i])
			// Trim \r if present
			if len(line) > 0 && line[len(line)-1] == '\r' {
				line = line[:len(line)-1]
			}
			if len(line) > 0 { // Skip empty lines
				lines = append(lines, line)
			}
			lineStart = i + 1
		}
	}

	// Handle last line if file doesn't end with newline
	if lineStart < len(allData) {
		line := string(allData[lineStart:])
		if len(line) > 0 && line[len(line)-1] == '\r' {
			line = line[:len(line)-1]
		}
		if len(line) > 0 {
			lines = append(lines, line)
		}
	}

	// Keep only last n lines
	startIdx := 0
	if len(lines) > n {
		startIdx = len(lines) - n
	}
	lines = lines[startIdx:]

	// Send lines to channel (in correct order)
	for _, line := range lines {
		matched := false
		if len(tail.patterns) > 0 {
			for _, p := range tail.patterns {
				if p.Match(line) {
					matched = true
					break
				}
			}
		} else {
			matched = true
		}

		if !matched {
			continue
		}
		for _, plugin := range tail.plugins {
			if ln, ok := plugin.Apply(line); ok {
				line = ln
			} else {
				// Plugin indicated to drop the line
				matched = false
				break
			}
		}

		if !matched {
			continue
		}
		select {
		case tail.c <- line:
		case <-tail.stopChan:
			return nil
		}
	}

	// Seek to end and update position
	pos, err := tail.file.Seek(0, io.SeekEnd)
	if err != nil {
		return fmt.Errorf("failed to seek to end: %w", err)
	}
	tail.lastPos = pos

	return nil
}

// Stop stops tailing the file
func (tail *Tail) Stop() error {
	close(tail.stopChan)

	// Wait for goroutine to finish before closing the channel
	tail.wg.Wait()

	close(tail.c)

	if tail.file != nil {
		return tail.file.Close()
	}

	return nil
}

// openFile opens the file for tailing
func (tail *Tail) openFile() error {
	file, err := openFileShared(tail.filepath)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}

	stat, err := file.Stat()
	if err != nil {
		file.Close()
		return fmt.Errorf("failed to stat file: %w", err)
	}

	tail.file = file
	tail.lastSize = stat.Size()
	tail.lastInode = getInode(stat)
	tail.lastPos = 0

	return nil
}

// run is the main loop that tails the file
func (tail *Tail) run() {
	defer tail.wg.Done()
	ticker := time.NewTicker(tail.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-tail.stopChan:
			return
		case <-ticker.C:
			if err := tail.checkAndRead(); err != nil {
				// If there's an error, try to reopen the file (might be rotated)
				if tail.file != nil {
					tail.file.Close()
				}

				// Wait a bit and try to open again
				time.Sleep(tail.pollInterval)
				if err := tail.reopenIfNeeded(); err != nil {
					// Still can't open, continue waiting
					continue
				}
			}
		}
	}
}

// checkAndRead checks for file changes and reads new lines
func (tail *Tail) checkAndRead() error {
	// Check if file still exists and hasn't been rotated
	stat, err := os.Stat(tail.filepath)
	if err != nil {
		return fmt.Errorf("failed to stat file: %w", err)
	}

	currentInode := getInode(stat)
	currentSize := stat.Size()

	// Check if file was rotated (inode changed)
	if currentInode != tail.lastInode {
		// File was rotated - read remaining content from old file then switch to new file
		if _, err := tail.file.Seek(tail.lastPos, io.SeekStart); err == nil {
			tail.readLines()
		}

		// Reopen the new file
		if tail.file != nil {
			tail.file.Close()
		}

		if err := tail.openFile(); err != nil {
			return err
		}

		// Start from beginning of new file
		tail.lastPos = 0
		if _, err := tail.file.Seek(0, io.SeekStart); err != nil {
			return fmt.Errorf("failed to seek to start: %w", err)
		}

		tail.readLines()
		return nil
	}

	// Check if file was truncated
	// This happens if: size decreased, OR our position is beyond current size
	if currentSize < tail.lastSize || tail.lastPos > currentSize {
		// File was truncated, seek to beginning
		tail.lastPos = 0
		tail.lastSize = 0
		if _, err := tail.file.Seek(0, io.SeekStart); err != nil {
			return fmt.Errorf("failed to seek to start: %w", err)
		}

		// Read all content from the beginning
		if currentSize > 0 {
			tail.readLines()
		}
		tail.lastSize = currentSize
		return nil
	}

	// Check if file has new content
	if currentSize > tail.lastSize {
		// Seek to our last known position
		if _, err := tail.file.Seek(tail.lastPos, io.SeekStart); err != nil {
			return fmt.Errorf("failed to seek: %w", err)
		}

		tail.readLines()
		tail.lastSize = currentSize
	}

	return nil
}

// readLines reads new lines from the file
func (tail *Tail) readLines() {
	buf := make([]byte, 4096)
	var lineBuf []byte

	for {
		n, err := tail.file.Read(buf)
		if n > 0 {
			// Process the data we read
			data := buf[:n]
			for len(data) > 0 {
				// Find newline
				nlIdx := -1
				for i, b := range data {
					if b == '\n' {
						nlIdx = i
						break
					}
				}

				if nlIdx >= 0 {
					// Found a complete line
					lineBuf = append(lineBuf, data[:nlIdx]...)

					// Convert to string and trim \r if present
					line := string(lineBuf)
					if len(line) > 0 && line[len(line)-1] == '\r' {
						line = line[:len(line)-1]
					}

					matched := false
					if len(tail.patterns) > 0 {
						for _, p := range tail.patterns {
							if p.Match(line) {
								matched = true
								break
							}
						}
					} else {
						matched = true
					}

					if !matched {
						// Move to next data
						data = data[nlIdx+1:]
						lineBuf = lineBuf[:0]
						tail.lastPos += int64(nlIdx + 1)
						continue
					}

					// Apply plugins
					for _, plugin := range tail.plugins {
						if ln, ok := plugin.Apply(line); ok {
							line = ln
						} else {
							// Plugin indicated to drop the line
							matched = false
							break
						}
					}

					if matched {
						// Send the line
						select {
						case tail.c <- line:
						case <-tail.stopChan:
							return
						}
					}

					// Move to next data
					data = data[nlIdx+1:]
					lineBuf = lineBuf[:0]
					tail.lastPos += int64(nlIdx + 1)
				} else {
					// No newline found, save to buffer
					lineBuf = append(lineBuf, data...)
					tail.lastPos += int64(len(data))
					break
				}
			}
		}

		if err != nil {
			if err == io.EOF {
				// End of file, save position and return
				return
			}
			// Other error, return
			return
		}

		if n == 0 {
			break
		}
	}
}

// reopenIfNeeded tries to reopen the file if it was rotated
func (tail *Tail) reopenIfNeeded() error {
	// Try to open the file
	return tail.openFile()
}
