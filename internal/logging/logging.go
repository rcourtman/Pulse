package logging

import (
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
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

	bytesPerMB        int64 = 1024 * 1024
	defaultMaxSizeMB        = 100
	defaultMaxAgeDays       = 30
	maxDurationDays         = int((1<<63 - 1) / int64(24*time.Hour))
	maxSafeSizeMB     int64 = (1<<63 - 1) / bytesPerMB
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
	fileCloser    io.Closer

	defaultTimeFmt = time.RFC3339
)

var (
	nowFn           = time.Now
	isTerminalFn    = term.IsTerminal
	mkdirAllFn      = os.MkdirAll
	chmodFn         = os.Chmod
	openFileFn      = os.OpenFile
	openFn          = os.Open
	statFn          = os.Stat
	lstatFn         = os.Lstat
	readDirFn       = os.ReadDir
	renameFn        = os.Rename
	removeFn        = os.Remove
	copyFn          = io.Copy
	gzipNewWriterFn = gzip.NewWriter
	statFileFn      = func(file *os.File) (os.FileInfo, error) { return file.Stat() }
	chmodFileFn     = func(file *os.File, mode os.FileMode) error { return file.Chmod(mode) }
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

	previousFileCloser := fileCloser
	fileCloser = nil

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
		if closer, ok := fileWriter.(io.Closer); ok {
			fileCloser = closer
		}
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

	if previousFileCloser != nil {
		if err := previousFileCloser.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "logging: unable to close previous log file writer: %v\n", err)
		}
	}

	return baseLogger
}

// Shutdown closes logging resources that outlive a single request lifecycle.
func Shutdown() {
	mu.Lock()
	defer mu.Unlock()

	if fileCloser != nil {
		if err := fileCloser.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "logging: unable to close log file writer: %v\n", err)
		}
		fileCloser = nil
	}

	GetBroadcaster().Shutdown()
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
	normalized := strings.ToLower(strings.TrimSpace(level))
	switch normalized {
	case "", "info":
		return zerolog.InfoLevel
	case "debug":
		return zerolog.DebugLevel
	case "trace":
		return zerolog.TraceLevel
	case "warn":
		return zerolog.WarnLevel
	case "warning":
		return zerolog.WarnLevel
	case "error":
		return zerolog.ErrorLevel
	case "fatal":
		return zerolog.FatalLevel
	case "panic":
		return zerolog.PanicLevel
	case "disabled":
		return zerolog.Disabled
	default:
		fmt.Fprintf(os.Stderr, "logging: invalid level %q; using %q\n", normalized, "info")
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
		fmt.Fprintf(os.Stderr, "logging: invalid format %q; using %q\n", format, "json")
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
	path = filepath.Clean(path)

	dir := filepath.Dir(path)
	if err := mkdirAllFn(dir, 0o700); err != nil {
		return nil, fmt.Errorf("create log directory: %w", err)
	}
	if err := validateExistingRegularFile(path); err != nil {
		return nil, fmt.Errorf("validate log file path: %w", err)
	}

	writer := &rollingFileWriter{
		path:     path,
		maxBytes: normalizeMaxBytes(cfg.MaxSizeMB),
		maxAge:   normalizeMaxAge(cfg.MaxAgeDays),
		compress: cfg.Compress,
	}

	if err := writer.openOrCreateLocked(); err != nil {
		return nil, fmt.Errorf("initialize rolling log file %s: %w", path, err)
	}
	writer.cleanupOldFiles()
	return writer, nil
}

func normalizeMaxBytes(sizeMB int) int64 {
	if sizeMB <= 0 {
		fmt.Fprintf(os.Stderr, "logging: invalid max size %dMB; using default %dMB\n", sizeMB, defaultMaxSizeMB)
		return int64(defaultMaxSizeMB) * bytesPerMB
	}
	if int64(sizeMB) > maxSafeSizeMB {
		fmt.Fprintf(os.Stderr, "logging: max size %dMB exceeds supported limit; using default %dMB\n", sizeMB, defaultMaxSizeMB)
		return int64(defaultMaxSizeMB) * bytesPerMB
	}
	return int64(sizeMB) * bytesPerMB
}

func normalizeMaxAge(days int) time.Duration {
	switch {
	case days < 0:
		fmt.Fprintf(os.Stderr, "logging: invalid max age %dd; using default %dd\n", days, defaultMaxAgeDays)
		days = defaultMaxAgeDays
	case days == 0:
		return 0
	}
	if days > maxDurationDays {
		fmt.Fprintf(os.Stderr, "logging: max age %dd exceeds supported limit; clamping to %dd\n", days, maxDurationDays)
		days = maxDurationDays
	}
	return time.Duration(days) * 24 * time.Hour
}

func (w *rollingFileWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if err := w.openOrCreateLocked(); err != nil {
		return 0, fmt.Errorf("open log file %s for write: %w", w.path, err)
	}

	if w.maxBytes > 0 && w.currentSize+int64(len(p)) > w.maxBytes {
		if err := w.rotateLocked(); err != nil {
			return 0, fmt.Errorf("rotate log file %s: %w", w.path, err)
		}
	}

	n, err := w.file.Write(p)
	if n > 0 {
		w.currentSize += int64(n)
	}
	if err != nil {
		return n, fmt.Errorf("write log file %s: %w", w.path, err)
	}
	return n, nil
}

func (w *rollingFileWriter) openOrCreateLocked() error {
	if w.file != nil {
		return nil
	}
	if err := validateExistingRegularFile(w.path); err != nil {
		return fmt.Errorf("validate log file path: %w", err)
	}

	file, err := openFileFn(w.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, logFilePerm)
	if err != nil {
		return fmt.Errorf("open log file: %w", err)
	}
	if err := chmodFileFn(file, logFilePerm); err != nil {
		_ = closeFileFn(file)
		return fmt.Errorf("secure log file permissions: %w", err)
	}
	w.file = file

	info, err := statFileFn(file)
	if err != nil {
		w.currentSize = 0
		fmt.Fprintf(os.Stderr, "logging: stat %s failed; continuing with size=0: %v\n", w.path, err)
		return nil
	}
	w.currentSize = info.Size()
	return nil
}

func (w *rollingFileWriter) rotateLocked() error {
	if err := w.closeLocked(); err != nil {
		return fmt.Errorf("close log file %s before rotation: %w", w.path, err)
	}

	if _, err := statFn(w.path); err == nil {
		rotated := fmt.Sprintf("%s.%s", w.path, nowFn().Format("20060102-150405"))
		if err := renameFn(w.path, rotated); err != nil {
			fmt.Fprintf(os.Stderr, "log rotation: rename %s -> %s failed: %v\n", w.path, rotated, err)
		} else if w.compress {
			go compressFn(rotated)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		fmt.Fprintf(os.Stderr, "log rotation: stat %s failed: %v\n", w.path, err)
	}

	w.cleanupOldFiles()
	if err := w.openOrCreateLocked(); err != nil {
		return fmt.Errorf("reopen log file %s after rotation: %w", w.path, err)
	}
	return nil
}

func (w *rollingFileWriter) closeLocked() error {
	if w.file == nil {
		return nil
	}
	err := closeFileFn(w.file)
	w.file = nil
	w.currentSize = 0
	if err != nil {
		return fmt.Errorf("close log file %s: %w", w.path, err)
	}
	return nil
}

func (w *rollingFileWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	return w.closeLocked()
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
		fmt.Fprintf(os.Stderr, "logging: read rotated log directory %s failed: %v\n", dir, err)
		return
	}

	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasPrefix(name, prefix) {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			fmt.Fprintf(os.Stderr, "logging: read metadata for rotated log %s failed: %v\n", filepath.Join(dir, name), err)
			continue
		}
		if info.ModTime().Before(cutoff) {
			fullPath := filepath.Join(dir, name)
			if err := removeFn(fullPath); err != nil {
				fmt.Fprintf(os.Stderr, "logging: remove old rotated log %s failed: %v\n", fullPath, err)
			}
		}
	}
}

func compressAndRemove(path string) {
	if err := validateExistingRegularFile(path); err != nil {
		return
	}
	in, err := openFn(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "logging: open rotated log %s for compression failed: %v\n", path, err)
		return
	}
	defer func() {
		if err := in.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "logging: close rotated log reader %s failed: %v\n", path, err)
		}
	}()

	outPath := path + ".gz"
	if err := validateExistingRegularFile(outPath); err != nil {
		if !isMissingPathError(err) {
			return
		}
	}
	out, err := openFileFn(outPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		fmt.Fprintf(os.Stderr, "logging: open gzip output %s failed: %v\n", outPath, err)
		return
	}

	gw := gzipNewWriterFn(out)
	if _, err = copyFn(gw, in); err != nil {
		if closeErr := gw.Close(); closeErr != nil {
			fmt.Fprintf(os.Stderr, "logging: close gzip writer %s after copy failure failed: %v\n", outPath, closeErr)
		}
		if closeErr := out.Close(); closeErr != nil {
			fmt.Fprintf(os.Stderr, "logging: close gzip output %s after copy failure failed: %v\n", outPath, closeErr)
		}
		fmt.Fprintf(os.Stderr, "logging: compress rotated log %s failed: %v\n", path, err)
		return
	}
	if err := gw.Close(); err != nil {
		if closeErr := out.Close(); closeErr != nil {
			fmt.Fprintf(os.Stderr, "logging: close gzip output %s after gzip close failure failed: %v\n", outPath, closeErr)
		}
		fmt.Fprintf(os.Stderr, "logging: finalize gzip stream %s failed: %v\n", outPath, err)
		return
	}
	if err := out.Close(); err != nil {
		fmt.Fprintf(os.Stderr, "logging: close gzip output %s failed: %v\n", outPath, err)
		return
	}
	if err := removeFn(path); err != nil {
		fmt.Fprintf(os.Stderr, "logging: remove uncompressed rotated log %s failed: %v\n", path, err)
	}
}

func ensureOwnerOnlyDir(dir string) error {
	if err := mkdirAllFn(dir, logDirPerm); err != nil {
		return err
	}
	info, err := lstatFn(dir)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("refusing symlink directory path %q", dir)
	}
	return chmodFn(dir, logDirPerm)
}

func validateExistingRegularFile(path string) error {
	info, err := lstatFn(path)
	if err != nil {
		if isMissingPathError(err) {
			return nil
		}
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("refusing symlink file path %q", path)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("non-regular file path %q", path)
	}
	return nil
}

func isMissingPathError(err error) bool {
	return errors.Is(err, os.ErrNotExist) || errors.Is(err, syscall.ENOTDIR)
}
