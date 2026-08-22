package tql

import (
	"bufio"
	"bytes"
	"crypto/sha1"
	"errors"
	"expvar"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode/utf8"

	"github.com/jellydator/ttlcache/v3"
	"github.com/machbase/neo-server/v8/mods/tql/expression"
	"github.com/machbase/neo-server/v8/mods/util/ssfs"
)

var instance *loader

func Init() {
	instance = &loader{}
	instance.vap = NewMemoryFS("/web/api/tql-assets/")
	go instance.vap.Start()
}

func Deinit() {
	if instance != nil && instance.vap != nil {
		instance.vap.Stop()
	}
	instance = nil
}

func HttpFileSystem() http.FileSystem {
	return instance.vap
}

func AssetProvider() VolatileAssetsProvider {
	return instance.vap
}

type VolatileAssetsProvider interface {
	VolatileFilePrefix() string
	VolatileFileWrite(name string, val []byte, deadline time.Time) fs.File
}

type Loader interface {
	Load(path string) (*Script, error)
}

type loader struct {
	vap *MemoryFS
}

func NewLoader() Loader {
	return instance
}

func (ld *loader) Load(path string) (*Script, error) {
	var ret *Script
	fsmgr := ssfs.Default()
	ent, err := fsmgr.Get("/" + strings.TrimPrefix(path, "/"))
	if err != nil || ent.IsDir {
		return nil, fmt.Errorf("not found '%s'", path)
	}
	ret = &Script{
		path0:   filepath.ToSlash(path),
		content: ent.Content,
		vap:     ld.vap,
	}
	return ret, nil
}

type Script struct {
	path0   string
	content []byte
	vap     VolatileAssetsProvider
}

func (sc *Script) String() string {
	return fmt.Sprintf("path: %s", sc.path0)
}

type MemoryFS struct {
	Prefix   string
	list     map[string]*MemoryFile
	listLock sync.Mutex
	stop     chan bool
}

func NewMemoryFS(prefix string) *MemoryFS {
	ret := &MemoryFS{
		Prefix: prefix,
		list:   map[string]*MemoryFile{},
		stop:   make(chan bool),
	}
	expvar.Publish("machbase:memoryfs:count", expvar.Func(ret.statzCount))
	expvar.Publish("machbase:memoryfs:total_size", expvar.Func(ret.statzTotalSize))
	return ret
}

var _ VolatileAssetsProvider = (*MemoryFS)(nil)
var _ http.FileSystem = (*MemoryFS)(nil)

func (fs *MemoryFS) Start() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case now := <-ticker.C:
			fs.listLock.Lock()
			for k, v := range fs.list {
				if v.deadline.Before(now) {
					delete(fs.list, k)
				}
			}
			fs.listLock.Unlock()
		case <-fs.stop:
			return
		}
	}
}

func (fs *MemoryFS) Stop() {
	fs.stop <- true
}

func (fs *MemoryFS) Open(name string) (http.File, error) {
	fs.listLock.Lock()
	defer fs.listLock.Unlock()
	if f, ok := fs.list[name]; ok {
		if time.Now().Before(f.deadline) {
			return f.Clone(), nil
		}
	}
	return nil, os.ErrNotExist
}

func (fs *MemoryFS) VolatileFilePrefix() string {
	return fs.Prefix
}

func (fs *MemoryFS) VolatileFileWrite(name string, val []byte, deadline time.Time) fs.File {
	ret := &MemoryFile{
		Name:     name,
		deadline: deadline,
		at:       0,
		data:     val,
		fs:       fs,
	}
	fs.listLock.Lock()
	fs.list[name] = ret
	fs.listLock.Unlock()
	return ret
}

func (fs *MemoryFS) statzCount() any { return len(fs.list) }

func (fs *MemoryFS) statzTotalSize() any {
	var total int64
	fs.listLock.Lock()
	for _, v := range fs.list {
		total += int64(len(v.data))
	}
	fs.listLock.Unlock()
	return total
}

type MemoryFile struct {
	Name     string
	deadline time.Time
	fs       *MemoryFS
	at       int64
	data     []byte
}

func (f *MemoryFile) Clone() *MemoryFile {
	return &MemoryFile{
		Name:     f.Name,
		deadline: f.deadline,
		fs:       f.fs,
		at:       0,
		data:     f.data,
	}
}

func (f *MemoryFile) Close() error {
	return nil
}

func (f *MemoryFile) Stat() (os.FileInfo, error) {
	return &memoryFileInfo{f}, nil
}

func (f *MemoryFile) Readdir(count int) ([]os.FileInfo, error) {
	f.fs.listLock.Lock()
	defer f.fs.listLock.Unlock()
	ret := make([]os.FileInfo, len(f.fs.list))
	i := 0
	for _, file := range f.fs.list {
		ret[i], _ = file.Stat()
		i++
	}
	return ret, nil
}

func (f *MemoryFile) Seek(offset int64, whence int) (int64, error) {
	switch whence {
	case 0:
		f.at = offset
	case 1:
		f.at += offset
	case 2:
		f.at = int64(len(f.data)) + offset
	}
	return f.at, nil
}

func (f *MemoryFile) Read(b []byte) (int, error) {
	i := 0
	for f.at < int64(len(f.data)) && i < len(b) {
		b[i] = f.data[f.at]
		i++
		f.at++
	}
	return i, nil
}

type memoryFileInfo struct {
	file *MemoryFile
}

func (fi *memoryFileInfo) Name() string       { return fi.file.Name }
func (fi *memoryFileInfo) Size() int64        { return int64(len(fi.file.data)) }
func (fi *memoryFileInfo) Mode() os.FileMode  { return os.ModeTemporary }
func (fi *memoryFileInfo) ModTime() time.Time { return time.Now() }
func (fi *memoryFileInfo) IsDir() bool        { return false }
func (fi *memoryFileInfo) Sys() any           { return nil }

type Line struct {
	text      string
	line      int
	isComment bool
	isPragma  bool
	tokens    []expression.Token
	start     int
	end       int
}

var functions = NewNode(nil).functions

func absolutizeParseError(err error, startLine int, startOffset int) error {
	var parseErr *expression.ParseError
	if !errors.As(err, &parseErr) {
		return err
	}
	adjusted := *parseErr
	adjusted.Span.Start.Offset = startOffset + parseErr.Span.Start.Offset
	adjusted.Span.End.Offset = startOffset + parseErr.Span.End.Offset
	if parseErr.Span.Start.Line > 0 {
		adjusted.Span.Start.Line = startLine + parseErr.Span.Start.Line - 1
	}
	if parseErr.Span.End.Line > 0 {
		adjusted.Span.End.Line = startLine + parseErr.Span.End.Line - 1
	}
	return &adjusted
}

func scanLines(codeReader io.Reader, functions map[string]expression.Function) ([]*Line, error) {
	reader := bufio.NewReader(codeReader)
	parts := []byte{}
	stmt := []string{}
	expressions := []*Line{}
	lineNo := 0
	lineFrom := 0
	lineOffset := 0
	stmtOffset := -1
	for {
		b, isPrefix, err := reader.ReadLine()
		if err != nil {
			if err == io.EOF {
				if len(stmt) > 0 {
					start := 0
					if stmtOffset >= 0 {
						start = stmtOffset
					}
					text := strings.Join(stmt, "\n")
					line := &Line{
						text:  text,
						line:  lineNo,
						start: start,
						end:   start + utf8.RuneCountInString(text),
					}
					if len(strings.TrimSpace(line.text)) > 0 {
						expressions = append(expressions, line)
					}
				}
				break
			}
			return nil, err
		}
		parts = append(parts, b...)
		if isPrefix {
			continue
		}
		lineNo++

		lineText := string(parts)
		lineStartOffset := lineOffset
		lineOffset += utf8.RuneCountInString(lineText)
		lineOffset += 1 // newline delimiter
		parts = parts[:0]

		trimLineText := strings.TrimSpace(lineText)
		if trimLineText == "" {
			if len(stmt) > 0 {
				stmt = append(stmt, lineText)
			}
			continue
		}
		inMultilineStmt := lineFrom != 0
		if strings.HasPrefix(trimLineText, "#pragma") && !inMultilineStmt {
			pragmaText := trimLineText[7:]
			start := lineStartOffset + strings.Index(lineText, pragmaText)
			if start < lineStartOffset {
				start = lineStartOffset
			}
			expressions = append(expressions, &Line{text: pragmaText, line: lineNo, isComment: true, isPragma: true, start: start, end: start + utf8.RuneCountInString(pragmaText)})
			continue
		}
		if strings.HasPrefix(trimLineText, "//+") && !inMultilineStmt {
			pragmaText := trimLineText[3:]
			start := lineStartOffset + strings.Index(lineText, pragmaText)
			if start < lineStartOffset {
				start = lineStartOffset
			}
			expressions = append(expressions, &Line{text: pragmaText, line: lineNo, isComment: true, isPragma: true, start: start, end: start + utf8.RuneCountInString(pragmaText)})
			continue
		}
		if strings.HasPrefix(trimLineText, "//") && !inMultilineStmt {
			stmt = append(stmt, "")
			commentText := trimLineText[2:]
			start := lineStartOffset + strings.Index(lineText, commentText)
			if start < lineStartOffset {
				start = lineStartOffset
			}
			expressions = append(expressions, &Line{text: commentText, line: lineNo, isComment: true, start: start, end: start + utf8.RuneCountInString(commentText)})
			continue
		}
		if strings.HasPrefix(trimLineText, "#") && !inMultilineStmt {
			commentText := trimLineText[1:]
			start := lineStartOffset + strings.Index(lineText, commentText)
			if start < lineStartOffset {
				start = lineStartOffset
			}
			expressions = append(expressions, &Line{text: commentText, line: lineNo, isComment: true, start: start, end: start + utf8.RuneCountInString(commentText)})
			continue
		}

		aStmt := strings.Join(append(stmt, lineText), "\n")
		tokens, pos, err := expression.ParseTokens(aStmt, functions)
		stmtStartLine := lineNo
		stmtStartOffsetForError := lineStartOffset
		if lineFrom != 0 {
			stmtStartLine = lineFrom
		}
		if stmtOffset >= 0 {
			stmtStartOffsetForError = stmtOffset
		}
		if utf8.RuneCountInString(aStmt) > pos /* && utf8.RuneCountInString(lineText) > pos */ {
			// lineText = string([]rune(lineText)[0:pos])
			lineText = strings.TrimPrefix(string([]rune(aStmt)[0:pos]), strings.Join(stmt, "\n")+"\n")
		}
		var parseErr *expression.ParseError
		if err != nil && errors.As(err, &parseErr) && parseErr.Kind == "unbalanced_parenthesis" {
			if lineFrom == 0 {
				lineFrom = lineNo
				stmtOffset = lineStartOffset
			}
			stmt = append(stmt, lineText)
			continue
		} else if err != nil {
			return nil, absolutizeParseError(err, stmtStartLine, stmtStartOffsetForError)
		} else {
			start := lineStartOffset
			if lineFrom != 0 && stmtOffset >= 0 {
				start = stmtOffset
			}
			stmt = append(stmt, lineText)

			line := &Line{
				text:   strings.Join(stmt, "\n"),
				line:   lineNo,
				tokens: tokens,
				start:  start,
				end:    start + pos,
			}
			if lineFrom != 0 {
				line.line = lineFrom
			}
			if len(strings.TrimSpace(line.text)) > 0 {
				expressions = append(expressions, line)
			}
			stmt = stmt[:0]
			lineFrom = 0
			stmtOffset = -1
		}
	}
	return expressions, nil
}

type StatementKind int

const (
	StatementUnknown StatementKind = iota
	StatementComment
	StatementPragma
	StatementSource
	StatementMap
	StatementSink
	StatementSourceOrMap
	StatementSourceOrSink
	StatementSourceOrMapOrSink
)

type TQLScript struct {
	Source     string
	Statements []*Statement
}

type Statement struct {
	Text      string
	Span      expression.SourceSpan
	Line      int
	IsComment bool
	IsPragma  bool
	Kind      StatementKind
	Name      string
	Expr      *expression.Expression
}

func (s *Statement) IsCode() bool {
	return s != nil && !s.IsComment && !s.IsPragma
}

func (s *Statement) toLine() *Line {
	if s == nil {
		return nil
	}
	return &Line{
		text:      s.Text,
		line:      s.Line,
		isComment: s.IsComment,
		isPragma:  s.IsPragma,
	}
}

func ValidateScriptStructure(script *TQLScript) error {
	if script == nil {
		return newScriptError("nil_script", nil, "script is nil", nil)
	}

	var codes []*Statement
	for _, stmt := range script.Statements {
		if stmt.IsCode() {
			codes = append(codes, stmt)
		}
	}

	if len(codes) == 0 {
		return newScriptError("no_source", nil, "no source exists", nil)
	}
	if len(codes) == 1 {
		return newScriptError("no_sink", codes[0], "no sink exists", nil)
	}

	head := codes[0]
	tail := codes[len(codes)-1]

	if !isApplicableForSource(head.Kind) {
		return newScriptError("invalid_source", head, fmt.Sprintf("%q is not applicable for SRC", head.Name), nil)
	}
	if !isApplicableForSink(tail.Kind) {
		return newScriptError("invalid_sink", tail, fmt.Sprintf("%q is not applicable for SINK", tail.Name), nil)
	}

	for _, stmt := range codes[1 : len(codes)-1] {
		if !isApplicableForMap(stmt.Kind) {
			return newScriptError("invalid_map", stmt, fmt.Sprintf("%q is not applicable for MAP", stmt.Name), nil)
		}
	}

	return nil
}

func isApplicableForSource(kind StatementKind) bool {
	switch kind {
	case StatementSource, StatementSourceOrMap, StatementSourceOrSink, StatementSourceOrMapOrSink:
		return true
	default:
		return false
	}
}

func isApplicableForMap(kind StatementKind) bool {
	switch kind {
	case StatementMap, StatementSourceOrMap, StatementSourceOrMapOrSink:
		return true
	default:
		return false
	}
}

func isApplicableForSink(kind StatementKind) bool {
	switch kind {
	case StatementSink, StatementSourceOrSink, StatementSourceOrMapOrSink:
		return true
	default:
		return false
	}
}

func ParseScript(source string, functions map[string]expression.Function) (*TQLScript, error) {
	return ParseScriptReader(strings.NewReader(source), functions)
}

func absolutizeStatementParseError(err error, positions []expression.SourcePosition, stmtSpan expression.SourceSpan) error {
	var parseErr *expression.ParseError
	if !errors.As(err, &parseErr) {
		return err
	}
	adjusted := *parseErr
	startOffset := stmtSpan.Start.Offset + parseErr.Span.Start.Offset
	endOffset := stmtSpan.Start.Offset + parseErr.Span.End.Offset
	if startOffset < 0 {
		startOffset = 0
	}
	if endOffset < startOffset {
		endOffset = startOffset
	}
	if startOffset < len(positions) {
		adjusted.Span.Start = positions[startOffset]
	}
	if endOffset < len(positions) {
		adjusted.Span.End = positions[endOffset]
	}
	return &adjusted
}

func ParseScriptReader(r io.Reader, functions map[string]expression.Function) (*TQLScript, error) {
	buf, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	source := string(buf)
	if functions == nil {
		functions = NewNode(nil).functions
	}

	lines, err := scanLines(bytes.NewBuffer(buf), functions)
	if err != nil {
		return nil, err
	}

	ret := &TQLScript{Source: source}
	positions := buildSourcePositions(source)
	for _, line := range lines {
		stmt := &Statement{
			Text:      line.text,
			Line:      line.line,
			IsComment: line.isComment,
			IsPragma:  line.isPragma,
			Span:      makeStatementSpan(source, positions, line.line, line.text, line.start, line.end),
		}
		if stmt.IsPragma {
			stmt.Kind = StatementPragma
		} else if stmt.IsComment {
			stmt.Kind = StatementComment
		} else {
			var expr *expression.Expression
			if len(line.tokens) > 0 {
				expr, err = expression.NewFromTokensWithExpression(line.tokens, line.text)
			} else {
				expr, err = expression.NewWithFunctions(line.text, functions)
			}
			if err != nil {
				return nil, absolutizeStatementParseError(err, positions, stmt.Span)
			}
			stmt.Expr = expr
			stmt.Name = asNodeName(expr)
			stmt.Kind = classifyStatementKind(stmt.Name)
		}
		ret.Statements = append(ret.Statements, stmt)
	}
	return ret, nil
}

func buildSourcePositions(source string) []expression.SourcePosition {
	runes := []rune(source)
	positions := make([]expression.SourcePosition, len(runes)+1)
	line := 1
	column := 1
	for i, r := range runes {
		positions[i] = expression.SourcePosition{Offset: i, Line: line, Column: column}
		if r == '\n' {
			line++
			column = 1
		} else {
			column++
		}
	}
	positions[len(runes)] = expression.SourcePosition{Offset: len(runes), Line: line, Column: column}
	return positions
}

func makeStatementSpan(source string, positions []expression.SourcePosition, startLine int, text string, startOffset int, endOffset int) expression.SourceSpan {
	runes := []rune(source)
	if startOffset < 0 {
		startOffset = 0
	}
	if startOffset > len(runes) {
		startOffset = len(runes)
	}
	if endOffset < startOffset {
		endOffset = startOffset
	}
	if endOffset > len(runes) {
		endOffset = len(runes)
	}

	if startOffset == endOffset && len(text) > 0 {
		fallbackEnd := startOffset + len([]rune(text))
		if fallbackEnd > len(runes) {
			fallbackEnd = len(runes)
		}
		endOffset = fallbackEnd
	}

	start := positions[startOffset]
	end := positions[endOffset]
	if start.Line == 0 {
		start = expression.SourcePosition{Offset: startOffset, Line: startLine, Column: 1}
	}
	if end.Line == 0 {
		end = expression.SourcePosition{Offset: endOffset, Line: start.Line, Column: start.Column}
	}
	return expression.SourceSpan{Start: start, End: end}
}

func classifyStatementKind(name string) StatementKind {
	trimmed := strings.TrimSuffix(name, "()")
	if kind, ok := statementKindByFunctionName(trimmed); ok {
		return kind
	}
	if name != "" {
		return StatementMap
	}
	return StatementUnknown
}

type ScriptError struct {
	Kind          string
	Message       string
	Span          expression.SourceSpan
	StatementSpan expression.SourceSpan
	StatementText string
	Cause         error
}

func (e *ScriptError) Error() string {
	if e == nil {
		return ""
	}
	message := e.Message
	if e.Span.Start.Line > 0 && e.Span.Start.Column > 0 {
		message = fmt.Sprintf("line %d, column %d: %s", e.Span.Start.Line, e.Span.Start.Column, message)
	} else if e.Span.Start.Line > 0 {
		message = fmt.Sprintf("line %d: %s", e.Span.Start.Line, message)
	}
	if e.StatementText != "" {
		snippet := strings.Join(strings.Fields(e.StatementText), " ")
		if len(snippet) > 120 {
			snippet = snippet[:117] + "..."
		}
		message = fmt.Sprintf("%s [statement: %s]", message, snippet)
	}
	return message
}

func (e *ScriptError) Unwrap() error {
	return e.Cause
}

func newScriptError(kind string, stmt *Statement, message string, cause error) error {
	err := &ScriptError{
		Kind:    kind,
		Message: message,
		Cause:   cause,
	}
	if stmt != nil {
		err.Span = stmt.Span
		err.StatementSpan = stmt.Span
		err.StatementText = stmt.Text
	}
	return err
}

var tqlResultCache atomic.Pointer[Cache]

type CacheOption struct {
	MaxCapacity uint64 // max number of items
	// TODO: ttlcache.WithMaxCost has a bug that introduces deadlock
	// MaxCost     uint64 // max cost (memory consumption in Bytes) of items
}

func StartCache(cap CacheOption) {
	cache := newCache(cap)
	tqlResultCache.Store(cache)
	cache.closeWg.Add(1)
	go func(cache *Cache) {
		defer cache.closeWg.Done()
		cache.cache.Start()
	}(cache)
}

func StopCache() {
	if cache := tqlResultCache.Swap(nil); cache != nil {
		cache.cache.Stop()
		close(cache.closeCh)
		cache.closeWg.Wait()
	}
}

type CacheStat struct {
	Evictions  uint64
	Insertions uint64
	Hits       uint64
	Misses     uint64
	Items      uint64
}

func StatCache() CacheStat {
	cache := tqlResultCache.Load()
	if cache == nil || cache.cache == nil {
		return CacheStat{}
	}
	stat := cache.cache.Metrics()
	return CacheStat{
		Evictions:  stat.Evictions,
		Insertions: stat.Insertions,
		Hits:       stat.Hits,
		Misses:     stat.Misses,
		Items:      uint64(cache.cache.Len()),
	}
}

type Cache struct {
	cache   *ttlcache.Cache[string, *CacheData]
	closeWg sync.WaitGroup
	closeCh chan struct{}
}

type CacheData struct {
	Data      []byte
	ExpiresAt time.Time
	TTL       time.Duration
	updates   atomic.Int32
}

func newCache(cap CacheOption) *Cache {
	cache := ttlcache.New(
		ttlcache.WithCapacity[string, *CacheData](cap.MaxCapacity),
		//
		// TODO: ttlcache.WithMaxCost has a bug that introduces deadlock
		//
		// ttlcache.WithMaxCost[string, *CacheData](cap.MaxCost, func(item *ttlcache.Item[string, *CacheData]) uint64 {
		// 	return uint64(len(item.Value().Data))
		// }),
	)
	return &Cache{
		cache:   cache,
		closeCh: make(chan struct{}),
	}
}

func (c *Cache) Set(key string, value []byte, ttl time.Duration) {
	data := &CacheData{
		Data: value,
	}
	c.cache.Set(key, data, ttl)
}

// Get returns cached content and its expiration time
// If the key is empty, it will return nil
func (c *Cache) Get(key string) *CacheData {
	if key == "" {
		return nil
	}
	item := c.cache.Get(key, ttlcache.WithDisableTouchOnHit[string, *CacheData]())
	if item == nil {
		// cache miss
		return nil
	}

	ret := item.Value()
	ret.ExpiresAt = item.ExpiresAt()
	ret.TTL = item.TTL()
	return ret
}

type CacheParam struct {
	key              string
	ttl              time.Duration
	preemptiveUpdate float64
}

func (co *CacheParam) String() string {
	return fmt.Sprintf("key: %s, ttl: %s, preemptiveUpdate: %f", co.key, co.ttl, co.preemptiveUpdate)
}

func (node *Node) fmCache(key string, ttlStr string, extra ...float64) (*CacheParam, error) {
	preemptiveUpdateRatio := 0.0
	if len(extra) > 0 {
		preemptiveUpdateRatio = extra[0]
	}
	return node.fmCachePreUpdate(key, ttlStr, preemptiveUpdateRatio)
}

func (node *Node) fmCachePreUpdate(key string, ttlStr string, preemptiveUpdate float64) (*CacheParam, error) {
	ttl := time.Minute
	if len(key) > 40 {
		// make a long key to shorten one via sha1 hash
		h := sha1.New()
		h.Write([]byte(key))
		key = fmt.Sprintf("%x", h.Sum(nil))
	}
	key = fmt.Sprintf("%s:%s:%s", node.task.sourcePath, node.task.sourceHash, key)
	if ttlStr != "" {
		if d, err := time.ParseDuration(ttlStr); err != nil || d <= time.Second {
			return nil, fmt.Errorf("invalid cache ttl %q", ttlStr)
		} else {
			ttl = d
		}
	}
	if preemptiveUpdate < 0 || preemptiveUpdate >= 1 {
		return nil, fmt.Errorf("invalid preemptive update ratio %f", preemptiveUpdate)
	}
	return &CacheParam{
		key:              key,
		ttl:              ttl,
		preemptiveUpdate: preemptiveUpdate,
	}, nil
}
