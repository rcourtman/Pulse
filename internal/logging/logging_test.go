package logging

import (
	"bytes"
	"os"
	"reflect"
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
