package logging

import (
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/rs/zerolog/pkgerrors"
	"golang.org/x/term"
)

type ctxKey string

const (
	requestIDKey ctxKey = "logging_request_id"
)

// Config controls logger initialization.
type Config struct {
	Format     string // "json", "console", or "auto"
	Level      string // "debug", "info", "warn", "error"
	Component  string // optional component name
	FilePath   string // optional log file path
	MaxSizeMB  int    // rotate after this size (MB)
	MaxAgeDays int    // keep rotated logs for this many days
	Compress   bool   // gzip rotated logs
}

var (
	mu            sync.RWMutex
	baseLogger    zerolog.Logger
	baseWriter    io.Writer = os.Stderr
	baseComponent string

	defaultTimeFmt = time.RFC3339
)

var (
	nowFn           = time.Now
	isTerminalFn    = term.IsTerminal
	mkdirAllFn      = os.MkdirAll
	openFileFn      = os.OpenFile
	openFn          = os.Open
	statFn          = os.Stat
	readDirFn       = os.ReadDir
	renameFn        = os.Rename
	removeFn        = os.Remove
	copyFn          = io.Copy
	gzipNewWriterFn = gzip.NewWriter
	statFileFn      = func(file *os.File) (os.FileInfo, error) { return file.Stat() }
	closeFileFn     = func(file *os.File) error { return file.Close() }
	compressFn      = compressAndRemove
)

var (
	defaultStatFileFn  = statFileFn
	defaultCloseFileFn = closeFileFn
)

func init() {
	baseLogger = zerolog.New(baseWriter).With().Timestamp().Logger()
	log.Logger = baseLogger
}

// Init configures zerolog globals and establishes the package baseline logger.
func Init(cfg Config) zerolog.Logger {
	mu.Lock()
	defer mu.Unlock()

	zerolog.TimeFieldFormat = defaultTimeFmt
	zerolog.SetGlobalLevel(parseLevel(cfg.Level))
	zerolog.ErrorStackMarshaler = pkgerrors.MarshalStack

	writer := selectWriter(cfg.Format)

	// Hook in the in-memory broadcaster for live UI streaming
	broadcaster := GetBroadcaster()
	writer = io.MultiWriter(writer, broadcaster)

	if fileWriter, err := newRollingFileWriter(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "logging: unable to configure file output: %v\n", err)
	} else if fileWriter != nil {
		writer = io.MultiWriter(writer, fileWriter)
	}
	component := strings.TrimSpace(cfg.Component)

	contextBuilder := zerolog.New(writer).With().Timestamp()
	if component != "" {
		contextBuilder = contextBuilder.Str("component", component)
	}

	baseLogger = contextBuilder.Logger()
	baseWriter = writer
	baseComponent = component
	log.Logger = baseLogger

	return baseLogger
}

// IsLevelEnabled reports whether the provided level is enabled for logging.
func IsLevelEnabled(level zerolog.Level) bool {
	return level >= zerolog.GlobalLevel()
}

// WithRequestID stores (or generates) a request ID on the context.
func WithRequestID(ctx context.Context, requestID string) (context.Context, string) {
	if ctx == nil {
		ctx = context.Background()
	}
	requestID = strings.TrimSpace(requestID)
	if requestID == "" {
		requestID = uuid.NewString()
	}
	return context.WithValue(ctx, requestIDKey, requestID), requestID
}

func parseLevel(level string) zerolog.Level {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "debug":
		return zerolog.DebugLevel
	case "warn":
		return zerolog.WarnLevel
	case "error":
		return zerolog.ErrorLevel
	default:
		return zerolog.InfoLevel
	}
}

func selectWriter(format string) io.Writer {
	format = strings.ToLower(strings.TrimSpace(format))
	switch format {
	case "console":
		return newConsoleWriter(os.Stderr)
	case "json":
		return os.Stderr
	case "auto", "":
		if isTerminal(os.Stderr) {
			return newConsoleWriter(os.Stderr)
		}
		return os.Stderr
	default:
		return os.Stderr
	}
}

func newConsoleWriter(out io.Writer) io.Writer {
	return zerolog.ConsoleWriter{
		Out:        out,
		TimeFormat: defaultTimeFmt,
	}
}

func isTerminal(file *os.File) bool {
	if file == nil {
		return false
	}
	return isTerminalFn(int(file.Fd()))
}

type rollingFileWriter struct {
	mu          sync.Mutex
	path        string
	file        *os.File
	currentSize int64
	maxBytes    int64
	maxAge      time.Duration
	compress    bool
}

func newRollingFileWriter(cfg Config) (io.Writer, error) {
	path := strings.TrimSpace(cfg.FilePath)
	if path == "" {
		return nil, nil
	}

	dir := filepath.Dir(path)
	if err := mkdirAllFn(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create log directory: %w", err)
	}

	writer := &rollingFileWriter{
		path:     path,
		maxBytes: int64(cfg.MaxSizeMB) * 1024 * 1024,
		maxAge:   time.Duration(cfg.MaxAgeDays) * 24 * time.Hour,
		compress: cfg.Compress,
	}

	if writer.maxBytes <= 0 {
		writer.maxBytes = 100 * 1024 * 1024 // default 100MB
	}

	if err := writer.openOrCreateLocked(); err != nil {
		return nil, err
	}
	writer.cleanupOldFiles()
	return writer, nil
}

func (w *rollingFileWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if err := w.openOrCreateLocked(); err != nil {
		return 0, err
	}

	if w.maxBytes > 0 && w.currentSize+int64(len(p)) > w.maxBytes {
		if err := w.rotateLocked(); err != nil {
			return 0, err
		}
	}

	n, err := w.file.Write(p)
	if n > 0 {
		w.currentSize += int64(n)
	}
	return n, err
}

func (w *rollingFileWriter) openOrCreateLocked() error {
	if w.file != nil {
		return nil
	}

	file, err := openFileFn(w.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("open log file: %w", err)
	}
	w.file = file

	info, err := statFileFn(file)
	if err != nil {
		w.currentSize = 0
		return nil
	}
	w.currentSize = info.Size()
	return nil
}

func (w *rollingFileWriter) rotateLocked() error {
	if err := w.closeLocked(); err != nil {
		return err
	}

	if _, err := statFn(w.path); err == nil {
		rotated := fmt.Sprintf("%s.%s", w.path, nowFn().Format("20060102-150405"))
		if err := renameFn(w.path, rotated); err != nil {
			fmt.Fprintf(os.Stderr, "log rotation: rename %s -> %s failed: %v\n", w.path, rotated, err)
		} else if w.compress {
			go compressFn(rotated)
		}
	}

	w.cleanupOldFiles()
	return w.openOrCreateLocked()
}

func (w *rollingFileWriter) closeLocked() error {
	if w.file == nil {
		return nil
	}
	err := closeFileFn(w.file)
	w.file = nil
	w.currentSize = 0
	return err
}

func (w *rollingFileWriter) cleanupOldFiles() {
	if w.maxAge <= 0 {
		return
	}

	dir := filepath.Dir(w.path)
	base := filepath.Base(w.path)
	prefix := base + "."
	cutoff := nowFn().Add(-w.maxAge)

	entries, err := readDirFn(dir)
	if err != nil {
		return
	}

	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasPrefix(name, prefix) {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		if info.ModTime().Before(cutoff) {
			_ = removeFn(filepath.Join(dir, name))
		}
	}
}

func compressAndRemove(path string) {
	in, err := openFn(path)
	if err != nil {
		return
	}
	defer in.Close()

	outPath := path + ".gz"
	out, err := openFileFn(outPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return
	}

	gw := gzipNewWriterFn(out)
	if _, err = copyFn(gw, in); err != nil {
		gw.Close()
		out.Close()
		return
	}
	if err := gw.Close(); err != nil {
		out.Close()
		return
	}
	out.Close()
	_ = removeFn(path)
}
