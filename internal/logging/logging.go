package logging

import (
	"context"
	"fmt"
	"io"
	"os"
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
