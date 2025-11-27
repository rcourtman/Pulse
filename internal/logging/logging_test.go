package logging

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
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
}

func readJSONLine(t *testing.T, buf *bytes.Buffer) map[string]interface{} {
	t.Helper()

	line := strings.TrimSpace(buf.String())
	if idx := strings.IndexByte(line, '\n'); idx >= 0 {
		line = line[:idx]
	}
	if line == "" {
		t.Fatalf("expected log output, got empty string")
	}

	var event map[string]interface{}
	if err := json.Unmarshal([]byte(line), &event); err != nil {
		t.Fatalf("failed to unmarshal log line: %v", err)
	}
	return event
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

	if baseWriter != os.Stderr {
		t.Fatalf("expected base writer to be os.Stderr, got %#v", baseWriter)
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

	if _, ok := baseWriter.(zerolog.ConsoleWriter); !ok {
		t.Fatalf("expected console writer, got %#v", baseWriter)
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

	if baseWriter != w {
		t.Fatalf("expected base writer to use provided pipe, got %#v", baseWriter)
	}
}

func TestNewLoggerWithComponentAndFields(t *testing.T) {
	t.Cleanup(resetLoggingState)

	Init(Config{
		Format:    "json",
		Level:     "info",
		Component: "root",
	})

	var buf bytes.Buffer
	logger := New("worker", WithWriter(&buf), WithFields(map[string]interface{}{
		"request": "sync",
	}))

	logger.Info().Msg("processing")

	event := readJSONLine(t, &buf)

	if event["component"] != "worker" {
		t.Fatalf("expected component worker, got %v", event["component"])
	}
	if event["request"] != "sync" {
		t.Fatalf("expected request field, got %v", event["request"])
	}
	if event["level"] != "info" {
		t.Fatalf("expected level info, got %v", event["level"])
	}
	if event["message"] != "processing" {
		t.Fatalf("expected message processing, got %v", event["message"])
	}
}

func TestNewLoggerInheritsComponentWhenEmpty(t *testing.T) {
	t.Cleanup(resetLoggingState)

	Init(Config{
		Format:    "json",
		Level:     "info",
		Component: "core",
	})

	var buf bytes.Buffer
	logger := New("", WithWriter(&buf))
	logger.Warn().Msg("warn")

	event := readJSONLine(t, &buf)
	if event["component"] != "core" {
		t.Fatalf("expected inherited component core, got %v", event["component"])
	}
}

func TestNewLoggerWithCaller(t *testing.T) {
	t.Cleanup(resetLoggingState)

	Init(Config{
		Format: "json",
		Level:  "info",
	})

	var buf bytes.Buffer
	logger := New("svc", WithWriter(&buf), WithCaller())
	logger.Error().Msg("boom")

	event := readJSONLine(t, &buf)
	caller, ok := event["caller"].(string)
	if !ok || !strings.Contains(caller, "logging_test.go") {
		t.Fatalf("expected caller information, got %v", event["caller"])
	}
}

func TestNewLoggerWithCustomWriter(t *testing.T) {
	t.Cleanup(resetLoggingState)

	Init(Config{
		Format: "json",
		Level:  "info",
	})

	var buf bytes.Buffer
	logger := New("custom", WithWriter(&buf))
	logger.Info().Msg("hello")

	if buf.Len() == 0 {
		t.Fatal("expected output on custom writer")
	}
}

func TestContextHelpersWithRequestID(t *testing.T) {
	t.Cleanup(resetLoggingState)

	Init(Config{
		Format: "json",
		Level:  "info",
	})

	ctx := context.Background()
	ctx, generated := WithRequestID(ctx, "")
	if generated == "" {
		t.Fatal("expected generated request id")
	}
	if got := GetRequestID(ctx); got != generated {
		t.Fatalf("expected stored request id %s, got %s", generated, got)
	}

	var buf bytes.Buffer
	logger := New("api", WithWriter(&buf))
	ctx = WithLogger(ctx, logger)

	info := FromContext(ctx)
	info.Info().Msg("ctx-log")

	event := readJSONLine(t, &buf)
	if event["request_id"] != generated {
		t.Fatalf("expected request_id %s, got %v", generated, event["request_id"])
	}
}

func TestContextHelpersWithExistingLogger(t *testing.T) {
	t.Cleanup(resetLoggingState)

	Init(Config{
		Format: "json",
		Level:  "debug",
	})

	var buf bytes.Buffer
	base := New("svc", WithWriter(&buf))
	ctx := WithLogger(context.Background(), base)
	ctx, id := WithRequestID(ctx, "custom-123")

	logger := FromContext(ctx)
	logger.Debug().Msg("debug")

	event := readJSONLine(t, &buf)
	if event["component"] != "svc" {
		t.Fatalf("expected component svc, got %v", event["component"])
	}
	if event["request_id"] != "custom-123" {
		t.Fatalf("expected request_id custom-123, got %v", event["request_id"])
	}
	if event["level"] != "debug" {
		t.Fatalf("expected level debug, got %v", event["level"])
	}
	if id != "custom-123" {
		t.Fatalf("expected returned id to match input, got %s", id)
	}
}

func TestWithLoggerNilContext(t *testing.T) {
	t.Cleanup(resetLoggingState)

	Init(Config{})

	ctx := WithLogger(context.Background(), New("svc", WithWriter(io.Discard)))
	if ctx == nil {
		t.Fatal("expected non-nil context")
	}
}

func TestWithRequestIDTrimsWhitespace(t *testing.T) {
	t.Cleanup(resetLoggingState)

	Init(Config{})

	ctx, id := WithRequestID(context.Background(), "   ")
	if id == "" {
		t.Fatal("expected generated id for whitespace input")
	}
	if GetRequestID(ctx) != id {
		t.Fatalf("expected context request id %s, got %s", id, GetRequestID(ctx))
	}
}

func TestFromContextWithoutRequestID(t *testing.T) {
	t.Cleanup(resetLoggingState)

	Init(Config{
		Format: "json",
		Level:  "info",
	})

	var buf bytes.Buffer
	mu.Lock()
	baseLogger = zerolog.New(&buf).With().Timestamp().Logger()
	baseWriter = &buf
	baseComponent = ""
	log.Logger = baseLogger
	mu.Unlock()

	base := FromContext(context.Background())
	base.Info().Msg("no-request")

	event := readJSONLine(t, &buf)
	if _, ok := event["request_id"]; ok {
		t.Fatalf("did not expect request_id, got %v", event["request_id"])
	}
}

func TestNewLoggerWithoutComponentOmitsField(t *testing.T) {
	t.Cleanup(resetLoggingState)

	Init(Config{
		Format: "json",
		Level:  "info",
	})

	var buf bytes.Buffer
	logger := New("", WithWriter(&buf))
	logger.Info().Msg("no-component")

	event := readJSONLine(t, &buf)
	if _, exists := event["component"]; exists {
		t.Fatalf("did not expect component field, got %v", event["component"])
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

func TestInitFromConfigWithDefaults(t *testing.T) {
	t.Cleanup(resetLoggingState)

	logger, err := InitFromConfig(context.Background(), Config{
		Component: "test",
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if reflect.DeepEqual(logger, zerolog.Logger{}) {
		t.Fatal("expected initialized logger")
	}

	if zerolog.GlobalLevel() != zerolog.InfoLevel {
		t.Fatalf("expected default info level, got %s", zerolog.GlobalLevel())
	}
}

func TestInitFromConfigWithEnvOverrides(t *testing.T) {
	t.Cleanup(resetLoggingState)
	t.Cleanup(func() {
		os.Unsetenv("LOG_LEVEL")
		os.Unsetenv("LOG_FORMAT")
	})

	os.Setenv("LOG_LEVEL", "debug")
	os.Setenv("LOG_FORMAT", "json")

	logger, err := InitFromConfig(context.Background(), Config{
		Level:     "info",
		Format:    "console",
		Component: "test",
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if reflect.DeepEqual(logger, zerolog.Logger{}) {
		t.Fatal("expected initialized logger")
	}

	// Env override should set debug level
	if zerolog.GlobalLevel() != zerolog.DebugLevel {
		t.Fatalf("expected debug level from env override, got %s", zerolog.GlobalLevel())
	}

	// Format should be JSON (from env)
	mu.RLock()
	defer mu.RUnlock()
	if _, ok := baseWriter.(zerolog.ConsoleWriter); ok {
		t.Fatal("expected JSON writer from env override, got console writer")
	}
}

func TestInitFromConfigInvalidLevel(t *testing.T) {
	t.Cleanup(resetLoggingState)

	_, err := InitFromConfig(context.Background(), Config{
		Level:  "invalid",
		Format: "json",
	})
	if err == nil {
		t.Fatal("expected error for invalid level")
	}
	if !strings.Contains(err.Error(), "invalid log level") {
		t.Fatalf("expected invalid level error, got %v", err)
	}
}

func TestInitFromConfigInvalidFormat(t *testing.T) {
	t.Cleanup(resetLoggingState)

	_, err := InitFromConfig(context.Background(), Config{
		Level:  "info",
		Format: "invalid",
	})
	if err == nil {
		t.Fatal("expected error for invalid format")
	}
	if !strings.Contains(err.Error(), "invalid log format") {
		t.Fatalf("expected invalid format error, got %v", err)
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
