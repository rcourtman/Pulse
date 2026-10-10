package monitoring

import (
	"bytes"
	"context"
	"testing"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func TestGuestFilesystemLoggerKeepsOrdinaryFallback(t *testing.T) {
	if guestFilesystemLogger(context.Background()) != &log.Logger {
		t.Fatal("ordinary filesystem guidance lost its process logger")
	}
	var output bytes.Buffer
	logger := zerolog.New(&output)
	ctx := logger.WithContext(context.Background())
	if guestFilesystemLogger(ctx) != log.Ctx(ctx) || guestFilesystemLogger(ctx) == &log.Logger {
		t.Fatal("filesystem guidance lost its explicit caller logger")
	}
}
