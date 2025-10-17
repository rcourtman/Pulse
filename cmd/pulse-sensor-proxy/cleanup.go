package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/rs/zerolog"
)

const cleanupRequestFilename = "cleanup-request.json"

func (p *Proxy) cleanupRequestPath() (string, error) {
	workDir := p.workDir
	if workDir == "" {
		workDir = defaultWorkDir()
	}

	if workDir == "" {
		return "", errors.New("cleanup working directory not configured")
	}

	return filepath.Join(workDir, cleanupRequestFilename), nil
}

func (p *Proxy) handleRequestCleanup(ctx context.Context, req *RPCRequest, logger zerolog.Logger) (interface{}, error) {
	cleanupPath, err := p.cleanupRequestPath()
	if err != nil {
		return nil, err
	}

	dir := filepath.Dir(cleanupPath)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return nil, fmt.Errorf("ensure cleanup directory: %w", err)
	}

	payload := map[string]any{
		"requestedAt": time.Now().UTC().Format(time.RFC3339),
	}
	if req != nil && req.Params != nil {
		if host, ok := req.Params["host"].(string); ok && host != "" {
			payload["host"] = host
		}
		if reason, ok := req.Params["reason"].(string); ok && reason != "" {
			payload["reason"] = reason
		}
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("encode cleanup payload: %w", err)
	}

	tmpFile, err := os.CreateTemp(dir, "cleanup-request-*.tmp")
	if err != nil {
		return nil, fmt.Errorf("prepare cleanup signal: %w", err)
	}
	tmpName := tmpFile.Name()
	if _, err := tmpFile.Write(append(data, '\n')); err != nil {
		tmpFile.Close()
		os.Remove(tmpName)
		return nil, fmt.Errorf("write cleanup payload: %w", err)
	}
	if err := tmpFile.Chmod(0o600); err != nil {
		logger.Warn().Err(err).Str("path", tmpName).Msg("Failed to set cleanup payload permissions")
	}
	if err := tmpFile.Close(); err != nil {
		os.Remove(tmpName)
		return nil, fmt.Errorf("close cleanup payload: %w", err)
	}

	// Replace any existing request atomically so systemd path units trigger on change.
	if err := os.Rename(tmpName, cleanupPath); err != nil {
		os.Remove(tmpName)
		return nil, fmt.Errorf("activate cleanup payload: %w", err)
	}

	logger.Info().
		Str("path", cleanupPath).
		Interface("payload", payload).
		Msg("Cleanup request signalled")

	return map[string]any{"queued": true}, nil
}
