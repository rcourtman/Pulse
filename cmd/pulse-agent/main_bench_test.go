package main

import (
	"testing"

	"github.com/rs/zerolog"
)

func BenchmarkLoadConfig(b *testing.B) {
	args := []string{"-token", "test-token", "-url", "http://localhost:7655"}
	env := func(key string) string {
		switch key {
		case "PULSE_URL":
			return "http://localhost:7655"
		case "PULSE_TOKEN":
			return "test-token"
		default:
			return ""
		}
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = loadConfig(args, env)
	}
}

func BenchmarkParseLogLevel(b *testing.B) {
	levels := []string{"debug", "info", "warn", "error", "INFO", "  debug  ", ""}

	for _, level := range levels {
		b.Run(level, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_, _ = parseLogLevel(level)
			}
		})
	}
}

func BenchmarkRetryLogEvent(b *testing.B) {
	logger := zerolog.New(zerolog.NewConsoleWriter()).Level(zerolog.DebugLevel)

	attempts := []int{1, 10, 11, 50, 51, 100}
	for _, attempt := range attempts {
		b.Run("", func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				event := retryLogEvent(&logger, attempt)
				event.Discard()
			}
		})
	}
}
