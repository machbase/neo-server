package tail

import (
	"fmt"
	"io"
	"os"
	"regexp"
	"time"
)

// Tail provides functionality to tail a file
// it works similar to 'tail -F' command in unix,
// which follows the file even if it is rotated
type Tail struct {
	filepath     string
	c            chan string
	stopChan     chan struct{}
	pollInterval time.Duration
	bufferSize   int
	patterns     []Pattern
	file         *os.File
	lastSize     int64
	lastInode    uint64
	lastPos      int64
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

// NewTail creates Tail instance
func NewTail(filepath string, opts ...Option) *Tail {
	t := &Tail{
		filepath:     filepath,
		bufferSize:   100,
		stopChan:     make(chan struct{}),
		pollInterval: 1 * time.Second,
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

	// Seek to the end of the file to start tailing
	pos, err := tail.file.Seek(0, io.SeekEnd)
	if err != nil {
		tail.file.Close()
		return fmt.Errorf("failed to seek to end: %w", err)
	}
	tail.lastPos = pos

	go tail.run()

	return nil
}

// Stop stops tailing the file
func (tail *Tail) Stop() error {
	close(tail.stopChan)
	close(tail.c)

	if tail.file != nil {
		return tail.file.Close()
	}

	return nil
}

// openFile opens the file for tailing
func (tail *Tail) openFile() error {
	file, err := os.Open(tail.filepath)
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
