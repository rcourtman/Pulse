package logging

import (
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
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
	loggerKey    ctxKey = "logging_logger"
)

// Config controls logger initialization.
type Config struct {
	Format    string // "json", "console", or "auto"
	Level     string // "debug", "info", "warn", "error"
	Component string // optional component name
	FilePath  string // optional log file path
	MaxSizeMB int    // rotate after this size (MB)
	MaxAgeDays int   // keep rotated logs for this many days
	Compress  bool   // gzip rotated logs
}

// Option customizes logger construction.
type Option func(*options)

type options struct {
	writer     io.Writer
	fields     map[string]interface{}
	withCaller bool
}

var (
	mu            sync.RWMutex
	baseLogger    zerolog.Logger
	baseWriter    io.Writer = os.Stderr
	baseComponent string

	defaultTimeFmt = time.RFC3339
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

// InitFromConfig initialises logging with environment overrides.
func InitFromConfig(ctx context.Context, cfg Config) (zerolog.Logger, error) {
	if cfg.Level == "" {
		cfg.Level = "info"
	}
	if cfg.Format == "" {
		cfg.Format = "auto"
	}

	if envLevel := os.Getenv("LOG_LEVEL"); envLevel != "" {
		cfg.Level = envLevel
	}
	if envFormat := os.Getenv("LOG_FORMAT"); envFormat != "" {
		cfg.Format = envFormat
	}
	if envFile := os.Getenv("LOG_FILE"); envFile != "" {
		cfg.FilePath = envFile
	}
	if envSize := os.Getenv("LOG_MAX_SIZE"); envSize != "" {
		if size, err := strconv.Atoi(envSize); err == nil {
			cfg.MaxSizeMB = size
		}
	}
	if envAge := os.Getenv("LOG_MAX_AGE"); envAge != "" {
		if age, err := strconv.Atoi(envAge); err == nil {
			cfg.MaxAgeDays = age
		}
	}
	if envCompress := os.Getenv("LOG_COMPRESS"); envCompress != "" {
		switch strings.ToLower(strings.TrimSpace(envCompress)) {
		case "0", "false", "no":
			cfg.Compress = false
		default:
			cfg.Compress = true
		}
	}

	if !isValidLevel(cfg.Level) {
		return zerolog.Logger{}, fmt.Errorf("invalid log level %q: must be debug, info, warn, or error", cfg.Level)
	}

	format := strings.ToLower(strings.TrimSpace(cfg.Format))
	if format != "" && format != "json" && format != "console" && format != "auto" {
		return zerolog.Logger{}, fmt.Errorf("invalid log format %q: must be json, console, or auto", cfg.Format)
	}

	logger := Init(cfg)
	return logger, nil
}

// IsLevelEnabled reports whether the provided level is enabled for logging.
func IsLevelEnabled(level zerolog.Level) bool {
	return level >= zerolog.GlobalLevel()
}

// New creates a logger tailored to a specific component.
func New(component string, opts ...Option) zerolog.Logger {
	cfg := collectOptions(opts...)

	mu.RLock()
	globalWriter := baseWriter
	globalComponent := baseComponent
	mu.RUnlock()

	writer := cfg.writer
	if writer == nil {
		writer = globalWriter
	}

	component = strings.TrimSpace(component)
	if component == "" {
		component = globalComponent
	}

	logger := zerolog.New(writer)
	contextBuilder := logger.With().Timestamp()

	if component != "" {
		contextBuilder = contextBuilder.Str("component", component)
	}
	if len(cfg.fields) > 0 {
		contextBuilder = contextBuilder.Fields(cfg.fields)
	}
	if cfg.withCaller {
		contextBuilder = contextBuilder.Caller()
	}

	return contextBuilder.Logger()
}

// WithCaller enables caller logging.
func WithCaller() Option {
	return func(o *options) {
		o.withCaller = true
	}
}

// WithWriter overrides the logger writer.
func WithWriter(w io.Writer) Option {
	return func(o *options) {
		if w != nil {
			o.writer = w
		}
	}
}

// WithFields adds static fields to the logger.
func WithFields(fields map[string]interface{}) Option {
	return func(o *options) {
		if len(fields) == 0 {
			return
		}
		if o.fields == nil {
			o.fields = make(map[string]interface{}, len(fields))
		}
		for k, v := range fields {
			o.fields[k] = v
		}
	}
}

// FromContext returns a logger enriched with context metadata.
func FromContext(ctx context.Context) zerolog.Logger {
	if ctx == nil {
		return getBaseLogger()
	}

	if existing, ok := ctx.Value(loggerKey).(zerolog.Logger); ok {
		return enrichWithRequestID(existing, ctx)
	}

	return enrichWithRequestID(getBaseLogger(), ctx)
}

// WithLogger stores a logger on the context.
func WithLogger(ctx context.Context, logger zerolog.Logger) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, loggerKey, logger)
}

// WithRequestID stores (or generates) a request ID on the context.
func WithRequestID(ctx context.Context, id string) (context.Context, string) {
	if ctx == nil {
		ctx = context.Background()
	}
	id = strings.TrimSpace(id)
	if id == "" {
		id = uuid.NewString()
	}
	return context.WithValue(ctx, requestIDKey, id), id
}

// GetRequestID retrieves the request ID from context.
func GetRequestID(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if id, ok := ctx.Value(requestIDKey).(string); ok {
		return id
	}
	return ""
}

// NewRequestID creates a new UUID string.
func NewRequestID() string {
	return uuid.NewString()
}

func collectOptions(opts ...Option) options {
	cfg := options{}
	for _, opt := range opts {
		if opt != nil {
			opt(&cfg)
		}
	}
	return cfg
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

func isValidLevel(level string) bool {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "debug", "info", "warn", "error":
		return true
	default:
		return false
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
	return term.IsTerminal(int(file.Fd()))
}

func getBaseLogger() zerolog.Logger {
	mu.RLock()
	defer mu.RUnlock()
	return baseLogger
}

func enrichWithRequestID(logger zerolog.Logger, ctx context.Context) zerolog.Logger {
	if ctx == nil {
		return logger
	}
	if id := GetRequestID(ctx); id != "" {
		return logger.With().Str("request_id", id).Logger()
	}
	return logger
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
	if err := os.MkdirAll(dir, 0o755); err != nil {
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

	file, err := os.OpenFile(w.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("open log file: %w", err)
	}
	w.file = file

	info, err := file.Stat()
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

	if _, err := os.Stat(w.path); err == nil {
		rotated := fmt.Sprintf("%s.%s", w.path, time.Now().Format("20060102-150405"))
		if err := os.Rename(w.path, rotated); err == nil {
			if w.compress {
				go compressAndRemove(rotated)
			}
		}
	}

	w.cleanupOldFiles()
	return w.openOrCreateLocked()
}

func (w *rollingFileWriter) closeLocked() error {
	if w.file == nil {
		return nil
	}
	err := w.file.Close()
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
	cutoff := time.Now().Add(-w.maxAge)

	entries, err := os.ReadDir(dir)
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
			_ = os.Remove(filepath.Join(dir, name))
		}
	}
}

func compressAndRemove(path string) {
	in, err := os.Open(path)
	if err != nil {
		return
	}
	defer in.Close()

	outPath := path + ".gz"
	out, err := os.OpenFile(outPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return
	}

	gw := gzip.NewWriter(out)
	if _, err = io.Copy(gw, in); err != nil {
		gw.Close()
		out.Close()
		return
	}
	if err := gw.Close(); err != nil {
		out.Close()
		return
	}
	out.Close()
	_ = os.Remove(path)
}
