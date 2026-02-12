package logging

import (
	"bytes"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"golang.org/x/term"
)

func resetLoggingState() {
	mu.Lock()
	defer mu.Unlock()

	baseWriter = os.Stderr
	baseComponent = ""
	baseLogger = zerolog.New(baseWriter).With().Timestamp().Logger()
	log.Logger = baseLogger
	zerolog.TimeFieldFormat = defaultTimeFmt
	zerolog.SetGlobalLevel(zerolog.InfoLevel)
	nowFn = time.Now
	isTerminalFn = term.IsTerminal
	mkdirAllFn = os.MkdirAll
	openFileFn = os.OpenFile
	openFn = os.Open
	statFn = os.Stat
	readDirFn = os.ReadDir
	renameFn = os.Rename
	removeFn = os.Remove
	copyFn = io.Copy
	gzipNewWriterFn = gzip.NewWriter
	statFileFn = defaultStatFileFn
	closeFileFn = defaultCloseFileFn
	compressFn = compressAndRemove
}

func baseWriterDebugString() string {
	return fmt.Sprintf("%#v", baseWriter)
}

func TestInitJSONFormatSetsLevelAndComponent(t *testing.T) {
	t.Cleanup(resetLoggingState)

	Init(Config{
		Format:    "json",
		Level:     "debug",
		Component: "apiserver",
	})

	mu.RLock()
	defer mu.RUnlock()

	repr := baseWriterDebugString()
	if !strings.Contains(repr, fmt.Sprintf("(%p)", os.Stderr)) {
		t.Fatalf("expected base writer to include os.Stderr, got %#v", baseWriter)
	}
	if !strings.Contains(repr, "LogBroadcaster") {
		t.Fatalf("expected base writer to include broadcaster, got %#v", baseWriter)
	}

	if zerolog.GlobalLevel() != zerolog.DebugLevel {
		t.Fatalf("expected global level debug, got %s", zerolog.GlobalLevel())
	}

	if baseComponent != "apiserver" {
		t.Fatalf("expected base component apiserver, got %s", baseComponent)
	}

	if !reflect.DeepEqual(log.Logger, baseLogger) {
		t.Fatal("expected global log.Logger to match baseLogger")
	}
}

func TestInitConsoleFormatUsesConsoleWriter(t *testing.T) {
	t.Cleanup(resetLoggingState)

	Init(Config{
		Format: "console",
		Level:  "info",
	})

	mu.RLock()
	defer mu.RUnlock()

	repr := baseWriterDebugString()
	if !strings.Contains(repr, "zerolog.ConsoleWriter") {
		t.Fatalf("expected console writer, got %#v", baseWriter)
	}
	if !strings.Contains(repr, "LogBroadcaster") {
		t.Fatalf("expected base writer to include broadcaster, got %#v", baseWriter)
	}
}

func TestInitAutoFormatWithPipe(t *testing.T) {
	t.Cleanup(resetLoggingState)

	origStderr := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create pipe: %v", err)
	}
	os.Stderr = w
	defer func() {
		os.Stderr = origStderr
		_ = r.Close()
		_ = w.Close()
	}()

	Init(Config{
		Format: "auto",
		Level:  "info",
	})

	mu.RLock()
	defer mu.RUnlock()

	repr := baseWriterDebugString()
	if !strings.Contains(repr, fmt.Sprintf("(%p)", w)) {
		t.Fatalf("expected base writer to use provided pipe, got %#v", baseWriter)
	}
	if !strings.Contains(repr, "LogBroadcaster") {
		t.Fatalf("expected base writer to include broadcaster, got %#v", baseWriter)
	}
}

func TestWithRequestID(t *testing.T) {
	t.Cleanup(resetLoggingState)

	Init(Config{
		Format: "json",
		Level:  "info",
	})

	ctx, generated := WithRequestID(nil, "")
	if generated == "" {
		t.Fatal("expected generated request id")
	}
	if ctx == nil {
		t.Fatal("expected non-nil context")
	}

	ctx2, id := WithRequestID(nil, "custom-123")
	if id != "custom-123" {
		t.Fatalf("expected custom-123, got %s", id)
	}
	if ctx2 == nil {
		t.Fatal("expected non-nil context")
	}
}

func TestWithRequestIDTrimsWhitespace(t *testing.T) {
	t.Cleanup(resetLoggingState)

	Init(Config{})

	_, id := WithRequestID(nil, "   ")
	if id == "" {
		t.Fatal("expected generated id for whitespace input")
	}
}

func TestInitThreadSafety(t *testing.T) {
	t.Cleanup(resetLoggingState)

	var wg sync.WaitGroup
	configs := []Config{
		{Format: "json", Level: "debug", Component: "worker"},
		{Format: "json", Level: "warn", Component: "api"},
	}

	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			Init(configs[idx%len(configs)])
		}(i)
	}
	wg.Wait()

	mu.RLock()
	defer mu.RUnlock()

	// Ensure baseLogger is valid and global logger matches it.
	if reflect.DeepEqual(baseLogger, zerolog.Logger{}) {
		t.Fatal("expected initialized base logger")
	}
	if !reflect.DeepEqual(log.Logger, baseLogger) {
		t.Fatal("expected global log.Logger to match baseLogger after concurrent init")
	}
}

func TestIsLevelEnabled(t *testing.T) {
	t.Cleanup(resetLoggingState)

	// Set global level to Info
	zerolog.SetGlobalLevel(zerolog.InfoLevel)

	if !IsLevelEnabled(zerolog.InfoLevel) {
		t.Fatal("expected info level to be enabled")
	}
	if !IsLevelEnabled(zerolog.WarnLevel) {
		t.Fatal("expected warn level to be enabled")
	}
	if !IsLevelEnabled(zerolog.ErrorLevel) {
		t.Fatal("expected error level to be enabled")
	}
	if IsLevelEnabled(zerolog.DebugLevel) {
		t.Fatal("expected debug level to be disabled")
	}

	// Change to debug level
	zerolog.SetGlobalLevel(zerolog.DebugLevel)
	if !IsLevelEnabled(zerolog.DebugLevel) {
		t.Fatal("expected debug level to be enabled after setting global level")
	}
}

func TestRollingFileWriter(t *testing.T) {
	t.Cleanup(resetLoggingState)

	dir := t.TempDir()
	logFile := dir + "/test.log"

	cfg := Config{
		Format:     "json",
		Level:      "info",
		FilePath:   logFile,
		MaxSizeMB:  1,
		MaxAgeDays: 7,
		Compress:   false,
	}

	Init(cfg)

	// Write some log output
	log.Info().Msg("test message")

	// Check file exists (this confirms rolling file writer was created)
	if _, err := os.Stat(logFile); os.IsNotExist(err) {
		t.Fatal("expected log file to be created")
	}

	// Check file has content
	data, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("expected log file to have content")
	}
}

func TestParseLevelDefaults(t *testing.T) {
	tests := []struct {
		input string
		want  zerolog.Level
	}{
		{"debug", zerolog.DebugLevel},
		{"DEBUG", zerolog.DebugLevel},
		{"info", zerolog.InfoLevel},
		{"INFO", zerolog.InfoLevel},
		{"warn", zerolog.WarnLevel},
		{"WARN", zerolog.WarnLevel},
		{"error", zerolog.ErrorLevel},
		{"ERROR", zerolog.ErrorLevel},
		{"unknown", zerolog.InfoLevel},
		{"", zerolog.InfoLevel},
	}

	for _, tc := range tests {
		got := parseLevel(tc.input)
		if got != tc.want {
			t.Errorf("parseLevel(%q) = %v, want %v", tc.input, got, tc.want)
		}
	}
}

func TestSelectWriter(t *testing.T) {
	tests := []struct {
		format  string
		isTTY   bool
		wantErr bool
	}{
		{"json", false, false},
		{"console", false, false},
		{"auto", false, false},
	}

	for _, tc := range tests {
		w := selectWriter(tc.format)
		if w == nil {
			t.Errorf("selectWriter(%q) returned nil", tc.format)
		}
	}
}

func TestSelectWriterAutoTerminal(t *testing.T) {
	t.Cleanup(resetLoggingState)
	isTerminalFn = func(int) bool { return true }

	w := selectWriter("auto")
	if _, ok := w.(zerolog.ConsoleWriter); !ok {
		t.Fatalf("expected console writer, got %#v", w)
	}
}

func TestSelectWriterDefault(t *testing.T) {
	t.Cleanup(resetLoggingState)

	w := selectWriter("unknown")
	if w != os.Stderr {
		t.Fatalf("expected default writer to be os.Stderr, got %#v", w)
	}
}

func TestIsTerminalNil(t *testing.T) {
	t.Cleanup(resetLoggingState)

	if isTerminal(nil) {
		t.Fatal("expected nil file to report false")
	}
}

func TestNewRollingFileWriter_EmptyPath(t *testing.T) {
	t.Cleanup(resetLoggingState)

	writer, err := newRollingFileWriter(Config{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if writer != nil {
		t.Fatalf("expected nil writer, got %#v", writer)
	}
}

func TestNewRollingFileWriter_MkdirError(t *testing.T) {
	t.Cleanup(resetLoggingState)

	mkdirAllFn = func(string, os.FileMode) error {
		return errors.New("mkdir failed")
	}

	_, err := newRollingFileWriter(Config{FilePath: "/tmp/logs/test.log"})
	if err == nil {
		t.Fatal("expected error from mkdir")
	}
}

func TestInitFileWriterError(t *testing.T) {
	t.Cleanup(resetLoggingState)

	mkdirAllFn = func(string, os.FileMode) error {
		return errors.New("mkdir failed")
	}

	Init(Config{
		Format:   "json",
		FilePath: "/tmp/logs/test.log",
	})
}

func TestNewRollingFileWriter_DefaultMaxSize(t *testing.T) {
	t.Cleanup(resetLoggingState)

	dir := t.TempDir()
	writer, err := newRollingFileWriter(Config{
		FilePath:  filepath.Join(dir, "app.log"),
		MaxSizeMB: 0,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	w, ok := writer.(*rollingFileWriter)
	if !ok {
		t.Fatalf("expected rollingFileWriter, got %#v", writer)
	}
	if w.maxBytes != 100*1024*1024 {
		t.Fatalf("expected default max bytes, got %d", w.maxBytes)
	}
	_ = w.closeLocked()
}

func TestNewRollingFileWriter_OpenError(t *testing.T) {
	t.Cleanup(resetLoggingState)

	openFileFn = func(string, int, os.FileMode) (*os.File, error) {
		return nil, errors.New("open failed")
	}

	_, err := newRollingFileWriter(Config{FilePath: filepath.Join(t.TempDir(), "app.log")})
	if err == nil {
		t.Fatal("expected error from openOrCreateLocked")
	}
}

func TestOpenOrCreateLocked_StatError(t *testing.T) {
	t.Cleanup(resetLoggingState)

	dir := t.TempDir()
	w := &rollingFileWriter{path: filepath.Join(dir, "app.log")}
	statFileFn = func(*os.File) (os.FileInfo, error) {
		return nil, errors.New("stat failed")
	}
	if err := w.openOrCreateLocked(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if w.currentSize != 0 {
		t.Fatalf("expected current size 0, got %d", w.currentSize)
	}
	_ = w.closeLocked()
}

func TestOpenOrCreateLocked_AlreadyOpen(t *testing.T) {
	t.Cleanup(resetLoggingState)

	file, err := os.CreateTemp(t.TempDir(), "log")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	w := &rollingFileWriter{path: file.Name(), file: file}
	if err := w.openOrCreateLocked(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_ = w.closeLocked()
}

func TestRollingFileWriter_WriteOpenError(t *testing.T) {
	t.Cleanup(resetLoggingState)

	openFileFn = func(string, int, os.FileMode) (*os.File, error) {
		return nil, errors.New("open failed")
	}
	w := &rollingFileWriter{path: filepath.Join(t.TempDir(), "app.log")}
	if _, err := w.Write([]byte("data")); err == nil {
		t.Fatal("expected write error")
	}
}

func TestRollingFileWriter_WriteRotateError(t *testing.T) {
	t.Cleanup(resetLoggingState)

	dir := t.TempDir()
	path := filepath.Join(dir, "app.log")
	callCount := 0
	openFileFn = func(name string, flag int, perm os.FileMode) (*os.File, error) {
		callCount++
		if callCount == 1 {
			return os.OpenFile(name, flag, perm)
		}
		return nil, errors.New("open failed")
	}
	w := &rollingFileWriter{path: path, maxBytes: 1}
	if _, err := w.Write([]byte("too big")); err == nil {
		t.Fatal("expected rotate error")
	}
}

func TestRollingFileWriter_RotateCompress(t *testing.T) {
	t.Cleanup(resetLoggingState)

	dir := t.TempDir()
	path := filepath.Join(dir, "app.log")
	if err := os.WriteFile(path, []byte("data"), 0600); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	w := &rollingFileWriter{path: path, maxBytes: 1, compress: true}
	if err := w.openOrCreateLocked(); err != nil {
		t.Fatalf("openOrCreateLocked error: %v", err)
	}

	ch := make(chan string, 1)
	compressFn = func(p string) { ch <- p }

	if err := w.rotateLocked(); err != nil {
		t.Fatalf("rotateLocked error: %v", err)
	}

	select {
	case <-ch:
	case <-time.After(2 * time.Second):
		t.Fatal("expected compress to be triggered")
	}
	_ = w.closeLocked()
}

func TestRollingFileWriter_RotateRenameError(t *testing.T) {
	t.Cleanup(resetLoggingState)

	dir := t.TempDir()
	path := filepath.Join(dir, "app.log")
	if err := os.WriteFile(path, []byte("data"), 0600); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	w := &rollingFileWriter{path: path, maxBytes: 1, compress: true}
	if err := w.openOrCreateLocked(); err != nil {
		t.Fatalf("openOrCreateLocked error: %v", err)
	}

	renameCalled := false
	renameFn = func(oldpath, newpath string) error {
		renameCalled = true
		return errors.New("rename failed")
	}

	compressCalled := make(chan struct{}, 1)
	compressFn = func(string) {
		compressCalled <- struct{}{}
	}

	if err := w.rotateLocked(); err != nil {
		t.Fatalf("rotateLocked error: %v", err)
	}
	if !renameCalled {
		t.Fatal("expected rename to be attempted")
	}

	select {
	case <-compressCalled:
		t.Fatal("expected compression to be skipped on rename error")
	default:
	}

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected log file to exist after rename failure: %v", err)
	}

	_ = w.closeLocked()
}

func TestRotateLockedCloseError(t *testing.T) {
	t.Cleanup(resetLoggingState)

	dir := t.TempDir()
	path := filepath.Join(dir, "app.log")
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatalf("failed to open file: %v", err)
	}
	w := &rollingFileWriter{path: path, file: file}
	closeFileFn = func(*os.File) error {
		return errors.New("close failed")
	}

	if err := w.rotateLocked(); err == nil {
		t.Fatal("expected close error")
	}
	_ = file.Close()
}

func TestCloseLocked(t *testing.T) {
	t.Cleanup(resetLoggingState)

	w := &rollingFileWriter{}
	if err := w.closeLocked(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	file, err := os.CreateTemp(t.TempDir(), "log")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	w.file = file
	if err := w.closeLocked(); err != nil {
		t.Fatalf("unexpected close error: %v", err)
	}
	if w.file != nil {
		t.Fatal("expected file to be cleared")
	}
	if w.currentSize != 0 {
		t.Fatalf("expected size reset, got %d", w.currentSize)
	}
}

func TestCleanupOldFilesNoMaxAge(t *testing.T) {
	t.Cleanup(resetLoggingState)

	w := &rollingFileWriter{path: filepath.Join(t.TempDir(), "app.log"), maxAge: 0}
	w.cleanupOldFiles()
}

func TestCleanupOldFilesReadDirError(t *testing.T) {
	t.Cleanup(resetLoggingState)

	readDirFn = func(string) ([]os.DirEntry, error) {
		return nil, errors.New("read dir failed")
	}
	w := &rollingFileWriter{path: filepath.Join(t.TempDir(), "app.log"), maxAge: time.Hour}
	w.cleanupOldFiles()
}

func TestCleanupOldFilesInfoError(t *testing.T) {
	t.Cleanup(resetLoggingState)

	readDirFn = func(string) ([]os.DirEntry, error) {
		return []os.DirEntry{errDirEntry{name: "app.log.20200101"}}, nil
	}
	w := &rollingFileWriter{path: filepath.Join(t.TempDir(), "app.log"), maxAge: time.Hour}
	w.cleanupOldFiles()
}

func TestCleanupOldFilesRemovesOld(t *testing.T) {
	t.Cleanup(resetLoggingState)

	dir := t.TempDir()
	path := filepath.Join(dir, "app.log")
	oldFile := filepath.Join(dir, "app.log.20200101-000000")
	newFile := filepath.Join(dir, "app.log.20250101-000000")
	otherFile := filepath.Join(dir, "other.log.20200101")

	if err := os.WriteFile(oldFile, []byte("old"), 0600); err != nil {
		t.Fatalf("failed to write old file: %v", err)
	}
	if err := os.WriteFile(newFile, []byte("new"), 0600); err != nil {
		t.Fatalf("failed to write new file: %v", err)
	}
	if err := os.WriteFile(otherFile, []byte("other"), 0600); err != nil {
		t.Fatalf("failed to write other file: %v", err)
	}

	fixedNow := time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC)
	nowFn = func() time.Time { return fixedNow }

	if err := os.Chtimes(oldFile, fixedNow.Add(-48*time.Hour), fixedNow.Add(-48*time.Hour)); err != nil {
		t.Fatalf("failed to set old file time: %v", err)
	}
	if err := os.Chtimes(newFile, fixedNow.Add(-time.Hour), fixedNow.Add(-time.Hour)); err != nil {
		t.Fatalf("failed to set new file time: %v", err)
	}

	w := &rollingFileWriter{path: path, maxAge: 24 * time.Hour}
	w.cleanupOldFiles()

	if _, err := os.Stat(oldFile); !os.IsNotExist(err) {
		t.Fatalf("expected old file to be removed")
	}
	if _, err := os.Stat(newFile); err != nil {
		t.Fatalf("expected new file to remain: %v", err)
	}
	if _, err := os.Stat(otherFile); err != nil {
		t.Fatalf("expected other file to remain: %v", err)
	}
}

func TestStatFileFnDefault(t *testing.T) {
	t.Cleanup(resetLoggingState)

	file, err := os.CreateTemp(t.TempDir(), "log")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	t.Cleanup(func() { _ = file.Close() })

	if _, err := statFileFn(file); err != nil {
		t.Fatalf("statFileFn error: %v", err)
	}
}

func TestCompressAndRemove(t *testing.T) {
	t.Run("OpenError", func(t *testing.T) {
		t.Cleanup(resetLoggingState)
		openFn = func(string) (*os.File, error) {
			return nil, errors.New("open failed")
		}
		compressAndRemove("/does/not/exist")
	})

	t.Run("OpenFileError", func(t *testing.T) {
		t.Cleanup(resetLoggingState)
		dir := t.TempDir()
		path := filepath.Join(dir, "app.log")
		if err := os.WriteFile(path, []byte("data"), 0600); err != nil {
			t.Fatalf("failed to write file: %v", err)
		}
		openFileFn = func(string, int, os.FileMode) (*os.File, error) {
			return nil, errors.New("open file failed")
		}
		compressAndRemove(path)
	})

	t.Run("CopyError", func(t *testing.T) {
		t.Cleanup(resetLoggingState)
		dir := t.TempDir()
		path := filepath.Join(dir, "app.log")
		if err := os.WriteFile(path, []byte("data"), 0600); err != nil {
			t.Fatalf("failed to write file: %v", err)
		}
		copyFn = func(io.Writer, io.Reader) (int64, error) {
			return 0, errors.New("copy failed")
		}
		compressAndRemove(path)
	})

	t.Run("CloseError", func(t *testing.T) {
		t.Cleanup(resetLoggingState)
		dir := t.TempDir()
		path := filepath.Join(dir, "app.log")
		if err := os.WriteFile(path, []byte("data"), 0600); err != nil {
			t.Fatalf("failed to write file: %v", err)
		}
		errWriter := errWriteCloser{err: errors.New("write failed")}
		gzipNewWriterFn = func(io.Writer) *gzip.Writer {
			return gzip.NewWriter(errWriter)
		}
		copyFn = func(io.Writer, io.Reader) (int64, error) {
			return 0, nil
		}
		compressAndRemove(path)
	})

	t.Run("Success", func(t *testing.T) {
		t.Cleanup(resetLoggingState)
		dir := t.TempDir()
		path := filepath.Join(dir, "app.log")
		if err := os.WriteFile(path, []byte("data"), 0600); err != nil {
			t.Fatalf("failed to write file: %v", err)
		}
		compressAndRemove(path)
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatal("expected original file to be removed")
		}
		if _, err := os.Stat(path + ".gz"); err != nil {
			t.Fatalf("expected gzip file to exist: %v", err)
		}
	})
}

// Test that the logging package doesn't panic under concurrent use
func TestConcurrentLogging(t *testing.T) {
	t.Cleanup(resetLoggingState)

	var buf bytes.Buffer
	mu.Lock()
	baseWriter = &buf
	baseLogger = zerolog.New(&buf).With().Timestamp().Logger()
	log.Logger = baseLogger
	mu.Unlock()

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			log.Info().Int("iteration", n).Msg("concurrent log")
		}(i)
	}
	wg.Wait()

	if buf.Len() == 0 {
		t.Fatal("expected log output from concurrent logging")
	}
}

type errDirEntry struct {
	name string
}

func (e errDirEntry) Name() string { return e.name }
func (e errDirEntry) IsDir() bool  { return false }
func (e errDirEntry) Type() os.FileMode {
	return 0
}
func (e errDirEntry) Info() (os.FileInfo, error) {
	return nil, errors.New("info error")
}

type errWriteCloser struct {
	err error
}

func (e errWriteCloser) Write(p []byte) (int, error) {
	return 0, e.err
}
